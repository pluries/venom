package venom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stepSetupKey is the context key the fake's Setup writes: the names of the fakes set up so far, in order. Run and
// TearDown read it back, which pins which context each of them receives.
const stepSetupKey = ContextKey("step.fake.setup")

// stepEventLog records, in order, what the fake executors were asked to do. Several fakes share one log so the
// relative order of their Setup, Run and TearDown calls can be asserted.
type stepEventLog struct {
	mu   sync.Mutex
	list []string
}

func (l *stepEventLog) add(format string, a ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list = append(l.list, fmt.Sprintf(format, a...))
}

func (l *stepEventLog) events() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.list...)
}

// stepFakeExecutor is the executor of the #2031 step-level characterisation tests. It implements Executor and
// ExecutorWithSetup, logs every call as "setup:<name>", "run:<name>:<label>:<setup chain>" and
// "teardown:<name>:<setup chain>", and can be told to fail any of them.
//
// Without configured results, Run returns stepResult built from the step's "code", "out" and "err" fields, so a YAML
// step decides its own outcome. The real executors live in package executors, which imports venom, so they cannot be
// used from a test inside this package.
type stepFakeExecutor struct {
	name        string
	log         *stepEventLog
	setupErr    error
	teardownErr error
	runErrs     []error           // runErrs[i] is returned by the i-th Run; nil or out of range: no error
	results     []interface{}     // results[i] is returned by the i-th Run, the last one beyond; empty: built from the step
	nilResult   bool              // Run returns a nil result
	holdFirst   bool              // the first Run waits for its context to be done and returns the context's error
	held        chan error        // when set, the held first Run also sends its context's error there; buffered, so it never blocks
	onRun       func(n int) error // when set, the n-th Run (from 0) calls it first and returns its error, if any

	mu          sync.Mutex
	runs        int
	setups      int
	teardowns   int
	setupVars   []H
	runDeadline []bool // whether the context given to each Run had a deadline
}

func stepNewFake(name string) *stepFakeExecutor {
	return &stepFakeExecutor{name: name, log: &stepEventLog{}}
}

func stepChain(ctx context.Context) string {
	s, _ := ctx.Value(stepSetupKey).(string) //nolint:errcheck // a missing chain is the empty string, which is what the log shows
	return s
}

func (f *stepFakeExecutor) Setup(ctx context.Context, vars H) (context.Context, error) {
	f.mu.Lock()
	f.setups++
	f.setupVars = append(f.setupVars, vars)
	f.mu.Unlock()
	f.log.add("setup:%s", f.name)
	if f.setupErr != nil {
		return ctx, f.setupErr
	}
	return context.WithValue(ctx, stepSetupKey, stepChain(ctx)+f.name), nil
}

func (f *stepFakeExecutor) TearDown(ctx context.Context) error {
	f.mu.Lock()
	f.teardowns++
	f.mu.Unlock()
	f.log.add("teardown:%s:%s", f.name, stepChain(ctx))
	return f.teardownErr
}

func (f *stepFakeExecutor) Run(ctx context.Context, step TestStep) (interface{}, error) {
	f.mu.Lock()
	n := f.runs
	f.runs++
	_, hasDeadline := ctx.Deadline()
	f.runDeadline = append(f.runDeadline, hasDeadline)
	f.mu.Unlock()

	label, _ := step["label"].(string) //nolint:errcheck // a step without a label logs an empty one
	f.log.add("run:%s:%s:%s", f.name, label, stepChain(ctx))
	if f.onRun != nil {
		if err := f.onRun(n); err != nil {
			return nil, err
		}
	}
	if n == 0 && f.holdFirst {
		<-ctx.Done()
		if f.held != nil {
			f.held <- ctx.Err()
		}
		return nil, ctx.Err()
	}
	if ms := cast.ToInt(step["ms"]); ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	if n < len(f.runErrs) && f.runErrs[n] != nil {
		return nil, f.runErrs[n]
	}
	switch {
	case f.nilResult:
		return nil, nil
	case len(f.results) == 0:
		return stepResult(cast.ToInt(step["code"]), cast.ToString(step["out"]), cast.ToString(step["err"])), nil
	case n < len(f.results):
		return f.results[n], nil
	default:
		return f.results[len(f.results)-1], nil
	}
}

func (f *stepFakeExecutor) counts() (runs, setups, teardowns int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runs, f.setups, f.teardowns
}

// stepDefaultsExecutor is a fake that also declares default assertions, the way exec and http do.
type stepDefaultsExecutor struct {
	*stepFakeExecutor
}

func (stepDefaultsExecutor) GetDefaultAssertions() *StepAssertions {
	return &StepAssertions{Assertions: []Assertion{"result.code ShouldEqual 0"}}
}

// stepValueDefaultsExecutor declares the same defaults in the value form, the way the plugins guide did before #1968.
type stepValueDefaultsExecutor struct {
	*stepFakeExecutor
}

func (stepValueDefaultsExecutor) GetDefaultAssertions() StepAssertions {
	return StepAssertions{Assertions: []Assertion{"result.code ShouldEqual 0"}}
}

