package venom

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/gosimple/slug"
	"github.com/ovh/venom/internal/yamljson"
	"github.com/ovh/venom/interpolate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Executor execute a testStep.
type Executor interface {
	// Run run a Test Step
	Run(context.Context, TestStep) (interface{}, error)
}

type ExecutorRunner interface {
	Executor
	executorWithDefaultAssertions
	executorWithZeroValueResult
	ExecutorWithSetup
	Name() string
	Retry() int
	RetryIf() []string
	Delay() int
	Timeout() int
	Info() []string
	Type() string
	GetExecutor() Executor
}

var _ Executor = new(executor)

// ExecutorWrap contains an executor implementation and some attributes
type executor struct {
	Executor
	name    string
	retry   int      // nb retry a test case if it is in failure.
	retryIf []string // retry conditions to check before performing any retries
	delay   int      // delay between two retries
	timeout int      // timeout on executor
	info    []string // info to display after the run and before the assertion
	stype   string   // builtin, plugin, user
}

func (e executor) Name() string {
	return e.name
}

func (e executor) Type() string {
	return e.stype
}

func (e executor) Retry() int {
	return e.retry
}

func (e executor) RetryIf() []string {
	return e.retryIf
}

func (e executor) Delay() int {
	return e.delay
}

func (e executor) Timeout() int {
	return e.timeout
}

func (e executor) Info() []string {
	return e.info
}

func (e executor) GetExecutor() Executor {
	return e.Executor
}

func (e executor) GetDefaultAssertions() *StepAssertions {
	if e.Executor == nil {
		return nil
	}
	x, ok := e.Executor.(executorWithDefaultAssertions)
	if ok {
		return x.GetDefaultAssertions()
	}
	return nil
}

func (e executor) ZeroValueResult() interface{} {
	if e.Executor == nil {
		return nil
	}
	x, ok := e.Executor.(executorWithZeroValueResult)
	if ok {
		return x.ZeroValueResult()
	}
	return nil
}

func (e executor) Setup(ctx context.Context, vars H) (context.Context, error) {
	if e.Executor == nil {
		return ctx, nil
	}
	x, ok := e.Executor.(ExecutorWithSetup)
	if ok {
		return x.Setup(ctx, vars)
	}
	return ctx, nil
}

func (e executor) TearDown(ctx context.Context) error {
	if e.Executor == nil {
		return nil
	}
	x, ok := e.Executor.(ExecutorWithSetup)
	if ok {
		return x.TearDown(ctx)
	}
	return nil
}

func (e executor) Run(ctx context.Context, step TestStep) (interface{}, error) {
	if e.Executor == nil {
		return nil, nil
	}
	return e.Executor.Run(ctx, step)
}

func newExecutorRunner(e Executor, name, stype string, retry int, retryIf []string, delay, timeout int, info []string) ExecutorRunner {
	return &executor{
		Executor: e,
		name:     name,
		retry:    retry,
		retryIf:  retryIf,
		delay:    delay,
		timeout:  timeout,
		info:     info,
		stype:    stype,
	}
}

// executorWithDefaultAssertions execute a testStep.
type executorWithDefaultAssertions interface {
	// GetDefaultAssertion returns default assertions
	GetDefaultAssertions() *StepAssertions
}

type executorWithZeroValueResult interface {
	ZeroValueResult() interface{}
}

type ExecutorWithSetup interface {
	Setup(ctx context.Context, vars H) (context.Context, error)
	TearDown(ctx context.Context) error
}

// dumpExecutorResult is the dump function GetExecutorResult uses; a package variable so a test can make it fail
// (a Dump error is otherwise unreachable).
var dumpExecutorResult = Dump

// GetExecutorResult dumps an executor result into a flat map.
//
// It panics when the dump fails. The signature and the panic are kept for the plugins built against this module;
// the engine itself calls getExecutorResult and handles the error.
func GetExecutorResult(r interface{}) map[string]interface{} {
	d, err := getExecutorResult(r)
	if err != nil {
		panic(err)
	}
	return d
}

