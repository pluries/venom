package venom

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/gosimple/slug"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// InitLogger initializes venom logger
func (v *Venom) InitLogger() error {
	v.Tests.TestSuites = []TestSuite{}

	switch v.Verbose {
	case 1:
		logrus.SetLevel(logrus.InfoLevel)
	case 2:
		logrus.SetLevel(logrus.DebugLevel)
	default:
		logrus.SetLevel(logrus.WarnLevel)
	}

	if v.OutputDir != "" {
		if err := os.MkdirAll(v.OutputDir, os.FileMode(0o755)); err != nil {
			return errors.Wrapf(err, "unable to create output dir")
		}
	}

	var err error
	logFile := filepath.Join(v.OutputDir, computeOutputFilename("venom.log"))
	v.LogOutput, err = openLogFile(logFile)
	if err != nil {
		return errors.Wrapf(err, "unable to write log file")
	}
	v.PrintlnTrace("writing " + logFile)
	logrus.SetOutput(v.LogOutput)

	logrus.SetFormatter(&nested.Formatter{
		HideKeys:       true,
		FieldsOrder:    []string{"testsuite", "testcase", "step", "executor"},
		NoColors:       true,
		NoFieldsColors: true,
	})
	logger = logrus.NewEntry(logrus.StandardLogger())

	slug.Lowercase = false

	return nil
}

// openLogFile creates the run's log owner-only (#2300): it can hold step output. The mode applies to a new file only.
func openLogFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.FileMode(0o600))
}

