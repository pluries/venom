package venom

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// This file is a standalone measurement for #1983 ("decide whether test cases should run in
// parallel"), not a test of production code: venom's actual runner (process_testsuite.go,
// process_testcase.go) stays untouched, because running it concurrently today would exercise the
// unguarded shared assertion list tracked separately as #1974 — a real, unfixed data race, not
// something this spike should paper over by accident. What follows models the *timing shape* of
// running files, cases and steps (see runSpikeSequential / runSpikeConcurrent) to answer the
// ticket's actual question: given venom's execution pattern, does file-level concurrency pay off,
// and does the answer depend on whether the work is I/O-bound or CPU-bound?

// simulatedWorkload stands in for one step's cost. ioWait models time spent waiting on another
// system (an HTTP call, a database query — most of what an integration test does, per the ticket);
// a sleeping goroutine yields the scheduler and costs nothing else. cpuIterations models a CPU-bound
// step (interpolation, #1981; variable-list flattening, #1982) as a FIXED amount of work rather than
// a wall-clock window: a first version used `for time.Now().Before(deadline) {}`, but that lets a
// goroutine that got descheduled mid-window "finish" the instant it's rescheduled and the deadline
// has already passed, having done less real work the longer it waited for a core — a genuine
// oversubscription would then look artificially fast instead of correctly slow (caught in code
// review: measured a 1.43x "speedup" from adding goroutines on a single core, which is physically
// impossible for real CPU-bound work). A fixed iteration count has no such escape: it costs the same
// CPU time regardless of how the scheduler interleaves it.
type simulatedWorkload struct {
	name          string
	ioWait        time.Duration
	cpuIterations uint64
}

// calibrateCPUIterations measures, once, how many loop iterations cpuBoundWork needs to occupy
// approximately d of CPU time on an idle core, so cpuBoundWorkload's cost is expressed as work,
// not time.
func calibrateCPUIterations(d time.Duration) uint64 {
	const probe = 200_000_000
	start := time.Now()
	cpuBoundWork(probe)
	elapsed := time.Since(start)
	return uint64(float64(probe) * float64(d) / float64(elapsed))
}

// cpuBoundWork burns CPU for a fixed number of iterations — real arithmetic the compiler cannot
// discard, since the result is used via runtime.KeepAlive.
func cpuBoundWork(iterations uint64) {
	var x uint64
	for i := uint64(0); i < iterations; i++ {
		x += i
	}
	runtime.KeepAlive(x)
}

var (
	ioBoundWorkload  = simulatedWorkload{name: "IOBound", ioWait: 15 * time.Millisecond}
	cpuBoundWorkload = simulatedWorkload{name: "CPUBound", cpuIterations: calibrateCPUIterations(15 * time.Millisecond)}
)

const (
	// spikeFiles is kept comfortably above every worker count this benchmark tries: a worker count
	// at or past the file count would let every file start at once and collapse the result to one
	// file's own duration, measuring "more workers than files" rather than actual scaling (caught
	// in code review against an earlier 24-files/32-workers combination).
	spikeFiles        = 48
	spikeCasesPerFile = 2 // cases within a file stay sequential either way (see runSpikeConcurrent)
)

// simulateCase costs the same wall time under ioBoundWorkload and cpuBoundWorkload when run alone;
// the difference only shows once multiple cases run at the same time.
func simulateCase(w simulatedWorkload) {
	if w.ioWait > 0 {
		time.Sleep(w.ioWait)
	}
	if w.cpuIterations > 0 {
		cpuBoundWork(w.cpuIterations)
	}
}

// runSpikeSequential models today's venom: every file, then every case, one after another.
func runSpikeSequential(w simulatedWorkload) {
	for f := 0; f < spikeFiles; f++ {
		for c := 0; c < spikeCasesPerFile; c++ {
			simulateCase(w)
		}
	}
}

// runSpikeConcurrent models the ticket's recommended unit — whole files run concurrently, bounded
// by workers; cases within a file stay sequential (the ticket: steps within a case cannot run in
// parallel because they pass values along, and cases within a file are the realistic unit, but
// file-level is "the safest and probably gives most of the benefit" — this models exactly that
// safest option).
func runSpikeConcurrent(w simulatedWorkload, workers int) {
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for f := 0; f < spikeFiles; f++ {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			for c := 0; c < spikeCasesPerFile; c++ {
				simulateCase(w)
			}
		}()
	}
	wg.Wait()
}

// BenchmarkFileLevelParallelism answers #1983: Sequential is the one baseline both workloads share
// (wall time per unit is the same whether it's spent waiting or computing, so one number covers
// both). The worker sweep (4/8/16/32) on an 8-logical-CPU machine shows whether each workload shape
// keeps scaling with more concurrency past the core count, or plateaus.
func BenchmarkFileLevelParallelism(b *testing.B) {
	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			runSpikeSequential(ioBoundWorkload)
		}
	})
	for _, workers := range []int{4, 8, 16, 32} {
		label := fmt.Sprintf("Workers%d", workers)
		b.Run("IOBound_"+label, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				runSpikeConcurrent(ioBoundWorkload, workers)
			}
		})
		b.Run("CPUBound_"+label, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				runSpikeConcurrent(cpuBoundWorkload, workers)
			}
		})
	}
}