// stepResult is what the fake returns. The type is named Result, like the real executors' results, because go-dump
// prefixes a struct's keys with its lower-cased type name: this is what makes "result.code" exist.
func stepResult(code int, out, errOut string) interface{} {
	type Result struct {
		Code      int    `json:"code"`
		Systemout string `json:"systemout"`
		Systemerr string `json:"systemerr"`
	}
	return Result{Code: code, Systemout: out, Systemerr: errOut}
}

// stepNoColor turns the console colours off for the test: they are on by default (IS_TTY unset, venom_output.go) and
// would put escape sequences in every captured line.
func stepNoColor(t *testing.T) {
	t.Helper()
	previous := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = previous })
}

// stepNewVenom returns a Venom with console output captured, colours off, and the given fakes registered as builtins.
func stepNewVenom(t *testing.T, verbose int, fakes ...*stepFakeExecutor) (*Venom, *capture) {
	t.Helper()
	InitTestLogger(t)
	stepNoColor(t)
	v := New()
	v.Verbose = verbose
	c := &capture{}
	v.PrintFunc = c.printf
	for _, f := range fakes {
		v.RegisterExecutorBuiltin(f.name, f)
	}
	return v, c
}

// stepBareCase is the smallest test case RunTestStep needs: a name, as runTestCases would have set it.
func stepBareCase() *TestCase {
	return &TestCase{TestCaseInput: TestCaseInput{Name: "tc"}, originalName: "tc"}
}

// stepErrorValues returns the Value of every failure, in order.
func stepErrorValues(fs []Failure) []string {
	if fs == nil {
		return nil
	}
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Value)
	}
	return out
}

const (
	stepAssertCode0 = "result.code ShouldEqual 0"
	stepCode1Failed = `Testcase "tc", step #1-0: Assertion "result.code ShouldEqual 0" failed. expected: 0  got: 1 (:0)`
)

// TestRunTestStepRetries characterises RunTestStep's retry loop before #2031 splits it: how many times the executor
// runs, the Retries count it leaves, and the failures it records, in order.
//
// quirk: Retries counts retries on a pass (a pass on the second attempt leaves 1) but attempts on a failure (three
// failed attempts leave 3), because the loop increments before the condition fails and a pass leaves by break.
func TestRunTestStepRetries(t *testing.T) {
	code := func(c int) interface{} { return stepResult(c, "", "") }
	tests := []struct {
		name        string
		fake        *stepFakeExecutor
		retry       int
		retryIf     []string
		wantRuns    int
		wantRetries int
		wantOK      bool
		wantErrors  []string
	}{
		{name: "a pass on the first attempt", fake: &stepFakeExecutor{results: []interface{}{code(0)}}, retry: 2, wantRuns: 1, wantOK: true},
		{name: "a failure then a pass: no failure is kept", fake: &stepFakeExecutor{results: []interface{}{code(1), code(0)}}, retry: 2, wantRuns: 2, wantRetries: 1, wantOK: true},
		{name: "no retry and a failure", fake: &stepFakeExecutor{results: []interface{}{code(1)}}, wantRuns: 1, wantRetries: 1, wantErrors: []string{stepCode1Failed}},
		{
			name: "one retry, both fail: the attempts message, then the last attempt's assertion failure",
			fake: &stepFakeExecutor{results: []interface{}{code(1)}}, retry: 1, wantRuns: 2, wantRetries: 2,
			wantErrors: []string{"It's a failure after 2 attempts", stepCode1Failed},
		},
		{
			name: "two retries, all fail", fake: &stepFakeExecutor{results: []interface{}{code(1)}}, retry: 2, wantRuns: 3, wantRetries: 3,
			wantErrors: []string{"It's a failure after 3 attempts", stepCode1Failed},
		},
		{
			name: "an executor error before the last attempt is dropped",
			fake: &stepFakeExecutor{runErrs: []error{errors.New("boom")}, results: []interface{}{code(0)}}, retry: 1, wantRuns: 2, wantRetries: 1, wantOK: true,
		},
		{
			name: "an executor error on the last attempt is the only failure, without the attempts message",
			fake: &stepFakeExecutor{runErrs: []error{errors.New("first"), errors.New("last")}}, retry: 1, wantRuns: 2, wantRetries: 2,
			wantErrors: []string{`Testcase "tc", step #1-0: last (:0)`},
		},
		// quirk: the assertion result of an earlier attempt survives a later executor error and is reported with it
		{
			name: "an assertion failure, then an executor error on the last attempt",
			fake: &stepFakeExecutor{runErrs: []error{nil, errors.New("last")}, results: []interface{}{code(1)}}, retry: 1, wantRuns: 2, wantRetries: 2,
			wantErrors: []string{`Testcase "tc", step #1-0: last (:0)`, "It's a failure after 2 attempts", stepCode1Failed},
		},
		{
			name: "retry_if not fulfilled stops retrying at once, before the assertion failure",
			fake: &stepFakeExecutor{results: []interface{}{code(1)}}, retry: 3, retryIf: []string{"result.code ShouldEqual 2"}, wantRuns: 1,
			wantErrors: []string{`Testcase "tc", step #1-0: retry conditions not fulfilled, skipping 3 remaining retries (:0)`, stepCode1Failed},
		},
		{
			name: "retry_if not fulfilled on a later attempt counts the retries left",
			fake: &stepFakeExecutor{results: []interface{}{code(2), code(1)}}, retry: 3, retryIf: []string{"result.code ShouldEqual 2"}, wantRuns: 2, wantRetries: 1,
			wantErrors: []string{`Testcase "tc", step #1-0: retry conditions not fulfilled, skipping 2 remaining retries (:0)`, stepCode1Failed},
		},
		{
			name: "retry_if fulfilled keeps retrying", fake: &stepFakeExecutor{results: []interface{}{code(1)}}, retry: 1, retryIf: []string{"result.code ShouldEqual 1"},
			wantRuns: 2, wantRetries: 2, wantErrors: []string{"It's a failure after 2 attempts", stepCode1Failed},
		},
		{
			name: "an invalid retry_if is an error and stops retrying",
			fake: &stepFakeExecutor{results: []interface{}{code(1)}}, retry: 3, retryIf: []string{"result.code ShouldBogus 1"}, wantRuns: 1,
			wantErrors: []string{"Error while evaluating retry condition: assertion not supported", stepCode1Failed},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, c := stepNewVenom(t, 0)
			tt.fake.name, tt.fake.log = "fake", &stepEventLog{}
			e := newExecutorRunner(tt.fake, "fake", "builtin", tt.retry, tt.retryIf, 0, 0, nil)
			ts := &TestStepResult{}
			v.RunTestStep(context.Background(), e, stepBareCase(), ts, 1, 0, TestStep{"assertions": []interface{}{stepAssertCode0}})

			runs, _, _ := tt.fake.counts()
			assert.Equal(t, tt.wantRuns, runs, "runs")
			assert.Equal(t, tt.wantRetries, ts.Retries, "Retries")
			assert.Equal(t, tt.wantOK, ts.AssertionsApplied.OK, "AssertionsApplied.OK")
			assert.Equal(t, tt.wantErrors, stepErrorValues(ts.Errors))
			assert.Empty(t, c.String(), "RunTestStep prints nothing")
		})
	}
}

