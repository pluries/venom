package venom

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/gosimple/slug"
	"github.com/pkg/errors"
)

func (v *Venom) runTestSuite(ctx context.Context, ts *TestSuite) error {
	if v.Verbose == 3 {
		stopProfiling, err := v.startTestSuiteProfiling(ts)
		if err != nil {
			return err
		}
		// A closure, not defer stopProfiling(ctx): the warnings carry the ctx built below, as they always have.
		defer func() { stopProfiling(ctx) }()
	}

	if err := v.initTestSuiteVars(ts); err != nil {
		return err
	}

	ctx = context.WithValue(ctx, ContextKey("testsuite"), ts.Name)
	ctx = v.processSecrets(ctx, ts, nil)
	Info(ctx, "Starting testsuite")
	defer Info(ctx, "Ending testsuite")

	totalSteps := 0
	for _, tc := range ts.TestCases {
		totalSteps += len(tc.RawTestSteps)
	}

	ts.Vars.Add(("venom.testsuite.totalSteps"), totalSteps)
	ts.Status = StatusRun
	Info(ctx, "With secrets in testsuite")
	for _, v := range ts.Secrets {
		Info(ctx, "secret  %+v", v)
	}
	// ##### RUN Test Cases Here
	v.runTestCases(ctx, ts)

	tallyTestCaseStatuses(ts)
	return nil
}

// startTestSuiteProfiling starts the CPU profile of ts and returns the function that stops it and
// writes the memory profile.
func (v *Venom) startTestSuiteProfiling(ts *TestSuite) (func(ctx context.Context), error) {
	filenameCPU := filepath.Join(v.OutputDir, "pprof_cpu_profile_"+ts.Filename+".prof")
	filenameMem := filepath.Join(v.OutputDir, "pprof_mem_profile_"+ts.Filename+".prof")
	fCPU, errCPU := os.Create(filenameCPU)
	fMem, errMem := os.Create(filenameMem)

	// Every path out of this block releases both files and leaves no empty profile behind: an
	// unopenable file that looks present is what #2024 was about.
	// Close and Remove are unchecked on purpose here: nothing was written, and the error that
	// brought us to this path is the one worth reporting. No //nolint is needed — the config's
	// std-error-handling exclusion already covers unchecked Close and os.Remove, and a directive
	// that suppresses nothing is itself a finding now that nolintlint is on (#2027).
	discard := func() {
		if fCPU != nil {
			fCPU.Close()
			os.Remove(filenameCPU)
		}
		if fMem != nil {
			fMem.Close()
			os.Remove(filenameMem)
		}
	}

	if errCPU != nil || errMem != nil {
		discard()
		return nil, fmt.Errorf("error while create profile file CPU:%v MEM:%v", errCPU, errMem)
	}
	// Go's CPU profiler is process-wide, so this fails if anything else already holds it. The
	// error used to be discarded, which left a zero-byte file pprof cannot open and a run that
	// reported success (#2024).
	if err := pprof.StartCPUProfile(fCPU); err != nil {
		discard()
		return nil, fmt.Errorf("unable to start cpu profiling for %q: %w", ts.Filename, err)
	}

	p := pprof.Lookup("heap")
	return func(ctx context.Context) {
		// Order matters: StopCPUProfile flushes into fCPU, so both files are closed after
		// their last write. More test files follow this one, so a failure here is reported
		// rather than swallowed — silently writing an empty profile is the defect being fixed.
		pprof.StopCPUProfile()
		if err := p.WriteTo(fMem, 1); err != nil {
			Warn(ctx, "unable to write memory profile %q: %v", filenameMem, err)
		}
		if err := fMem.Close(); err != nil {
			Warn(ctx, "unable to close memory profile %q: %v", filenameMem, err)
		}
		if err := fCPU.Close(); err != nil {
			Warn(ctx, "unable to close cpu profile %q: %v", filenameCPU, err)
		}
	}, nil
}

