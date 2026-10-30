package venom

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gosimple/slug"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanUpSecrets(t *testing.T) {
	v := New()
	ts := TestSuite{
		Name:    "HTTP Auth Test",
		Secrets: []string{"basic_auth_password"},
		Vars: H{
			"url":                 "http://127.0.0.1:8000",
			"basic_auth_user":     "testuser",
			"basic_auth_password": "my_secret",
		},
		TestCases: []TestCase{
			{
				TestCaseInput: TestCaseInput{
					Name: "GET with credentials",
					Vars: H{
						"basic_auth_password": "my_secret",
					},
				},
				TestStepResults: []TestStepResult{
					{
						Name: "GET-with-credentials",
						InputVars: map[string]string{
							"basic_auth_user":     "testuser",
							"basic_auth_password": "my_secret",
						},
						Raw: []byte(`type: http
basic_auth_password: "{{.basic_auth_password}}"
`),
						Interpolated: []byte(`type: http
basic_auth_password: my_secret
basic_auth_user: testuser
method: GET
`),
						Systemout: "Authorization: Basic dGVzdHVzZXI6bXlfc2VjcmV0",
					},
				},
			},
		},
	}

	cleaned := v.CleanUpSecrets(ts)

	assert.Equal(t, "__hidden__", cleaned.Vars["basic_auth_password"])
	assert.Equal(t, "testuser", cleaned.Vars["basic_auth_user"])

	result := cleaned.TestCases[0].TestStepResults[0]
	assert.Equal(t, "__hidden__", result.InputVars["basic_auth_password"])
	rawBytes, ok := result.Raw.([]byte)
	require.True(t, ok, "expected result.Raw to be []byte")
	assert.NotContains(t, string(rawBytes), "my_secret")
	interpolatedBytes, ok := result.Interpolated.([]byte)
	require.True(t, ok, "expected result.Interpolated to be []byte")
	assert.NotContains(t, string(interpolatedBytes), "my_secret")
	assert.NotContains(t, result.Systemout, "my_secret")
	assert.NotContains(t, result.Systemout, "dGVzdHVzZXI6bXlfc2VjcmV0")

	data, err := json.Marshal(cleaned)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "my_secret")
}

func TestHideSensitiveBytes(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKey("secrets"), []string{"my_secret"})

	redacted := hideSensitiveBytes(ctx, []byte("basic_auth_password: my_secret\n"))
	assert.Equal(t, "basic_auth_password: __hidden__\n", string(redacted))
}

func TestReplaceSecretsLongestFirst(t *testing.T) {
	secrets := []string{"foo", "foobar"}
	assert.Equal(t, "__hidden__", replaceSecrets("foobar", secrets))
}

func TestAppendDerivedSecretsBasicAuth(t *testing.T) {
	v := New()
	ts := TestSuite{
		Secrets: []string{"basic_auth_password"},
		Vars: H{
			"basic_auth_user":     "testuser",
			"basic_auth_password": "my_secret",
		},
	}
	ctx := v.processSecrets(context.Background(), &ts, nil)
	secrets, ok := ctx.Value(ContextKey("secrets")).([]string)
	require.True(t, ok, "expected secrets in context to be []string")

	token := base64.StdEncoding.EncodeToString([]byte("testuser:my_secret"))
	found := false
	for _, s := range secrets {
		if s == token {
			found = true
			break
		}
	}
	assert.True(t, found, "basic auth token should be registered as a derived secret")
	assert.Equal(t, "__hidden__", HideSensitive(ctx, token))
}

func TestAppendDerivedSecretsFromTestCaseVars(t *testing.T) {
	v := New()
	ts := TestSuite{
		Secrets: []string{"basic_auth_password"},
		Vars:    H{},
		TestCases: []TestCase{
			{
				TestCaseInput: TestCaseInput{
					Vars: H{
						"basic_auth_user":     "testuser",
						"basic_auth_password": "my_secret",
					},
				},
			},
		},
	}
	ctx := v.processSecrets(context.Background(), &ts, &ts.TestCases[0])

	token := base64.StdEncoding.EncodeToString([]byte("testuser:my_secret"))
	assert.Equal(t, "__hidden__", HideSensitive(ctx, token))
	assert.Equal(t, "__hidden__", HideSensitive(ctx, base64.StdEncoding.EncodeToString([]byte("my_secret"))))
	assert.NotContains(t, HideSensitive(ctx, "Authorization: Basic "+token), token)
}

func TestHideSensitiveBasicAuthHeaderInCleanupContext(t *testing.T) {
	v := New()
	ts := TestSuite{
		Secrets: []string{"basic_auth_password"},
		Vars: H{
			"basic_auth_user":     "testuser",
			"basic_auth_password": "my_secret",
		},
		TestCases: []TestCase{
			{TestCaseInput: TestCaseInput{Vars: H{"basic_auth_password": "my_secret"}}},
		},
	}
	ctx := v.processSecrets(context.Background(), &ts, &ts.TestCases[0])
	out := HideSensitive(ctx, "Authorization: Basic dGVzdHVzZXI6bXlfc2VjcmV0")
	assert.NotContains(t, out, "dGVzdHVzZXI6bXlfc2VjcmV0")
}

