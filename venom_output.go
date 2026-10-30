package venom

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	tap "github.com/mndrix/tap-go"
	"github.com/pkg/errors"
	yaml "go.yaml.in/yaml/v3"
)

func init() {
	color.NoColor = true
	if os.Getenv("IS_TTY") == "" || strings.ToLower(os.Getenv("IS_TTY")) == "true" || os.Getenv("IS_TTY") == "1" {
		color.NoColor = false
	}
}

// CleanUpSecrets This method tries to hide all the sensitive variables
func (v *Venom) CleanUpSecrets(testSuite TestSuite) TestSuite {
	suiteCtx := v.processSecrets(context.Background(), &testSuite, nil)
	testcaseCtxs := make([]context.Context, len(testSuite.TestCases))
	for i := range testSuite.TestCases {
		testcaseCtxs[i] = v.processSecrets(context.Background(), &testSuite, &testSuite.TestCases[i])
	}

	testSuite.Name = HideSensitive(suiteCtx, testSuite.Name)
	redactMapVars(suiteCtx, testSuite.Vars, testSuite.Secrets)
	// The suite's computed variables gather those of every test case, each key prefixed with the test case name.
	redactComputedVars(contextWithAllSecrets(append([]context.Context{suiteCtx}, testcaseCtxs...)), testSuite.ComputedVars)

	for i := range testSuite.TestCases {
		testCase := &testSuite.TestCases[i]
		ctx := testcaseCtxs[i]
		testCase.Name = HideSensitive(ctx, testCase.Name)
		redactMapVars(ctx, testCase.Vars, testSuite.Secrets)
		redactRawSteps(ctx, testCase.RawTestSteps)

		for j := range testCase.TestStepResults {
			redactStepResult(ctx, &testCase.TestStepResults[j], testSuite.Secrets)
		}
	}
	return testSuite
}

// redactComputedVars hides the secrets of the suite's computed variables in place with the venom.* rule for every
// value: a value is replaced only when it holds a secret, any other keeps its value and type. Keys are cleaned too.
func redactComputedVars(ctx context.Context, vars H) {
	secrets := contextSecrets(ctx)
	if len(vars) == 0 || len(secrets) == 0 {
		return
	}
	redactKeys(vars, secrets, func(_ string, val interface{}) interface{} {
		return redactLogArg(secrets, val)
	})
}

// contextWithAllSecrets returns a context holding the secrets of every context of ctxs, once each.
func contextWithAllSecrets(ctxs []context.Context) context.Context {
	all := []string{}
	seen := map[string]struct{}{}
	for _, c := range ctxs {
		for _, s := range contextSecrets(c) {
			if _, ok := seen[s]; !ok {
				seen[s] = struct{}{}
				all = append(all, s)
			}
		}
	}
	return context.WithValue(context.Background(), ContextKey("secrets"), all)
}

