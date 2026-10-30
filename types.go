package venom

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"maps"
	"strings"
	"time"
	"unicode"

	"github.com/fatih/color"
	"github.com/spf13/cast"
)

type Status string

const (
	StatusRun  Status = "RUN"
	StatusFail Status = "FAIL"
	StatusSkip Status = "SKIP"
	StatusPass Status = "PASS"
)

type H map[string]interface{}

func (h H) Clone() H {
	return maps.Clone(h)
}

func (h *H) Add(k string, v interface{}) {
	if h == nil || *h == nil {
		*h = make(map[string]interface{})
	}
	(*h)[k] = v
}

func (h *H) AddWithPrefix(p, k string, v interface{}) {
	(*h)[p+"."+k] = v
}

func (h *H) AddAll(h2 H) {
	for k, v := range h2 {
		h.Add(k, v)
	}
}

func (h *H) AddAllWithPrefix(p string, h2 H) {
	if h2 == nil {
		return
	}
	if h == nil {
		_h := H{}
		*h = _h
	}
	for k, v := range h2 {
		h.AddWithPrefix(p, k, v)
	}
}

// Assertion (usually a string, but could be a dictionary when using logical operators)
type Assertion interface{}

// StepAssertions contains step assertions
type StepAssertions struct {
	Assertions []Assertion `json:"assertions,omitempty" yaml:"assertions,omitempty"`
}

type TestsXML struct {
	XMLName    xml.Name       `xml:"testsuites" json:"-" yaml:"-"`
	TestSuites []TestSuiteXML `xml:"testsuite" json:"test_suites"`
}

type Tests struct {
	TestSuites       []TestSuite `json:"test_suites" yml:"tests_suites"`
	Status           Status      `json:"status" yml:"status"`
	NbTestsuitesFail int         `json:"nbTestsuitesFail"  yaml:"-"`
	NbTestsuitesPass int         `json:"nbTestsuitesPass"  yaml:"-"`
	NbTestsuitesSkip int         `json:"nbTestsuitesSkip"  yaml:"-"`
	Duration         float64     `json:"duration" yaml:"-"`
	Start            time.Time   `json:"start" yaml:"-"`
	End              time.Time   `json:"end" yaml:"-"`
}

// TestSuite is a single JUnit test suite which may contain many
// testcases.
type TestSuiteXML struct {
	XMLName   xml.Name      `xml:"testsuite" json:"-" yaml:"-"`
	Disabled  int           `xml:"disabled,attr,omitempty" json:"disabled" yaml:""`
	Errors    int           `xml:"errors,attr,omitempty" json:"errors" yaml:"-"`
	Failures  int           `xml:"failures,attr,omitempty" json:"failures" yaml:"-"`
	Hostname  string        `xml:"hostname,attr,omitempty" json:"hostname" yaml:"-"`
	ID        string        `xml:"id,attr,omitempty" json:"id" yaml:"-"`
	Name      string        `xml:"name,attr" json:"name" yaml:"name"`
	Package   string        `xml:"package,attr,omitempty" json:"package" yaml:"-"`
	Skipped   int           `xml:"skipped,attr,omitempty" json:"skipped" yaml:"skipped,omitempty"`
	Total     int           `xml:"tests,attr" json:"total" yaml:"total,omitempty"`
	TestCases []TestCaseXML `xml:"testcase" json:"testcases" yaml:"testcases"`
	Version   string        `xml:"version,omitempty" json:"version" yaml:"version,omitempty"`
	Time      string        `xml:"time,attr,omitempty" json:"time" yaml:"-"`
	Timestamp string        `xml:"timestamp,attr,omitempty" json:"timestamp" yaml:"-"`
}

type TestSuiteInput struct {
	Name        string          `json:"name" yaml:"name"`
	Description string          `json:"description" yaml:"description"`
	TestCases   []TestCaseInput `json:"testcases" yaml:"testcases"`
	Vars        H               `json:"vars" yaml:"vars"`
	Secrets     []string        `json:"secrets" yaml:"secrets"`
}