func TestProcessSecretsUsesTestSuiteVars(t *testing.T) {
	v := New()
	ts := TestSuite{
		Secrets: []string{"token"},
		Vars: H{
			"token": "secret-value",
		},
	}

	ctx := v.processSecrets(context.Background(), &ts, nil)
	assert.Equal(t, "__hidden__", HideSensitive(ctx, "secret-value"))
}

// outputSecret is the synthetic secret of outputFixture: it must not reach a report except where noted.
const outputSecret = "s3cr3t-synthetic"

// outputFixture is the run OutputResult reports on in the characterisation tests of #2031: two suites, the first with a
// passing case, a case that was never evaluated and a failing case whose output holds a secret; the second with one
// skipped case. Every call returns a fresh copy, because OutputResult rewrites v.Tests.
func outputFixture(dir, format string) (*Venom, *capture) {
	v := New()
	c := &capture{}
	v.PrintFunc = c.printf
	v.OutputDir = dir
	v.OutputFormat = format
	v.Tests = Tests{
		Status: StatusFail, NbTestsuitesFail: 1, NbTestsuitesPass: 0, NbTestsuitesSkip: 1, Duration: 1.5,
		TestSuites: []TestSuite{
			{
				Name: "first", Filename: "first.yml", Filepath: "suites/first.yml", Status: StatusFail, Duration: 0.25,
				Secrets: []string{"token"}, Vars: H{"token": outputSecret},
				TestCases: []TestCase{
					{
						TestCaseInput: TestCaseInput{Name: "passes", ID: "p1"}, IsEvaluated: true, Status: StatusPass,
						TestStepResults: []TestStepResult{{Name: "step", Systemout: "hello\x03", Systemerr: "warn"}},
					},
					{TestCaseInput: TestCaseInput{Name: "never evaluated"}},
					{
						TestCaseInput: TestCaseInput{Name: "fails"}, IsEvaluated: true, Status: StatusFail,
						TestStepResults: []TestStepResult{{
							Name: "step", Errors: []Failure{{Value: "boom " + outputSecret}}, Systemout: "got " + outputSecret,
						}},
					},
				},
			},
			{
				Name: "second", Filename: "second.yaml", Filepath: "other/second.yaml", Status: StatusSkip,
				TestCases: []TestCase{{
					TestCaseInput: TestCaseInput{Name: "skipped"}, IsEvaluated: true, Status: StatusSkip,
					Skipped: []Skipped{{Value: "condition not met"}},
				}},
			},
		},
	}
	return v, c
}

// reportedSuites is what OutputResult reports for outputFixture: the evaluated cases of each suite, secrets cleaned,
// computed from a fresh fixture the same way the function does it.
func reportedSuites() []TestSuite {
	fx, _ := outputFixture("", "")
	out := make([]TestSuite, 0, len(fx.Tests.TestSuites))
	for _, ts := range fx.Tests.TestSuites {
		kept := []TestCase{}
		for _, tc := range ts.TestCases {
			if tc.IsEvaluated {
				kept = append(kept, tc)
			}
		}
		ts.TestCases = kept
		out = append(out, fx.CleanUpSecrets(ts))
	}
	return out
}

// reportedTests wraps suites with the run-wide fields of outputFixture, as OutputResult does for each file.
func reportedTests(suites ...TestSuite) Tests {
	fx, _ := outputFixture("", "")
	return Tests{
		TestSuites: suites, Status: fx.Tests.Status, NbTestsuitesFail: fx.Tests.NbTestsuitesFail,
		NbTestsuitesPass: fx.Tests.NbTestsuitesPass, NbTestsuitesSkip: fx.Tests.NbTestsuitesSkip,
		Duration: fx.Tests.Duration, Start: fx.Tests.Start, End: fx.Tests.End,
	}
}

// readDir returns the files of dir by name.
func readDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	files := map[string]string{}
	for _, e := range entries {
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		files[e.Name()] = string(content)
	}
	return files
}