// getExecutorResult is GetExecutorResult with the dump error returned instead of raised.
func getExecutorResult(r interface{}) (map[string]interface{}, error) {
	return dumpExecutorResult(r)
}

type UserExecutor struct {
	Executor  string
	Input     H                 `json:"input" yaml:"input"`
	TestSteps []json.RawMessage `json:"steps" yaml:"steps"`
	Raw       []byte            `json:"-" yaml:"-"` // the raw file content of the executor
	RawInputs []byte            `json:"-" yaml:"-"`
	Filename  string            `json:"-" yaml:"-"`
	Output    json.RawMessage   `json:"output" yaml:"output"`
}

// Run is not implemented on user executor
func (ux UserExecutor) Run(_ context.Context, _ TestStep) (interface{}, error) {
	return nil, errors.New("Run not implemented for user interface, use RunUserExecutor instead")
}

func (ux UserExecutor) ZeroValueResult() interface{} {
	type Output struct {
		Result interface{} `json:"result"`
	}
	output := &Output{
		Result: ux.Output,
	}
	outputS, err := json.Marshal(output)
	if err != nil {
		return ""
	}

	result := make(map[string]interface{})
	err = JSONUnmarshal(outputS, &result)
	if err != nil {
		return ""
	}
	return result
}

// RunUserExecutor runs a user executor with the given context, runner, test case, test step result, and step.
func (v *Venom) RunUserExecutor(ctx context.Context, runner ExecutorRunner, tcIn *TestCase, tsIn *TestStepResult, step TestStep) (interface{}, error) {
	ux, ok := runner.GetExecutor().(UserExecutor)
	if !ok {
		return nil, fmt.Errorf("RunUserExecutor called with a runner whose executor is not a UserExecutor (got %T)", runner.GetExecutor())
	}
	call, err := prepareUserExecutor(ux, step, tcIn)
	if err != nil {
		return nil, err
	}
	tc := call.tc
	ux.Output = call.output

	Debug(ctx, "running user executor %v", tc.Name)
	if logger.Logger.IsLevelEnabled(logrus.DebugLevel) {
		Debug(ctx, "with vars: %v", tc.fullVars(tc.Vars))
	}

	v.runTestSteps(ctx, tc, tsIn)

	mergeUserExecutorResults(tsIn, tc.TestStepResults)

	return userExecutorResult(ctx, ux, tc, tsIn, call.sanitized)
}

// userExecutorCall is a user executor ready to run: the test case that holds its steps, the output it declares and the
// text its steps were read from.
type userExecutorCall struct {
	tc        *TestCase
	output    json.RawMessage
	sanitized string
}

// prepareUserExecutor interpolates the inputs and the steps of ux for step, and builds the test case that runs them.
func prepareUserExecutor(ux UserExecutor, step TestStep, tcIn *TestCase) (*userExecutorCall, error) {
	vrs, over := userExecutorStartVars(tcIn)
	var tsVars scopeLayers
	newUX := UserExecutor{}
	var err error

	if len(ux.RawInputs) != 0 {
		vrs, tsVars, over, err = addUserExecutorInputs(ux, step, vrs, &newUX, tcIn.suite, over)
		if err != nil {
			return nil, err
		}
	}

	interpolatedFull, err := tsVars.do(protectLaterActions(string(ux.Raw), func(name string) bool { _, ok := tsVars.get(name); return ok }))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to interpolate executor %q", ux.Executor)
	}
	// quote any remaining template expressions to ensure proper YAML parsing
	sanitized := quoteTemplateExpressions([]byte(interpolatedFull))

	err = yamljson.Unmarshal(sanitized, &newUX)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to unmarshal executor %q - raw interpolated :\n%v", ux.Executor, string(sanitized))
	}

	tc := newUserExecutorTestCase(ux, vrs, newUX.TestSteps, tcIn)
	tc.varsOverSuite = over
	return &userExecutorCall{tc: tc, output: newUX.Output, sanitized: string(sanitized)}, nil
}