// TestRunTestStepResultFields pins what a passing step leaves in the result: the dumped executor result as computed
// variables, the assertions applied, and systemout / systemerr each suffixed with a newline, appended to what the
// result already held.
func TestRunTestStepResultFields(t *testing.T) {
	v, _ := stepNewVenom(t, 0)
	fake := stepNewFake("fake")
	e := newExecutorRunner(fake, "fake", "builtin", 0, nil, 0, 0, nil)
	ts := &TestStepResult{Systemout: "before|", Systemerr: "before|"}
	v.RunTestStep(context.Background(), e, stepBareCase(), ts, 1, 0,
		TestStep{"code": 0, "out": "hello", "err": "warn", "assertions": []interface{}{stepAssertCode0}})

	assert.Equal(t, H{
		"__type__": "Result", "result.__Len__": 3, "result.code": 0, "result.systemout": "hello", "result.systemerr": "warn",
	}, ts.ComputedVars)
	assert.True(t, ts.AssertionsApplied.OK)
	assert.Equal(t, []AssertionApplied{{Assertion: stepAssertCode0, IsOK: true}}, ts.AssertionsApplied.Assertions)
	assert.Equal(t, "before|hello\n", ts.Systemout)
	assert.Equal(t, "before|warn\n", ts.Systemerr)
	assert.Empty(t, ts.Errors)
	assert.Equal(t, Status(""), ts.Status, "RunTestStep leaves the status to its caller")

	t.Run("without assertions the step passes and the suffix is a bare newline", func(t *testing.T) {
		ts := &TestStepResult{}
		v.RunTestStep(context.Background(), e, stepBareCase(), ts, 1, 0, TestStep{"code": 1})
		assert.True(t, ts.AssertionsApplied.OK)
		assert.Equal(t, "\n", ts.Systemout)
		assert.Equal(t, "\n", ts.Systemerr)
	})
}

