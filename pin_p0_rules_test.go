package venom

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseAssertions pins how one assertion line is read (RULE-001): the key is looked up in the dumped input, the
// operator in the registry, a "Must" prefix is the same test with the required flag set, and each way a line can be
// refused has its own message.
func TestParseAssertions(t *testing.T) {
	input := map[string]interface{}{"count": 3, "name": "venom"}
	tests := []struct {
		name         string
		line         string
		wantErr      string
		wantActual   interface{}
		wantArgs     []interface{}
		wantRequired bool
	}{
		{name: "a key found in the input", line: "count ShouldEqual 3", wantActual: 3, wantArgs: []interface{}{3}},
		{name: "a Must prefix sets the required flag", line: "count MustEqual 3", wantActual: 3, wantArgs: []interface{}{3}, wantRequired: true},
		{name: "a text argument", line: `name ShouldEqual venom`, wantActual: "venom", wantArgs: []interface{}{"venom"}},
		{name: "a quoted argument stays one argument", line: `name ShouldEqual "ven om"`, wantActual: "venom", wantArgs: []interface{}{"ven om"}},
		{name: "an operator with no argument", line: "count ShouldBeGreaterThanOrEqualTo", wantActual: 3, wantArgs: []interface{}{}},
		{name: "an unknown key reads as nil", line: "missing ShouldEqual x", wantActual: nil, wantArgs: []interface{}{"x"}},
		{name: "an unsupported operator", line: "count ShouldFly 3", wantErr: "assertion not supported"},
		{name: "a Must on an unsupported operator", line: "count MustFly 3", wantErr: "assertion not supported"},
		{name: "a line with only a key", line: "count", wantErr: "assertion syntax error"},
		{name: "an empty line", line: "", wantErr: "assertion syntax error"},
		{name: "an argument that does not match the type of the value", line: "count ShouldEqual abc", wantErr: "mismatched type between 'count' and 'abc'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAssertions(context.Background(), tt.line, input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantActual, got.Actual)
			assert.Equal(t, tt.wantArgs, got.Args)
			assert.Equal(t, tt.wantRequired, got.Required)
			assert.NotNil(t, got.Func)
		})
	}
}

// rollupSuites writes the suites a roll-up case names, each in its own file, and parses them all.
func rollupSuites(t *testing.T, v *Venom, kinds ...string) {
	t.Helper()
	bodies := map[string]string{
		"pass": "name: s\ntestcases:\n- name: c\n  steps:\n  - type: fake\n    code: 0\n    assertions:\n    - result.code ShouldEqual 0\n",
		"fail": "name: s\ntestcases:\n- name: c\n  steps:\n  - type: fake\n    code: 1\n    assertions:\n    - result.code ShouldEqual 0\n",
		"skip": "name: s\ntestcases:\n- name: c\n  skip:\n  - 1 ShouldEqual 1\n  steps:\n  - type: fake\n    code: 0\n",
	}
	dir := t.TempDir()
	paths := []string{}
	for i, k := range kinds {
		paths = append(paths, writeSuiteFile(t, dir, string(rune('a'+i))+".yml", bodies[k]))
	}
	require.NoError(t, v.Parse(context.Background(), paths))
}

// TestProcessRollsUpTheRunStatus pins the status of the whole run (RULE-013): FAIL when any suite failed, SKIP only
// when every suite is skipped, PASS otherwise, and the counters it keeps for each status.
func TestProcessRollsUpTheRunStatus(t *testing.T) {
	tests := []struct {
		name                string
		kinds               []string
		want                Status
		fail, pass, skipped int
	}{
		{name: "one failing suite among passing ones", kinds: []string{"pass", "fail", "pass"}, want: StatusFail, fail: 1, pass: 2},
		{name: "a failing suite and a skipped one", kinds: []string{"skip", "fail"}, want: StatusFail, fail: 1, skipped: 1},
		{name: "every suite skipped", kinds: []string{"skip", "skip"}, want: StatusSkip, skipped: 2},
		{name: "a skipped suite and a passing one", kinds: []string{"skip", "pass"}, want: StatusPass, pass: 1, skipped: 1},
		{name: "every suite passes", kinds: []string{"pass", "pass"}, want: StatusPass, pass: 2},
		{name: "no suite at all", kinds: nil, want: StatusPass},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := suiteNewVenom(t, 0)
			v.OutputDir = t.TempDir()
			if len(tt.kinds) > 0 {
				rollupSuites(t, v, tt.kinds...)
			}

			require.NoError(t, v.Process(context.Background(), nil))

			assert.Equal(t, tt.want, v.Tests.Status)
			assert.Equal(t, tt.fail, v.Tests.NbTestsuitesFail)
			assert.Equal(t, tt.pass, v.Tests.NbTestsuitesPass)
			assert.Equal(t, tt.skipped, v.Tests.NbTestsuitesSkip)
		})
	}
}

// TestAnUntypedStepWithNoScriptPasses pins what happens to a step whose type is missing and that has no script or
// command, for example a typo such as "scripts:" (RULE-019): it is given no executor, does nothing, and passes. The
// open maintainer question is whether that is intended; this test pins the behaviour as it is, it does not endorse it.
func TestAnUntypedStepWithNoScriptPasses(t *testing.T) {
	v, _ := suiteNewVenom(t, 0)
	v.OutputDir = t.TempDir()
	p := writeSuiteFile(t, t.TempDir(), "untyped.yml", "name: u\ntestcases:\n- name: typo\n  steps:\n  - scripts: echo hi\n")
	require.NoError(t, v.Parse(context.Background(), []string{p}))

	require.NoError(t, v.Process(context.Background(), nil))

	require.Len(t, v.Tests.TestSuites, 1)
	tc := v.Tests.TestSuites[0].TestCases[0]
	assert.Equal(t, StatusPass, tc.Status)
	assert.Equal(t, StatusPass, v.Tests.TestSuites[0].Status)
	assert.Equal(t, StatusPass, v.Tests.Status)
	require.Len(t, tc.TestStepResults, 1)
	assert.Empty(t, tc.TestStepResults[0].Errors)
}
