package venom

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ovh/venom/interpolate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stepUserExecutor builds a UserExecutor from its file content the way registerUserExecutors does: the name from the
// executor line, the raw input block, the whole content.
func stepUserExecutor(content string) UserExecutor {
	name := strings.TrimSpace(strings.Replace(readPartialYML([]byte(content), "executor"), "executor:", "", 1))
	return UserExecutor{
		Filename:  "lib/" + name + ".yml",
		Executor:  name,
		RawInputs: []byte(readPartialYML([]byte(content), "input")),
		Raw:       []byte(content),
	}
}

// stepUserRun is what one RunUserExecutor call returned and left in the caller's step result.
type stepUserRun struct {
	result interface{}
	err    error
	tsIn   *TestStepResult
	log    *stepEventLog
}

// stepRunUser runs the user executor for the step, from a test case "outer" of a suite with the given variables. Its
// inner steps use the fakes, registered as builtins and sharing one log.
func stepRunUser(t *testing.T, ux UserExecutor, step TestStep, suiteVars H, fakes ...*stepFakeExecutor) stepUserRun {
	t.Helper()
	v, c, log := stepUserVenom(t, nil, fakes...)
	r := stepCallUser(v, ux, stepUserCaller(suiteVars), step)
	r.log = log
	assert.Empty(t, c.String(), "the inner steps print nothing")
	return r
}

// stepUserVenom returns a Venom whose fakes, registered as builtins, share one log, and on which every user executor of
// uxs is registered as Parse registers them, so that a step of one can call another.
func stepUserVenom(t *testing.T, uxs []UserExecutor, fakes ...*stepFakeExecutor) (*Venom, *capture, *stepEventLog) {
	t.Helper()
	log := &stepEventLog{}
	for _, f := range fakes {
		f.log = log
	}
	v, c := stepNewVenom(t, 1, fakes...)
	for _, ux := range uxs {
		require.NoError(t, v.RegisterExecutorUser(ux.Executor, ux))
	}
	return v, c, log
}

// stepUserCaller is the test case "outer" a user executor step runs in, in a suite with the given variables.
func stepUserCaller(suiteVars H) *TestCase {
	return &TestCase{TestCaseInput: TestCaseInput{Name: "outer"}, originalName: "outer", TestSuiteVars: suiteVars}
}

// stepCallUser runs ux once for step from tcIn, into a fresh step result; the log is the caller's to fill in.
func stepCallUser(v *Venom, ux UserExecutor, tcIn *TestCase, step TestStep) stepUserRun {
	tsIn := &TestStepResult{}
	result, err := v.RunUserExecutor(context.Background(), newExecutorRunner(ux, ux.Executor, "user", 0, nil, 0, 0, nil), tcIn, tsIn, step)
	return stepUserRun{result: result, err: err, tsIn: tsIn}
}

const stepGreet = `executor: Greet
input:
  who: world
  empty: fallback
  missing: fallback
  venom.custom: raw
  input.leak: never
steps:
- type: fake
  label: "{{.input.who}}|{{.input.empty}}|{{.input.missing}}|{{.venom.custom}}|{{.input.other}}|{{.input.leak}}|{{.suite}}"
  out: 'say "hi" {{.input.who}}'
  vars:
    said:
      from: result.systemout
- type: fake
  label: "{{.venom.testcase}}|{{.venom.executor.name}}|{{.venom.executor.filename}}"
output:
  said: {{.said}}
  constant: plain
`

// TestRunUserExecutorInputs pins how the executor's inputs are filled: from the step when it gives a non-empty value,
// else the executor's default; "venom…" inputs are added under their own name, "input.…" inputs are dropped, and the
// step's keys that are not declared inputs are ignored. The inner test case is named after the slug of the executor.
func TestRunUserExecutorInputs(t *testing.T) {
	fake := stepNewFake("fake")
	r := stepRunUser(t, stepUserExecutor(stepGreet), TestStep{"type": "Greet", "who": "alice", "empty": "", "missing": nil, "other": "x"}, H{"suite": "s"}, fake)
	require.NoError(t, r.err)

	assert.Equal(t, []string{
		"setup:fake",
		"run:fake:alice|fallback|fallback|raw|{{.input.other}}|{{.input.leak}}|s:fake",
		"run:fake:greet|Greet|lib/Greet.yml:fake",
		"teardown:fake:fake",
	}, r.log.events())
	assert.Empty(t, r.tsIn.Errors)
}

