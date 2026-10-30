package venom

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
)

// setUpExecutor sets up e the first time the test case uses it, and returns the context Setup gave
// back with the matching teardown for the caller to defer. A nil or already set up executor returns
// ctx unchanged and no teardown.
func setUpExecutor(ctx context.Context, e ExecutorRunner, tc *TestCase, tsResult *TestStepResult, knowExecutors map[string]struct{}) (context.Context, func(), error) {
	if e == nil {
		return ctx, nil, nil
	}
	if _, known := knowExecutors[e.Name()]; known {
		return ctx, nil, nil
	}
	tcVars := tc.Vars
	if setupReadsVars(e) {
		tcVars = tc.fullVars(tc.Vars)
	}
	ctx, err := e.Setup(ctx, tcVars)
	if err != nil {
		tsResult.appendError(err)
		Error(ctx, "unable to setup executor: %v", err)
		return ctx, nil, err
	}
	knowExecutors[e.Name()] = struct{}{}
	return ctx, func() {
		if err := e.TearDown(ctx); err != nil {
			tsResult.appendError(err)
			Error(ctx, "unable to teardown executor: %v", err)
		}
	}, nil
}

// setupReadsVars reports whether e may read the variables its Setup is given. A builtin that has no Setup ignores them,
// and the whole scope of a test case that holds only its own variables is not built for it.
func setupReadsVars(e ExecutorRunner) bool {
	if x, ok := e.(*executor); ok {
		_, has := x.Executor.(ExecutorWithSetup)
		return has
	}
	return true
}

// printTestStepName prints the name of a step before it runs (if verbosity is enabled), hidden with the secrets of ctx
func (v *Venom) printTestStepName(ctx context.Context, tc *TestCase, tsResult *TestStepResult, tsIn *TestStepResult) {
	fromUserExecutor := tsIn != nil
	if v.Verbose >= 1 && !fromUserExecutor {
		v.out(tc).Print(" \t\t• %s", HideSensitive(ctx, tsResult.Name))
	}
}

// stepToRun is one step as runOrSkipTestStep takes it: the raw JSON its skip clause is read from,
// the decoded step, and its position in the test case.
type stepToRun struct {
	raw         json.RawMessage
	step        TestStep
	number      int
	rangedIndex int
}

// runOrSkipTestStep evaluates the step's skip clause against the vars earlier steps assigned, runs
// the step unless it is skipped, and sets its status.
func (v *Venom) runOrSkipTestStep(ctx context.Context, e ExecutorRunner, tc *TestCase, tsResult *TestStepResult, s stepToRun, assignedVars H) {
	skipVars := func() H {
		vars := tc.fullVars(tc.Vars)
		if !tc.varsOverSuite {
			vars = vars.Clone()
		}
		vars.AddAllWithPrefix(tc.Name, assignedVars)
		return vars
	}
	skip, err := parseSkip(ctx, tc, tsResult, s.raw, s.number, skipVars)
	switch {
	case err != nil:
		tsResult.appendError(err)
		tsResult.Status = StatusFail
	case skip:
		tsResult.Status = StatusSkip
	default:
		tsResult.Start = time.Now()
		tsResult.Status = StatusRun
		v.RunTestStep(ctx, e, tc, tsResult, s.number, s.rangedIndex, s.step)
		if len(tsResult.Errors) > 0 || !tsResult.AssertionsApplied.OK {
			tsResult.Status = StatusFail
		} else {
			tsResult.Status = StatusPass
		}

		tsResult.End = time.Now()
		tsResult.Duration = tsResult.End.Sub(tsResult.Start).Seconds()

		tc.testSteps = append(tc.testSteps, s.step)
	}
}

// reportFailedTestStep logs and prints a failed step; it reports true when a required failure skips
// the remaining steps of the test case.
func (v *Venom) reportFailedTestStep(ctx context.Context, tc *TestCase, tsResult, tsIn *TestStepResult, stepNumber, rangedIndex int) bool {
	var isRequired bool

	Error(ctx, "Step %q result is %q", tsResult.Name, tsResult.Status)
	Error(ctx, "Errors: ")
	for _, e := range tsResult.Errors {
		Error(ctx, "%v", e)
		isRequired = isRequired || e.AssertionRequired || v.StopOnFailure
	}

	Error(ctx, "teststep output vars are: %v", tsResult.ComputedVars)

	if isRequired {
		failure := newFailure(ctx, *tc, stepNumber, rangedIndex, -1, "", errors.New("At least one required assertion failed, skipping remaining steps"))
		tsResult.appendFailure(*failure)
		v.printTestStepResult(ctx, tc, tsResult, tsIn, stepNumber, true)
		return true
	}
	v.printTestStepResult(ctx, tc, tsResult, tsIn, stepNumber, false)
	return false
}