// initTestSuiteVars initializes the testsuite variables, computes a first interpolation over them
// and adds the venom.* variables.
func (v *Venom) initTestSuiteVars(ts *TestSuite) error {
	ts.Vars.AddAll(v.variables.Clone())
	vars, err := DumpStringPreserveCase(ts.Vars)
	if err != nil {
		return errors.Wrapf(err, "error while dumping testsuite variables")
	}
	for k, v := range vars {
		computedV, err := interpolateSuiteVar(k, fmt.Sprintf("%v", v), vars)
		if err != nil {
			return errors.Wrapf(err, "error while computing variable %s=%q", k, v)
		}
		ts.Vars.Add(k, computedV)
	}

	exePath, err := os.Executable()
	if err != nil {
		return errors.Wrapf(err, "failed to get executable path")
	}
	ts.Vars.Add("venom.executable", exePath)

	ts.Vars.Add("venom.outputdir", v.OutputDir)
	ts.Vars.Add("venom.libdir", v.LibDir)
	ts.Vars.Add("venom.testsuite", ts.Name)
	ts.ComputedVars = H{}
	return nil
}

// tallyTestCaseStatuses counts the test cases of ts per status and sets the status of ts from them.
func tallyTestCaseStatuses(ts *TestSuite) {
	var isFailed bool
	var nSkip int
	for _, tc := range ts.TestCases {
		switch tc.Status {
		case StatusFail:
			isFailed = true
			ts.NbTestcasesFail++
		case StatusSkip:
			nSkip++
			ts.NbTestcasesSkip++
		case StatusPass:
			ts.NbTestcasesPass++
		}
	}

	// The run-wide tallies are derived from these statuses once every suite has finished
	// (countTestSuiteStatuses, called from Process) rather than incremented here: this function
	// runs once per test file, and #2018 will run those concurrently (#2019).
	switch {
	case isFailed:
		ts.Status = StatusFail
	case nSkip > 0 && nSkip == len(ts.TestCases):
		ts.Status = StatusSkip
	default:
		ts.Status = StatusPass
	}
}

func (v *Venom) runTestCases(ctx context.Context, ts *TestSuite) {
	verboseReport := v.Verbose >= 1

	// Every line this file prints goes through out: untagged and lock-free at --parallel 1, tagged
	// with the file name and serialised above it, so concurrent files stay readable (#2021).
	out := v.newSuiteOut(ts)
	// A line left unterminated when the file ends is still the reader's business.
	defer out.Flush()
	// A name may hold a declared secret: the suite line prints it hidden with the suite's secrets, the lines of a test
	// case and of its steps with the test case's.
	out.Println(" • %s (%s)", HideSensitive(ctx, ts.Name), ts.Filepath)

	for i := range ts.TestCases {
		tc := &ts.TestCases[i]
		tc.IsEvaluated = true
		// The deeper functions that print (runTestSteps, printTestStepResult) reach it through
		// v.out(tc); see the field's comment in types.go for why it rides on the test case.
		tc.out = out
		tcCtx := v.processSecrets(ctx, ts, tc)
		out.Print(" \t• %s", HideSensitive(tcCtx, tc.Name))

		hasFailure, skippedSteps := v.runSuiteTestCase(ctx, ts, tc, out, verboseReport)
		setTestCaseStatus(tc, hasFailure, skippedSteps)

		if skipRest := reportTestCase(tcCtx, out, tc, verboseReport, hasFailure); skipRest {
			continue
		}

		if v.StopOnFailure && skipRemainingTestCasesOnError(ts, tc) {
			return
		}
		ts.ComputedVars.AddAllWithPrefix(tc.Name, tc.computedVars)
	}
}

// runSuiteTestCase runs tc unless it was skipped up front, then reads its step results: whether
// one failed, and how many were skipped.
func (v *Venom) runSuiteTestCase(ctx context.Context, ts *TestSuite, tc *TestCase, out *suiteOut, verboseReport bool) (hasFailure bool, skippedSteps int) {
	hasSkipped := len(tc.Skipped) > 0
	if !hasSkipped {
		start := time.Now()
		tc.Start = start
		ts.Status = StatusRun
		// Ranged cases never got this newline: the flag meant for them was read before it was set (#2031 D60).
		if verboseReport {
			out.Print("\n")
		}
		// ##### RUN Test Case Here
		v.runTestCase(ctx, ts, tc)
		tc.End = time.Now()
		tc.Duration = tc.End.Sub(tc.Start).Seconds()
	}

	for _, testStepResult := range tc.TestStepResults {
		if testStepResult.Status == StatusFail {
			hasFailure = true
		}
		if testStepResult.Status == StatusSkip {
			skippedSteps++
		}
	}
	return hasFailure, skippedSteps
}

