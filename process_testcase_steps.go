package venom

import (
	"context"
	"encoding/json"
)

// stepsRun is the state of the steps of one test case while runTestSteps walks them.
type stepsRun struct {
	v    *Venom
	ctx  context.Context
	tc   *TestCase
	tsIn *TestStepResult
	// knowExecutors holds the executors already set up for this test case.
	knowExecutors map[string]struct{}
	// previousStepVars holds the variables the steps already run assigned.
	previousStepVars H
	// teardowns run when the steps are done, the last one set up first.
	teardowns []func()
}

// stepFlow says what runTestSteps does after one range item of a step.
type stepFlow int

const (
	flowNext    stepFlow = iota // go on with the next range item
	flowAbandon                 // give up the other items of the step and go on with the next step
	flowStop                    // run no other step of the test case
)

// rangeItem is one iteration of a step: its position in the test case and the data of its range.
type rangeItem struct {
	stepNumber int // 1-based
	index      int
	data       RangeData
}

func (v *Venom) runTestSteps(ctx context.Context, tc *TestCase, tsIn *TestStepResult) {
	if stopBeforeTestSteps(ctx, tc) {
		return
	}

	r := &stepsRun{v: v, ctx: ctx, tc: tc, tsIn: tsIn, knowExecutors: map[string]struct{}{}, previousStepVars: H{}}
	defer r.tearDown()
	for stepIndex, rawStep := range tc.RawTestSteps {
		// Use stepNumber as a 1-based index
		if r.runRawStep(rawStep, stepIndex+1) {
			return
		}
	}
}

// tearDown runs the teardowns of the executors the steps set up, the last one first, as deferred calls would.
func (r *stepsRun) tearDown() {
	for i := len(r.teardowns) - 1; i >= 0; i-- {
		r.teardowns[i]()
	}
}

// startVars returns the variables a step starts from: the test case's own, what the earlier steps assigned, and the
// computed variables of the test case. For a test case that holds only its own variables (TestCase.varsOverSuite) it is
// as small as they are.
func (r *stepsRun) startVars(stepNumber int) H {
	stepVars := r.tc.Vars.Clone()
	stepVars.AddAll(r.previousStepVars)
	stepVars.AddAllWithPrefix(r.tc.Name, r.tc.computedVars)
	stepVars.Add("venom.teststep.number", stepNumber)
	return stepVars
}

// runRawStep runs one step of the test case, once for each item of its range. It reports true when no other step must run.
func (r *stepsRun) runRawStep(rawStep json.RawMessage, stepNumber int) bool {
	stepVars := r.startVars(stepNumber)

	ranged, err := parseRanged(r.ctx, rawStep, func() H { return r.tc.fullVars(stepVars) })
	if err != nil {
		Error(r.ctx, "unable to parse \"range\" attribute: %v", err)
		testStepResult := TestStepResult{}
		testStepResult.appendError(err)
		r.tc.TestStepResults = append(r.tc.TestStepResults, testStepResult)
		return true
	}

	for rangedIndex, rangedData := range ranged.Items {
		switch r.runRangeItem(rawStep, stepVars, &ranged, rangeItem{stepNumber: stepNumber, index: rangedIndex, data: rangedData}) {
		case flowStop:
			return true
		case flowAbandon:
			return false
		case flowNext:
		}
	}
	return false
}

// prepareStep resolves the scope of one range item of a step, interpolates the step with it and decodes the result. It
// reports false when the step failed, which the step result records.
func (r *stepsRun) prepareStep(tsResult *TestStepResult, rawStep json.RawMessage, stepVars H, ranged *Range, it rangeItem) (TestStep, *stepScope, bool) {
	scope, err := rangedStepVars(r.ctx, r.tc, tsResult, stepVars, ranged, it.index, it.data)
	if err != nil {
		return nil, nil, false
	}

	content, err := r.v.interpolateTestStep(r.ctx, tsResult, rawStep, scope, ranged, it.stepNumber, it.index)
	if err != nil {
		return nil, nil, false
	}

	step, err := r.v.decodeTestStep(r.ctx, r.tc, tsResult, r.tsIn, rawStep, content, it.stepNumber)
	return step, scope, err == nil
}