// TestOutputResultWritesOneFilePerSuite characterises OutputResult before #2031 splits it: one file per suite, named
// test_results_<file name without extension>.<format>, holding what the format's marshaller produces for that suite
// alone, and one "Writing file" line per file.
func TestOutputResultWritesOneFilePerSuite(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		verbose int
		render  func(t *testing.T, tests Tests) string
	}{
		{name: "json", format: "json", render: func(t *testing.T, tests Tests) string {
			data, err := json.MarshalIndent(tests, "", "  ")
			require.NoError(t, err)
			return string(data)
		}},
		{name: "yaml", format: "yaml", render: func(t *testing.T, tests Tests) string {
			data, err := marshalYAML(tests)
			require.NoError(t, err)
			return string(data)
		}},
		{name: "yml keeps its own extension", format: "yml", render: func(t *testing.T, tests Tests) string {
			data, err := marshalYAML(tests)
			require.NoError(t, err)
			return string(data)
		}},
		{name: "xml", format: "xml", render: func(t *testing.T, tests Tests) string {
			data, err := outputXMLFormat(tests, 0)
			require.NoError(t, err)
			return string(data)
		}},
		{name: "xml with -vv", format: "xml", verbose: 2, render: func(t *testing.T, tests Tests) string {
			data, err := outputXMLFormat(tests, 2)
			require.NoError(t, err)
			return string(data)
		}},
		{name: "tap", format: "tap", render: func(t *testing.T, tests Tests) string {
			data, err := outputTapFormat(tests)
			require.NoError(t, err)
			return string(data)
		}},
		// quirk: a format the switch does not know writes an empty file instead of failing
		{name: "an unknown format writes empty files", format: "csv", render: func(_ *testing.T, _ Tests) string { return "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			v, c := outputFixture(dir, tt.format)
			v.Verbose = tt.verbose
			require.NoError(t, v.OutputResult())

			suites := reportedSuites()
			first := "test_results_first." + tt.format
			second := "test_results_second." + tt.format
			assert.Equal(t, map[string]string{
				first:  tt.render(t, reportedTests(suites[0])),
				second: tt.render(t, reportedTests(suites[1])),
			}, readDir(t, dir))
			assert.Equal(t, "Writing file "+filepath.Join(dir, first)+"\n"+"Writing file "+filepath.Join(dir, second)+"\n", c.String())
		})
	}
}

// TestMarshalYAMLLayout pins the layout of the yaml report (#2898): two spaces of indent and list items at the level of
// their key, as gopkg.in/yaml.v2 wrote it. yaml.v3's defaults (four spaces, indented lists) would change every report.
func TestMarshalYAMLLayout(t *testing.T) {
	type item struct {
		Name string   `yaml:"name"`
		Tags []string `yaml:"tags"`
	}
	data, err := marshalYAML(map[string][]item{"items": {{Name: "a", Tags: []string{"x", "z"}}, {Name: "b", Tags: []string{}}}})
	require.NoError(t, err)
	assert.Equal(t, "items:\n- name: a\n  tags:\n  - x\n  - z\n- name: b\n  tags: []\n", string(data))
}

