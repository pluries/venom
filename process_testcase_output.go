package venom

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ovh/venom/internal/yamljson"
)

// assignTestStepVars processes the `vars:` assignments of a step that did not fail, then prints the
// step result.
func (v *Venom) assignTestStepVars(ctx context.Context, tc *TestCase, tsResult, tsIn *TestStepResult, rawStep json.RawMessage, stepNumber int) (H, error) {
	// the step's own computed variables win over the test case's
	lookup := func(name string) (interface{}, bool) {
		if v, ok := tsResult.ComputedVars[name]; ok {
			return v, true
		}
		return tc.lookupVar(name)
	}
	assign, _, errAssignment := assignVariables(ctx, tc.Name, lookup, rawStep)
	if errAssignment != nil {
		tsResult.appendError(errAssignment)
		Error(ctx, "unable to process variable assignments: %v", errAssignment)
	}

	v.printTestStepResult(ctx, tc, tsResult, tsIn, stepNumber, false)
	return assign, errAssignment
}

// Set test step name (defaults to executor name, excepted if it got a "name" attribute. in range, also print key)
func (v *Venom) setTestStepName(ts *TestStepResult, e ExecutorRunner, step TestStep, ranged *Range, rangedData *RangeData, _ int) {
	name := e.Name()
	if value, ok := step["name"]; ok {
		if value, ok := value.(string); ok {
			name = value
		}
	}
	if ranged.Enabled {
		name = fmt.Sprintf("%s (range=%s)", name, rangedData.Key)
	}
	ts.Name = name
}

// stepDetailFormat is the layout of every detail line printed under a step on the console.
const stepDetailFormat = " \t\t  %s"

// Print a single step result (if verbosity is enabled), its info and failure lines hidden with the secrets of ctx.
func (v *Venom) printTestStepResult(ctx context.Context, tc *TestCase, ts *TestStepResult, tsIn *TestStepResult, stepNumber int, mustAssertionFailed bool) {
	fromUserExecutor := tsIn != nil
	if fromUserExecutor || v.Verbose < 1 {
		return
	}
	out := v.out(tc)
	switch {
	case len(ts.Errors) > 0:
		printFailedStep(ctx, out, tc, ts, stepNumber, mustAssertionFailed)
	case ts.Status == StatusSkip:
		out.Println(" %s", Gray(StatusSkip))
	default:
		if ts.Retries == 0 {
			out.Println(" %s", Green(StatusPass))
		} else {
			out.Println(" %s (after %d attempts)", Green(StatusPass), ts.Retries)
		}
		printStepInfo(ctx, out, ts)
	}
}

// printFailedStep prints the FAIL status of a step, its info and failure lines, and how many steps its failure skips.
func printFailedStep(ctx context.Context, out *suiteOut, tc *TestCase, ts *TestStepResult, stepNumber int, mustAssertionFailed bool) {
	out.Println(" %s", Red(StatusFail))
	printStepInfo(ctx, out, ts)
	for _, f := range ts.Errors {
		out.Println(stepDetailFormat, Yellow(HideSensitive(ctx, f.Value)))
	}
	if !mustAssertionFailed {
		return
	}
	skipped := len(tc.RawTestSteps) - stepNumber
	if skipped == 1 {
		out.Println(stepDetailFormat, Gray(fmt.Sprintf("%d other step was skipped", skipped)))
		return
	}
	out.Println(stepDetailFormat, Gray(fmt.Sprintf("%d other steps were skipped", skipped)))
}

// printStepInfo prints the [info] lines of a step, hidden with the secrets of ctx.
func printStepInfo(ctx context.Context, out *suiteOut, ts *TestStepResult) {
	for _, i := range ts.ComputedInfo {
		out.Println(" \t\t  %s %s", Cyan("[info]"), Cyan(HideSensitive(ctx, i)))
	}
}

// Parse and format skip conditional
func parseSkip(ctx context.Context, tc *TestCase, ts *TestStepResult, rawStep []byte, stepNumber int, vars func() H) (bool, error) {
	// Load "skip" attribute from step
	var assertions struct {
		Skip []string `yaml:"skip"`
	}
	if err := yamljson.Unmarshal(rawStep, &assertions); err != nil {
		return false, fmt.Errorf("unable to parse \"skip\" assertions: %v", err)
	}

	// Evaluate skip assertions
	if len(assertions.Skip) > 0 {
		results, err := testConditionalStatement(ctx, tc, assertions.Skip, vars(), fmt.Sprintf("skipping testcase %%q step #%d: %%v", stepNumber))
		if err != nil {
			Error(ctx, "unable to evaluate \"skip\" assertions: %v", err)
			return false, err
		}
		if len(results) > 0 {
			for i := range results {
				ts.Skipped = append(ts.Skipped, Skipped{Value: results[i]})
				Warn(ctx, results[i], nil)
			}
			return true, nil
		}
	}
	return false, nil
}