type TestSuite struct {
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description,omitempty" yaml:"description"`
	TestCases   []TestCase `json:"testcases" yaml:"testcases"`
	Vars        H          `json:"vars" yaml:"vars"`
	Secrets     []string   `json:"secrets" yaml:"secrets"`

	// computed
	ShortName    string `json:"shortname" yaml:"-"`
	Filename     string `json:"filename" yaml:"-"`
	Filepath     string `json:"filepath" yaml:"-"`
	ComputedVars H      `json:"computed_vars" yaml:"-"`
	WorkDir      string `json:"workdir" yaml:"_"`
	Status       Status `json:"status" yaml:"status"`

	Duration float64   `json:"duration" yaml:"-"`
	Start    time.Time `json:"start" yaml:"-"`
	End      time.Time `json:"end" yaml:"-"`

	NbTestcasesFail int `json:"nbTestcasesFail"  yaml:"-"`
	NbTestcasesPass int `json:"nbTestcasesPass"  yaml:"-"`
	NbTestcasesSkip int `json:"nbTestcasesSkip"  yaml:"-"`
}

// TestCase is a single test case with its result.
type TestCaseXML struct {
	XMLName   xml.Name     `xml:"testcase" json:"-" yaml:"-"`
	Classname string       `xml:"classname,attr,omitempty" json:"classname" yaml:"-"`
	Errors    []FailureXML `xml:"error,omitempty" json:"errors" yaml:"errors,omitempty"`
	Failures  []FailureXML `xml:"failure,omitempty" json:"failures" yaml:"failures,omitempty"`
	Name      string       `xml:"name,attr" json:"name" yaml:"name"`
	Skipped   []Skipped    `xml:"skipped,omitempty" json:"skipped" yaml:"skipped,omitempty"`
	Systemout InnerResult  `xml:"system-out,omitempty" json:"systemout" yaml:"systemout,omitempty"`
	Systemerr InnerResult  `xml:"system-err,omitempty" json:"systemerr" yaml:"systemerr,omitempty"`
	Time      float64      `xml:"time,attr,omitempty" json:"time" yaml:"time,omitempty"`
	ID        string       `xml:"id,attr,omitempty" json:"id" yaml:"id"`
}

type TestCaseInput struct {
	Name         string            `json:"name" yaml:"name"`
	Vars         H                 `json:"vars" yaml:"vars"`
	Skip         []string          `json:"skip" yaml:"skip"`
	RawTestSteps []json.RawMessage `json:"steps" yaml:"steps"`
	ID           string            `json:"id" yaml:"id"`
}

type TestCase struct {
	TestCaseInput

	// Computed
	originalName string
	number       int
	Skipped      []Skipped `json:"skipped" yaml:"-"`
	Status       Status    `json:"status" yaml:"-"`

	Duration float64   `json:"duration" yaml:"-"`
	Start    time.Time `json:"start" yaml:"-"`
	End      time.Time `json:"end" yaml:"-"`

	testSteps       []TestStep       `json:"-" yaml:"-"`
	TestStepResults []TestStepResult `json:"results" yaml:"-"`
	TestSuiteVars   H                `json:"-" yaml:"-"`

	computedVars    H        `json:"-" yaml:"-"`
	computedVerbose []string `json:"-" yaml:"-"`
	// varsFlattener caches the flattened variable scope between the steps of this test case
	// (#2017). It belongs to the test case rather than to Venom so that nothing is shared between
	// test files, which keeps it correct as-is when files start running concurrently (#1983).
	varsFlattener varsFlattener `json:"-" yaml:"-"`
	// suite is the prepared form of TestSuiteVars, made once for a root test case and inherited by the inner cases of
	// its custom executors (#2475). Nil on a case built outside runTestCase, which then takes the full path.
	suite *suiteScope `json:"-" yaml:"-"`
	// varsOverSuite says that Vars holds only the case's own variables (the inputs of a custom executor, what its steps
	// add) and that the suite's variables, TestSuiteVars, are implied under them. It is set only on the inner case of a
	// custom executor whose root case has a prepared suite, so that the case does not hold a second copy of a scope of
	// thousands of entries (#2475, ADR-0011). fullVars builds the whole scope where a caller needs one.
	varsOverSuite bool `json:"-" yaml:"-"`

	// out is the console printer of the test file this case belongs to (#2021). It rides on the
	// test case because every function that prints already receives one — including RunTestStep
	// and RunUserExecutor, which are exported and shared with upstream, so threading a parameter
	// through them would break the library API and conflict on every upstream merge. Read it with
	// v.out(tc), which falls back to the untagged printer for a TestCase built outside a suite.
	out         *suiteOut `json:"-" yaml:"-"`
	IsExecutor  bool      `json:"-" yaml:"-"`
	IsEvaluated bool      `json:"-" yaml:"-"`

	// Source line numbers extracted from YAML parsing (1-indexed)
	SourceLine           int     `json:"-" yaml:"-"` // line of the testcase definition
	StepSourceLines      []int   `json:"-" yaml:"-"` // line of each step, zero-based, by step index
	AssertionSourceLines [][]int `json:"-" yaml:"-"` // [stepIdx][assertIdx] -> line number, both zero-based
}

