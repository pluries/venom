package venom

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/ovh/venom/internal/yamljson"
	"github.com/ovh/venom/interpolate"
	"github.com/pkg/errors"
)

var varRegEx = regexp.MustCompile("{{.*}}")

// Parse the testcase to find unreplaced and extracted variables
func (v *Venom) parseTestCase(_ *TestSuite, tc *TestCase) ([]string, []string, error) {
	dvars, err := DumpStringPreserveCase(tc.Vars)
	if err != nil {
		return nil, nil, err
	}

	vars := []string{}
	extractedVars := []string{}

	// the value of each var can contain a double-quote -> "
	// if the value is not escaped, it will be used as is, and the json sent to unmarshall will be incorrect.
	// This also avoids injections into the json structure of a step
	// Use strconv.Quote for proper escaping to avoid double-escaping issues
	for i := range dvars {
		dvars[i] = escapeQuotes(dvars[i])
	}
	for _, rawStep := range tc.RawTestSteps {
		step, err := interpolateStep(rawStep, dvars)
		if err != nil {
			return nil, nil, err
		}

		_, exec, err := v.GetExecutorRunner(context.Background(), step, tc.Vars)
		if err != nil {
			return nil, nil, err
		}

		extractedVars, err = appendResultVars(extractedVars, vars, tc.Name, exec.ZeroValueResult())
		if err != nil {
			return nil, nil, err
		}

		vars, extractedVars, err = appendStepVars(vars, extractedVars, tc.Name, step)
		if err != nil {
			return nil, nil, err
		}
	}
	return vars, extractedVars, nil
}

// interpolateStep interpolates a raw step with the testcase vars and unmarshals it.
func interpolateStep(rawStep json.RawMessage, dvars map[string]string) (TestStep, error) {
	content, err := interpolate.Do(string(rawStep), dvars)
	if err != nil {
		return nil, err
	}

	var step TestStep
	if err := yamljson.Unmarshal([]byte(content), &step); err != nil {
		return nil, errors.Wrapf(err, "unable to unmarshal teststep")
	}
	return step, nil
}

// appendResultVars adds the vars a step extracts from its executor's result, read from the zero-value result.
func appendResultVars(extractedVars, vars []string, tcName string, defaultResult interface{}) ([]string, error) {
	if defaultResult == nil {
		return extractedVars, nil
	}
	dumpE, err := DumpString(defaultResult)
	if err != nil {
		return nil, err
	}
	for k := range dumpE {
		var found bool
		for i := 0; i < len(vars); i++ {
			if vars[i] == k {
				found = true
				break
			}
		}
		if !found {
			extractedVars = append(extractedVars, k)
		}
		extractedVars = append(extractedVars, tcName+"."+k)
		if strings.HasSuffix(k, "__Type__") && dumpE[k] == "Map" {
			// go-dump doesn't dump the map name, here is a workaround
			k = strings.TrimSuffix(k, "__Type__")
			extractedVars = append(extractedVars, tcName+"."+k)
		}
	}
	return extractedVars, nil
}

// appendStepVars adds the vars a step defines (vars, extracts) and the templated vars it uses from its dump.
func appendStepVars(vars, extractedVars []string, tcName string, step TestStep) ([]string, []string, error) {
	dumpE, err := DumpStringPreserveCase(step)
	if err != nil {
		return nil, nil, err
	}

	for k, v := range dumpE {
		if strings.HasPrefix(k, "vars.") {
			s := tcName + "." + strings.Split(k[5:], ".")[0]
			extractedVars = append(extractedVars, s)
			continue
		}
		if strings.HasPrefix(k, "range.") {
			continue
		}
		if strings.HasPrefix(k, "extracts.") {
			s := tcName + "." + strings.Split(k[9:], ".")[0]
			extractedVars = append(extractedVars, s)
			continue
		}
		if strings.HasPrefix(k, "info") {
			continue
		}
		if varRegEx.MatchString(v) {
			vars = appendUsedVars(vars, extractedVars, k, v)
		}
	}
	return vars, extractedVars, nil
}