// userExecutorStartVars returns the variables the inner test case of a custom executor starts from: a copy of the suite's,
// or, when the root case has a prepared suite, none, and true: the case then holds its own variables over the suite's,
// which are shared and never copied (#2475).
func userExecutorStartVars(tcIn *TestCase) (H, bool) {
	if tcIn.suite.prepared() != nil {
		return H{}, true
	}
	return tcIn.TestSuiteVars.Clone(), false
}

// userExecutorResult builds what RunUserExecutor returns once the inner steps have run: the executor's output, or the
// failure of one of them.
func userExecutorResult(ctx context.Context, ux UserExecutor, tc *TestCase, tsIn *TestStepResult, sanitized string) (interface{}, error) {
	outputResult, err := interpolateUserExecutorOutput(ux.Output, tc.computedVars)
	if err != nil {
		return nil, err
	}

	// here, we have the user executor results.
	// and for each key in output, we try to add the json version
	// this will allow user to use json version of output (map, etc...)
	// because, it's not possible to to that:
	// output:
	//   therawout: {{.result.systemout}}
	//
	// test is in file user_executor.yml

	result, err := Dump(outputResult)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to compute result")
	}

	if len(tsIn.Errors) > 0 {
		Error(ctx, "user executor %q failed - raw interpolated:\n%v\n", ux.Executor, sanitized)
		return outputResult, fmt.Errorf("executor %q failed", ux.Executor)
	}

	if err := addUserExecutorJSONOutputs(result); err != nil {
		return nil, err
	}
	return result, nil
}

// processUserExecutorInputs interpolates the executor's inputs into newUX and adds them to vrs, the whole scope of the
// case, the step's values winning over the defaults; it returns vrs and its dump.
func processUserExecutorInputs(ux UserExecutor, step TestStep, vrs H, newUX *UserExecutor, suite *suiteScope) (H, scopeLayers, error) {
	vrs, tsVars, _, err := addUserExecutorInputs(ux, step, vrs, newUX, suite, false)
	return vrs, tsVars, err
}

// addUserExecutorInputs is processUserExecutorInputs for vrs held over the suite's variables when over is true: vrs then
// holds only the case's own, and the suite's are read through the prepared dump (#2475). It reports false, and returns
// the whole scope, when the inputs change the suite's variables in a way the prepared dump cannot follow.
func addUserExecutorInputs(ux UserExecutor, step TestStep, vrs H, newUX *UserExecutor, suite *suiteScope, over bool) (H, scopeLayers, bool, error) {
	if over && suite.lowerOver() == nil {
		vrs, over = suite.under(vrs), false
	}
	tsVars, err := lowerDumpOf(vrs, suite, over)
	if err != nil {
		return nil, scopeLayers{}, over, err
	}

	interpolatedInput, err := tsVars.do(string(ux.RawInputs))
	if err != nil {
		return nil, scopeLayers{}, over, errors.Wrapf(err, "unable to interpolate executor inputs %q", ux.Executor)
	}

	err = yamljson.Unmarshal([]byte(interpolatedInput), newUX)
	if err != nil {
		return nil, scopeLayers{}, over, errors.Wrapf(err, "unable to unmarshal inputs for executor %q - raw interpolated:\n%v", ux.Executor, interpolatedInput)
	}

	var under H
	if over {
		under = suite.src
	}
	vrs, edit := setExecutorInputs(step, vrs, under, newUX.Input)
	// tsVars is the dump of vrs before the inputs; bring it up to date rather than dump the whole scope again (#2465)
	if edit.applyLayered(tsVars, DumpString, true, "__len__", "__type__") {
		return vrs, tsVars, over, nil
	}
	if over {
		vrs, over = suite.under(vrs), false
	}
	if tsVars, err = lowerDumpOf(vrs, nil, false); err != nil {
		return nil, scopeLayers{}, over, err
	}
	return vrs, tsVars, over, nil
}