// TestRunUserExecutorOutput pins the returned map: the dumped output — the nested "result" map itself included —
// where a computed variable holding quotes is escaped before interpolation and comes back intact, and an unquoted
// template is quoted before the YAML is parsed.
func TestRunUserExecutorOutput(t *testing.T) {
	r := stepRunUser(t, stepUserExecutor(stepGreet), TestStep{"who": "bob"}, H{}, stepNewFake("fake"))
	require.NoError(t, r.err)
	assert.Equal(t, map[string]interface{}{
		"__type__":        "Map",
		"__len__":         int64(1),
		"result":          map[string]interface{}{"constant": "plain", "said": `say "hi" bob`},
		"result.__Len__":  int64(2),
		"result.__Type__": "Map",
		"result.constant": "plain",
		"result.said":     `say "hi" bob`,
	}, r.result)
}

// TestRunUserExecutorJSONOutput pins the "json" twins: every output string that parses as JSON gets a <key>json entry
// and its dump. An object's dump is prefixed with <key>json. (without its __Len__ / __Type__); an array's with
// <key>json.<last segment of key>json, glued to the dumped key without a dot.
//
// quirk: a JSON scalar — here the string "42" — gets a twin too, plus a dump whose only key ends with a dot.
func TestRunUserExecutorJSONOutput(t *testing.T) {
	ux := stepUserExecutor(`executor: shapes
steps:
- type: fake
  out: '{"k":"v"}'
  vars:
    obj:
      from: result.systemout
- type: fake
  out: '["a"]'
  vars:
    arr:
      from: result.systemout
output:
  obj: "{{.obj}}"
  arr: "{{.arr}}"
  num: "42"
`)
	r := stepRunUser(t, ux, TestStep{}, H{}, stepNewFake("fake"))
	require.NoError(t, r.err)
	got, ok := r.result.(map[string]interface{})
	require.True(t, ok, "the result is a map")
	assert.Equal(t, map[string]interface{}{
		"__type__": "Map", "__len__": int64(1), "result.__Len__": int64(3), "result.__Type__": "Map",
		"result":     map[string]interface{}{"obj": `{"k":"v"}`, "arr": `["a"]`, "num": "42"},
		"result.obj": `{"k":"v"}`, "result.arr": `["a"]`, "result.num": "42",
		"result.objjson": map[string]interface{}{"k": "v"}, "result.objjson.k": "v",
		"result.arrjson": []interface{}{"a"}, "result.arrjson.arrjson0": "a", "result.arrjson.arrjson__len__": 1, "result.arrjson.arrjson__type__": "Array",
		"result.numjson": json.Number("42"), "result.numjson.": json.Number("42"),
	}, got)
}

// TestRunUserExecutorInnerFailure pins how the inner steps report back: their failures and info lines are appended to
// the caller's result, their systemout / systemerr only when not blank, and the call returns the raw output (not the
// dumped map) with an error.
func TestRunUserExecutorInnerFailure(t *testing.T) {
	ux := stepUserExecutor(`executor: failing
steps:
- type: fake
  code: 1
  out: partial
  err: oops
  info: "code {{.result.code}}"
  assertions: [result.code ShouldEqual 0]
- type: fake
output:
  x: y
`)
	r := stepRunUser(t, ux, TestStep{}, H{}, stepNewFake("fake"))
	require.EqualError(t, r.err, `executor "failing" failed`)
	assert.Equal(t, map[string]interface{}{"result": map[string]interface{}{"x": "y"}}, r.result)
	assert.Equal(t, []string{`Testcase "failing", step #1-0: Assertion "result.code ShouldEqual 0" failed. expected: 0  got: 1 (:0)`}, stepErrorValues(r.tsIn.Errors))
	assert.Equal(t, []string{"code 1"}, r.tsIn.ComputedInfo)
	assert.Equal(t, "partial\n", r.tsIn.Systemout)
	assert.Equal(t, "oops\n", r.tsIn.Systemerr)
}