type TestStepResult struct {
	Name              string            `json:"name"`
	Errors            []Failure         `json:"errors"`
	Skipped           []Skipped         `json:"skipped" yaml:"skipped"`
	Status            Status            `json:"status" yaml:"status"`
	Raw               interface{}       `json:"raw" yaml:"raw"`
	Interpolated      interface{}       `json:"interpolated" yaml:"interpolated"`
	Number            int               `json:"number" yaml:"number"`
	RangedIndex       int               `json:"rangedIndex" yaml:"rangedIndex"`
	RangedEnable      bool              `json:"rangedEnable" yaml:"rangedEnable"`
	InputVars         map[string]string `json:"inputVars" yaml:"-"`
	ComputedVars      H                 `json:"computedVars" yaml:"-"`
	ComputedInfo      []string          `json:"computedInfos" yaml:"-"`
	AssertionsApplied AssertionsApplied `json:"assertionsApplied" yaml:"-"`
	Retries           int               `json:"retries" yaml:"retries"`

	// lazyInputs is the scope InputVars is built from when the result is reported (materializeInputVars): holding the
	// layers keeps a step from retaining its own copy of a scope of thousands of entries (#2475).
	lazyInputs *scopeLayers

	Systemout string    `json:"systemout"`
	Systemerr string    `json:"systemerr"`
	Duration  float64   `json:"duration"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
}

// materializeInputVars fills InputVars from the scope the step ran with, once. The reports call it before they redact
// the variables; a result that has no scope pending, or was filled already, is left as it is.
func (ts *TestStepResult) materializeInputVars() {
	if ts.lazyInputs == nil {
		return
	}
	ts.InputVars = ts.lazyInputs.materialize()
	ts.lazyInputs = nil
}

func (ts *TestStepResult) appendError(err error) {
	ts.Errors = append(ts.Errors, Failure{Value: RemoveNotPrintableChar(err.Error())})
	ts.Status = StatusFail
}

// Append an error to a test step and its associated test case
func (ts *TestStepResult) appendFailure(failure ...Failure) {
	ts.Errors = append(ts.Errors, failure...)
}

// TestStep represents a testStep
type TestStep map[string]interface{}

func (t TestStep) IntValue(name string) (int, error) {
	out, err := cast.ToIntE(t[name])
	if err != nil {
		return -1, fmt.Errorf("attribute %q is not an integer", name)
	}
	return out, nil
}

func (t TestStep) StringValue(name string) (string, error) {
	out, err := cast.ToStringE(t[name])
	if err != nil {
		return "", fmt.Errorf("attribute %q is not an string", name)
	}
	return out, nil
}

func (t TestStep) StringSliceValue(name string) ([]string, error) {
	out, err := cast.ToStringE(t[name])
	if err != nil {
		out, err := cast.ToStringSliceE(t[name])
		if err != nil {
			return nil, fmt.Errorf("attribute %q is neither a string nor a string array", name)
		}
		return out, nil
	}
	// If string is empty, return an empty slice instead
	if len(out) == 0 {
		return []string{}, nil
	}
	return []string{out}, nil
}

// Range contains data related to iterable user values
type Range struct {
	Enabled    bool
	Items      []RangeData
	RawContent interface{} `json:"range"`
}

// RangeData contains a single iterable user value
type RangeData struct {
	Key   string
	Value interface{}
}

// Skipped contains data related to a skipped test.
type Skipped struct {
	Value string `xml:",cdata" json:"value" yaml:"value,omitempty"`
}

// Failure contains data related to a failed test.
type Failure struct {
	TestcaseClassname  string `xml:"-" json:"-" yaml:"-"`
	TestcaseName       string `xml:"-" json:"-" yaml:"-"`
	TestcaseLineNumber int    `xml:"-" json:"-" yaml:"-"`
	StepNumber         int    `xml:"-" json:"-" yaml:"-"`
	Assertion          string `xml:"-" json:"-" yaml:"-"`
	AssertionRequired  bool   `xml:"-" json:"-" yaml:"-"`
	Error              error  `xml:"-" json:"-" yaml:"-"`

	Value string `json:"value" yaml:"value,omitempty"`
}

type FailureXML struct {
	Value   string `xml:",cdata" json:"value" yaml:"value,omitempty"`
	Type    string `xml:"type,attr,omitempty" json:"type" yaml:"type,omitempty"`
	Message string `xml:"message,attr,omitempty" json:"message" yaml:"message,omitempty"`
}

func newFailure(ctx context.Context, tc TestCase, stepNumber int, rangedIndex int, assertionIndex int, assertion string, err error) *Failure {
	filepath := StringVarFromCtx(ctx, "venom.testsuite.filepath")

	// Try to get line number from stored source lines
	lineNumber := tc.findSourceLine(stepNumber, assertionIndex)

	var value string
	if assertion != "" {
		value = fmt.Sprintf(`Testcase %q, step #%d-%d: Assertion %q failed. %s (%v:%d)`,
			tc.originalName,
			stepNumber,
			rangedIndex,
			RemoveNotPrintableChar(assertion),
			RemoveNotPrintableChar(err.Error()),
			filepath,
			lineNumber,
		)
	} else {
		value = fmt.Sprintf(`Testcase %q, step #%d-%d: %s (%v:%d)`,
			tc.originalName,
			stepNumber,
			rangedIndex,
			RemoveNotPrintableChar(err.Error()),
			filepath,
			lineNumber,
		)
	}

	failure := Failure{
		TestcaseClassname:  filepath,
		TestcaseName:       tc.Name,
		TestcaseLineNumber: lineNumber,
		StepNumber:         stepNumber,
		Assertion:          assertion,
		Error:              err,
		Value:              value,
	}

	return &failure
}