// lowerDumpOf returns the dump of vrs that the inputs of a user executor are interpolated with: layers over the prepared
// dump of the suite when vrs is the suite's variables, or holds the case's own over them (over), a dump of vrs alone
// otherwise.
func lowerDumpOf(vrs H, suite *suiteScope, over bool) (scopeLayers, error) {
	layers := suite.lowerLayers(vrs)
	if over {
		layers = suite.lowerOver()
	}
	if layers != nil {
		return *layers, nil
	}
	dump, err := dumpHLowerFirst(vrs)
	if err != nil {
		return scopeLayers{}, errors.Wrapf(err, "error processing executor inputs: unable to dump testsuite vars")
	}
	return scopeLayers{overlay: dump}, nil
}

// inputPrefix is the prefix of the variables that carry the inputs of a user executor.
const inputPrefix = "input."

// setExecutorInputs adds the executor's inputs to vrs, the step's values winning over the defaults, and returns vrs with
// the edits it made. under holds the variables vrs is laid over, when vrs is only the case's own.
func setExecutorInputs(step TestStep, vrs, under, inputs H) (H, scopeEdit) {
	var edit scopeEdit
	for k, va := range inputs {
		switch {
		case strings.HasPrefix(k, inputPrefix):
			// do not reinject input.vars from parent user executor if exists
			continue
		case !strings.HasPrefix(k, "venom"):
			if vl, ok := step[k]; ok && vl != nil && vl != "" { // value from step and not nil/empty
				edit.setUnder(vrs, under, inputPrefix+k, vl)
			} else { // default value from executor
				edit.setUnder(vrs, under, inputPrefix+k, va)
			}
		default:
			if vrs == nil { // as H.Add does
				vrs = H{}
			}
			edit.setUnder(vrs, under, k, va)
		}
	}
	return vrs, edit
}

// newUserExecutorTestCase builds the inner test case that runs the steps of a user executor.
func newUserExecutorTestCase(ux UserExecutor, vrs H, rawTestSteps []json.RawMessage, tcIn *TestCase) *TestCase {
	tc := &TestCase{
		TestCaseInput: TestCaseInput{
			Name:         ux.Executor,
			Vars:         vrs,
			RawTestSteps: rawTestSteps,
		},
		number:          tcIn.number,
		TestSuiteVars:   tcIn.TestSuiteVars,
		suite:           tcIn.suite,
		IsExecutor:      true,
		TestStepResults: make([]TestStepResult, 0),
		// The inner case belongs to the same test file as the one that invoked the user executor,
		// so it carries that file's printer (#2021).
		//
		// Nothing prints through it today: every call site here reaches runTestSteps with a non-nil
		// tsIn, so fromUserExecutor is true and the step prints are all gated off. This is set so
		// that the day that gate is lifted, the output is attributed rather than silently untagged.
		// Whoever lifts it must also deal with the flush: runTestCases flushes when the file's
		// cases are done, so output produced after that would sit in the buffer and be lost, not
		// merely untagged.
		out: tcIn.out,
	}

	tc.originalName = tc.Name
	tc.Name = slug.Make(tc.Name)
	tc.Vars.Add("venom.testcase", tc.Name)
	tc.Vars.Add("venom.executor.filename", ux.Filename)
	tc.Vars.Add("venom.executor.name", ux.Executor)
	tc.computedVars = H{}
	return tc
}

// mergeUserExecutorResults merges inner step results into the parent test step result so they
// appear in JUnit XML, JSON, HTML, and other report outputs.
func mergeUserExecutorResults(tsIn *TestStepResult, innerResults []TestStepResult) {
	for _, innerResult := range innerResults {
		if len(innerResult.Errors) > 0 {
			tsIn.Errors = append(tsIn.Errors, innerResult.Errors...)
		}
		if len(innerResult.ComputedInfo) > 0 {
			tsIn.ComputedInfo = append(tsIn.ComputedInfo, innerResult.ComputedInfo...)
		}
		if strings.TrimSpace(innerResult.Systemout) != "" {
			tsIn.Systemout += innerResult.Systemout
		}
		if strings.TrimSpace(innerResult.Systemerr) != "" {
			tsIn.Systemerr += innerResult.Systemerr
		}
	}
}