// TestRunUserExecutorErrors pins the error of each stage, wrapped with the stage and the executor name.
func TestRunUserExecutorErrors(t *testing.T) {
	tests := []struct {
		name    string
		ux      UserExecutor
		wantErr string
	}{
		{
			name:    "inputs that cannot be interpolated",
			ux:      UserExecutor{Executor: "broken", RawInputs: []byte("input:\n  a: \"{{ .x\"\n"), Raw: []byte("executor: broken\n")},
			wantErr: `unable to interpolate executor inputs "broken": invalid template format "input:` + "\n" + `  a: "{{ .x"` + "\n" + `": template: input:2: bad character U+0022 '"'`,
		},
		{
			name:    "inputs that are not YAML",
			ux:      UserExecutor{Executor: "broken", RawInputs: []byte("input: [unclosed"), Raw: []byte("executor: broken\n")},
			wantErr: "unable to unmarshal inputs for executor \"broken\" - raw interpolated:\ninput: [unclosed: error converting YAML to JSON: yaml: line 1: did not find expected ',' or ']'",
		},
		{
			name:    "an executor that cannot be interpolated",
			ux:      UserExecutor{Executor: "broken", Raw: []byte("executor: broken\nsteps:\n- label: \"{{ .x\"\n")},
			wantErr: `unable to interpolate executor "broken": invalid template format "executor: broken` + "\n" + `steps:` + "\n" + `- label: "{{ .x"` + "\n" + `": template: input:3: bad character U+0022 '"'`,
		},
		{
			name:    "an executor that is not YAML",
			ux:      UserExecutor{Executor: "broken", Raw: []byte("executor: broken\nsteps: [unclosed\n")},
			wantErr: "unable to unmarshal executor \"broken\" - raw interpolated :\nexecutor: broken\nsteps: [unclosed\n: error converting YAML to JSON: yaml: line 1: did not find expected ',' or ']'",
		},
		// quirk: only values holding a quote are escaped, so a computed variable ending in a backslash breaks the output
		{
			name: "an output that is no longer YAML once interpolated",
			ux: stepUserExecutor("executor: backslash\nsteps:\n- type: fake\n  out: 'a\\'\n  vars:\n    v:\n      from: result.systemout\n" +
				"output:\n  v: \"{{.v}}\"\n"),
			wantErr: "unable to unmarshal: error converting YAML to JSON: yaml: found unexpected end of stream",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := stepRunUser(t, tt.ux, TestStep{}, H{}, stepNewFake("fake"))
			require.EqualError(t, r.err, tt.wantErr)
			assert.Nil(t, r.result)
		})
	}

	t.Run("a runner whose executor is not a UserExecutor", func(t *testing.T) {
		v, _ := stepNewVenom(t, 0)
		_, err := v.RunUserExecutor(context.Background(), newExecutorRunner(stepNewFake("fake"), "fake", "user", 0, nil, 0, 0, nil), stepBareCase(), &TestStepResult{}, TestStep{})
		require.EqualError(t, err, "RunUserExecutor called with a runner whose executor is not a UserExecutor (got *venom.stepFakeExecutor)")
	})
}