// findSourceLine returns the best source line number for a given step and assertion.
// It tries the assertion line first, then falls back to the step line.
// stepNumber is one-based, as users see it, while the lines are stored by zero-based step index (#1967);
// a number below 1 has no step line.
func (tc TestCase) findSourceLine(stepNumber int, assertionIndex int) int {
	stepIndex := stepNumber - 1
	// Try assertion-level line number
	if stepIndex >= 0 && assertionIndex >= 0 && stepIndex < len(tc.AssertionSourceLines) {
		assertions := tc.AssertionSourceLines[stepIndex]
		if assertionIndex < len(assertions) && assertions[assertionIndex] > 0 {
			return assertions[assertionIndex]
		}
	}
	// Fall back to step line
	if stepIndex >= 0 && stepIndex < len(tc.StepSourceLines) && tc.StepSourceLines[stepIndex] > 0 {
		return tc.StepSourceLines[stepIndex]
	}
	// Fall back to testcase line
	if tc.SourceLine > 0 {
		return tc.SourceLine
	}
	return 0
}

func (f Failure) String() string {
	if f.Value != "" {
		return color.YellowString(f.Value)
	}
	if f.Error != nil {
		return color.YellowString(f.Error.Error())
	}
	return ""
}

// InnerResult is used by TestCase
type InnerResult struct {
	Value string `xml:",cdata" json:"value" yaml:"value"`
}

type AssignStep struct {
	Assignments map[string]Assignment `json:"vars" yaml:"vars" mapstructure:"vars"`
}

type Assignment struct {
	From    string      `json:"from" yaml:"from"`
	Regex   string      `json:"regex" yaml:"regex"`
	Default interface{} `json:"default" yaml:"default"`
}

// RemoveNotPrintableChar removes not printable character from a string
func RemoveNotPrintableChar(in string) string {
	m := func(r rune) rune {
		if unicode.IsPrint(r) || unicode.IsSpace(r) || unicode.IsPunct(r) {
			return r
		}
		return ' '
	}
	return strings.Map(m, in)
}

var (
	Red    = color.New(color.FgRed).SprintFunc()
	Yellow = color.New(color.FgYellow).SprintFunc()
	Green  = color.New(color.FgGreen).SprintFunc()
	Cyan   = color.New(color.FgCyan).SprintFunc()
	Gray   = color.New(color.Attribute(90)).SprintFunc()
)