// TestOutputResultTapAndXMLContent pins the literal TAP and XML files of the first suite of outputFixture.
func TestOutputResultTapAndXMLContent(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		verbose int
		want    string
	}{
		{
			// quirks: the plan comes last, a failed case is also reported "ok" (3 lines for a plan of 2); the error value
			// is cleaned of secrets (#2037 AC 1)
			name: "tap", format: "tap",
			want: "ok 1 - first / passes\n" +
				"not ok 2 - first / fails\n" +
				"# Error: boom __hidden__\n" +
				"ok 3 - first / fails\n" +
				"TAP version 13\n" +
				"1..2\n",
		},
		{
			// the system-out of a passing step is left out below -vv; \x03 is stripped; the error value is cleaned (#2037 AC 1)
			name: "xml", format: "xml",
			want: `<?xml version="1.0" encoding="utf-8"?><testsuites>
  <testsuite errors="1" name="first" package="suites/first.yml" tests="2" time="0.250000">
    <testcase classname="first.yml" name="passes" id="p1">
      <system-out></system-out>
      <system-err><![CDATA[warn]]></system-err>
    </testcase>
    <testcase classname="first.yml" name="fails">
      <error><![CDATA[boom __hidden__]]></error>
      <system-out><![CDATA[got __hidden__]]></system-out>
      <system-err></system-err>
    </testcase>
  </testsuite>
</testsuites>`,
		},
		{
			name: "xml with -vv", format: "xml", verbose: 2,
			want: `<?xml version="1.0" encoding="utf-8"?><testsuites>
  <testsuite errors="1" name="first" package="suites/first.yml" tests="2" time="0.250000">
    <testcase classname="first.yml" name="passes" id="p1">
      <system-out><![CDATA[hello]]></system-out>
      <system-err><![CDATA[warn]]></system-err>
    </testcase>
    <testcase classname="first.yml" name="fails">
      <error><![CDATA[boom __hidden__]]></error>
      <system-out><![CDATA[got __hidden__]]></system-out>
      <system-err></system-err>
    </testcase>
  </testsuite>
</testsuites>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			v, _ := outputFixture(dir, tt.format)
			v.Verbose = tt.verbose
			require.NoError(t, v.OutputResult())
			assert.Equal(t, tt.want, readDir(t, dir)["test_results_first."+tt.format])
		})
	}
}

// TestOutputResultJSONContent decodes the JSON file of the first suite and checks the fields a report reader relies on.
func TestOutputResultJSONContent(t *testing.T) {
	dir := t.TempDir()
	v, _ := outputFixture(dir, "json")
	require.NoError(t, v.OutputResult())

	var got Tests
	require.NoError(t, json.Unmarshal([]byte(readDir(t, dir)["test_results_first.json"]), &got))
	require.Len(t, got.TestSuites, 1)
	ts := got.TestSuites[0]
	assert.Equal(t, "first", ts.Name)
	names := []string{}
	for _, tc := range ts.TestCases {
		names = append(names, tc.Name)
	}
	assert.Equal(t, []string{"passes", "fails"}, names, "only evaluated cases are reported")
	assert.Equal(t, "__hidden__", ts.Vars["token"])
	assert.Equal(t, "got __hidden__", ts.TestCases[1].TestStepResults[0].Systemout)
	assert.Equal(t, "boom __hidden__", ts.TestCases[1].TestStepResults[0].Errors[0].Value, "the failure text is cleaned of secrets (#2037 AC 1)")
	assert.Equal(t, "hello\x03", ts.TestCases[0].TestStepResults[0].Systemout, "\\x03 is only stripped in XML")
	assert.Equal(t, StatusFail, got.Status)
	assert.Equal(t, 1, got.NbTestsuitesFail)
	assert.Equal(t, 1, got.NbTestsuitesSkip)
	assert.InDelta(t, 1.5, got.Duration, 0)
}

// rawStepPlain is a step definition without a secret, with a key order and characters that a re-encoding would
// change (sorted keys, HTML escaping).
const rawStepPlain = `{"type":"exec","script":"test 1 < 2 && echo ok","assertions":["result.code ShouldEqual 0"]}`

// rawStepsSuite is a suite whose one evaluated test case holds two step definitions: the first carries the suite-level
// secret in a nested value and in a key, the second is rawStepPlain.
func rawStepsSuite() TestSuite {
	return TestSuite{
		Name: "raw", Filename: "raw.yml", Filepath: "suites/raw.yml", Status: StatusPass,
		Secrets: []string{"token"}, Vars: H{"token": outputSecret},
		TestCases: []TestCase{{
			TestCaseInput: TestCaseInput{Name: "steps", RawTestSteps: []json.RawMessage{
				json.RawMessage(`{"type":"http","headers":{"X-` + outputSecret + `":"v"},"body":{"list":["a","b ` + outputSecret + `"]},"timeout":1.50}`),
				json.RawMessage(rawStepPlain),
			}},
			IsEvaluated: true, Status: StatusPass,
		}},
	}
}

// TestCleanUpSecretsRawSteps proves that the step definitions the JSON and YAML reports print (testcases[].steps) are
// cleaned with the test case's secrets, here declared at the suite level (#2037, plan step 8), and that a step without
// a secret keeps its exact bytes.
func TestCleanUpSecretsRawSteps(t *testing.T) {
	wantHidden := map[string]any{
		"type": "http", "headers": map[string]any{"X-__hidden__": "v"},
		"body": map[string]any{"list": []any{"a", "b __hidden__"}}, "timeout": 1.5,
	}
	decoded := func(t *testing.T, raw []byte) any {
		t.Helper()
		var out any
		require.NoError(t, json.Unmarshal(raw, &out))
		return out
	}

	t.Run("CleanUpSecrets hides the secret in values and keys and leaves the other step alone", func(t *testing.T) {
		steps := New().CleanUpSecrets(rawStepsSuite()).TestCases[0].RawTestSteps
		require.Len(t, steps, 2)
		assert.NotContains(t, string(steps[0]), outputSecret)
		assert.Equal(t, wantHidden, decoded(t, steps[0]))
		assert.Contains(t, string(steps[0]), `"timeout":1.50`, "a number keeps its text")
		assert.Equal(t, rawStepPlain, string(steps[1]), "a step without a secret keeps its bytes")
	})

	t.Run("a secret written as a bare number becomes the hidden string, other numbers keep their text", func(t *testing.T) {
		ts := rawStepsSuite()
		ts.Secrets = []string{"pin"}
		ts.Vars = H{"pin": "424242"}
		ts.TestCases[0].RawTestSteps = []json.RawMessage{json.RawMessage(`{"type":"exec","pin":424242,"timeout":1.50}`)}
		steps := New().CleanUpSecrets(ts).TestCases[0].RawTestSteps
		assert.Equal(t, `{"pin":"__hidden__","timeout":1.50,"type":"exec"}`, string(steps[0]))
	})

	t.Run("a step that does not decode is left as it is", func(t *testing.T) {
		ts := rawStepsSuite()
		ts.TestCases[0].RawTestSteps = []json.RawMessage{json.RawMessage(`{"type":`)}
		steps := New().CleanUpSecrets(ts).TestCases[0].RawTestSteps
		assert.Equal(t, `{"type":`, string(steps[0]))
	})

	t.Run("the JSON report holds the cleaned steps and stays valid", func(t *testing.T) {
		dir := t.TempDir()
		v := New()
		v.PrintFunc = (&capture{}).printf
		v.OutputDir, v.OutputFormat = dir, "json"
		v.Tests.TestSuites = []TestSuite{rawStepsSuite()}
		require.NoError(t, v.OutputResult())

		report := readDir(t, dir)["test_results_raw.json"]
		assert.NotContains(t, report, outputSecret)
		var got Tests
		require.NoError(t, json.Unmarshal([]byte(report), &got))
		steps := got.TestSuites[0].TestCases[0].RawTestSteps
		require.Len(t, steps, 2)
		assert.Equal(t, wantHidden, decoded(t, steps[0]))
		assert.Equal(t, decoded(t, []byte(rawStepPlain)), decoded(t, steps[1]))
	})
}

// assertionsSuite is a suite whose one evaluated test case has a step result with three applied assertions: one holds
// the suite-level secret, one holds none, one is not a string.
func assertionsSuite() TestSuite {
	return TestSuite{
		Name: "assertions", Filename: "assertions.yml", Filepath: "suites/assertions.yml", Status: StatusPass,
		Secrets: []string{"token"}, Vars: H{"token": outputSecret},
		TestCases: []TestCase{{
			TestCaseInput: TestCaseInput{Name: "applied"},
			TestStepResults: []TestStepResult{{
				Name: "step",
				AssertionsApplied: AssertionsApplied{Assertions: []AssertionApplied{
					{Assertion: "result.systemout ShouldEqual " + outputSecret},
					{Assertion: "result.code ShouldEqual 0", IsOK: true},
					{Assertion: 42, IsOK: true},
				}},
			}},
			IsEvaluated: true, Status: StatusPass,
		}},
	}
}

// TestCleanUpSecretsAssertions proves that the assertion texts the JSON and HTML reports print
// (results[].assertionsApplied.assertions[].assertion) are cleaned with the test case's secrets, like the failure text
// and the step definitions (#2037, plan step 10). An assertion without a secret keeps its value and its type.
func TestCleanUpSecretsAssertions(t *testing.T) {
	t.Run("CleanUpSecrets hides the secret and leaves the other assertions alone", func(t *testing.T) {
		applied := New().CleanUpSecrets(assertionsSuite()).TestCases[0].TestStepResults[0].AssertionsApplied.Assertions
		require.Len(t, applied, 3)
		assert.Equal(t, "result.systemout ShouldEqual __hidden__", applied[0].Assertion)
		assert.Equal(t, "result.code ShouldEqual 0", applied[1].Assertion)
		assert.Equal(t, 42, applied[2].Assertion, "an assertion without a secret keeps its type")
	})

	t.Run("the JSON report holds the cleaned assertions", func(t *testing.T) {
		dir := t.TempDir()
		v := New()
		v.PrintFunc = (&capture{}).printf
		v.OutputDir, v.OutputFormat = dir, "json"
		v.Tests.TestSuites = []TestSuite{assertionsSuite()}
		require.NoError(t, v.OutputResult())

		report := readDir(t, dir)["test_results_assertions.json"]
		assert.NotContains(t, report, outputSecret)
		assert.Contains(t, report, "result.systemout ShouldEqual __hidden__")
	})
}

// TestCleanUpSecretsRunSecret proves that a secret the run declares (AddRunSecrets, from .venomrc) and a run variable
// holds is hidden in the reports like a suite-level one, for a suite that declares no secret itself (#2125).
func TestCleanUpSecretsRunSecret(t *testing.T) {
	InitTestLogger(t)
	p := writeSuiteFile(t, t.TempDir(), "s.yml", "name: s\ntestcases:\n- name: a\n")
	v := New()
	v.AddVariables(map[string]interface{}{"token": outputSecret})
	v.AddRunSecrets([]string{"token"})
	require.NoError(t, v.readFiles(context.Background(), []string{p}))
	require.Len(t, v.Tests.TestSuites, 1)
	ts := v.Tests.TestSuites[0]
	ts.TestCases[0].TestStepResults = []TestStepResult{{Name: "step", Systemout: "got " + outputSecret}}

	cleaned := v.CleanUpSecrets(ts)

	assert.Equal(t, "__hidden__", cleaned.Vars["token"])
	assert.Equal(t, "got __hidden__", cleaned.TestCases[0].TestStepResults[0].Systemout)
}

// namesSuite is a suite whose name, first test case name and first step name hold the suite-level secret; the second
// test case and its step hold none.
func namesSuite() TestSuite {
	return TestSuite{
		Name: "suite-" + outputSecret, Filename: "names.yml", Filepath: "suites/names.yml", Status: StatusPass,
		Secrets: []string{"token"}, Vars: H{"token": outputSecret},
		TestCases: []TestCase{
			{
				TestCaseInput:   TestCaseInput{Name: "tc-" + outputSecret},
				TestStepResults: []TestStepResult{{Name: "step-" + outputSecret}},
				IsEvaluated:     true, Status: StatusPass,
			},
			{
				TestCaseInput:   TestCaseInput{Name: "plain"},
				TestStepResults: []TestStepResult{{Name: "plain-step"}},
				IsEvaluated:     true, Status: StatusPass,
			},
		},
	}
}

// TestCleanUpSecretsNames proves that a secret inside the suite name, a test case name or a step name is hidden in the
// reports, and that a name without a secret is unchanged (#2132 AC 1 and 4).
func TestCleanUpSecretsNames(t *testing.T) {
	t.Run("CleanUpSecrets hides the names that hold a secret", func(t *testing.T) {
		cleaned := New().CleanUpSecrets(namesSuite())
		assert.Equal(t, "suite-__hidden__", cleaned.Name)
		assert.Equal(t, "tc-__hidden__", cleaned.TestCases[0].Name)
		assert.Equal(t, "step-__hidden__", cleaned.TestCases[0].TestStepResults[0].Name)
		assert.Equal(t, "plain", cleaned.TestCases[1].Name)
		assert.Equal(t, "plain-step", cleaned.TestCases[1].TestStepResults[0].Name)
	})

	for _, format := range []string{"json", "xml", "tap"} {
		t.Run("the "+format+" report holds the hidden names", func(t *testing.T) {
			dir := t.TempDir()
			v := New()
			v.PrintFunc = (&capture{}).printf
			v.OutputDir, v.OutputFormat = dir, format
			v.Tests.TestSuites = []TestSuite{namesSuite()}
			require.NoError(t, v.OutputResult())

			report := readDir(t, dir)["test_results_names."+format]
			assert.NotContains(t, report, outputSecret)
			assert.Contains(t, report, "tc-__hidden__")
			assert.Contains(t, report, "plain")
		})
	}

	t.Run("the xml report names the suite and the test case hidden", func(t *testing.T) {
		dir := t.TempDir()
		v := New()
		v.PrintFunc = (&capture{}).printf
		v.OutputDir, v.OutputFormat = dir, "xml"
		v.Tests.TestSuites = []TestSuite{namesSuite()}
		require.NoError(t, v.OutputResult())

		report := readDir(t, dir)["test_results_names.xml"]
		assert.Contains(t, report, `name="suite-__hidden__"`)
		assert.Contains(t, report, `name="tc-__hidden__"`)
		assert.Contains(t, report, `name="plain"`)
	})
}

// TestCleanUpSecretsVenomVars proves that the venom.* variables of the report hide a secret held in the suite or test
// case name while the other venom.* values keep their value and type, and that a computed variable of the suite, whose
// key is prefixed with the test case name, has its key hidden (#2132 AC 2).
func TestCleanUpSecretsVenomVars(t *testing.T) {
	suite := func() TestSuite {
		ts := namesSuite()
		ts.Vars = H{
			"token": outputSecret, "venom.testsuite": ts.Name, "venom.testsuite.totalSteps": 2, "venom.libdir": "lib",
		}
		ts.TestCases[0].Vars = H{"token": outputSecret, "venom.testcase": ts.TestCases[0].Name, "venom.testsuite.totalSteps": 2}
		ts.ComputedVars = H{"tc-" + outputSecret + ".result.systemout": "ok"}
		return ts
	}

	t.Run("CleanUpSecrets", func(t *testing.T) {
		cleaned := New().CleanUpSecrets(suite())
		assert.Equal(t, "suite-__hidden__", cleaned.Vars["venom.testsuite"])
		assert.Equal(t, 2, cleaned.Vars["venom.testsuite.totalSteps"], "a venom.* number keeps its type")
		assert.Equal(t, "lib", cleaned.Vars["venom.libdir"])
		assert.Equal(t, "tc-__hidden__", cleaned.TestCases[0].Vars["venom.testcase"])
		assert.Equal(t, 2, cleaned.TestCases[0].Vars["venom.testsuite.totalSteps"])
		assert.Equal(t, H{"tc-__hidden__.result.systemout": "ok"}, cleaned.ComputedVars)
	})

	t.Run("the JSON report", func(t *testing.T) {
		dir := t.TempDir()
		v := New()
		v.PrintFunc = (&capture{}).printf
		v.OutputDir, v.OutputFormat = dir, "json"
		v.Tests.TestSuites = []TestSuite{suite()}
		require.NoError(t, v.OutputResult())

		report := readDir(t, dir)["test_results_names.json"]
		assert.NotContains(t, report, outputSecret)
		assert.Contains(t, report, `"venom.testsuite.totalSteps": 2`, "a venom.* number stays a JSON number")
	})
}

// TestCleanUpSecretsComputedVarsKeepTheirType proves that the suite's computed variables follow the venom.* rule for
// every value: a value is replaced only when it holds a secret, any other keeps its value and type (#2132, maker's
// decision on deviation 1).
func TestCleanUpSecretsComputedVarsKeepTheirType(t *testing.T) {
	ts := namesSuite()
	ts.ComputedVars = H{"tc.result.code": 0, "tc.result.systemout": "got " + outputSecret, "tc.ok": true}

	cleaned := New().CleanUpSecrets(ts)

	assert.Equal(t, H{"tc.result.code": 0, "tc.result.systemout": "got __hidden__", "tc.ok": true}, cleaned.ComputedVars)
}

// TestOutputResultHTMLReport characterises the HTML branch: one test_results.html for all suites next to the per-suite
// files, announced before it is written; the "html" format on its own is refused.
func TestOutputResultHTMLReport(t *testing.T) {
	t.Run("the report holds every suite and comes after the per-suite files", func(t *testing.T) {
		dir := t.TempDir()
		v, c := outputFixture(dir, "json")
		v.HtmlReport = true
		require.NoError(t, v.OutputResult())

		all := reportedTests(reportedSuites()...)
		want, err := outputHTML(&all)
		require.NoError(t, err)
		files := readDir(t, dir)
		assert.Len(t, files, 3)
		assert.Equal(t, string(want), files["test_results.html"])
		assert.Equal(t, "Writing file "+filepath.Join(dir, "test_results_first.json")+"\n"+
			"Writing file "+filepath.Join(dir, "test_results_second.json")+"\n"+
			"Writing html file "+filepath.Join(dir, "test_results.html")+"\n", c.String())
	})
	t.Run("the report is written even with no suite", func(t *testing.T) {
		dir := t.TempDir()
		v, _ := outputFixture(dir, "json")
		v.Tests.TestSuites = nil
		v.HtmlReport = true
		require.NoError(t, v.OutputResult())
		assert.Equal(t, []string{"test_results.html"}, sortedKeys(readDir(t, dir)))
	})
	t.Run("the html format is refused at the first suite, even with the report on", func(t *testing.T) {
		dir := t.TempDir()
		v, c := outputFixture(dir, "html")
		v.HtmlReport = true
		require.EqualError(t, v.OutputResult(), "Error: you have to use the --html-report flag")
		assert.Empty(t, readDir(t, dir))
		assert.Empty(t, c.String())
	})
	t.Run("the html format is accepted when there is no suite", func(t *testing.T) {
		dir := t.TempDir()
		v, _ := outputFixture(dir, "html")
		v.Tests.TestSuites = nil
		require.NoError(t, v.OutputResult())
		assert.Empty(t, readDir(t, dir))
	})
}

// TestOutputResultHTMLReportName pins where the "already exists?" check of the report name looks: computeOutputFilename
// is given the bare name, so it checks the working directory, not the output directory.
func TestOutputResultHTMLReportName(t *testing.T) {
	t.Run("an existing report in the output directory is overwritten", func(t *testing.T) {
		t.Chdir(t.TempDir())
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "test_results.html"), []byte("old"), 0o600))
		v, _ := outputFixture(dir, "json")
		v.HtmlReport = true
		require.NoError(t, v.OutputResult())
		files := readDir(t, dir)
		assert.NotEqual(t, "old", files["test_results.html"])
		assert.NotContains(t, files, "test_results.0.html")
	})
	t.Run("a report in the working directory makes the name test_results.0.html in the output directory", func(t *testing.T) {
		cwd := t.TempDir()
		t.Chdir(cwd)
		require.NoError(t, os.WriteFile(filepath.Join(cwd, "test_results.html"), []byte("old"), 0o600))
		dir := t.TempDir()
		v, _ := outputFixture(dir, "json")
		v.HtmlReport = true
		require.NoError(t, v.OutputResult())
		assert.Equal(t, []string{"test_results.0.html", "test_results_first.json", "test_results_second.json"}, sortedKeys(readDir(t, dir)))
	})
}

// TestOutputResultFileNames pins how the per-suite file name is derived from the suite's Filepath.
func TestOutputResultFileNames(t *testing.T) {
	tests := []struct {
		name      string
		filepaths []string
		want      map[string]string // file name → the suite whose report it holds
	}{
		{name: "no extension", filepaths: []string{"dir/noext"}, want: map[string]string{"test_results_noext.json": "s0"}},
		{name: "only the last extension is dropped", filepaths: []string{"dir/a.b.yml"}, want: map[string]string{"test_results_a.b.json": "s0"}},
		{name: "an empty file path", filepaths: []string{""}, want: map[string]string{"test_results_..json": "s0"}},
		// quirk: two suites with the same base name in different directories write the same file; the last one wins
		{name: "the same base name in two directories", filepaths: []string{"a/same.yml", "b/same.yml"}, want: map[string]string{"test_results_same.json": "s1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			v := New()
			v.PrintFunc = (&capture{}).printf
			v.OutputDir, v.OutputFormat = dir, "json"
			for i, fp := range tt.filepaths {
				v.Tests.TestSuites = append(v.Tests.TestSuites, TestSuite{Name: "s" + string(rune('0'+i)), Filepath: fp})
			}
			require.NoError(t, v.OutputResult())

			got := map[string]string{}
			for name, content := range readDir(t, dir) {
				var decoded Tests
				require.NoError(t, json.Unmarshal([]byte(content), &decoded))
				got[name] = decoded.TestSuites[0].Name
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestOutputResultErrors pins the error text when a file cannot be written.
func TestOutputResultErrors(t *testing.T) {
	t.Run("a per-suite file", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing")
		v, _ := outputFixture(dir, "json")
		file := filepath.Join(dir, "test_results_first.json")
		require.EqualError(t, v.OutputResult(), "Error while creating file "+file+": open "+file+": no such file or directory")
	})
	t.Run("the html report, announced before the write fails", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing")
		v, c := outputFixture(dir, "json")
		v.Tests.TestSuites = nil
		v.HtmlReport = true
		file := filepath.Join(dir, "test_results.html")
		require.EqualError(t, v.OutputResult(), "Error while creating file "+file+": open "+file+": no such file or directory")
		assert.Equal(t, "Writing html file "+file+"\n", c.String())
	})
}

// TestOutputResultRewritesTheRunResults pins OutputResult's side effects on v.Tests: with an output directory it drops
// the cases that were never evaluated and, through the maps and slices it shares with the cleaned copy, redacts the
// secrets in place; without one it returns before touching anything.
func TestOutputResultRewritesTheRunResults(t *testing.T) {
	t.Run("with an output directory", func(t *testing.T) {
		v, _ := outputFixture(t.TempDir(), "json")
		require.NoError(t, v.OutputResult())
		first := v.Tests.TestSuites[0]
		assert.Len(t, first.TestCases, 2)
		assert.Equal(t, "__hidden__", first.Vars["token"])
		assert.Equal(t, "got __hidden__", first.TestCases[1].TestStepResults[0].Systemout)
	})
	t.Run("without an output directory", func(t *testing.T) {
		v, c := outputFixture("", "json")
		require.NoError(t, v.OutputResult())
		first := v.Tests.TestSuites[0]
		assert.Len(t, first.TestCases, 3)
		assert.Equal(t, outputSecret, first.Vars["token"])
		assert.Empty(t, c.String())
	})
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestCleanUpSecretsSluggedTestCaseName proves that a test case name is hidden even though venom turns it into a slug
// when it parses the suite (parseTestCases, slug.Lowercase false as InitLogger sets it): a secret with characters the slug
// changes ("@", "+", a space) is matched in its slug form too (#2132, code review).
func TestCleanUpSecretsSluggedTestCaseName(t *testing.T) {
	lower := slug.Lowercase
	slug.Lowercase = false
	t.Cleanup(func() { slug.Lowercase = lower })

	pwd := "P@ss w0rd" + "+x"
	ts := TestSuite{
		Name: "suite", Secrets: []string{"pwd"}, Vars: H{"pwd": pwd},
		TestCases: []TestCase{{
			TestCaseInput: TestCaseInput{Name: slug.Make("login as " + pwd)},
			IsEvaluated:   true, Status: StatusPass,
		}},
	}
	require.NotContains(t, ts.TestCases[0].Name, pwd, "the slug changes the secret, which is the point of the test")

	got := New().CleanUpSecrets(ts).TestCases[0].Name
	assert.NotContains(t, got, slug.Make(pwd))
	assert.Equal(t, "login-as-__hidden__", got)
}

// TestCleanUpSecretsShortSlugNotRegistered pins the guard on the slug form: a secret whose slug is shorter than four
// characters ("!!x" slugs to "x") is not matched in its slug form, so it cannot blank every such letter in a report.
func TestCleanUpSecretsShortSlugNotRegistered(t *testing.T) {
	lower := slug.Lowercase
	slug.Lowercase = false
	t.Cleanup(func() { slug.Lowercase = lower })

	ts := TestSuite{
		Name: "suite", Secrets: []string{"pwd"}, Vars: H{"pwd": "!!" + "x"},
		TestCases: []TestCase{{TestCaseInput: TestCaseInput{Name: "box"}, IsEvaluated: true, Status: StatusPass}},
	}
	assert.Equal(t, "box", New().CleanUpSecrets(ts).TestCases[0].Name)
}

// TestAppendDerivedSecretsURLEncoded proves that a declared secret is also hidden in the forms an URL carries it: the
// query form ("+" for a space) and the path form ("%20"). A secret with nothing to encode adds no extra entry, and a
// secret whose encoded form is shorter than minSlugSecret is not registered in that form (#2302).
func TestAppendDerivedSecretsURLEncoded(t *testing.T) {
	pwd := "P@ss w0rd" + "/x"
	secrets := appendDerivedSecrets(nil, map[string]struct{}{}, H{"pwd": pwd}, []string{"pwd"})

	assert.Contains(t, secrets, url.QueryEscape(pwd))
	assert.Contains(t, secrets, url.PathEscape(pwd))
	assert.NotEqual(t, url.QueryEscape(pwd), url.PathEscape(pwd), "the test needs a secret whose two forms differ")

	plain := appendDerivedSecrets(nil, map[string]struct{}{}, H{"pwd": "plainsecret"}, []string{"pwd"})
	assert.Equal(t, []string{base64.StdEncoding.EncodeToString([]byte("plainsecret")), "plainsecret"}, plain,
		"a secret without special characters has no URL-encoded form of its own: only the base64 and the slug remain")
}

// TestCleanUpSecretsURLEncodedInStepResult proves that a step result which echoes a secret inside an URL, as a query
// or a path component, is reported hidden (#2302).
func TestCleanUpSecretsURLEncodedInStepResult(t *testing.T) {
	pwd := "P@ss w0rd" + "/x"
	ctx := context.WithValue(context.Background(), ContextKey("secrets"),
		appendDerivedSecrets([]string{pwd}, map[string]struct{}{pwd: {}}, H{"pwd": pwd}, []string{"pwd"}))

	got := HideSensitive(ctx, "GET /login?pw="+url.QueryEscape(pwd)+" and /u/"+url.PathEscape(pwd)+"/x")
	assert.NotContains(t, got, url.QueryEscape(pwd))
	assert.NotContains(t, got, url.PathEscape(pwd))
	assert.Equal(t, "GET /login?pw=__hidden__ and /u/__hidden__/x", got)
}