// TestRunUserExecutorCallerState pins two ways the caller's state changes the outcome.
//
// quirk: any failure already in the caller's result fails the call, even when every inner step passes — so a user
// executor step that fails once can never pass on a retry.
//
// quirk: a caller without suite variables panics as soon as the executor declares an input (AddWithPrefix on a nil
// map); without inputs it runs.
func TestRunUserExecutorCallerState(t *testing.T) {
	passing := stepUserExecutor("executor: passing\ninput:\n  a: b\nsteps:\n- type: fake\noutput:\n  x: y\n")

	t.Run("an earlier failure in the caller's result fails the call", func(t *testing.T) {
		v, _ := stepNewVenom(t, 0, stepNewFake("fake"))
		tcIn := &TestCase{TestCaseInput: TestCaseInput{Name: "outer"}, TestSuiteVars: H{}}
		tsIn := &TestStepResult{Errors: []Failure{{Value: "earlier"}}}
		_, err := v.RunUserExecutor(context.Background(), newExecutorRunner(passing, "passing", "user", 0, nil, 0, 0, nil), tcIn, tsIn, TestStep{})
		require.EqualError(t, err, `executor "passing" failed`)
	})

	t.Run("a retried user executor step stays failed", func(t *testing.T) {
		fake := stepNewFake("fake")
		fake.results = []interface{}{stepResult(1, "", ""), stepResult(0, "", "")}
		v, _ := stepNewVenom(t, 0, fake)
		ux := stepUserExecutor("executor: flaky\nsteps:\n- type: fake\n  assertions: [result.code ShouldEqual 0]\noutput:\n  x: y\n")
		tc := &TestCase{TestCaseInput: TestCaseInput{Name: "outer"}, originalName: "outer", TestSuiteVars: H{}}
		ts := &TestStepResult{}
		v.RunTestStep(context.Background(), newExecutorRunner(ux, "flaky", "user", 1, nil, 0, 0, nil), tc, ts, 1, 0, TestStep{})

		runs, _, _ := fake.counts()
		assert.Equal(t, 2, runs, "the inner step passed on the second attempt")
		assert.Equal(t, []string{
			`Testcase "flaky", step #1-0: Assertion "result.code ShouldEqual 0" failed. expected: 0  got: 1 (:0)`,
			`Testcase "outer", step #1-0: executor "flaky" failed (:0)`,
		}, stepErrorValues(ts.Errors))
	})

	t.Run("no suite variables and an input", func(t *testing.T) {
		v, _ := stepNewVenom(t, 0, stepNewFake("fake"))
		tcIn := &TestCase{TestCaseInput: TestCaseInput{Name: "outer"}}
		assert.PanicsWithError(t, "assignment to entry in nil map", func() {
			_, _ = v.RunUserExecutor(context.Background(), newExecutorRunner(passing, "passing", "user", 0, nil, 0, 0, nil), tcIn, &TestStepResult{}, TestStep{}) //nolint:errcheck // the call panics
		})
	})

	t.Run("no suite variables and no input", func(t *testing.T) {
		v, _ := stepNewVenom(t, 0, stepNewFake("fake"))
		ux := stepUserExecutor("executor: plain\nsteps:\n- type: fake\noutput:\n  x: y\n")
		tcIn := &TestCase{TestCaseInput: TestCaseInput{Name: "outer"}}
		_, err := v.RunUserExecutor(context.Background(), newExecutorRunner(ux, "plain", "user", 0, nil, 0, 0, nil), tcIn, &TestStepResult{}, TestStep{})
		require.NoError(t, err)
	})
}

// stepChainSuiteVars are the suite variables of the nested-chain tests (#1980): every value is one a flatten or a
// substitution can get wrong, so a change to how the variables reach a level shows in the labels the fakes log.
func stepChainSuiteVars() H {
	return H{
		// Looks like an integer: read bare by a YAML or JSON decoder, it becomes 7 and loses its zeros.
		"num": "007",
		// Looks like a float in exponent notation.
		"exp": "1e3",
		// A double quote: the step loop escapes it before it decodes a step, the executor file is substituted raw.
		"quoted": `say "hi"`,
		// A newline: substituted into the JSON of a step, it ends up inside a quoted scalar.
		"multi": "line1\nline2",
		// Template syntax inside a value: it must come out as text, never be evaluated as a variable.
		"tmpl": "{{.not.a.var}}",
		// An empty value must substitute to nothing, not stay as the expression.
		"empty": "",
		// A decoded JSON body, small so the test stays fast: the shape of the 110 KB value of the benchmark, and the kind of
		// value #2017's cache keeps. Nothing else shares its prefix, so the step loop caches it.
		"resp": nestedBody(1, 2),
		// "body" and "body.status" share the prefix "body.", so #2017's cache keeps both out (sharesPrefixWithAnotherName).
		// They are chosen not to produce the same flat key: when they do, go-dump's map order picks the winner, which no
		// test can pin.
		"body":        map[string]interface{}{"code": "200", "items": nestedBody(1, 1)},
		"body.status": "ok",
		// An upper-case first letter: the executor file is substituted with DumpString, which knows it as "greeting", the
		// steps with DumpStringPreserveCase, which knows it as "Greeting" — the two key styles a cache must keep apart.
		"Greeting": "Hello",
	}
}