// TestRunTestStepInfo pins the info lines: interpolated against the dumped result, dropped when they interpolate to
// "" or fail to interpolate, suffixed with " (file:line)" when the step has a source line, and added once per attempt.
func TestRunTestStepInfo(t *testing.T) {
	info := []string{"code is {{.result.code}}", "{{.result.systemout}}", "{{ .broken", "unknown {{.result.nothing}} stays"}
	ctx := context.WithValue(context.Background(), ContextKey("var.venom.testsuite.filepath"), "suite.yml")
	step := TestStep{"code": 0, "out": "", "assertions": []interface{}{stepAssertCode0}}

	t.Run("without a source line", func(t *testing.T) {
		v, _ := stepNewVenom(t, 0)
		e := newExecutorRunner(stepNewFake("fake"), "fake", "builtin", 0, nil, 0, 0, info)
		ts := &TestStepResult{}
		v.RunTestStep(ctx, e, stepBareCase(), ts, 1, 0, step)
		assert.Equal(t, []string{"code is 0", "unknown {{.result.nothing}} stays"}, ts.ComputedInfo)
	})

	t.Run("with a source line", func(t *testing.T) {
		v, _ := stepNewVenom(t, 0)
		e := newExecutorRunner(stepNewFake("fake"), "fake", "builtin", 0, nil, 0, 0, info[:1])
		tc := stepBareCase()
		tc.StepSourceLines = []int{12}
		ts := &TestStepResult{}
		v.RunTestStep(ctx, e, tc, ts, 1, 0, step)
		assert.Equal(t, []string{"code is 0 (suite.yml:12)"}, ts.ComputedInfo)
	})

	// quirk: every attempt adds its info lines, so a step that passes on its second attempt shows them twice
	t.Run("once per attempt", func(t *testing.T) {
		v, _ := stepNewVenom(t, 0)
		fake := &stepFakeExecutor{name: "fake", log: &stepEventLog{}, results: []interface{}{stepResult(1, "", ""), stepResult(0, "", "")}}
		e := newExecutorRunner(fake, "fake", "builtin", 1, nil, 0, 0, info[:1])
		ts := &TestStepResult{}
		v.RunTestStep(ctx, e, stepBareCase(), ts, 1, 0, step)
		assert.Equal(t, []string{"code is 1", "code is 0"}, ts.ComputedInfo)
	})
}

// TestRunTestStepSourceLines is the repro of #1967: the " (file:line)" of a failure or an info line of step N is step
// N's line, or its assertion's line for a failed assertion, the last step included. The lines are set the way
// extractLineNumbers stores them, by zero-based step index: a test case at line 6, its steps at lines 8 and 12.
func TestRunTestStepSourceLines(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKey("var.venom.testsuite.filepath"), "suite.yml")
	boom := func() *stepFakeExecutor { return &stepFakeExecutor{runErrs: []error{errors.New("boom")}} }
	tests := []struct {
		name       string
		fake       *stepFakeExecutor
		info       []string
		stepNumber int
		step       TestStep
		wantErrors []string
		wantInfo   []string
	}{
		{name: "an error of step 1 carries step 1's line", fake: boom(), stepNumber: 1, wantErrors: []string{`Testcase "tc", step #1-0: boom (suite.yml:8)`}},
		{name: "an error of the last step carries its own line, not the test case's", fake: boom(), stepNumber: 2, wantErrors: []string{`Testcase "tc", step #2-0: boom (suite.yml:12)`}},
		{
			name: "a failed assertion of the last step carries the assertion's line", fake: &stepFakeExecutor{}, stepNumber: 2,
			step:       TestStep{"code": 1, "assertions": []interface{}{stepAssertCode0}},
			wantErrors: []string{`Testcase "tc", step #2-0: Assertion "result.code ShouldEqual 0" failed. expected: 0  got: 1 (suite.yml:14)`},
		},
		{
			name: "an info line of step 1 carries step 1's line", fake: &stepFakeExecutor{}, info: []string{"code is {{.result.code}}"}, stepNumber: 1,
			step: TestStep{"code": 0}, wantInfo: []string{"code is 0 (suite.yml:8)"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := stepNewVenom(t, 0)
			tt.fake.name, tt.fake.log = "fake", &stepEventLog{}
			tc := stepBareCase()
			tc.SourceLine = 6
			tc.StepSourceLines = []int{8, 12}
			tc.AssertionSourceLines = [][]int{nil, {14}}
			ts := &TestStepResult{}
			v.RunTestStep(ctx, newExecutorRunner(tt.fake, "fake", "builtin", 0, nil, 0, 0, tt.info), tc, ts, tt.stepNumber, 0, tt.step)
			assert.Equal(t, tt.wantErrors, stepErrorValues(ts.Errors))
			assert.Equal(t, tt.wantInfo, ts.ComputedInfo)
		})
	}
}

