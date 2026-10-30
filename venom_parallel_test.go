package venom

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// sleeperExecutor is a test executor that waits, which is the shape --parallel exists for: an
// integration test spends its time waiting on another system. The real executors live in package
// executors, which imports venom, so they cannot be used from a test inside this package.
type sleeperExecutor struct{}

func (sleeperExecutor) Run(_ context.Context, step TestStep) (interface{}, error) {
	// JSON numbers arrive as float64.
	if ms, ok := step["ms"].(float64); ok && ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	code := 0
	if fail, ok := step["fail"].(bool); ok && fail {
		code = 1
	}
	return map[string]interface{}{"code": code}, nil
}

func sleeperStep(t *testing.T, fields map[string]interface{}) json.RawMessage {
	t.Helper()
	fields["type"] = "sleeper"
	raw, err := json.Marshal(fields)
	require.NoError(t, err)
	return raw
}

// newParallelTestVenom builds a Venom with the sleeper executor registered and console output
// silenced, so the tests measure the pool rather than the terminal.
func newParallelTestVenom(t *testing.T, parallel int) *Venom {
	t.Helper()
	v := New()
	v.Parallel = parallel
	v.RegisterExecutorBuiltin("sleeper", sleeperExecutor{})
	v.PrintFunc = func(string, ...interface{}) (int, error) { return 0, nil }
	return v
}

// waitingSuites builds n test files whose single step waits for the given duration.
func waitingSuites(t *testing.T, n int, wait time.Duration) []TestSuite {
	t.Helper()
	suites := make([]TestSuite, n)
	for i := range suites {
		suites[i] = TestSuite{
			Name:    fmt.Sprintf("suite-%02d", i),
			WorkDir: t.TempDir(),
			TestCases: []TestCase{{
				TestCaseInput: TestCaseInput{
					Name:         "wait",
					RawTestSteps: []json.RawMessage{sleeperStep(t, map[string]interface{}{"ms": wait.Milliseconds()})},
				},
			}},
		}
	}
	return suites
}

// failingSuite builds a test file whose single step fails an assertion.
func failingSuite(t *testing.T, name string) TestSuite {
	t.Helper()
	return TestSuite{
		Name:    name,
		WorkDir: t.TempDir(),
		TestCases: []TestCase{{
			TestCaseInput: TestCaseInput{
				Name: "boom",
				RawTestSteps: []json.RawMessage{sleeperStep(t, map[string]interface{}{
					"fail":       true,
					"assertions": []string{"result.code ShouldEqual 0"},
				})},
			},
		}},
	}
}

// TestProcessDefaultTakesTheSequentialPath pins the promise that the default is not "a pool of
// one": Parallel unset (0, as an embedder would leave it) and Parallel 1 both run files in order.
func TestProcessDefaultTakesTheSequentialPath(t *testing.T) {
	InitTestLogger(t)

	for _, parallel := range []int{0, 1} {
		t.Run(fmt.Sprintf("parallel=%d", parallel), func(t *testing.T) {
			v := newParallelTestVenom(t, parallel)
			v.Tests.TestSuites = waitingSuites(t, 3, 0)

			require.NoError(t, v.Process(context.Background(), nil))

			require.Equal(t, 3, v.Tests.NbTestsuitesPass)
			require.Equal(t, []string{"suite-00", "suite-01", "suite-02"},
				[]string{v.Tests.TestSuites[0].Name, v.Tests.TestSuites[1].Name, v.Tests.TestSuites[2].Name})

			// Sequential means no overlap at all: each file starts after the previous one ended.
			for i := 1; i < len(v.Tests.TestSuites); i++ {
				require.False(t, v.Tests.TestSuites[i].Start.Before(v.Tests.TestSuites[i-1].End),
					"file %d started before file %d finished — the default must not run files concurrently", i, i-1)
			}
		})
	}
}

// TestProcessNeverExceedsTheWorkerCap is the AC-3 guard: whatever N is, no more than N files may
// be in flight. The peak overlap is reconstructed from each file's recorded interval.
func TestProcessNeverExceedsTheWorkerCap(t *testing.T) {
	InitTestLogger(t)

	for _, workers := range []int{2, 3, 5} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			v := newParallelTestVenom(t, workers)
			v.Tests.TestSuites = waitingSuites(t, 12, 40*time.Millisecond)

			require.NoError(t, v.Process(context.Background(), nil))

			type event struct {
				at    time.Time
				delta int64
			}
			events := make([]event, 0, len(v.Tests.TestSuites)*2)
			for i := range v.Tests.TestSuites {
				ts := &v.Tests.TestSuites[i]
				require.False(t, ts.Start.IsZero(), "suite %d never started", i)
				require.False(t, ts.End.IsZero(), "suite %d never finished", i)
				events = append(events, event{ts.Start, 1}, event{ts.End, -1})
			}
			// Sort by time; on a tie an End sorts before a Start, so intervals that merely touch
			// are not counted as overlapping.
			sort.Slice(events, func(i, j int) bool {
				if events[i].at.Equal(events[j].at) {
					return events[i].delta < events[j].delta
				}
				return events[i].at.Before(events[j].at)
			})

			var inFlight, peak int64
			for _, e := range events {
				inFlight += e.delta
				if inFlight > peak {
					peak = inFlight
				}
			}

			require.LessOrEqual(t, peak, int64(workers),
				"at most %d files may be in flight, saw %d", workers, peak)
			require.Greater(t, peak, int64(1),
				"with %d workers the files should actually overlap", workers)
		})
	}
}