// stepChainC is the leaf of the chain: it reads its input, scalar and nested suite variables, and the number of
// variables its file is substituted with (__len__: the suite variables plus its input), all at file level.
const stepChainC = `executor: chainC
input:
  n: c-default
steps:
- type: fake
  label: 'C|{{.input.n}}|{{.num}}|{{.exp}}|[{{.empty}}]|{{.body.status}}|{{.body.code}}|{{.resp.resp1.name}}|{{.__len__}}'
  info: 'C got {{.input.n}}'
  out: 'c={{.input.n}}'
  vars:
    cOut:
      from: result.systemout
output:
  got: '{{.cOut}}'
`

// stepChainD has no input block, so its file is not substituted at all (no variables are flattened for it): every
// expression is left to the step loop, which substitutes with DumpStringPreserveCase and escapes double quotes.
const stepChainD = `executor: chainD
steps:
- type: fake
  label: 'D|{{.multi}}|{{.quoted}}|{{.tmpl}}|{{.Greeting}}|{{.greeting}}|{{.body.status}}|{{.body.code}}|{{.resp.resp0.name}}|{{.input.m}}|{{.__Len__}}'
  out: d
  vars:
    dOut:
      from: result.systemout
output:
  got: '{{.dOut}}'
`

// stepChainB is the middle level: it reads its inputs — two of them given bare by A, so read as numbers — calls C twice
// in a row with different inputs, then D, and returns what each returned.
const stepChainB = `executor: chainB
input:
  m: b-default
  k: k-default
  j: j-default
steps:
- type: fake
  label: 'B|{{.input.m}}|{{.input.k}}|{{.input.j}}|{{.quoted}}|{{.tmpl}}|{{.greeting}}|{{.Greeting}}'
  out: 'b={{.input.m}}'
  vars:
    bOut:
      from: result.systemout
- type: chainC
  n: '{{.bOut}}/first'
  vars:
    c1:
      from: result.got
- type: chainC
  n: second
  vars:
    c2:
      from: result.got
- type: chainD
  vars:
    d:
      from: result.got
output:
  b: '{{.bOut}}'
  c1: '{{.c1}}'
  c2: '{{.c2}}'
  d: '{{.d}}'
`

// stepChainA is the entry of the chain: it reads its input and the suite variables, then calls B with a value derived
// from its own step and two suite values substituted bare.
const stepChainA = `executor: chainA
input:
  who: a-default
steps:
- type: fake
  label: 'A|{{.input.who}}|{{.num}}|{{.exp}}|[{{.empty}}]|{{.tmpl}}|{{.body.status}}|{{.body.code}}|{{.resp.resp0.name}}|{{.greeting}}|{{.__len__}}'
  info: 'A got {{.input.who}}'
  out: 'a={{.input.who}}'
  vars:
    aOut:
      from: result.systemout
- type: chainB
  m: '{{.aOut}}'
  k: {{.num}}
  j: {{.exp}}
  vars:
    fromB:
      from: result.b
    fromC1:
      from: result.c1
    fromC2:
      from: result.c2
    fromD:
      from: result.d
output:
  a: '{{.aOut}}'
  b: '{{.fromB}}'
  c1: '{{.fromC1}}'
  c2: '{{.fromC2}}'
  d: '{{.fromD}}'
`