// TestRunTestStepAssertionSources pins what the assertions are applied to: the variables of the context when the
// executor returns nil, and the executor's default assertions when the step declares none.
func TestRunTestStepAssertionSources(t *testing.T) {
	t.Run("a nil result is checked against the context variables", func(t *testing.T) {
		v, _ := stepNewVenom(t, 0)
		fake := stepNewFake("fake")
		fake.nilResult = true
		ctx, _, err := withStepVars(context.Background(), H{"a": "1"})
		require.NoError(t, err)
		e := newExecutorRunner(fake, "fake", "builtin", 0, nil, 0, 0, nil)

		ts := &TestStepResult{}
		v.RunTestStep(ctx, e, stepBareCase(), ts, 1, 0, TestStep{"assertions": []interface{}{"a ShouldEqual 1"}})
		assert.True(t, ts.AssertionsApplied.OK)
		assert.Nil(t, ts.ComputedVars, "a nil result adds no computed variable")

		ts = &TestStepResult{}
		v.RunTestStep(ctx, e, stepBareCase(), ts, 1, 0, TestStep{"assertions": []interface{}{"a ShouldEqual 2"}})
		assert.Equal(t, []string{`Testcase "tc", step #1-0: Assertion "a ShouldEqual 2" failed. expected: 2  got: 1 (:0)`}, stepErrorValues(ts.Errors))
	})

	defaults := stepDefaultsExecutor{stepNewFake("fake")}
	tests := []struct {
		name       string
		exec       Executor
		step       TestStep
		wantErrors []string
	}{
		{name: "the default assertions apply to a step without assertions", exec: defaults, step: TestStep{"code": 1}, wantErrors: []string{stepCode1Failed}},
		{name: "the step's own assertions replace the defaults", exec: defaults, step: TestStep{"code": 1, "assertions": []interface{}{"result.code ShouldEqual 1"}}},
		{name: "an executor without defaults checks nothing", exec: stepNewFake("fake"), step: TestStep{"code": 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := stepNewVenom(t, 0)
			ts := &TestStepResult{}
			v.RunTestStep(context.Background(), newExecutorRunner(tt.exec, "fake", "builtin", 0, nil, 0, 0, nil), stepBareCase(), ts, 1, 0, tt.step)
			assert.Equal(t, tt.wantErrors, stepErrorValues(ts.Errors))
		})
	}
}

// TestRunTestStepDefaultAssertionsForm pins the #1968 contract: the engine takes an executor's default assertions only
// when GetDefaultAssertions returns *StepAssertions. The value form matches nothing and its defaults are dropped without
// a word, which is why the plugins carry a compile-time guard.
func TestRunTestStepDefaultAssertionsForm(t *testing.T) {
	tests := []struct {
		name       string
		exec       Executor
		wantErrors []string
	}{
		{name: "the pointer form's defaults apply", exec: stepDefaultsExecutor{stepNewFake("fake")}, wantErrors: []string{stepCode1Failed}},
		{name: "the value form's defaults are ignored", exec: stepValueDefaultsExecutor{stepNewFake("fake")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := stepNewVenom(t, 0)
			ts := &TestStepResult{}
			v.RunTestStep(context.Background(), newExecutorRunner(tt.exec, "fake", "builtin", 0, nil, 0, 0, nil), stepBareCase(), ts, 1, 0, TestStep{"code": 1})
			assert.Equal(t, tt.wantErrors, stepErrorValues(ts.Errors))
		})
	}
}

// TestRunTestStepTimeout pins a step with a timeout: the executor runs in a goroutine with the step's deadline and a
// slow one becomes a failure on the last attempt.
func TestRunTestStepTimeout(t *testing.T) {
	t.Run("a fast executor returns its result", func(t *testing.T) {
		v, _ := stepNewVenom(t, 0)
		fake := stepNewFake("fake")
		ts := &TestStepResult{}
		v.RunTestStep(context.Background(), newExecutorRunner(fake, "fake", "builtin", 0, nil, 0, 5, nil), stepBareCase(), ts, 1, 0,
			TestStep{"code": 0, "assertions": []interface{}{stepAssertCode0}})
		assert.True(t, ts.AssertionsApplied.OK)
		assert.Empty(t, ts.Errors)
		assert.Equal(t, []bool{true}, fake.runDeadline, "Run gets the step's deadline")
	})

	t.Run("a slow executor times out", func(t *testing.T) {
		v, _ := stepNewVenom(t, 0)
		fake := stepNewFake("fake")
		ts := &TestStepResult{}
		v.RunTestStep(context.Background(), newExecutorRunner(fake, "fake", "builtin", 0, nil, 0, 1, nil), stepBareCase(), ts, 1, 0,
			TestStep{"ms": 1300, "assertions": []interface{}{stepAssertCode0}})
		assert.Equal(t, []string{`Testcase "tc", step #1-0: Timeout after 1 second(s) (:0)`}, stepErrorValues(ts.Errors))
		assert.False(t, ts.AssertionsApplied.OK)
		assert.Equal(t, "\n", ts.Systemout)
	})
}

// stepCtxExecutor is a fake that honours cancellation: Run returns only once its context is done.
type stepCtxExecutor struct{}

func (stepCtxExecutor) Run(ctx context.Context, _ TestStep) (interface{}, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// stepExecutorGoroutines returns the stack of every goroutine still running runTestStepExecutor's closure, the one
// that calls the executor of a step with a timeout.
func stepExecutorGoroutines() []string {
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	for n == len(buf) {
		buf = make([]byte, 2*len(buf))
		n = runtime.Stack(buf, true)
	}
	var left []string
	for _, g := range strings.Split(string(buf[:n]), "\n\n") {
		if strings.Contains(g, "venom.(*Venom).runTestStepExecutor.func") {
			left = append(left, g)
		}
	}
	return left
}

// stepNoExecutorGoroutineLeft fails the test unless every goroutine found by stepExecutorGoroutines has exited within
// a little while, so a goroutine on its way out can finish.
func stepNoExecutorGoroutineLeft(t *testing.T) {
	t.Helper()
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		if left := stepExecutorGoroutines(); len(left) > 0 {
			c.Errorf("%d executor goroutine(s) left:\n%s", len(left), strings.Join(left, "\n\n"))
		}
	}, 2500*time.Millisecond, 50*time.Millisecond)
}