// runRangeItem runs one range item of a step: it prepares the step, sets its executor up, runs it and assigns the variables
// it extracts.
func (r *stepsRun) runRangeItem(rawStep json.RawMessage, stepVars H, ranged *Range, it rangeItem) stepFlow {
	tc := r.tc
	tc.TestStepResults = append(tc.TestStepResults, TestStepResult{})
	tsResult := &tc.TestStepResults[len(tc.TestStepResults)-1]

	step, scope, ok := r.prepareStep(tsResult, rawStep, stepVars, ranged, it)
	if !ok {
		return flowStop
	}

	tsResult.Number = it.stepNumber
	tsResult.RangedIndex = it.index
	tsResult.RangedEnable = ranged.Enabled
	// InputVars is built when the result is reported, so that the step does not keep its own copy of the scope (#2475)
	tsResult.lazyInputs = scope.inputLayers()

	tc.testSteps = append(tc.testSteps, step)
	e, ok := r.executorFor(tsResult, step, scope, it.stepNumber)
	if !ok {
		return flowStop
	}
	if !r.setUpFor(e, tsResult) {
		return flowAbandon
	}
	r.v.setTestStepName(tsResult, e, step, ranged, &it.data, it.index)
	r.v.printTestStepName(r.ctx, tc, tsResult, r.tsIn)

	r.v.runOrSkipTestStep(r.ctx, e, tc, tsResult, stepToRun{raw: rawStep, step: step, number: it.stepNumber, rangedIndex: it.index}, r.previousStepVars)
	return r.finishStep(tsResult, rawStep, it)
}

// executorFor resolves the executor of a step, and reports false when it cannot be: the step result records the error.
func (r *stepsRun) executorFor(tsResult *TestStepResult, step TestStep, scope *stepScope, stepNumber int) (ExecutorRunner, bool) {
	// GetExecutorRunner returns a nil context on some errors: the step result is printed with the test
	// case's context, which holds the secrets to hide.
	stepCtx, e, err := r.v.getExecutorRunnerFlat(r.ctx, step, scope)
	if err != nil {
		tsResult.appendError(err)
		Error(stepCtx, "unable to get executor: %v", err)
		r.v.printTestStepResult(r.ctx, r.tc, tsResult, r.tsIn, stepNumber, false)
		return nil, false
	}
	r.ctx = stepCtx
	if r.tc.IsExecutor {
		decodeStepFields(e.GetExecutor(), step)
	}
	return e, true
}

// setUpFor sets the executor up the first time the test case uses it, and keeps its teardown for the end of the steps. It
// reports false when the setup failed.
func (r *stepsRun) setUpFor(e ExecutorRunner, tsResult *TestStepResult) bool {
	ctx, teardown, err := setUpExecutor(r.ctx, e, r.tc, tsResult, r.knowExecutors)
	r.ctx = ctx
	if teardown != nil {
		// Kept here, not in setUpExecutor: teardowns run when runTestSteps returns.
		r.teardowns = append(r.teardowns, teardown)
	}
	return err == nil
}

// finishStep reports a step that failed, or assigns the variables it extracts.
func (r *stepsRun) finishStep(tsResult *TestStepResult, rawStep json.RawMessage, it rangeItem) stepFlow {
	if tsResult.Status == StatusFail {
		if r.v.reportFailedTestStep(r.ctx, r.tc, tsResult, r.tsIn, it.stepNumber, it.index) {
			return flowStop
		}
		return flowNext
	}
	Info(r.ctx, "Step %q result is %q", tsResult.Name, tsResult.Status)

	assign, err := r.v.assignTestStepVars(r.ctx, r.tc, tsResult, r.tsIn, rawStep, it.stepNumber)
	if err != nil {
		return flowStop
	}

	r.tc.computedVars.AddAll(assign)
	r.previousStepVars.AddAll(assign)
	return flowNext
}

// stopBeforeTestSteps evaluates the test case's skip clause; it reports true when no step must run,
// because the clause holds or because it cannot be evaluated.
func stopBeforeTestSteps(ctx context.Context, tc *TestCase) bool {
	var vars H
	if len(tc.Skip) > 0 {
		vars = tc.fullVars(tc.Vars)
	}
	results, err := testConditionalStatement(ctx, tc, tc.Skip, vars, "skipping testcase %q: %v")
	if err != nil {
		Error(ctx, "unable to evaluate \"skip\" assertions: %v", err)
		testStepResult := TestStepResult{}
		testStepResult.appendError(err)
		tc.TestStepResults = append(tc.TestStepResults, testStepResult)
		return true
	}
	if len(results) > 0 {
		tc.Status = StatusSkip
		for i := range results {
			tc.Skipped = append(tc.Skipped, Skipped{Value: results[i]})
			Warn(ctx, results[i], nil)
		}
		return true
	}
	return false
}