// stepNestedChain returns the user executors of the chain A → B → (C, C, D), and a Venom on which they are registered.
func stepNestedChain(t *testing.T) ([]UserExecutor, *Venom, *capture, *stepEventLog) {
	t.Helper()
	uxs := []UserExecutor{
		stepUserExecutor(stepChainA), stepUserExecutor(stepChainB), stepUserExecutor(stepChainC), stepUserExecutor(stepChainD),
	}
	v, c, log := stepUserVenom(t, uxs, stepNewFake("fake"))
	return uxs, v, c, log
}

// TestRunUserExecutorNestedChain pins a nested call A → B → C, a safety net for a future change that shares flattened variables across the chain (#2405):
// the variables every inner step received (the fake's log of labels), the step result the calls leave behind and the
// map the outermost call returns. A cache that served one level's or one call's variables to another would show here.
func TestRunUserExecutorNestedChain(t *testing.T) {
	uxs, v, c, log := stepNestedChain(t)
	r := stepCallUser(v, uxs[0], stepUserCaller(stepChainSuiteVars()), TestStep{"type": "chainA", "who": "outer-who"})
	require.NoError(t, r.err)
	assert.Empty(t, c.String(), "the inner steps print nothing")

	// Each inner test case sets the fake up again, in the context its caller's fake left: the setup chain grows by one
	// "fake" per level. Values read in the executor file (A, B, C) come from DumpString, those D reads from the step loop.
	assert.Equal(t, []string{
		"setup:fake",
		// __len__ is the number of variables the file is substituted with: the ten suite variables and the one input.
		"run:fake:A|outer-who|007|1e3|[]|{{.not.a.var}}|ok|200|item-0|Hello|11:fake",
		"setup:fake",
		// quirk: A gives k and j bare, so YAML reads 007 as 7 and 1e3 as 1000 before B sees them.
		`run:fake:B|a=outer-who|7|1000|say "hi"|{{.not.a.var}}|Hello|Hello:fakefake`,
		"setup:fake",
		"run:fake:C|b=a=outer-who/first|007|1e3|[]|ok|200|item-1|11:fakefakefake",
		"teardown:fake:fakefakefake",
		"setup:fake",
		// The second call of C in a row sees its own input, not the first call's.
		"run:fake:C|second|007|1e3|[]|ok|200|item-1|11:fakefakefake",
		"teardown:fake:fakefakefake",
		"setup:fake",
		// The step loop knows the upper-case name only as written ({{.greeting}} stays), the newline is folded into a space
		// by the decoder, B's input does not reach D, and __Len__ counts the suite variables plus venom.testcase,
		// venom.executor.filename, venom.executor.name and venom.teststep.number.
		`run:fake:D|line1 line2|say "hi"|{{.not.a.var}}|Hello|{{.greeting}}|ok|200|item-0|{{.input.m}}|14:fakefakefake`,
		"teardown:fake:fakefakefake",
		"teardown:fake:fakefake",
		"teardown:fake:fake",
	}, log.events())

	assert.Equal(t, map[string]interface{}{
		"__type__": "Map", "__len__": int64(1), "result.__Len__": int64(5), "result.__Type__": "Map",
		"result": map[string]interface{}{
			"a": "a=outer-who", "b": "b=a=outer-who", "c1": "c=b=a=outer-who/first", "c2": "c=second", "d": "d",
		},
		"result.a": "a=outer-who", "result.b": "b=a=outer-who", "result.c1": "c=b=a=outer-who/first", "result.c2": "c=second",
		"result.d": "d",
	}, r.result)

	// Every level's inner results are merged into its caller's, so the outermost step result holds them all; a user
	// executor step adds a blank line of its own after its inner output.
	assert.Equal(t, "a=outer-who\nb=a=outer-who\nc=b=a=outer-who/first\n\nc=second\n\nd\n\n\n", r.tsIn.Systemout)
	assert.Empty(t, r.tsIn.Systemerr)
	assert.Equal(t, []string{"A got outer-who", "C got b=a=outer-who/first", "C got second"}, r.tsIn.ComputedInfo)
	assert.Empty(t, r.tsIn.Errors)
}

