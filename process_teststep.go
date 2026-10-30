package venom

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/gosimple/slug"
	"github.com/ovh/venom/interpolate"
)

type dumpFile struct {
	Variables H           `json:"variables"`
	TestStep  TestStep    `json:"step"`
	Result    interface{} `json:"result"`
}

// RunTestStep executes a venom testcase is a venom context
func (v *Venom) RunTestStep(ctx context.Context, e ExecutorRunner, tc *TestCase, tsResult *TestStepResult, stepNumber int, rangedIndex int, step TestStep) {
	ctx = context.WithValue(ctx, ContextKey("executor"), e.Name())

	var assertRes AssertionsApplied
	var result interface{}

	for tsResult.Retries = 0; tsResult.Retries <= e.Retry() && !assertRes.OK; tsResult.Retries++ {
		sleepBeforeRetry(ctx, e, tsResult, assertRes)

		var err error
		result, err = v.runTestStepExecutor(ctx, e, tc, tsResult, step)
		if err != nil {
			failTestStepAttempt(ctx, e, tc, tsResult, stepNumber, rangedIndex, err)
			continue
		}

		Debug(ctx, "result of executor: %s", result)
		mapResult, err := getExecutorResult(result)
		if err != nil {
			// a result that cannot be dumped fails the step like an executor error does
			failTestStepAttempt(ctx, e, tc, tsResult, stepNumber, rangedIndex, fmt.Errorf("unable to dump executor result: %w", err))
			continue
		}
		mapResultString := dumpResultString(ctx, result)

		if err := v.dumpTestStepIfVerbose(ctx, tc, result, step, stepNumber, rangedIndex); err != nil {
			return
		}

		appendTestStepInfo(ctx, e, tc, tsResult, mapResultString, stepNumber)

		assertRes = applyTestStepAssertions(ctx, e, tc, result, stepNumber, rangedIndex, step)

		tsResult.AssertionsApplied = assertRes
		tsResult.ComputedVars.AddAll(H(mapResult))

		if assertRes.OK {
			break
		}
		if stopTestStepRetries(ctx, e, tc, tsResult, stepNumber, rangedIndex) {
			break
		}
	}

	appendAssertionFailures(tsResult, assertRes)
}

// sleepBeforeRetry waits the executor's delay before every attempt but the first.
func sleepBeforeRetry(ctx context.Context, e ExecutorRunner, tsResult *TestStepResult, assertRes AssertionsApplied) {
	if tsResult.Retries >= 1 && !assertRes.OK {
		Debug(ctx, "Sleep %d, it's %d attempt", e.Delay(), tsResult.Retries)
		time.Sleep(time.Duration(e.Delay()) * time.Second)
	}
}

// failTestStepAttempt saves the failure of an attempt only if it is the last one.
func failTestStepAttempt(ctx context.Context, e ExecutorRunner, tc *TestCase, tsResult *TestStepResult, stepNumber, rangedIndex int, err error) {
	if tsResult.Retries == e.Retry() {
		failure := newFailure(ctx, *tc, stepNumber, rangedIndex, -1, "", err)
		tsResult.appendFailure(*failure)
	}
}

// dumpResultString dumps the result for the info lines; RunTestStep has no error return, so a failure is logged
// and the map stays empty, the same degradation as the info-interpolation failure handled in appendTestStepInfo.
func dumpResultString(ctx context.Context, result interface{}) map[string]string {
	mapResultString, err := DumpString(result)
	if err != nil {
		Error(ctx, "unable to dump executor result: %v", err)
	}
	return mapResultString
}

// dumpTestStepIfVerbose writes the verbose dump file at verbosity 2 and above.
func (v *Venom) dumpTestStepIfVerbose(ctx context.Context, tc *TestCase, result interface{}, step TestStep, stepNumber, rangedIndex int) error {
	if v.Verbose < 2 {
		return nil
	}
	return v.writeTestStepDump(ctx, tc, result, step, stepNumber, rangedIndex)
}

// writeTestStepDump writes the verbose dump file of one attempt and records its name on the test
// case; it returns the write error, which ends the step with no failure recorded.
func (v *Venom) writeTestStepDump(ctx context.Context, tc *TestCase, result interface{}, step TestStep, stepNumber, rangedIndex int) error {
	fdump := dumpFile{
		Result:    result,
		TestStep:  step,
		Variables: AllVarsFromCtx(ctx),
	}
	output, err := json.MarshalIndent(fdump, "", " ")
	if err != nil {
		Error(ctx, "unable to marshal result: %v", err)
	}

	// The test case name may hold a declared secret: the file name carries it hidden.
	filename := filepath.Join(v.OutputDir, fmt.Sprintf("%s.%s.testcase.%d.step.%d.%d.dump.json",
		slug.Make(StringVarFromCtx(ctx, "venom.testsuite.shortName")), slug.Make(HideSensitive(ctx, tc.Name)), tc.number, stepNumber, rangedIndex))

	if err := os.WriteFile(filename, []byte(HideSensitive(ctx, string(output))), 0o600); err != nil {
		Error(ctx, "Error while creating file %s: %v", filename, err)
		return err
	}
	tc.computedVerbose = append(tc.computedVerbose, fmt.Sprintf("writing %s", filename))
	return nil
}