// stepLogWriter is the io.Writer of a logger whose lines are captured.
type stepLogWriter struct{ *capture }

func (w stepLogWriter) Write(p []byte) (int, error) { return w.printf("%s", p) }

// TestRunTestStepTimeoutLeavesNoGoroutine is the #1969 repro: once a step has timed out, the goroutine that ran its
// executor exits, whether the executor honours its context or ignores it and returns late.
func TestRunTestStepTimeoutLeavesNoGoroutine(t *testing.T) {
	tests := []struct {
		name string
		exec Executor
		step TestStep
	}{
		{name: "an executor waiting on its context is stopped", exec: stepCtxExecutor{}, step: TestStep{}},
		{name: "an executor ignoring its context returns late", exec: stepNewFake("fake"), step: TestStep{"ms": 1300}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := stepNewVenom(t, 0)
			ts := &TestStepResult{}
			v.RunTestStep(context.Background(), newExecutorRunner(tt.exec, "fake", "builtin", 0, nil, 0, 1, nil), stepBareCase(), ts, 1, 0, tt.step)
			require.Equal(t, []string{`Testcase "tc", step #1-0: Timeout after 1 second(s) (:0)`}, stepErrorValues(ts.Errors))
			stepNoExecutorGoroutineLeft(t)
		})
	}
}

const (
	// stepSlowExecutor is a user executor with one inner step, run by the fake named "fake".
	stepSlowExecutor = "executor: slow\nsteps:\n- type: fake\noutput:\n  x: y\n"
	// stepOuterTimeout is the failure of a user executor step of stepOuterCase that timed out after 1 second.
	stepOuterTimeout = `Testcase "outer", step #1-0: Timeout after 1 second(s) (:0)`
	// stepAttemptFailed is in the line RunUserExecutor logs when an attempt fails, the last thing the attempt does with
	// shared state.
	stepAttemptFailed = "failed - raw interpolated"
)

// stepOuterCase is the test case that calls a user executor, with the suite variables RunUserExecutor reads.
func stepOuterCase() *TestCase {
	return &TestCase{TestCaseInput: TestCaseInput{Name: "outer"}, originalName: "outer", TestSuiteVars: H{}}
}

// stepUserTimeoutVenom returns a Venom with the fakes registered and its log lines captured, and the context for a user
// executor step with a timeout whose first attempt fails, at its timeout or later.
//
// At cleanup the context is cancelled, then the cleanup waits for that attempt to log its failure and for every executor
// goroutine to exit. The log line orders the attempt's writes before the next test's under -race; the exit keeps the
// package-wide leak check of TestRunTestStepTimeoutLeavesNoGoroutine clean in any order. The cancel ends an attempt its
// timeout failed to stop, so that regression fails its own test and does not leak into the next ones.
func stepUserTimeoutVenom(t *testing.T, fakes ...*stepFakeExecutor) (*Venom, *capture, context.Context) {
	t.Helper()
	v, _ := stepNewVenom(t, 0, fakes...)
	logs := &capture{}
	logger.Logger.SetOutput(stepLogWriter{logs})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		stepWaitAttemptFailed(t, logs)
		stepNoExecutorGoroutineLeft(t)
	})
	return v, logs, ctx
}

// stepWaitAttemptFailed fails the test unless an attempt of a user executor logs its failure within a little while.
func stepWaitAttemptFailed(t *testing.T, logs *capture) {
	t.Helper()
	assert.Eventually(t, func() bool { return strings.Contains(logs.String(), stepAttemptFailed) }, 2*time.Second, 10*time.Millisecond,
		"the attempt logs its failure")
}

// stepHeldErr returns the error the fake's held first Run got from its context, or stops the test when that Run is
// still waiting a while after the step is done.
func stepHeldErr(t *testing.T, f *stepFakeExecutor) error {
	t.Helper()
	select {
	case err := <-f.held:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("the held inner step is still waiting on its context after the step is done")
		return nil
	}
}