// TestRunUserExecutorTwiceFromOneTestCase pins two calls in a row of one user executor from the same test case, with
// different inputs: the second call sees its own input and returns its own output, and neither call changes the
// caller's suite variables — the inputs are added to a copy. A cache keyed on the caller would get this wrong first.
func TestRunUserExecutorTwiceFromOneTestCase(t *testing.T) {
	uxs, v, _, log := stepNestedChain(t)
	tcIn := stepUserCaller(stepChainSuiteVars())

	first := stepCallUser(v, uxs[2], tcIn, TestStep{"type": "chainC", "n": "call-1"})
	require.NoError(t, first.err)
	second := stepCallUser(v, uxs[2], tcIn, TestStep{"type": "chainC", "n": "call-2"})
	require.NoError(t, second.err)

	assert.Equal(t, []string{
		"setup:fake", "run:fake:C|call-1|007|1e3|[]|ok|200|item-1|11:fake", "teardown:fake:fake",
		"setup:fake", "run:fake:C|call-2|007|1e3|[]|ok|200|item-1|11:fake", "teardown:fake:fake",
	}, log.events())
	assert.Equal(t, "c=call-1", stepChainOutput(t, first))
	assert.Equal(t, "c=call-2", stepChainOutput(t, second))
	assert.Equal(t, []string{"C got call-2"}, second.tsIn.ComputedInfo, "each call fills its own step result")
	assert.Equal(t, stepChainSuiteVars(), tcIn.TestSuiteVars, "the caller's suite variables are left as they were")
}

// stepChainOutput returns the "got" output of a call of chainC.
func stepChainOutput(t *testing.T, r stepUserRun) interface{} {
	t.Helper()
	result, ok := r.result.(map[string]interface{})
	require.True(t, ok, "the result is a map")
	return result["result.got"]
}

// failDump makes the dump of an executor result fail for the test: a Dump error is otherwise unreachable.
func failDump(t *testing.T, err error) {
	t.Helper()
	previous := dumpExecutorResult
	dumpExecutorResult = func(interface{}) (map[string]interface{}, error) { return nil, err }
	t.Cleanup(func() { dumpExecutorResult = previous })
}

// TestGetExecutorResultDumpError pins the exported function kept for the plugins: it still panics on a Dump error,
// and the unexported one the engine uses returns the error instead (#2918).
func TestGetExecutorResultDumpError(t *testing.T) {
	failDump(t, errors.New("dump boom"))

	assert.PanicsWithError(t, "dump boom", func() { GetExecutorResult(stepResult(0, "", "")) })

	got, err := getExecutorResult(stepResult(0, "", ""))
	require.EqualError(t, err, "dump boom")
	assert.Nil(t, got)
}

// TestRunTestStepDumpErrorFailsTheStep: a result that cannot be dumped fails the step with that error and does not
// panic, whatever the retries (#2918).
func TestRunTestStepDumpErrorFailsTheStep(t *testing.T) {
	v, _ := stepNewVenom(t, 0)
	failDump(t, errors.New("dump boom"))
	ts := &TestStepResult{}

	require.NotPanics(t, func() {
		v.RunTestStep(context.Background(), newExecutorRunner(stepNewFake("fake"), "fake", "builtin", 0, nil, 0, 0, nil), stepBareCase(), ts, 1, 0,
			TestStep{"code": 0, "assertions": []interface{}{stepAssertCode0}})
	})
	require.Len(t, ts.Errors, 1)
	assert.Contains(t, ts.Errors[0].Value, "dump boom")
}

// TestApplyAssertionsDumpError: applyAssertions returns the dump error as a failed assertion result (#2918).
func TestApplyAssertionsDumpError(t *testing.T) {
	InitTestLogger(t)
	failDump(t, errors.New("dump boom"))

	var res AssertionsApplied
	require.NotPanics(t, func() {
		res = applyAssertions(context.Background(), stepResult(0, "", ""), TestCase{}, 1, 0, TestStep{"assertions": []interface{}{stepAssertCode0}}, nil)
	})
	assert.False(t, res.OK)
	require.Len(t, res.errors, 1)
	assert.Contains(t, res.errors[0].Value, "dump boom")
}