// interpolateUserExecutorOutput interpolates the executor's `output:` with the vars its steps
// computed and decodes it.
func interpolateUserExecutorOutput(uxOutput json.RawMessage, tcComputedVars H) (interface{}, error) {
	computedVars, err := DumpStringPreserveCase(tcComputedVars)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to dump testcase computedVars")
	}

	type Output struct {
		Result json.RawMessage `json:"result"`
	}
	output := Output{
		Result: uxOutput,
	}
	outputString, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}

	// the value of each var can contains a double-quote -> "
	// if the value is not escaped, it will be used as is, and the json sent to unmarshall will be incorrect.
	// This also avoids injections into the json structure of a user executor
	// Use strconv.Quote for proper escaping to avoid double-escaping issues
	for i := range computedVars {
		computedVars[i] = escapeQuotes(computedVars[i])
	}

	outputS, err := interpolate.Do(string(outputString), computedVars)
	if err != nil {
		return nil, err
	}

	var outputResult interface{}
	if err := yamljson.Unmarshal([]byte(outputS), &outputResult); err != nil {
		return nil, errors.Wrapf(err, "unable to unmarshal")
	}
	return outputResult, nil
}

// addUserExecutorJSONOutputs adds, next to each string output that holds JSON, its decoded twin
// under the key suffixed with "json", dumped under that prefix.
func addUserExecutorJSONOutputs(result map[string]interface{}) error {
	for k, v := range result {
		if err := addUserExecutorJSONOutput(result, k, v); err != nil {
			return err
		}
	}
	return nil
}

// addUserExecutorJSONOutput adds the "json" twin of one output value into result, when that value
// is a string holding JSON — the per-key body addUserExecutorJSONOutputs ranges over. It mutates
// the same result map that range is iterating, exactly as before the split (deliberate, see D47 in
// docs/feat/2031/plan.md): do not change this to collect into a fresh map first.
func addUserExecutorJSONOutput(result map[string]interface{}, key string, value interface{}) error {
	z, ok := value.(string)
	if !ok {
		return nil
	}
	var outJSON interface{}
	if err := JSONUnmarshal([]byte(z), &outJSON); err != nil {
		return nil
	}
	result[key+"json"] = outJSON
	// Now we have to dump this object, but the key will change if this is a array or not
	if reflect.ValueOf(outJSON).Kind() == reflect.Slice {
		prefix := key + "json"
		splitPrefix := strings.Split(prefix, ".")
		prefix += "." + splitPrefix[len(splitPrefix)-1]
		outJSONDump, err := Dump(outJSON)
		if err != nil {
			return errors.Wrapf(err, "unable to compute result")
		}
		for ko, vo := range outJSONDump {
			result[prefix+ko] = vo
		}
	} else {
		outJSONDump, err := DumpWithPrefix(outJSON, key+"json")
		if err != nil {
			return errors.Wrapf(err, "unable to compute result")
		}
		for ko, vo := range outJSONDump {
			result[ko] = vo
		}
	}
	return nil
}

// quoteTemplateExpressions adds double quotes around template expressions in YAML content.
// It specifically targets expressions that follow a colon and whitespace like 'key: {{.variable}}'
// and are not already enclosed in quotes. This ensures proper YAML parsing of template variables.
func quoteTemplateExpressions(content []byte) []byte {
	// First capture group matches everything up to the colon, checking the last non-whitespace
	// character isn't a quote (to skip JSON keys)
	re := regexp.MustCompile(`(?m)(^.*[^"\s][\s]*)(:\s+)({{.*?}})(.*?)(?:\s*)$`)

	// Put quotes around the template expression and what follows it
	return re.ReplaceAll(content, []byte(`$1$2"$3$4"`))
}
