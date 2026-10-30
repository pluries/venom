package venom

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gosimple/slug"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// suiteCaptureLogs routes the package logger to a buffer at debug level, so a test can read what Parse logs. The test
// logger is put back afterwards.
func suiteCaptureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	l := logrus.New()
	l.SetOutput(buf)
	l.SetLevel(logrus.DebugLevel)
	l.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true, DisableQuote: true})
	logger = logrus.NewEntry(l)
	t.Cleanup(func() { InitTestLogger(t) })
	return buf
}

// suiteWriteLib writes a user executor file named <name>.yml, declaring executor <executor>, under dir/lib and returns
// the lib directory.
func suiteWriteLib(t *testing.T, dir, name, executor string) string {
	t.Helper()
	lib := filepath.Join(dir, "lib")
	require.NoError(t, os.MkdirAll(lib, 0o755))
	content := "input:\n  who: world\nsteps:\n- type: fake\n"
	if executor != "" {
		content = "executor: " + executor + "\n" + content
	}
	writeSuiteFile(t, lib, name+".yml", content)
	return lib
}

// suitePassingYAML is a suite that parses: one test case, one step of the fake executor.
const suitePassingYAML = "name: fine\ntestcases:\n- name: ok\n  steps:\n  - type: fake\n"

// TestParseErrors pins each error Parse can return, in the order it meets them — files, reading, user executors,
// parsing — and how many suites are left in v.Tests when it does.
func TestParseErrors(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, v *Venom, dir string) // writes the files under dir, which Parse is given
		wantErr    string                                   // $DIR stands for dir
		wantSuites int
	}{
		{
			name:    "no YAML file in the path",
			setup:   func(t *testing.T, _ *Venom, dir string) { writeSuiteFile(t, dir, "notes.txt", "not a suite") },
			wantErr: "no YAML (*.yml or *.yaml) file found or defined",
		},
		{
			name:    "a readFiles error is returned as is",
			setup:   func(t *testing.T, _ *Venom, dir string) { writeSuiteFile(t, dir, "bad.yml", "name: [\n") },
			wantErr: `error while unmarshal file "$DIR/bad.yml": error converting YAML to JSON: yaml: line 1: did not find expected node content`,
		},
		{
			name: "a user executor file without an executor key",
			setup: func(t *testing.T, v *Venom, dir string) {
				writeSuiteFile(t, dir, "suite.yml", suitePassingYAML)
				v.LibDir = suiteWriteLib(t, dir, "anonymous", "")
			},
			wantErr:    `unable to register user executors: missing key 'executor' in "$DIR/lib/anonymous.yml"`,
			wantSuites: 1,
		},
		{
			name: "two user executor files declaring the same executor",
			setup: func(t *testing.T, v *Venom, dir string) {
				writeSuiteFile(t, dir, "suite.yml", suitePassingYAML)
				suiteWriteLib(t, dir, "a", "dup")
				v.LibDir = suiteWriteLib(t, dir, "b", "dup")
			},
			wantErr: `unable to register user executors: unable to register user executor "dup" from file "$DIR/lib/b.yml": ` +
				`executor "dup" already exists (from file "$DIR/lib/a.yml")`,
			wantSuites: 1,
		},
		{
			name: "a parseTestSuite error is returned as is: an unknown executor type",
			setup: func(t *testing.T, _ *Venom, dir string) {
				writeSuiteFile(t, dir, "suite.yml", "name: s\ntestcases:\n- name: c\n  steps:\n  - type: nope\n")
			},
			wantErr:    `user executor "nope" not found - loaded executors are: []`,
			wantSuites: 1,
		},
		{
			name: "the unknown executor error lists the user executors",
			setup: func(t *testing.T, v *Venom, dir string) {
				writeSuiteFile(t, dir, "suite.yml", "name: s\ntestcases:\n- name: c\n  steps:\n  - type: nope\n")
				v.LibDir = suiteWriteLib(t, dir, "hello", "hello")
			},
			wantErr:    `user executor "nope" not found - loaded executors are: [hello]`,
			wantSuites: 1,
		},
		{
			name: "a failing second suite: the first one is already read and parsed",
			setup: func(t *testing.T, _ *Venom, dir string) {
				writeSuiteFile(t, dir, "a.yml", suitePassingYAML)
				writeSuiteFile(t, dir, "b.yml", "name: s\ntestcases:\n- name: c\n  steps:\n  - type: nope\n")
			},
			wantErr:    `user executor "nope" not found - loaded executors are: []`,
			wantSuites: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := suiteNewVenom(t, 0)
			// The suite-relative lib is looked up in the working directory (#2031 D64): keep it empty.
			t.Chdir(t.TempDir())
			dir := t.TempDir()
			tt.setup(t, v, dir)

			err := v.Parse(context.Background(), []string{dir})

			require.Error(t, err)
			assert.Equal(t, suiteReplaceDir(tt.wantErr, dir), err.Error())
			assert.Len(t, v.Tests.TestSuites, tt.wantSuites)
		})
	}
}