// an action with a format verb that reads a variable the executor does not have is kept for the step pass, with its
// strings as raw strings so the JSON of the step holds no escaped quote (#3305)
func TestPrepareUserExecutorKeepsPrintfForTheStepPass(t *testing.T) {
	const body = `executor: Printf
steps:
- type: fake
  script: echo '{{ printf "%s" .testvar }}'
`
	call, err := prepareUserExecutor(stepUserExecutor(body), TestStep{"type": "Printf"}, stepUserCaller(H{"testvar": "hello"}))
	require.NoError(t, err)
	require.Len(t, call.tc.RawTestSteps, 1)
	assert.NotContains(t, string(call.tc.RawTestSteps[0]), `\"`)

	stepPass, err := interpolate.Do(string(call.tc.RawTestSteps[0]), map[string]string{"testvar": "hello"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"fake","script":"echo 'hello'"}`, stepPass)
}

// a body is left as it is, whole, unless protecting it is safe
func TestProtectLaterActions(t *testing.T) {
	none := func(string) bool { return false }
	known := func(name string) bool { return name == "known" }
	for name, tc := range map[string]struct {
		body, want string
		has        func(string) bool
	}{
		"no action":                   {"echo hi", "echo hi", none},
		"known variable renders":      {`{{ printf "%s" .known }}`, `{{ printf "%s" .known }}`, known},
		"unknown is protected":        {`a {{ printf "%s" .nope }} b`, "a {{\"{{ printf `%s` .nope }}\"}} b", known},
		"readable by interpolate":     {`{{ .nope }}`, `{{ .nope }}`, none},
		"default stays":               {`{{ .nope | default "a,b" }}`, `{{ .nope | default "a,b" }}`, none},
		"ternary stays":               {`{{ printf "%s" (ternary "a" "b" .nope) }}`, `{{ printf "%s" (ternary "a" "b" .nope) }}`, none},
		"template variable, all":      {`{{ $x := .nope }}{{ $x }} {{ printf "%s" .nope }}`, `{{ $x := .nope }}{{ $x }} {{ printf "%s" .nope }}`, none},
		"block, all":                  {`{{ if .nope }}x{{ end }} {{ printf "%s" .nope }}`, `{{ if .nope }}x{{ end }} {{ printf "%s" .nope }}`, none},
		"does not parse":              {`{{ printf "%s" .nope`, `{{ printf "%s" .nope`, none},
		"escape in a literal stays":   {`{{ printf "a\tb %s" .nope }}`, `{{ printf "a\tb %s" .nope }}`, none},
		"trim marker stays":           {`{{- printf "%s" .nope }}`, `{{- printf "%s" .nope }}`, none},
		"backtick literal is kept":    {"{{ printf `a` .nope }}", "{{\"{{ printf `a` .nope }}\"}}", none},
		"action over two lines stays": {"{{ printf \"%s\"\n .nope }}", "{{ printf \"%s\"\n .nope }}", none},
		"quote stays":                 {`{{ printf "%s" .nope | quote }}`, `{{ printf "%s" .nope | quote }}`, none},
		"toJSON stays":                {`{{ printf "%s" .nope | toJSON }}`, `{{ printf "%s" .nope | toJSON }}`, none},
		"toPrettyJSON stays":          {`{{ printf "%s" .nope | toPrettyJSON }}`, `{{ printf "%s" .nope | toPrettyJSON }}`, none},
		"quoting verb stays":          {`{{ printf "%q" .nope }}`, `{{ printf "%q" .nope }}`, none},
		"squote is protected":         {`{{ printf "%s" .nope | squote }}`, "{{\"{{ printf `%s` .nope | squote }}\"}}", none},
	} {
		assert.Equal(t, tc.want, protectLaterActions(tc.body, tc.has), name)
	}
}