// TestRunTestStepUserExecutorRetryStaysFailed characterises the retry rule of a user executor step, kept by #2074 and
// left to #2102: an inner failure in one attempt fails every later attempt, because RunUserExecutor fails an attempt on
// any failure the step's result holds. The inner step fails on the first attempt and passes on the second, yet the step
// fails with the first attempt's failure and the second's `executor "flaky" failed`, with a timeout or without one.
func TestRunTestStepUserExecutorRetryStaysFailed(t *testing.T) {
	tests := []struct {
		name    string
		timeout int
	}{
		{name: "without a timeout"},
		{name: "with a timeout", timeout: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := stepNewFake("fake")
			fake.results = []interface{}{stepResult(1, "out1", "err1"), stepResult(0, "out2", "err2")}
			v, _ := stepNewVenom(t, 0, fake)
			ux := stepUserExecutor("executor: flaky\nsteps:\n- type: fake\n  info: \"code {{.result.code}}\"\n  assertions: [result.code ShouldEqual 0]\noutput:\n  x: y\n")
			ts := &TestStepResult{}
			v.RunTestStep(context.Background(), newExecutorRunner(ux, "flaky", "user", 1, nil, 0, tt.timeout, nil), stepOuterCase(), ts, 1, 0, TestStep{})

			runs, _, _ := fake.counts()
			assert.Equal(t, 2, runs, "the inner step passed on the second attempt")
			assert.Equal(t, []string{
				`Testcase "flaky", step #1-0: Assertion "result.code ShouldEqual 0" failed. expected: 0  got: 1 (:0)`,
				`Testcase "outer", step #1-0: executor "flaky" failed (:0)`,
			}, stepErrorValues(ts.Errors))
			assert.False(t, ts.AssertionsApplied.OK)
			assert.Equal(t, 2, ts.Retries)
			// Both attempts ended, so the step keeps the info lines and the output of each attempt's inner step, in order;
			// the last "\n" is the one RunTestStep appends for the step itself.
			assert.Equal(t, []string{"code 1", "code 0"}, ts.ComputedInfo)
			assert.Equal(t, "out1\nout2\n\n", ts.Systemout)
			assert.Equal(t, "err1\nerr2\n\n", ts.Systemerr)
		})
	}
}

// TestRunTestStepTimeoutStopsUserExecutor is the #2074 repro of its first criterion: a user executor step is stopped at
// its timeout. The inner step waits on its context, which ends at the step's deadline, not when the step's own context
// is cancelled at cleanup, and the attempt's goroutine exits.
func TestRunTestStepTimeoutStopsUserExecutor(t *testing.T) {
	fake := stepNewFake("fake")
	fake.holdFirst, fake.held = true, make(chan error, 1)
	v, logs, ctx := stepUserTimeoutVenom(t, fake)
	ts := &TestStepResult{}
	v.RunTestStep(ctx, newExecutorRunner(stepUserExecutor(stepSlowExecutor), "slow", "user", 0, nil, 0, 1, nil), stepOuterCase(), ts, 1, 0, TestStep{})
	require.Equal(t, []string{stepOuterTimeout}, stepErrorValues(ts.Errors))

	require.ErrorIs(t, stepHeldErr(t, fake), context.DeadlineExceeded, "the inner step's context ends at the step's timeout")
	stepWaitAttemptFailed(t, logs)
	stepNoExecutorGoroutineLeft(t)
}

// TestRunTestStepTimeoutUserExecutorRetry is the #2074 repro of its second criterion: a user executor step whose first
// attempt outlives its timeout passes on its retry, and -race reports nothing. Each attempt writes into a result of its
// own, and the step keeps only an attempt that ends in time.
func TestRunTestStepTimeoutUserExecutorRetry(t *testing.T) {
	run := func(t *testing.T, v *Venom, ctx context.Context, fake *stepFakeExecutor) {
		t.Helper()
		ts := &TestStepResult{}
		v.RunTestStep(ctx, newExecutorRunner(stepUserExecutor(stepSlowExecutor), "slow", "user", 1, nil, 0, 1, nil), stepOuterCase(), ts, 1, 0, TestStep{})

		runs, _, _ := fake.counts()
		assert.Equal(t, 2, runs, "the inner step ran once per attempt")
		assert.Empty(t, stepErrorValues(ts.Errors))
		assert.True(t, ts.AssertionsApplied.OK)
		assert.Equal(t, 1, ts.Retries, "the step passed on its retry")
	}

	t.Run("a first attempt waiting on its context ends at the timeout", func(t *testing.T) {
		fake := stepNewFake("fake")
		fake.holdFirst, fake.held = true, make(chan error, 1)
		v, _, ctx := stepUserTimeoutVenom(t, fake)
		run(t, v, ctx, fake)
		assert.ErrorIs(t, stepHeldErr(t, fake), context.DeadlineExceeded, "the first attempt ends at its timeout, not at cleanup")
	})

	// The first attempt's inner step ignores its context and fails only once the retry runs; the retry waits for that
	// failure to be logged, so the failure is in by the time the retry judges its own errors.
	t.Run("a first attempt failing after its timeout does not fail the retry", func(t *testing.T) {
		fake := stepNewFake("fake")
		v, logs, ctx := stepUserTimeoutVenom(t, fake)
		release := make(chan struct{})
		fake.onRun = func(n int) error {
			if n == 0 {
				<-release
				return errors.New("late failure")
			}
			close(release)
			stepWaitAttemptFailed(t, logs)
			return nil
		}
		run(t, v, ctx, fake)
	})
}