// appendUsedVars adds the templated vars of a step value that no earlier step extracts.
func appendUsedVars(vars, extractedVars []string, k, v string) []string {
	var found bool
	for i := 0; i < len(vars); i++ {
		if vars[i] == k {
			found = true
			break
		}
	}

	// varRegEx has no capture group: submatches holds only index 0, so nothing is added here (kept as is, #2031 D28).
	submatches := varRegEx.FindStringSubmatch(v)
	for submatcheIndex, s := range submatches {
		if submatcheIndex == 0 {
			continue
		}
		for i := 0; i < len(extractedVars); i++ {
			prefix := "{{." + extractedVars[i]
			if strings.HasPrefix(s, prefix) {
				found = true
				break
			}
		}
		if !found {
			vars = append(vars, s)

			s = strings.ReplaceAll(s, "{{ .", "")
			s = strings.ReplaceAll(s, "{{.", "")
			s = strings.ReplaceAll(s, "}}", "")
			vars = append(vars, s)
		}
	}
	return vars
}

func (v *Venom) runTestCase(ctx context.Context, ts *TestSuite, tc *TestCase) {
	ctx = context.WithValue(ctx, ContextKey("testcase"), tc.Name)

	tc.TestSuiteVars = ts.Vars.Clone()
	tc.suite = newSuiteScope(tc.TestSuiteVars)
	tc.Vars = ts.Vars.Clone()
	tc.Vars.Add("venom.testcase", tc.Name)
	tc.Vars.AddAll(ts.ComputedVars)
	tc.Vars.Add("venom.testcase.totalSteps", len(tc.RawTestSteps))
	tc.computedVars = H{}

	ctx = v.processSecrets(ctx, ts, tc)

	Info(ctx, "Starting testcase")

	defer Info(ctx, "Ending testcase")
	// ##### RUN Test Steps Here
	v.runTestSteps(ctx, tc, nil)
	tc.materializeInputVars()
}

func (v *Venom) processSecrets(ctx context.Context, ts *TestSuite, tc *TestCase) context.Context {
	computedSecrets := []string{}
	seen := map[string]struct{}{}
	collect := func(vars H) {
		for k, val := range vars {
			for _, s := range ts.Secrets {
				if k == s {
					secretVal := fmt.Sprint(val)
					if _, ok := seen[secretVal]; !ok {
						seen[secretVal] = struct{}{}
						computedSecrets = append(computedSecrets, secretVal)
					}
				}
			}
		}
	}
	collect(ts.Vars)
	if tc != nil {
		collect(tc.Vars)
	}

	// Derived secrets (e.g. base64 basic-auth tokens) must be computed from the
	// effective vars, which may only define the secret at the test case level.
	derivedVars := ts.Vars.Clone()
	if derivedVars == nil {
		derivedVars = H{}
	}
	if tc != nil {
		derivedVars.AddAll(tc.Vars)
	}
	computedSecrets = appendDerivedSecrets(computedSecrets, seen, derivedVars, ts.Secrets)
	return context.WithValue(ctx, ContextKey("secrets"), computedSecrets)
}

// interpolateStepContent renders rawStep by substituting vars. Neither rawStep nor vars changes
// between calls, and interpolate.Do only reads vars — it never mutates them or feeds a call's
// output back into the next one. A repeated call can therefore never resolve more than the first
// one did: the previous code called it up to ten times against the same unchanged input and vars,
// wastefully — not because interpolate.Do is always deterministic in general (it isn't:
// InterpolateHelperFuncs exposes randAlpha and friends), but because nothing here gives a later
// call more information than the first one already had (#1981).
func interpolateStepContent(rawStep []byte, scope *stepScope) (string, error) {
	return scope.Interpolate(string(rawStep))
}
