package venom

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/ovh/venom/internal/yamljson"
	"github.com/ovh/venom/interpolate"
)

// rangedStepVars resolves the scope one range item of a step is interpolated with, the double quotes of the variables
// escaped for the JSON content of the step. The scope also holds the flattened vars as they were before that, which the
// step's executor is resolved with.
func rangedStepVars(ctx context.Context, tc *TestCase, tsResult *TestStepResult, stepVars H, ranged *Range, rangedIndex int, rangedData RangeData) (*stepScope, error) {
	if ranged.Enabled {
		Debug(ctx, "processing range index: %d", rangedIndex)
		stepVars.Add("index", rangedIndex)
		stepVars.Add("key", rangedData.Key)
		stepVars.Add("value", rangedData.Value)
	}

	if scope, ok := tc.layeredScope(stepVars); ok {
		return scope, nil
	}
	// the prepared suite cannot answer: the full path starts from the whole scope
	stepVars = tc.fullVars(stepVars)

	flat, err := tc.varsFlattener.flatten(stepVars)
	if err != nil {
		Error(ctx, "unable to dump testcase vars: %v", err)
		tsResult.appendError(err)
		return nil, err
	}
	vars := maps.Clone(flat)

	for k, v := range vars {
		content, err := interpolate.Do(v, vars)
		if err != nil {
			tsResult.appendError(err)
			Error(ctx, "unable to interpolate variable %q: %v", k, err)
			return nil, err
		}
		vars[k] = content
	}

	// the value of each var can contains a double-quote -> "
	// if the value is not escaped, it will be used as is, and the json sent to unmarshall will be incorrect.
	// This also avoids injections into the json structure of a step
	for i := range vars {
		if strings.Contains(vars[i], `"`) {
			vars[i] = escapeForJSON(vars[i])
		}
	}
	return newWholeScope(vars, flat), nil
}

// layeredScope lays the variables of a step over the prepared suite of the test case, and reports false when the full
// path must be taken instead (suiteScope.stepVars).
func (tc *TestCase) layeredScope(stepVars H) (*stepScope, bool) {
	if tc.varsOverSuite {
		return tc.suite.stepVarsOver(tc, stepVars)
	}
	return tc.suite.stepVars(tc, stepVars)
}

// fullVars returns the whole scope of a test case whose own variables are overlay: the variables of the test case, or
// those of a step of it. An ordinary test case holds its whole scope, and overlay is returned as it is. A test case that
// holds only its own variables over the suite's (varsOverSuite) gets a new map, the suite's variables with the overlay on
// top: that is the one place the whole scope of such a case is built, so a caller asks for it only when it reads it all.
func (tc *TestCase) fullVars(overlay H) H {
	if !tc.varsOverSuite {
		return overlay
	}
	return tc.suite.under(overlay)
}

// lookupVar returns one of the variables of the test case, without building its whole scope.
func (tc *TestCase) lookupVar(name string) (interface{}, bool) {
	if v, ok := tc.Vars[name]; ok || !tc.varsOverSuite {
		return v, ok
	}
	v, ok := tc.TestSuiteVars[name]
	return v, ok
}

// interpolateTestStep interpolates the content of one range item of a step with its scope and logs it. With
// StrictVariables on, a step that names an unknown variable fails instead of keeping the text as written.
func (v *Venom) interpolateTestStep(ctx context.Context, tsResult *TestStepResult, rawStep json.RawMessage, scope *stepScope, ranged *Range, stepNumber, rangedIndex int) (string, error) {
	var err error
	if v.StrictVariables {
		err = strictVariablesError(rawStep, scope, ranged, stepNumber, rangedIndex)
	}
	var content string
	if err == nil {
		content, err = interpolateStepContent(rawStep, scope)
	}
	if err != nil {
		tsResult.appendError(err)
		Error(ctx, "unable to interpolate step: %v", err)
		return "", err
	}

	if ranged.Enabled {
		Info(ctx, "Step #%d-%d content is: %s", stepNumber, rangedIndex, content)
	} else {
		Info(ctx, "Step #%d content is: %s", stepNumber, content)
	}
	return content, nil
}

// isResultVariable reports whether name is in the result namespace. Venom resolves those variables after the step ran
// (info, assertions, extracted variables), so they are not unknown when the step is interpolated.
func isResultVariable(name string) bool {
	return name == "result" || strings.HasPrefix(name, "result.")
}

// strictVariablesError returns an error naming the step and the variables it uses that venom does not know, or nil.
// Only names are reported, never values.
func strictVariablesError(rawStep json.RawMessage, scope *stepScope, ranged *Range, stepNumber, rangedIndex int) error {
	unknown, err := scope.UnknownVariables(string(rawStep))
	if err != nil {
		return err
	}
	unknown = slices.DeleteFunc(unknown, isResultVariable)
	if len(unknown) == 0 {
		return nil
	}
	step := fmt.Sprintf("#%d", stepNumber)
	if ranged.Enabled {
		step = fmt.Sprintf("#%d-%d", stepNumber, rangedIndex)
	}
	quoted := make([]string, len(unknown))
	for i, name := range unknown {
		quoted[i] = strconv.Quote(name)
	}
	return fmt.Errorf("step %s: unknown variable(s) %s", step, strings.Join(quoted, ", "))
}

// decodeTestStep decodes the interpolated content of a step, recording the raw and interpolated
// forms in tsResult; a decoding error is printed and stops the test case.
func (v *Venom) decodeTestStep(ctx context.Context, tc *TestCase, tsResult, tsIn *TestStepResult, rawStep json.RawMessage, content string, stepNumber int) (TestStep, error) {
	data, err := yamljson.Marshal(rawStep)
	if err != nil {
		tsResult.appendError(err)
		Error(ctx, "unable to marshal raw: %v", err)
	}
	tsResult.Raw = data

	var step TestStep
	if err := yamljson.Unmarshal([]byte(content), &step); err != nil {
		tsResult.appendError(err)
		Error(ctx, "unable to parse step #%d: %v", stepNumber, err)
		Error(ctx, content, nil)
		v.printTestStepResult(ctx, tc, tsResult, tsIn, stepNumber, false)
		return nil, err
	}

	data2, err := yamljson.JSONToYAML([]byte(content))
	if err != nil {
		tsResult.appendError(err)
		Error(ctx, "unable to marshal step #%d to json: %v", stepNumber, err)
	}
	tsResult.Interpolated = data2
	return step, nil
}

// materializeInputVars fills the InputVars of the steps of a root test case. The inner cases of custom executors keep
// theirs lazy because no report reads them, but InputVars is an exported field of the results a run hands back, so the
// ones that leave the engine are complete (#2475).
func (tc *TestCase) materializeInputVars() {
	for i := range tc.TestStepResults {
		tc.TestStepResults[i].materializeInputVars()
	}
}