// suiteReplaceDir expands $DIR in s.
func suiteReplaceDir(s, dir string) string { return os.Expand(s, func(string) string { return dir }) }

// TestParseSetsTestCaseIdentity pins what Parse sets on each suite and test case: venom.testsuite, the test case
// number (1-based, per suite), the slugged name, the original name, and venom.testcase. Nothing is run: no status.
func TestParseSetsTestCaseIdentity(t *testing.T) {
	v, _ := suiteNewVenom(t, 0)
	dir := t.TempDir()
	writeSuiteFile(t, dir, "a.yml", "name: Alpha\ntestcases:\n"+
		"- name: My First Case\n  steps:\n  - type: fake\n"+
		"- name: Déjà vu, again!\n  steps:\n  - type: fake\n")
	writeSuiteFile(t, dir, "b.yml", "name: Beta\ntestcases:\n- name: only\n  steps:\n  - type: fake\n")

	require.NoError(t, v.Parse(context.Background(), []string{dir}))

	type identity struct {
		number         int
		name, original string
		vars           interface{}
	}
	want := map[string][]identity{
		"Alpha": {{1, "My-First-Case", "My First Case", "My-First-Case"}, {2, "Deja-vu-again", "Déjà vu, again!", "Deja-vu-again"}},
		"Beta":  {{1, "only", "only", "only"}},
	}
	require.Len(t, v.Tests.TestSuites, 2)
	for i := range v.Tests.TestSuites {
		ts := &v.Tests.TestSuites[i]
		assert.Equal(t, ts.Name, ts.Vars["venom.testsuite"])
		got := []identity{}
		for _, tc := range ts.TestCases {
			got = append(got, identity{tc.number, tc.Name, tc.originalName, tc.Vars["venom.testcase"]})
			assert.Equal(t, Status(""), tc.Status)
			assert.False(t, tc.IsEvaluated)
		}
		assert.Equal(t, want[ts.Name], got, ts.Name)
	}
}

// TestParseAcceptsUndefinedVariables pins that a suite using variables nobody defines still parses. parseTestCase never
// reports a used variable (#2031 D28), so Parse's "missing variables" filtering — the range builtins, the extracted
// variables, the run variables, the venom.* prefix — has nothing to filter and its error cannot be reached; there is no
// test of it because no input reaches it.
func TestParseAcceptsUndefinedVariables(t *testing.T) {
	v, _ := suiteNewVenom(t, 0)
	ts, _ := suiteParsed(t, v, `name: undefined
testcases:
- name: uses
  steps:
  - type: fake
    code: "{{.never.defined}}"
    info: "{{.not.there.either}}"
    assertions:
    - result.code ShouldEqual {{.nor.this}}
`)
	require.Len(t, ts.TestCases, 1)
	assert.Equal(t, "uses", ts.TestCases[0].Name)
}

// TestParseReadsTheSuiteLibFromTheWorkingDirectory pins where Parse looks for user executors. Besides LibDir, it reads
// "<venom.testsuite.workdir>/lib" — but taken from the run's variables, which never hold that key, so the directory is
// "lib" under the process's working directory, not the suite's (venom.go:437 fed by process.go:93, #2031 D64).
func TestParseReadsTheSuiteLibFromTheWorkingDirectory(t *testing.T) {
	const suite = "name: lib\ntestcases:\n- name: greet\n  steps:\n  - type: hello\n"
	tests := []struct {
		name    string
		prepare func(t *testing.T, v *Venom, dir string)
		wantErr string
	}{
		{
			name:    "the suite's own lib is not read from another working directory",
			prepare: func(t *testing.T, _ *Venom, _ string) { t.Chdir(t.TempDir()) },
			wantErr: `user executor "hello" not found - loaded executors are: []`,
		},
		{
			name:    "it is read when the working directory is the suite's",
			prepare: func(t *testing.T, _ *Venom, dir string) { t.Chdir(dir) },
		},
		{
			name: "LibDir is read from any working directory",
			prepare: func(t *testing.T, v *Venom, dir string) {
				t.Chdir(t.TempDir())
				v.LibDir = filepath.Join(dir, "lib")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := suiteNewVenom(t, 0)
			dir := t.TempDir()
			p := writeSuiteFile(t, dir, "suite.yml", suite)
			suiteWriteLib(t, dir, "hello", "hello")
			tt.prepare(t, v, dir)

			err := v.Parse(context.Background(), []string{p})

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
				assert.Empty(t, v.executorsUser)
				return
			}
			require.NoError(t, err)
			ux, ok := v.executorsUser["hello"].(UserExecutor)
			require.True(t, ok, "hello is registered as a user executor")
			assert.Equal(t, filepath.Join(dir, "lib", "hello.yml"), ux.Filename)
		})
	}
}