// appendTestStepInfo interpolates the executor's info lines with the result of one attempt and
// records them on the step.
func appendTestStepInfo(ctx context.Context, e ExecutorRunner, tc *TestCase, tsResult *TestStepResult, mapResultString map[string]string, stepNumber int) {
	for _, i := range e.Info() {
		info, err := interpolate.Do(i, mapResultString)
		if err != nil {
			Error(ctx, "unable to parse %q: %v", i, err)
			continue
		}
		if info == "" {
			continue
		}
		filepath := StringVarFromCtx(ctx, "venom.testsuite.filepath")
		lineNumber := tc.findSourceLine(stepNumber, -1)
		if lineNumber > 0 {
			info += fmt.Sprintf(" (%s:%d)", filepath, lineNumber)
		}
		Info(ctx, info, nil)
		tsResult.ComputedInfo = append(tsResult.ComputedInfo, info)
	}
}

// applyTestStepAssertions applies the step's assertions to the result of one attempt, or to the
// vars when the executor returned no result.
func applyTestStepAssertions(ctx context.Context, e ExecutorRunner, tc *TestCase, result interface{}, stepNumber, rangedIndex int, step TestStep) AssertionsApplied {
	if result == nil {
		Debug(ctx, "empty testcase, applying assertions on variables: %v", AllVarsFromCtx(ctx))
		return applyAssertions(ctx, AllVarsFromCtx(ctx), *tc, stepNumber, rangedIndex, step, nil)
	}
	if h, ok := e.(executorWithDefaultAssertions); ok {
		return applyAssertions(ctx, result, *tc, stepNumber, rangedIndex, step, h.GetDefaultAssertions())
	}
	return applyAssertions(ctx, result, *tc, stepNumber, rangedIndex, step, nil)
}

// stopTestStepRetries evaluates the executor's retry conditions after a failed attempt; it reports
// true when the remaining retries are skipped.
func stopTestStepRetries(ctx context.Context, e ExecutorRunner, tc *TestCase, tsResult *TestStepResult, stepNumber, rangedIndex int) bool {
	failures, err := testConditionalStatement(ctx, tc, e.RetryIf(), tsResult.ComputedVars, "")
	if err != nil {
		tsResult.appendError(fmt.Errorf("Error while evaluating retry condition: %v", err))
		return true
	}
	if len(failures) > 0 {
		failure := newFailure(ctx, *tc, stepNumber, rangedIndex, -1, "", fmt.Errorf("retry conditions not fulfilled, skipping %d remaining retries", e.Retry()-tsResult.Retries))
		tsResult.Errors = append(tsResult.Errors, *failure)
		return true
	}
	return false
}

// appendAssertionFailures records the assertion failures and the output of the last attempt on the
// step.
func appendAssertionFailures(tsResult *TestStepResult, assertRes AssertionsApplied) {
	if tsResult.Retries > 1 && len(assertRes.errors) > 0 {
		tsResult.appendFailure(Failure{Value: fmt.Sprintf("It's a failure after %d attempts", tsResult.Retries)})
	}

	if len(assertRes.errors) > 0 {
		tsResult.appendFailure(assertRes.errors...)
	}

	tsResult.Systemerr += assertRes.systemerr + "\n"
	tsResult.Systemout += assertRes.systemout + "\n"
}

func (v *Venom) runTestStepExecutor(ctx context.Context, e ExecutorRunner, tc *TestCase, ts *TestStepResult, step TestStep) (interface{}, error) {
	ctx = context.WithValue(ctx, ContextKey("executor"), e.Name())

	if e.Timeout() == 0 {
		if e.Type() == "user" {
			return v.RunUserExecutor(ctx, e, tc, ts, step)
		}
		return e.Run(ctx, step)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(e.Timeout())*time.Second)
	defer cancel()

	// A user executor's attempt writes into a result of its own, seeded with ts's errors (the retry rule, #2102) and
	// merged only if it ends in time: one stopped at the timeout can neither race with the engine nor fail a retry (#2074).
	var attempt *TestStepResult
	if e.Type() == "user" {
		attempt = &TestStepResult{Errors: slices.Clone(ts.Errors)}
	}

	// Buffered so the goroutine can still send, and exit, after a timeout has left nobody receiving.
	ch := make(chan interface{}, 1)
	cherr := make(chan error, 1)
	go func(e ExecutorRunner, step TestStep) {
		var err error
		var result interface{}
		if attempt != nil {
			result, err = v.RunUserExecutor(ctxTimeout, e, tc, attempt, step)
		} else {
			result, err = e.Run(ctxTimeout, step)
		}
		if err != nil {
			cherr <- err
		} else {
			ch <- result
		}
	}(e, step)

	select {
	case err := <-cherr:
		mergeUserExecutorAttempt(ts, attempt)
		return nil, err
	case result := <-ch:
		mergeUserExecutorAttempt(ts, attempt)
		return result, nil
	case <-ctxTimeout.Done():
		return nil, fmt.Errorf("Timeout after %d second(s)", e.Timeout())
	}
}

// mergeUserExecutorAttempt merges a user executor's attempt that ended in time into the step's result, as the attempt
// would have written it there itself: the errors become the attempt's, which began as a copy of the step's, and its info
// lines and output are appended. A nil attempt, a built-in or plugin executor's, merges nothing.
func mergeUserExecutorAttempt(ts, attempt *TestStepResult) {
	if attempt == nil {
		return
	}
	ts.Errors = attempt.Errors
	ts.ComputedInfo = append(ts.ComputedInfo, attempt.ComputedInfo...)
	ts.Systemout += attempt.Systemout
	ts.Systemerr += attempt.Systemerr
}
