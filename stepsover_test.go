package venom

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The inner test case of a custom executor holds only its own variables over the prepared suite (#2475, ADR-0011). That
// is an economy, never a different answer: the same call on a root case without a prepared suite, which holds the whole
// scope as before, must give the same steps, outputs, errors and computed variables. These tests run both.

const (
	overFoo   = "input.foo"
	overFooEx = "fromsuite"
	overCustm = "venom.custom"
)

// overExecutor reads the executor's inputs and the suite's variables in every way the layered scope has to answer: a
// template, an assignment from a suite variable and from an input, a skip clause, and a range that is templated. The
// executor's own text is interpolated first with the suite's variables, the first segment of their names in lower case
// (processUserExecutorInputs), so the step reads the capitalised ones (Upper, Rng) itself.
const overExecutor = `executor: Over
input:
  foo: default
  num: 1
  venom.custom: raw
steps:
- type: fake
  label: '{{.input.foo}}|{{.input.num}}|{{.venom.custom}}|{{.plain}}|{{.quoted}}|{{.nested.k0.k0}}|{{.absent}}|{{.venom.testcase}}|{{.Upper}}|{{.venom.teststep.number}}'
  out: 'say "hi" {{.input.foo}}'
  vars:
    said:
      from: result.systemout
    fromsuite:
      from: plain
    frominput:
      from: input.foo
- type: fake
  label: '{{.said}}|{{.fromsuite}}|{{.frominput}}|{{.Over.said}}|{{.Upper}}'
  skip: [plain ShouldEqual nope]
- type: fake
  range: '{{.Rng}}'
  label: "{{.key}}={{.value}}"
output:
  said: '{{.said}}'
`

// innerStep is what a step of the inner test case reports.
type innerStep struct {
	Name     string
	Status   Status
	Input    map[string]string
	Computed H
	Errors   []string
}

// innerOutcome is everything a call of a custom executor leaves behind.
type innerOutcome struct {
	events    []string
	setupVars []H
	vars      H
	steps     []innerStep
	computed  H
	result    interface{}
	err       string
	over      bool
}

// runInner runs ux for step from a root case over suite, with the prepared scope of the suite or without it.
func runInner(t *testing.T, ux UserExecutor, step TestStep, suite H, prepared bool, strict bool) innerOutcome {
	t.Helper()
	fake := stepNewFake("fake")
	v, _, log := stepUserVenom(t, nil, fake)
	v.StrictVariables = strict
	tcIn := stepUserCaller(suite)
	if prepared {
		tcIn.suite = newSuiteScope(suite)
	}
	call, err := prepareUserExecutor(ux, step, tcIn)
	require.NoError(t, err)
	tc := call.tc
	out := innerOutcome{over: tc.varsOverSuite, vars: tc.fullVars(tc.Vars)}

	tsIn := &TestStepResult{}
	v.runTestSteps(context.Background(), tc, tsIn)
	mergeUserExecutorResults(tsIn, tc.TestStepResults)
	ux.Output = call.output
	result, err := userExecutorResult(context.Background(), ux, tc, tsIn, call.sanitized)
	if err != nil {
		out.err = err.Error()
	}
	out.result, out.computed, out.events = result, tc.computedVars, log.events()
	out.setupVars = fake.setupVars
	for i := range tc.TestStepResults {
		res := &tc.TestStepResults[i]
		res.materializeInputVars()
		out.steps = append(out.steps, innerStep{res.Name, res.Status, res.InputVars, res.ComputedVars, stepErrorValues(res.Errors)})
	}
	return out
}

// overSuites are the suites a custom executor runs over, each with whether its inner case holds only its own variables.
var overSuites = []struct {
	name     string
	extra    H
	wantOver bool
	late     H // set after the suite was initialised, as a template left in a value
}{
	{"plain", H{"Rng": `["a","b"]`}, true, nil},
	{"input replaces a suite string", H{"Rng": `["a"]`, overFoo: overFooEx}, true, nil},
	{"venom input replaces a suite string", H{"Rng": `["a"]`, overCustm: "suite-custom"}, true, nil},
	{"input replaces a suite number", H{"Rng": `["a"]`, "input.num": 7}, false, nil},
	{"input replaces a suite map", H{"Rng": `["a"]`, "input": H{"foo": "m", "x": H{"y": "z"}}}, true, nil},
	{"a suite string shares a prefix", H{"Rng": `["a"]`, "plain.sub": "x"}, true, nil},
	{"a suite key with a space", H{"Rng": `["a"]`, "a key": "v"}, true, nil},
	{"a suite variable holds quotes", H{"Rng": `["a"]`, "q2": `a "b" \ c`}, true, nil},
	{"a templated suite value", H{"Rng": `["a"]`}, false, H{"tpl": "{{.plain}}-x"}},
}

func TestInnerCaseOverTheSuiteMatchesTheWholeScope(t *testing.T) {
	ux := stepUserExecutor(overExecutor)
	for _, sc := range overSuites {
		for _, strict := range []bool{false, true} {
			t.Run(sc.name, func(t *testing.T) {
				extra := sc.extra.Clone()
				extra["Upper"] = "UP"
				suite := suiteScopeFixture(t, extra)
				suite.AddAll(sc.late)
				step := TestStep{"type": "Over", "foo": "from step"}
				want := runInner(t, ux, step, suite, false, strict)
				got := runInner(t, ux, step, suite, true, strict)
				assert.False(t, want.over)
				assert.Equal(t, sc.wantOver, got.over, "the inner case is held over the suite")
				if !reflect.DeepEqual(want.vars, got.vars) {
					t.Errorf("whole scope differs: %d and %d variables", len(want.vars), len(got.vars))
				}
				want.over, got.over = false, false
				assert.Equal(t, want, got)
				assert.NotEmpty(t, got.steps)
			})
		}
	}
}

// the executor holds only its own variables when the suite is prepared, and the whole scope when an input changes a suite
// variable the prepared dump cannot follow
func TestInnerCaseHoldsOnlyItsOwnVariables(t *testing.T) {
	ux := stepUserExecutor(overExecutor)
	for name, want := range map[string]bool{"plain": true, "input replaces a suite number": false, "a templated suite value": false} {
		var extra, late H
		for _, sc := range overSuites {
			if sc.name == name {
				extra, late = sc.extra, sc.late
			}
		}
		suite := suiteScopeFixture(t, extra)
		suite.AddAll(late)
		call, err := prepareUserExecutor(ux, TestStep{"type": "Over"}, &TestCase{TestSuiteVars: suite, suite: newSuiteScope(suite)})
		require.NoError(t, err)
		assert.Equal(t, want, call.tc.varsOverSuite, name)
		if want {
			assert.Less(t, len(call.tc.Vars), 10, name)
			assert.Greater(t, len(call.tc.fullVars(call.tc.Vars)), len(suite), name)
		}
	}
}

// the whole scope of an inner case is the suite's variables with its own on top, the map the case held before
func TestInnerCaseFullVars(t *testing.T) {
	suite := suiteScopeFixture(t, H{overFoo: overFooEx})
	tc := &TestCase{TestSuiteVars: suite, suite: newSuiteScope(suite), varsOverSuite: true,
		TestCaseInput: TestCaseInput{Vars: H{overFoo: "own", "venom.testcase": "inner"}}}
	want := suite.Clone()
	want[overFoo], want["venom.testcase"] = "own", "inner"
	assert.Equal(t, want, tc.fullVars(tc.Vars))
	assert.Equal(t, overFooEx, suite[overFoo], "the suite's variables are not changed")
	assert.Len(t, tc.Vars, 2, "nor is the case's own")

	whole := &TestCase{TestCaseInput: TestCaseInput{Vars: want}}
	whole.fullVars(whole.Vars)["added"] = "x"
	assert.Contains(t, whole.Vars, "added", "an ordinary case returns the map it holds")

	v, ok := tc.lookupVar("plain")
	assert.True(t, ok)
	assert.Equal(t, "value", v)
	v, _ = tc.lookupVar(overFoo)
	assert.Equal(t, "own", v)
	_, ok = tc.lookupVar("absent")
	assert.False(t, ok)
}

// a custom executor that calls another, which replaces a suite string with an input, gives the same answers whether or
// not the suite is prepared
func TestNestedExecutorsOverTheSuite(t *testing.T) {
	outer := stepUserExecutor(`executor: OverOuter
input:
  foo: outer
steps:
- type: fake
  label: "outer {{.input.foo}} {{.plain}} {{.input.nested}}"
- type: OverInner
  foo: '{{.input.foo}}-in'
output:
  got: '{{.got}}'
`)
	inner := stepUserExecutor(`executor: OverInner
input:
  foo: inner
steps:
- type: fake
  label: "inner {{.input.foo}} {{.plain}}"
  out: '{{.input.foo}}'
  vars:
    got:
      from: result.systemout
output:
  got: '{{.got}}'
`)
	for _, extra := range []H{{"input.nested": "n"}, {overFoo: overFooEx, "input.nested": "n"}} {
		suite := suiteScopeFixture(t, extra)
		var outcomes [2]stepUserRun
		for i, prepared := range []bool{false, true} {
			fake := stepNewFake("fake")
			v, _, log := stepUserVenom(t, []UserExecutor{outer, inner}, fake)
			tcIn := stepUserCaller(suite)
			if prepared {
				tcIn.suite = newSuiteScope(suite)
			}
			outcomes[i] = stepCallUser(v, outer, tcIn, TestStep{"type": "OverOuter", "foo": "top"})
			outcomes[i].log = log
		}
		require.NoError(t, outcomes[0].err)
		assert.Equal(t, outcomes[0].err, outcomes[1].err)
		assert.Equal(t, outcomes[0].result, outcomes[1].result)
		assert.Equal(t, outcomes[0].log.events(), outcomes[1].log.events())
		assert.Equal(t, outcomes[0].tsIn.Errors, outcomes[1].tsIn.Errors)
		assert.Contains(t, outcomes[1].log.events(), "run:fake:inner top-in value:fakefake")
	}
}

// InputVars is built when the result is reported: once, and each copy of a result gets a map of its own
func TestLazyInputVarsAreMaterializedOnceAndPerCopy(t *testing.T) {
	suite := suiteScopeFixture(t, nil)
	scope := layeredScope(t, suite, suiteStepVars(suite, H{"own": `a "b"`}))
	want := scope.Materialize()

	res := TestStepResult{lazyInputs: scope.inputLayers()}
	assert.Nil(t, res.InputVars, "nothing is built before the result is reported")
	other := res
	res.materializeInputVars()
	other.materializeInputVars()
	assert.Equal(t, want, res.InputVars)
	assert.Equal(t, want, other.InputVars)
	assert.Nil(t, res.lazyInputs)

	res.InputVars["own"] = "__hidden__"
	assert.Equal(t, want["own"], other.InputVars["own"], "a copy of the result does not share the map")
	res.materializeInputVars()
	assert.Equal(t, "__hidden__", res.InputVars["own"], "a second call leaves the map as it is")
}

// the reports redact the variables of a result that was never materialised, as they redact the others
func TestCleanUpSecretsMaterializesInputVars(t *testing.T) {
	suite := suiteScopeFixture(t, H{"basic_auth_password": "my_secret"})
	scope := layeredScope(t, suite, suiteStepVars(suite, H{"basic_auth_user": "testuser"}))
	ts := TestSuite{
		Name: "suite", Secrets: []string{"basic_auth_password"}, Vars: suite.Clone(),
		TestCases: []TestCase{{TestStepResults: []TestStepResult{{Name: "step", lazyInputs: scope.inputLayers()}}}},
	}
	cleaned := New().CleanUpSecrets(ts)
	res := cleaned.TestCases[0].TestStepResults[0]
	assert.Equal(t, "__hidden__", res.InputVars["basic_auth_password"])
	assert.Equal(t, "testuser", res.InputVars["basic_auth_user"])
	assert.Equal(t, "value", res.InputVars[plainVar])
	assert.Nil(t, res.lazyInputs)
}

// TestMaterializeInputVarsFillsEveryStepOfACase pins that a root test case hands its results back with InputVars filled.
func TestMaterializeInputVarsFillsEveryStepOfACase(t *testing.T) {
	layers := newWholeScope(map[string]string{"who": "tester"}, map[string]string{"who": "tester"})
	tc := &TestCase{TestStepResults: []TestStepResult{{lazyInputs: &layers.escaped}, {}}}
	tc.materializeInputVars()
	assert.Equal(t, map[string]string{"who": "tester"}, tc.TestStepResults[0].InputVars)
	assert.Nil(t, tc.TestStepResults[0].lazyInputs)
	assert.Nil(t, tc.TestStepResults[1].InputVars)
}