func computeOutputFilename(filename string) string {
	// example of filename: venom.log
	t := strings.Split(filename, ".")

	if !fileExists(filename) {
		return filename
	}
	for i := 0; ; i++ {
		filename := fmt.Sprintf("%s.%d.%s", t[0], i, t[1])
		if !fileExists(filename) {
			return filename
		}
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Parse parses tests suite to check context and variables
func (v *Venom) Parse(ctx context.Context, path []string) error {
	filesPath, err := getFilesPath(path)
	if err != nil {
		return err
	}

	if err := v.readFiles(ctx, filesPath); err != nil {
		return err
	}

	err = v.registerUserExecutors(ctx)
	if err != nil {
		return errors.Wrapf(err, "unable to register user executors")
	}

	missingVars := []string{}
	extractedVars := []string{}
	for i := range v.Tests.TestSuites {
		ts := &v.Tests.TestSuites[i]
		ts.Vars.Add("venom.testsuite", ts.Name)

		Info(ctx, "Parsing testsuite %s", ts.Filepath)
		tvars, textractedVars, err := v.parseTestSuite(ts)
		if err != nil {
			return err
		}

		for k := range ts.Vars {
			textractedVars = append(textractedVars, k)
		}

		// The variable names carry the test case names, which can hold a declared secret (#2132): log them under the
		// suite's secrets, which no context holds yet at parse time.
		Debug(v.processSecrets(ctx, ts, nil), "Testsuite (%s) variables: %s", ts.Filepath, strings.Join(textractedVars, ","))

		missingVars = appendAbsentVars(missingVars, tvars)
		extractedVars = appendAbsentVars(extractedVars, textractedVars)
	}

	vars, err := DumpStringPreserveCase(v.variables)
	if err != nil {
		return errors.Wrapf(err, "unable to parse variables")
	}

	reallyMissingVars := filterReallyMissingVars(missingVars, extractedVars, vars)
	if len(reallyMissingVars) > 0 {
		return fmt.Errorf("missing variables %v", reallyMissingVars)
	}

	return nil
}

// appendAbsentVars appends to vars, in order, each of candidates that vars does not hold yet.
func appendAbsentVars(vars, candidates []string) []string {
	for _, k := range candidates {
		var found bool
		for i := 0; i < len(vars); i++ {
			if vars[i] == k {
				found = true
				break
			}
		}
		if !found {
			vars = append(vars, k)
		}
	}
	return vars
}

// filterReallyMissingVars keeps the missing vars that are neither range builtins, extracted by a
// step, defined in the run's variables nor venom.* variables.
func filterReallyMissingVars(missingVars, extractedVars []string, vars map[string]string) []string {
	reallyMissingVars := []string{}
	for _, k := range missingVars {
		// Skip "range" builtin variables
		if strings.HasPrefix(k, "value") || k == "index" || k == "key" {
			continue
		}
		if isVarExtracted(k, extractedVars, vars) {
			continue
		}
		// ignore {{.venom.var..}}
		if strings.HasPrefix(k, "venom.") {
			continue
		}
		reallyMissingVars = append(reallyMissingVars, k)
	}
	return reallyMissingVars
}

// isVarExtracted reports whether k is already accounted for: equal to, or having as a prefix, one
// of extractedVars (a step's output), or an exact key of vars (the run's own variables).
func isVarExtracted(k string, extractedVars []string, vars map[string]string) bool {
	for _, e := range extractedVars {
		if k == e || strings.HasPrefix(k, e) {
			return true
		}
	}
	for t := range vars {
		if t == k {
			return true
		}
	}
	return false
}

// Process runs tests suite and return a Tests result
func (v *Venom) Process(ctx context.Context, _ []string) error {
	v.Tests.Status = StatusRun
	v.Tests.Start = time.Now()
	Debug(ctx, "nb testsuites: %d", len(v.Tests.TestSuites))

	// Anything below 1 means sequential. The CLI refuses those values, but Process is also the
	// entry point for an embedder, whose zero-valued Venom would otherwise run no test at all.
	workers := v.Parallel
	if workers < 1 {
		workers = 1
	}

	var err error
	if workers == 1 {
		// The default takes the path venom has always taken, unchanged, rather than a pool of one:
		// "behaviourally identical, not merely equivalent" (#2020).
		err = v.runTestSuitesSequentially(ctx)
	} else {
		Debug(ctx, "running test files through %d workers", workers)
		err = v.runTestSuitesConcurrently(ctx, workers)
	}
	if err != nil {
		// Tally what did finish before giving up. The counters used to be incremented inside
		// runTestSuite, so an aborted run still reported the suites that had completed; an
		// embedder — or the coverage binary, which does not exit on error because IsTest is
		// set — can still read them off the report (#2019).
		v.Tests.NbTestsuitesFail, v.Tests.NbTestsuitesPass, v.Tests.NbTestsuitesSkip = countTestSuiteStatuses(v.Tests.TestSuites)
		return err
	}

	v.Tests.End = time.Now()
	v.Tests.Duration = v.Tests.End.Sub(v.Tests.Start).Seconds()

	v.Tests.NbTestsuitesFail, v.Tests.NbTestsuitesPass, v.Tests.NbTestsuitesSkip = countTestSuiteStatuses(v.Tests.TestSuites)

	switch {
	case v.Tests.NbTestsuitesFail > 0:
		v.Tests.Status = StatusFail
	case v.Tests.NbTestsuitesSkip > 0 && v.Tests.NbTestsuitesSkip == len(v.Tests.TestSuites):
		v.Tests.Status = StatusSkip
	default:
		v.Tests.Status = StatusPass
	}

	Debug(ctx, "final status: %s", v.Tests.Status)

	return nil
}

// runTestSuitesSequentially runs the test files one after another — the path venom has always
// taken, and the one the default still takes.
func (v *Venom) runTestSuitesSequentially(ctx context.Context) error {
	for i := range v.Tests.TestSuites {
		v.Tests.TestSuites[i].Start = time.Now()
		// ##### RUN Test Suite Here
		if err := v.runTestSuite(ctx, &v.Tests.TestSuites[i]); err != nil {
			return err
		}

		v.Tests.TestSuites[i].End = time.Now()
		v.Tests.TestSuites[i].Duration = v.Tests.TestSuites[i].End.Sub(v.Tests.TestSuites[i].Start).Seconds()

		if v.stopRunAfter(&v.Tests.TestSuites[i]) {
			v.skipTestSuitesFrom(i + 1)
			break
		}
	}
	return nil
}

// runTestSuitesConcurrently runs the test files through a bounded pool of workers. Each worker
// owns one element of v.Tests.TestSuites and touches nothing else on the shared struct: the
// run-wide tallies were moved out of the per-file path in #2019, and the slice is never appended
// to or re-sliced while the run is in progress, so distinct indices are distinct memory.
//
// Console output is the one thing still shared here: two files printing at once interleave until
// #2021 gives each file its own buffer.
func (v *Venom) runTestSuitesConcurrently(ctx context.Context, workers int) error {
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		stop     bool
		// Why the scheduler stopped matters. A failing test file with --stop-on-failure means the
		// files that never ran are reported as skipped; an engine error means the run is aborted
		// and they are left untouched, exactly as the sequential path leaves them when Process
		// returns the error. Conflating the two would mark files "stop-on-failure" with the flag
		// off, and make the tallies disagree between --parallel 1 and --parallel 4.
		stoppedByFailure bool
	)

	// The semaphore admits at most `workers` files at a time. Acquiring before the goroutine
	// starts is what bounds the run: it also bounds how far ahead the scheduler reads, so a stop
	// takes effect within one slot rather than after every file has been launched.
	sem := make(chan struct{}, workers)

	scheduled := len(v.Tests.TestSuites)
	for i := range v.Tests.TestSuites {
		sem <- struct{}{}

		mu.Lock()
		halted := stop
		mu.Unlock()
		if halted {
			<-sem
			scheduled = i
			break
		}

		wg.Add(1)
		go func(ts *TestSuite) {
			defer wg.Done()
			defer func() { <-sem }()

			ts.Start = time.Now()
			if err := v.runTestSuite(ctx, ts); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				stop = true
				mu.Unlock()
				return
			}
			ts.End = time.Now()
			ts.Duration = ts.End.Sub(ts.Start).Seconds()

			if v.stopRunAfter(ts) {
				mu.Lock()
				stop = true
				stoppedByFailure = true
				mu.Unlock()
			}
		}(&v.Tests.TestSuites[i])
	}

	// Files already in flight are never cancelled: they finish and report, so the run's output
	// accounts for every file that started.
	wg.Wait()

	if stoppedByFailure && scheduled < len(v.Tests.TestSuites) {
		v.skipTestSuitesFrom(scheduled)
	}
	return firstErr
}

