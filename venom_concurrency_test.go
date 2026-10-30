package venom

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestExecutorRegistryConcurrentAccess pins #2019's first fix: executorsPlugin is written while a
// run is in progress (GetExecutorRunner loads a plugin the first time a step names an unknown
// type), and #2018 will run test files concurrently. Run with `go test -race` — without it this
// only proves the goroutines don't deadlock.
//
// Verified during development that this fails with the mutex removed: the race detector reports a
// concurrent map write in RegisterExecutorPlugin against a read in lookupExecutorPlugin.
func TestExecutorRegistryConcurrentAccess(t *testing.T) {
	InitTestLogger(t)
	v := New()

	var wg sync.WaitGroup

	// Writers: each registers its own executor name, as a plugin load would.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v.RegisterExecutorPlugin(fmt.Sprintf("pluginExecutor%d", i), nil)
		}(i)
	}

	// Readers: the lookup path GetExecutorRunner takes for a type it does not know.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				v.lookupExecutorPlugin(fmt.Sprintf("pluginExecutor%d", j%20))
			}
		}()
	}

	wg.Wait()

	// Every registration landed: the lock protects the map without losing writes.
	for i := 0; i < 20; i++ {
		_, ok := v.lookupExecutorPlugin(fmt.Sprintf("pluginExecutor%d", i))
		require.True(t, ok, "registration %d was lost", i)
	}
}

// TestGetExecutorRunnerConcurrent exercises the real entry point rather than the helper, so the
// read path this fix guards is covered as callers actually reach it.
func TestGetExecutorRunnerConcurrent(t *testing.T) {
	InitTestLogger(t)
	v := New()
	v.RegisterExecutorBuiltin("exec", nil)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// An unknown type drives GetExecutorRunner down the plugin path, which is where the
			// registry is both written and read. Whether it resolves depends on whether the
			// writer goroutine below got there first, so the outcome is deliberately racy — what
			// matters here is that -race stays quiet, not what comes back.
			//nolint:errcheck // the result is deliberately nondeterministic; this test is a race check
			_, _, _ = v.GetExecutorRunner(context.Background(), TestStep{"type": fmt.Sprintf("unknown%d", i)}, H{})
		}(i)
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v.RegisterExecutorPlugin(fmt.Sprintf("unknown%d", i), nil)
		}(i)
	}
	wg.Wait()
}

// TestCountTestSuiteStatuses pins #2019's second fix. The run-wide tallies used to be incremented
// from inside runTestSuite, once per test file; they are now derived from the finished suites. The
// numbers must match what the increments produced, including for a suite whose status was never
// set — a run that stopped on an error — which counted in none of the three.
func TestCountTestSuiteStatuses(t *testing.T) {
	suite := func(status Status) TestSuite { return TestSuite{Status: status} }

	tests := []struct {
		name                         string
		suites                       []TestSuite
		wantFail, wantPass, wantSkip int
	}{
		{name: "no suites"},
		{
			name:     "all pass",
			suites:   []TestSuite{suite(StatusPass), suite(StatusPass)},
			wantPass: 2,
		},
		{
			name:     "one fail among passes",
			suites:   []TestSuite{suite(StatusPass), suite(StatusFail), suite(StatusPass)},
			wantFail: 1, wantPass: 2,
		},
		{
			name:     "all skipped",
			suites:   []TestSuite{suite(StatusSkip), suite(StatusSkip)},
			wantSkip: 2,
		},
		{
			name:     "mixed",
			suites:   []TestSuite{suite(StatusFail), suite(StatusPass), suite(StatusSkip), suite(StatusFail)},
			wantFail: 2, wantPass: 1, wantSkip: 1,
		},
		{
			name:     "a suite whose status was never set counts in none",
			suites:   []TestSuite{suite(StatusPass), {}, suite(StatusRun)},
			wantPass: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fail, pass, skip := countTestSuiteStatuses(tt.suites)
			require.Equal(t, tt.wantFail, fail, "fail count")
			require.Equal(t, tt.wantPass, pass, "pass count")
			require.Equal(t, tt.wantSkip, skip, "skip count")
		})
	}
}

// TestProcessTalliesSuitesFinishedBeforeAnAbort pins the path code review found missing: when a
// test suite errors, Process returns immediately. The old per-suite increments had already counted
// every suite that finished before it, so an aborted run still reported them. Deriving the tallies
// only at the end of Process would have reported zeros instead — invisible in the shipped CLI
// (which exits on the error) but not in the coverage binary, which does not exit because IsTest is
// set and still writes a report.
func TestProcessTalliesSuitesFinishedBeforeAnAbort(t *testing.T) {
	InitTestLogger(t)
	v := New()
	v.Tests.TestSuites = []TestSuite{
		{Name: "finishes first", WorkDir: t.TempDir()},
		// An unclosed action makes the suite-variable interpolation fail, so runTestSuite returns
		// an error before it can set a status.
		{Name: "aborts the run", WorkDir: t.TempDir(), Vars: H{"broken": "{{.unterminated"}},
	}

	err := v.Process(context.Background(), nil)
	require.Error(t, err, "the second suite must abort the run")

	require.Equal(t, 1, v.Tests.NbTestsuitesPass, "the suite that finished must still be counted")
	require.Equal(t, 0, v.Tests.NbTestsuitesFail)
	require.Equal(t, 0, v.Tests.NbTestsuitesSkip)
}