// TestParseLogs pins the two lines Parse logs per suite: "Parsing testsuite <path>" at info, and the suite's variable
// names at debug.
func TestParseLogs(t *testing.T) {
	v, _ := suiteNewVenom(t, 0)
	logs := suiteCaptureLogs(t)

	_, p := suiteParsed(t, v, "name: logged\nvars:\n  foo: bar\ntestcases:\n- name: ok\n  steps:\n  - type: fake\n")

	assert.Contains(t, logs.String(), "level=info msg=Parsing testsuite "+p+"\n")
	assert.Contains(t, logs.String(), "level=debug msg=Testsuite ("+p+") variables: ")
	assert.Regexp(t, `msg=Testsuite \(.*\) variables: .*\bfoo\b`, logs.String())
	assert.Regexp(t, `msg=Testsuite \(.*\) variables: .*\bvenom\.testsuite\b`, logs.String())
}

// TestFilterReallyMissingVars pins filterReallyMissingVars directly: Parse's own "missing variables" check can never
// reach it (TestParseAcceptsUndefinedVariables), so this is the only coverage of its filtering logic (#2031 S3).
func TestFilterReallyMissingVars(t *testing.T) {
	tests := []struct {
		name          string
		missingVars   []string
		extractedVars []string
		vars          map[string]string
		want          []string
	}{
		{
			name:        "range builtin variables are skipped regardless of extractedVars or vars",
			missingVars: []string{"value", "value0", "index", "key"},
			want:        []string{},
		},
		{
			name:          "a var extracted by exact equality is skipped",
			missingVars:   []string{"foo"},
			extractedVars: []string{"foo"},
			want:          []string{},
		},
		{
			name:          "a var extracted because an extractedVars entry is a prefix of it is skipped",
			missingVars:   []string{"foobar"},
			extractedVars: []string{"foo"},
			want:          []string{},
		},
		{
			name:          "the reverse prefix does not match: the var itself being a prefix of the entry is not enough",
			missingVars:   []string{"foo"},
			extractedVars: []string{"foobar"},
			want:          []string{"foo"},
		},
		{
			name:        "a var defined as an exact key of vars is skipped",
			missingVars: []string{"bar"},
			vars:        map[string]string{"bar": "1"},
			want:        []string{},
		},
		{
			name:        "a venom.* var not otherwise extracted is ignored",
			missingVars: []string{"venom.testcase"},
			want:        []string{},
		},
		{
			name:          "a venom.* var that is also extracted stays skipped, not appended twice",
			missingVars:   []string{"venom.testcase"},
			extractedVars: []string{"venom.testcase"},
			want:          []string{},
		},
		{
			name:        "a var matching none of the above is really missing, in missingVars order",
			missingVars: []string{"alpha", "beta"},
			want:        []string{"alpha", "beta"},
		},
		{
			name: "no missing vars gives an empty, non-nil result",
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterReallyMissingVars(tt.missingVars, tt.extractedVars, tt.vars)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseLogHidesSecretsInVariableNames proves that the debug line listing a suite's variables, written while the
// suite is parsed and before any run, hides a declared secret that reaches those names through a test case name, in
// its slug form too (#2132, code review).
func TestParseLogHidesSecretsInVariableNames(t *testing.T) {
	lower := slug.Lowercase
	slug.Lowercase = false
	t.Cleanup(func() { slug.Lowercase = lower })

	v, _ := suiteNewVenom(t, 0)
	v.RegisterExecutorBuiltin("zv", zeroValueExecutor{}) // its result fields become variables prefixed with the test case name
	buf := suiteCaptureLogs(t)
	pwd := "P@ssW0rd" + "+x"
	dir := t.TempDir()
	writeSuiteFile(t, dir, "s.yml", "name: s\nvars:\n  pwd: \""+pwd+"\"\nsecrets:\n  - pwd\ntestcases:\n"+
		"- name: login as {{.pwd}}\n  steps:\n  - type: zv\n")

	require.NoError(t, v.Parse(context.Background(), []string{dir}))
	require.Contains(t, buf.String(), "variables:", "the debug line is written")
	assert.NotContains(t, buf.String(), slug.Make(pwd))
	assert.NotContains(t, buf.String(), pwd)
}