// stopRunAfter reports whether a finished test file should stop the rest of the run.
//
// Before #2020, --stop-on-failure only skipped the remaining test cases *inside* the file that
// failed (runTestCases, process_testsuite.go): the run carried on with the next file. It now stops
// the whole run, at every --parallel value, which is a deliberate change to what the flag does —
// recorded in docs/feat/2020/plan.md and carved out of #2018's AC-1.
func (v *Venom) stopRunAfter(ts *TestSuite) bool {
	return v.StopOnFailure && ts.Status == StatusFail
}

// skipTestSuitesFrom marks the test files that never started as skipped, so a stopped run still
// accounts for every file instead of leaving holes the report cannot explain. It mirrors what
// runTestCases already does to the test cases of a file it stops.
func (v *Venom) skipTestSuitesFrom(first int) {
	for i := first; i < len(v.Tests.TestSuites); i++ {
		ts := &v.Tests.TestSuites[i]
		if ts.Status != "" {
			continue
		}
		ts.Status = StatusSkip
		for j := range ts.TestCases {
			tc := &ts.TestCases[j]
			if tc.Status == "" {
				tc.Status = StatusSkip
				tc.IsEvaluated = true
				tc.Skipped = append(tc.Skipped, Skipped{Value: "===== stop-on-failure: enabled ====="})
				// runTestSuite is what normally fills these, and it never ran for this file. The
				// JSON report serialises them per suite, so without this a skipped file reports
				// every test case as SKIP next to "nbTestcasesSkip": 0. XML and TAP derive their
				// counts from the test cases and are unaffected either way.
				ts.NbTestcasesSkip++
			}
		}
	}
}

// countTestSuiteStatuses tallies how many test suites passed, failed and were skipped. The tally
// used to be kept by incrementing counters on the shared Tests struct from inside runTestSuite;
// deriving it once here keeps run-wide bookkeeping out of the per-file path, which #2018 will run
// concurrently (#2019). A suite whose status was never set — a run that stopped on an error before
// the suite finished — counts in none of the three, exactly as the increments did.
func countTestSuiteStatuses(testSuites []TestSuite) (fail, pass, skip int) {
	for i := range testSuites {
		switch testSuites[i].Status {
		case StatusFail:
			fail++
		case StatusPass:
			pass++
		case StatusSkip:
			skip++
		}
	}
	return fail, pass, skip
}