// TestProcessParallelIsFasterOnWaitingFiles is the AC-2 guard at the engine level. It compares a
// run against itself rather than asserting an absolute duration, so it does not depend on how
// fast the machine is.
func TestProcessParallelIsFasterOnWaitingFiles(t *testing.T) {
	InitTestLogger(t)

	const files = 8
	const wait = 150 * time.Millisecond

	run := func(parallel int) time.Duration {
		v := newParallelTestVenom(t, parallel)
		v.Tests.TestSuites = waitingSuites(t, files, wait)
		start := time.Now()
		require.NoError(t, v.Process(context.Background(), nil))
		return time.Since(start)
	}

	sequential := run(1)
	concurrent := run(4)

	t.Logf("%d files waiting %v each: sequential %v, --parallel 4 %v (%.2fx)",
		files, wait, sequential, concurrent, float64(sequential)/float64(concurrent))

	require.Less(t, concurrent*2, sequential,
		"four workers should be at least twice as fast on %d waiting files (sequential %v, concurrent %v)",
		files, sequential, concurrent)
}

// TestStopOnFailureStopsTheWholeRun pins the behaviour change #2020 introduces, at both
// parallelism settings. Before this change --stop-on-failure only skipped the remaining test cases
// of the file that failed and the run carried on with the next file; it now stops the run.
func TestStopOnFailureStopsTheWholeRun(t *testing.T) {
	InitTestLogger(t)

	for _, parallel := range []int{1, 4} {
		t.Run(fmt.Sprintf("parallel=%d", parallel), func(t *testing.T) {
			v := newParallelTestVenom(t, parallel)
			v.StopOnFailure = true
			v.Tests.TestSuites = append(
				[]TestSuite{failingSuite(t, "fails")},
				waitingSuites(t, 6, 10*time.Millisecond)...)

			require.NoError(t, v.Process(context.Background(), nil))

			require.Equal(t, StatusFail, v.Tests.TestSuites[0].Status, "the first file must fail")
			require.Equal(t, StatusFail, v.Tests.Status)

			// Every file is accounted for: the ones that never started are skipped, not left blank.
			for i := range v.Tests.TestSuites {
				require.NotEmpty(t, v.Tests.TestSuites[i].Status,
					"suite %d (%s) has no status — a stopped run must still account for it",
					i, v.Tests.TestSuites[i].Name)
			}
			require.Equal(t, len(v.Tests.TestSuites),
				v.Tests.NbTestsuitesFail+v.Tests.NbTestsuitesPass+v.Tests.NbTestsuitesSkip,
				"the tallies must add up to the number of files")
			require.Greater(t, v.Tests.NbTestsuitesSkip, 0,
				"stop-on-failure must stop later files from running")
		})
	}
}

// TestStopOnFailureLetsInFlightFilesFinish checks the other half of the promise: files already
// running are not cancelled, so the report carries a real result for every file that started.
func TestStopOnFailureLetsInFlightFilesFinish(t *testing.T) {
	InitTestLogger(t)

	v := newParallelTestVenom(t, 4)
	v.StopOnFailure = true
	// One fast failure alongside slow files: the slow ones are in flight when the stop is raised.
	v.Tests.TestSuites = append(
		[]TestSuite{failingSuite(t, "fails-fast")},
		waitingSuites(t, 3, 200*time.Millisecond)...)

	require.NoError(t, v.Process(context.Background(), nil))

	var started, finished int
	for i := 1; i < len(v.Tests.TestSuites); i++ {
		ts := &v.Tests.TestSuites[i]
		if ts.Start.IsZero() {
			continue
		}
		started++
		if !ts.End.IsZero() && ts.Status == StatusPass {
			finished++
		}
	}
	require.Greater(t, started, 0, "the slow files should have been in flight")
	require.Equal(t, started, finished,
		"every file that started must finish and report — in-flight files are never cancelled")
}