// setTestCaseStatus sets the status of tc from what its step results showed.
func setTestCaseStatus(tc *TestCase, hasFailure bool, skippedSteps int) {
	switch {
	case hasFailure:
		tc.Status = StatusFail
	case skippedSteps == len(tc.TestStepResults):
		// If all test steps were skipped, consider the test case as skipped
		tc.Status = StatusSkip
	case tc.Status != StatusSkip:
		tc.Status = StatusPass
	}
}

// reportTestCase prints the result of tc and reports whether the rest of its iteration is skipped; ctx holds the
// secrets of tc.
func reportTestCase(ctx context.Context, out *suiteOut, tc *TestCase, verboseReport, hasFailure bool) (skipRest bool) {
	// Verbose mode already reported tests status, so just print them when non-verbose
	indent := ""
	if verboseReport {
		indent = "\t  "
		// If the testcase was entirely skipped, then the verbose mode will not have any output
		// Print something to inform that the testcase was indeed processed although skipped
		if len(tc.TestStepResults) == 0 {
			out.Println("\t\t%s", Gray("• (all steps were skipped)"))
			return true
		}
	} else {
		switch {
		case hasFailure:
			out.Println(" %s", Red(StatusFail))
		case tc.Status == StatusSkip:
			out.Println(" %s", Gray(StatusSkip))
			return true
		default:
			out.Println(" %s", Green(StatusPass))
		}
	}

	for _, i := range tc.computedVerbose {
		out.PrintlnIndentedTrace(i, indent)
	}

	// Verbose mode already reported failures, so just print them when non-verbose
	if !verboseReport && hasFailure {
		printTestCaseFailures(ctx, out, tc)
	}
	return false
}

// printTestCaseFailures prints the info and the errors of every step of tc that has any, under the step name, all
// hidden with the secrets of ctx.
func printTestCaseFailures(ctx context.Context, out *suiteOut, tc *TestCase) {
	for _, testStepResult := range tc.TestStepResults {
		if len(testStepResult.ComputedInfo) > 0 || len(testStepResult.Errors) > 0 {
			out.Println(" \t\t• %s", HideSensitive(ctx, testStepResult.Name))
			for _, f := range testStepResult.ComputedInfo {
				out.Println(" \t\t  %s", Cyan(HideSensitive(ctx, f)))
			}
			for _, f := range testStepResult.Errors {
				out.Println(" \t\t  %s", Yellow(HideSensitive(ctx, f.Value)))
			}
		}
	}
}

// skipRemainingTestCasesOnError marks every test case of ts without a status as skipped when a
// step of failed has errors, and reports whether it did.
func skipRemainingTestCasesOnError(ts *TestSuite, failed *TestCase) bool {
	for _, testStepResult := range failed.TestStepResults {
		if len(testStepResult.Errors) > 0 {
			// break TestSuite
			for i := range ts.TestCases {
				tc := &ts.TestCases[i]
				if tc.Status == "" {
					tc.Status = StatusSkip
					tc.IsEvaluated = true
					tc.Skipped = append(tc.Skipped, Skipped{Value: "===== stop-on-failure: enabled ====="})
				}
			}
			return true
		}
	}
	return false
}

// Parse the suite to find unreplaced and extracted variables
func (v *Venom) parseTestSuite(ts *TestSuite) ([]string, []string, error) {
	return v.parseTestCases(ts)
}

// Parse the testscases to find unreplaced and extracted variables
func (v *Venom) parseTestCases(ts *TestSuite) ([]string, []string, error) {
	var vars []string
	var extractsVars []string
	for i := range ts.TestCases {
		tc := &ts.TestCases[i]
		tc.originalName = tc.Name
		tc.number = i + 1
		tc.Name = slug.Make(tc.Name)
		tc.Vars = ts.Vars.Clone()
		tc.Vars.Add("venom.testcase", tc.Name)

		if len(tc.Skipped) == 0 {
			tvars, tExtractedVars, err := v.parseTestCase(ts, tc)
			if err != nil {
				return nil, nil, err
			}
			for _, k := range tvars {
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
			for _, k := range tExtractedVars {
				var found bool
				for i := 0; i < len(extractsVars); i++ {
					if extractsVars[i] == k {
						found = true
						break
					}
				}
				if !found {
					extractsVars = append(extractsVars, k)
				}
			}
		}
	}

	return vars, extractsVars, nil
}