// TestRunTestStepTimeoutDropsUserExecutorAttempt is the #2074 repro of its third criterion: an attempt stopped at its
// timeout leaves nothing in the step's result but the timeout failure. Its first inner step passes with output and an
// info line; its second waits on its context and fails once that context ends. The step's context is cancelled once the
// step is done so that the attempt ends whichever context it was given, and the test reads the result once the attempt
// has logged its failure.
func TestRunTestStepTimeoutDropsUserExecutorAttempt(t *testing.T) {
	held := stepNewFake("held")
	held.holdFirst = true
	v, logs, ctx := stepUserTimeoutVenom(t, stepNewFake("fake"), held)
	ux := stepUserExecutor("executor: slow\nsteps:\n- type: fake\n  out: partial\n  err: oops\n  info: \"code {{.result.code}}\"\n- type: held\noutput:\n  x: y\n")
	ctx, cancel := context.WithCancel(ctx)
	ts := &TestStepResult{}
	v.RunTestStep(ctx, newExecutorRunner(ux, "slow", "user", 0, nil, 0, 1, nil), stepOuterCase(), ts, 1, 0, TestStep{})
	cancel()
	stepWaitAttemptFailed(t, logs)
	stepNoExecutorGoroutineLeft(t)

	assert.Equal(t, []string{stepOuterTimeout}, stepErrorValues(ts.Errors))
	assert.Empty(t, ts.ComputedInfo)
	assert.Equal(t, "\n", ts.Systemout)
	assert.Equal(t, "\n", ts.Systemerr)
}

// TestRunTestStepDump pins --verbose 2 and more: one dump file per step in the output directory, named from the suite
// short name, the test case, its number, the step and the range index, with secrets hidden. A dump that cannot be
// written ends RunTestStep at once, before the assertions and the systemout suffix.
func TestRunTestStepDump(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKey("var.venom.testsuite.shortName"), "My Suite")
	ctx = context.WithValue(ctx, ContextKey("secrets"), []string{outputSecret})
	step := TestStep{"code": 0, "out": "token=" + outputSecret, "assertions": []interface{}{stepAssertCode0}}
	newCase := func() *TestCase {
		return &TestCase{TestCaseInput: TestCaseInput{Name: "My Case"}, originalName: "My Case", number: 3}
	}

	t.Run("the dump is written", func(t *testing.T) {
		v, _ := stepNewVenom(t, 2)
		v.OutputDir = t.TempDir()
		tc := newCase()
		ts := &TestStepResult{}
		v.RunTestStep(ctx, newExecutorRunner(stepNewFake("fake"), "fake", "builtin", 0, nil, 0, 0, nil), tc, ts, 2, 1, step)

		name := filepath.Join(v.OutputDir, "my-suite.my-case.testcase.3.step.2.1.dump.json")
		content, err := os.ReadFile(name)
		require.NoError(t, err)
		assert.NotContains(t, string(content), outputSecret)
		var dump map[string]interface{}
		require.NoError(t, json.Unmarshal(content, &dump))
		assert.Equal(t, map[string]interface{}{"code": float64(0), "systemout": "token=__hidden__", "systemerr": ""}, dump["result"])
		assert.Equal(t, map[string]interface{}{"code": float64(0), "out": "token=__hidden__", "assertions": []interface{}{stepAssertCode0}}, dump["step"])
		assert.Contains(t, dump, "variables")
		assert.Equal(t, []string{"writing " + name}, tc.computedVerbose)
		assert.True(t, ts.AssertionsApplied.OK)
		assert.Equal(t, "token="+outputSecret+"\n", ts.Systemout, "only the dump file is redacted")
	})

	// #2132: the name of the file in the output directory must not show a secret held in the test case name.
	t.Run("a secret in the test case name is hidden in the file name", func(t *testing.T) {
		v, _ := stepNewVenom(t, 2)
		v.OutputDir = t.TempDir()
		tc := newCase()
		tc.Name = "tc-" + outputSecret
		v.RunTestStep(ctx, newExecutorRunner(stepNewFake("fake"), "fake", "builtin", 0, nil, 0, 0, nil), tc, &TestStepResult{}, 2, 1, step)

		entries, err := os.ReadDir(v.OutputDir)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "my-suite.tc-__hidden.testcase.3.step.2.1.dump.json", entries[0].Name())
	})

	t.Run("a dump that cannot be written stops the step", func(t *testing.T) {
		v, _ := stepNewVenom(t, 2)
		v.OutputDir = filepath.Join(t.TempDir(), "missing")
		tc := newCase()
		ts := &TestStepResult{}
		v.RunTestStep(ctx, newExecutorRunner(stepNewFake("fake"), "fake", "builtin", 2, nil, 0, 0, nil), tc, ts, 2, 1, step)

		assert.Equal(t, 0, ts.Retries)
		assert.False(t, ts.AssertionsApplied.OK)
		assert.Nil(t, ts.ComputedVars)
		assert.Empty(t, ts.Errors)
		assert.Empty(t, ts.Systemout, "the newline suffix is skipped too")
		assert.Empty(t, tc.computedVerbose)
	})
}