// redactRawSteps hides the secrets in the step definitions of a test case, which the JSON and YAML reports print, in
// place. A step is re-encoded only when something was hidden in it: re-encoding sorts the keys of every object, so a
// step without a secret keeps its exact bytes and key order. A step that does not decode is left as it is.
func redactRawSteps(ctx context.Context, steps []json.RawMessage) {
	for i, step := range steps {
		dec := json.NewDecoder(bytes.NewReader(step))
		dec.UseNumber()
		var value any
		if err := dec.Decode(&value); err != nil {
			continue
		}
		redacted, changed := redactJSONValue(ctx, value)
		if !changed {
			continue
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(redacted); err != nil {
			continue
		}
		steps[i] = bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	}
}

// redactJSONValue returns a decoded JSON value with HideSensitive applied to every string, number and object key, and
// whether anything changed. A number that holds a secret becomes the hidden string; booleans and null are kept.
func redactJSONValue(ctx context.Context, value any) (any, bool) {
	switch v := value.(type) {
	case string:
		hidden := HideSensitive(ctx, v)
		return hidden, hidden != v
	case json.Number:
		if hidden := HideSensitive(ctx, v.String()); hidden != v.String() {
			return hidden, true
		}
		return v, false
	case []any:
		changed := false
		for i, item := range v {
			var itemChanged bool
			v[i], itemChanged = redactJSONValue(ctx, item)
			changed = changed || itemChanged
		}
		return v, changed
	case map[string]any:
		return redactJSONObject(ctx, v)
	default:
		return value, false
	}
}

// redactJSONObject is redactJSONValue for an object: keys and values both go through HideSensitive.
func redactJSONObject(ctx context.Context, obj map[string]any) (map[string]any, bool) {
	out := make(map[string]any, len(obj))
	changed := false
	for k, item := range obj {
		key := HideSensitive(ctx, k)
		redacted, itemChanged := redactJSONValue(ctx, item)
		out[key] = redacted
		changed = changed || itemChanged || key != k
	}
	return out, changed
}

// redactStepResult hides the secrets of one step result in place: its name, its variables, its raw and interpolated
// forms, its outputs, its computed info, the text of its failures and the texts of its applied assertions, which the
// reports print.
func redactStepResult(ctx context.Context, result *TestStepResult, secrets []string) {
	result.Name = HideSensitive(ctx, result.Name)
	redactMapVars(ctx, result.ComputedVars, secrets)
	result.materializeInputVars()
	redactStringMap(ctx, result.InputVars, secrets)
	result.Raw = hideSensitiveBytes(ctx, result.Raw)
	result.Interpolated = hideSensitiveBytes(ctx, result.Interpolated)
	result.Systemout = HideSensitive(ctx, result.Systemout)
	result.Systemerr = HideSensitive(ctx, result.Systemerr)
	for k, info := range result.ComputedInfo {
		result.ComputedInfo[k] = HideSensitive(ctx, info)
	}
	for k := range result.Errors {
		result.Errors[k].Value = HideSensitive(ctx, result.Errors[k].Value)
	}
	redactAssertions(ctx, result.AssertionsApplied.Assertions)
}

// redactAssertions hides the secrets of the interpolated assertion texts the JSON and HTML reports print. An assertion
// that holds none is left as it is, with its type.
func redactAssertions(ctx context.Context, applied []AssertionApplied) {
	for i, a := range applied {
		if a.Assertion == nil {
			continue
		}
		text := fmt.Sprint(a.Assertion)
		if hidden := HideSensitive(ctx, text); hidden != text {
			applied[i].Assertion = hidden
		}
	}
}

// OutputResult output result to sdtout, files...
func (v *Venom) OutputResult() error {
	if v.OutputDir == "" {
		return nil
	}
	cleanedTs := []TestSuite{}
	for i := range v.Tests.TestSuites {
		ts := v.prepareSuiteOutput(i)
		cleanedTs = append(cleanedTs, ts)
		if err := v.writeSuiteResult(ts); err != nil {
			return err
		}
	}

	if v.HtmlReport {
		return v.writeHTMLReport(cleanedTs)
	}

	return nil
}

// prepareSuiteOutput drops the never-evaluated cases of v.Tests' suite i in place and returns the suite redacted.
func (v *Venom) prepareSuiteOutput(i int) TestSuite {
	tcFiltered := []TestCase{}
	for _, tc := range v.Tests.TestSuites[i].TestCases {
		if tc.IsEvaluated {
			tcFiltered = append(tcFiltered, tc)
		}
	}
	v.Tests.TestSuites[i].TestCases = tcFiltered
	return v.CleanUpSecrets(v.Tests.TestSuites[i])
}

// reportOf returns the run-wide results of v.Tests restricted to suites.
func (v *Venom) reportOf(suites []TestSuite) *Tests {
	return &Tests{
		TestSuites:       suites,
		Status:           v.Tests.Status,
		NbTestsuitesFail: v.Tests.NbTestsuitesFail,
		NbTestsuitesPass: v.Tests.NbTestsuitesPass,
		NbTestsuitesSkip: v.Tests.NbTestsuitesSkip,
		Duration:         v.Tests.Duration,
		Start:            v.Tests.Start,
		End:              v.Tests.End,
	}
}

// writeSuiteResult writes the result file of one suite in v.OutputFormat.
func (v *Venom) writeSuiteResult(ts TestSuite) error {
	data, err := v.marshalResult(v.reportOf([]TestSuite{ts}))
	if err != nil {
		return err
	}

	fname := strings.TrimSuffix(filepath.Base(ts.Filepath), filepath.Ext(ts.Filepath))
	filename := filepath.Join(v.OutputDir, "test_results_"+fname+"."+v.OutputFormat)
	if err := os.WriteFile(filename, data, 0o600); err != nil {
		return fmt.Errorf("Error while creating file %s: %v", filename, err)
	}
	v.PrintFunc("Writing file %s\n", filename) //nolint:errcheck // console progress message; a print failure has no control-flow impact (same idiom as venom.go's PrintFunc call sites)
	return nil
}

// marshalYAML renders v the way gopkg.in/yaml.v2's Marshal did: two spaces of indent, list items not indented
// under their key. yaml.v3 defaults to four spaces and indented lists.
func marshalYAML(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	enc.CompactSeqIndent()
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// marshalResult renders testsResult in v.OutputFormat; an unknown format renders nothing.
func (v *Venom) marshalResult(testsResult *Tests) ([]byte, error) {
	var data []byte
	var err error

	switch v.OutputFormat {
	case "json":
		data, err = json.MarshalIndent(testsResult, "", "  ")
		if err != nil {
			return nil, errors.Wrapf(err, "Error: cannot format output json (%s)", err)
		}
	case "tap":
		data, err = outputTapFormat(*testsResult)
		if err != nil {
			return nil, errors.Wrapf(err, "Error: cannot format output tap (%s)", err)
		}
	case "yml", "yaml":
		data, err = marshalYAML(testsResult)
		if err != nil {
			return nil, errors.Wrapf(err, "Error: cannot format output yaml (%s)", err)
		}
	case "xml":
		data, err = outputXMLFormat(*testsResult, v.Verbose)
		if err != nil {
			return nil, errors.Wrapf(err, "Error: cannot format output xml (%s)", err)
		}
	case "html":
		return nil, errors.New("Error: you have to use the --html-report flag")
	}
	return data, nil
}

// writeHTMLReport writes one HTML report covering suites.
func (v *Venom) writeHTMLReport(suites []TestSuite) error {
	data, err := outputHTML(v.reportOf(suites))
	if err != nil {
		return errors.Wrapf(err, "Error: cannot format output html")
	}
	filename := filepath.Join(v.OutputDir, computeOutputFilename("test_results.html"))
	v.PrintFunc("Writing html file %s\n", filename) //nolint:errcheck // console progress message; a print failure has no control-flow impact (same idiom as venom.go's PrintFunc call sites)
	if err := os.WriteFile(filename, data, 0o600); err != nil {
		return errors.Wrapf(err, "Error while creating file %s", filename)
	}
	return nil
}

func outputTapFormat(tests Tests) ([]byte, error) {
	tapValue := tap.New()
	buf := new(bytes.Buffer)
	tapValue.Writer = buf
	var total int
	for _, ts := range tests.TestSuites {
		for _, tc := range ts.TestCases {
			total++
			name := ts.Name + " / " + tc.Name
			if len(tc.Skipped) > 0 {
				tapValue.Skip(1, name)
				continue
			}

			for _, testStepResult := range tc.TestStepResults {
				if len(testStepResult.Errors) > 0 {
					tapValue.Fail(name)
					for _, e := range testStepResult.Errors {
						tapValue.Diagnosticf("Error: %s", e.Value)
					}
					continue
				}
			}
			tapValue.Pass(name)
		}
	}
	tapValue.Header(total)

	return buf.Bytes(), nil
}

func outputXMLFormat(tests Tests, verbose int) ([]byte, error) {
	testsXML := TestsXML{}

	for _, ts := range tests.TestSuites {
		tsXML := TestSuiteXML{
			Name:    ts.Name,
			Package: ts.Filepath,
			Time:    fmt.Sprintf("%f", ts.Duration),
		}

		for _, tc := range ts.TestCases {
			switch tc.Status {
			case StatusFail:
				tsXML.Errors++
			case StatusSkip:
				tsXML.Skipped++
			}
			tsXML.Total++

			failuresXML := []FailureXML{}
			systemout := InnerResult{}
			systemerr := InnerResult{}
			for _, result := range tc.TestStepResults {
				for _, failure := range result.Errors {
					failuresXML = append(failuresXML, FailureXML{
						Value: failure.Value,
					})
				}
				if len(result.Errors) > 0 {
					appendCleanValue(&systemout.Value, result.Systemout)
				} else if verbose > 1 {
					appendCleanValue(&systemout.Value, result.Systemout)
				}
				appendCleanValue(&systemerr.Value, result.Systemerr)
			}

			tcXML := TestCaseXML{
				Classname: ts.Filename,
				Errors:    failuresXML,
				Name:      tc.Name,
				Skipped:   tc.Skipped,
				Systemout: systemout,
				Systemerr: systemerr,
				Time:      tc.Duration,
				ID:        tc.ID,
			}
			tsXML.TestCases = append(tsXML.TestCases, tcXML)
		}
		testsXML.TestSuites = append(testsXML.TestSuites, tsXML)
	}

	dataxml, err := xml.MarshalIndent(testsXML, "", "  ")
	if err != nil {
		return nil, errors.Wrapf(err, "Error: cannot format xml output")
	}
	data := append([]byte(`<?xml version="1.0" encoding="utf-8"?>`), dataxml...)

	return data, nil
}

func appendCleanValue(dest *string, source string) {
	cleanedValue := strings.ReplaceAll(source, "\x03", "")
	*dest += cleanedValue
}