// TestSkippedSuitesCountTheirTestCases pins a reporting hole found in review: a file the pool
// never started is reported as SKIP, and its per-suite counters have to agree. runTestSuite is
// what normally fills them and it never ran for that file, so `--format=json` would otherwise
// serialise a suite whose every test case is SKIP next to "nbTestcasesSkip": 0.
func TestSkippedSuitesCountTheirTestCases(t *testing.T) {
	InitTestLogger(t)

	v := newParallelTestVenom(t, 1)
	v.StopOnFailure = true
	v.Tests.TestSuites = append(
		[]TestSuite{failingSuite(t, "fails")},
		waitingSuites(t, 4, 0)...)

	require.NoError(t, v.Process(context.Background(), nil))

	var checked int
	for i := range v.Tests.TestSuites {
		ts := &v.Tests.TestSuites[i]
		if ts.Status != StatusSkip {
			continue
		}
		checked++
		require.Equal(t, len(ts.TestCases), ts.NbTestcasesSkip,
			"suite %q is SKIP with %d test cases but reports nbTestcasesSkip=%d",
			ts.Name, len(ts.TestCases), ts.NbTestcasesSkip)
	}
	require.Greater(t, checked, 0, "the run should have skipped at least one file")
}

// TestEngineErrorDoesNotClaimStopOnFailure pins the other review finding: the scheduler stops for
// two different reasons and they must not be conflated. An engine error aborts the run; the files
// that never started are then left untouched, exactly as the sequential path leaves them when
// Process returns the error. Marking them SKIP with "stop-on-failure: enabled" would assert a flag
// that is off, and would make the tallies disagree between --parallel 1 and --parallel 4.
func TestEngineErrorDoesNotClaimStopOnFailure(t *testing.T) {
	InitTestLogger(t)

	for _, parallel := range []int{1, 4} {
		t.Run(fmt.Sprintf("parallel=%d", parallel), func(t *testing.T) {
			v := newParallelTestVenom(t, parallel)
			v.StopOnFailure = false // the flag is OFF — nothing may claim otherwise

			// An unclosed action makes the suite-variable interpolation fail, so runTestSuite
			// returns an error rather than a failing test.
			broken := TestSuite{Name: "errors", WorkDir: t.TempDir(), Vars: H{"broken": "{{.unterminated"}}
			v.Tests.TestSuites = append([]TestSuite{broken}, waitingSuites(t, 12, 20*time.Millisecond)...)

			require.Error(t, v.Process(context.Background(), nil), "an engine error must abort the run")

			var untouched int
			for i := range v.Tests.TestSuites {
				ts := &v.Tests.TestSuites[i]
				if ts.Status == "" {
					untouched++
				}
				require.NotEqual(t, StatusSkip, ts.Status,
					"suite %q was marked SKIP after an engine error, with --stop-on-failure off", ts.Name)
				for j := range ts.TestCases {
					require.Empty(t, ts.TestCases[j].Skipped,
						"suite %q carries a stop-on-failure skip message but the flag is off", ts.Name)
				}
			}
			require.Greater(t, untouched, 0, "some files should never have started")
			require.Equal(t, 0, v.Tests.NbTestsuitesSkip,
				"files that never started must count in none of the tallies, as they do at --parallel 1")
		})
	}
}

// TestProcessReportOrderIsFileOrder pins AC-4 and AC-6: whatever the parallelism, the report lists
// the files in the order they were read, and repeated runs agree.
func TestProcessReportOrderIsFileOrder(t *testing.T) {
	InitTestLogger(t)

	var want []string
	for _, parallel := range []int{1, 4, 4, 4} {
		v := newParallelTestVenom(t, parallel)
		v.Tests.TestSuites = waitingSuites(t, 10, 5*time.Millisecond)

		require.NoError(t, v.Process(context.Background(), nil))

		got := make([]string, len(v.Tests.TestSuites))
		for i := range v.Tests.TestSuites {
			got[i] = v.Tests.TestSuites[i].Name
		}
		if want == nil {
			want = got
		}
		require.Equal(t, want, got, "report order must be file order at --parallel %d", parallel)
		require.Equal(t, 10, v.Tests.NbTestsuitesPass)
	}
}

// TestProcessConcurrentIsRaceFree is the -race guard for the pool itself.
func TestProcessConcurrentIsRaceFree(t *testing.T) {
	InitTestLogger(t)

	v := newParallelTestVenom(t, 8)
	v.Tests.TestSuites = waitingSuites(t, 24, 5*time.Millisecond)

	require.NoError(t, v.Process(context.Background(), nil))
	require.Equal(t, 24, v.Tests.NbTestsuitesPass)
	require.Equal(t, StatusPass, v.Tests.Status)
}
