package venom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempDir(t *testing.T) (string, error) {
	dir := os.TempDir()
	name := path.Join(dir, randomString(5))
	if err := os.MkdirAll(name, os.FileMode(0o744)); err != nil {
		return "", err
	}
	t.Logf("Creating directory %s", name)
	return name, nil
}

func randomString(n int) string {
	letter := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))] //nolint:gosec // test-only random string generation, no cryptographic need
	}
	return string(b)
}

// getFilesPathCase is one Test_getFilesPath case: init builds the fixture and returns the paths to pass.
type getFilesPathCase struct {
	init    func(t *testing.T) ([]string, error)
	name    string
	want    []string
	wantErr bool
}

// Test_getFilesPath keeps its five original cases, inputs and assertions; #2031 moved the fixture builders and the
// checks into the helpers below so the test itself stays under the gocyclo threshold.
func Test_getFilesPath(t *testing.T) {
	InitTestLogger(t)
	// No explicit Seed: math/rand's global source has been auto-seeded since Go 1.20 (SA1019, #2028).

	tests := []getFilesPathCase{
		{name: "Check an empty directory", init: filesPathEmptyDir, wantErr: true},
		{name: "Check an directory with one yaml file", init: filesPathOneFile, want: []string{"d1.yml"}, wantErr: false},
		{
			name:    "Check an directory with one yaml file and a subdirectory with another file",
			init:    filesPathFileAndSubdir,
			want:    []string{"d1.yml", "d2.yml"},
			wantErr: false,
		},
		{name: "Check globstars", init: filesPathGlobstars, want: []string{"d1.yml", "d2.yml", "d3.yml", "d4.yml"}, wantErr: false},
		{
			name:    "Check globstars with duplicate files",
			init:    filesPathGlobstarsWithDuplicates,
			want:    []string{"d1.yml", "d2.yml", "d3.yml", "d4.yml"},
			wantErr: false,
		},
	}

	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			path, err := tt.init(t)
			if err != nil {
				t.Fatal(err)
			}
			got, err := getFilesPath(path)
			checkFilesPath(t, tt, got, err)
		})
	}
}

// checkFilesPath asserts what Test_getFilesPath always asserted: the error matches wantErr and, when it does, every
// wanted file name is the suffix of one returned path.
func checkFilesPath(t *testing.T, tt getFilesPathCase, got []string, err error) {
	t.Helper()
	if (err != nil) != tt.wantErr {
		t.Errorf("getFilesPath() name:%s error = %v, wantErr %v", tt.name, err, tt.wantErr)
		return
	}

	for _, f := range tt.want {
		var found bool
		for _, g := range got {
			if strings.HasSuffix(g, f) {
				found = true
			}
		}
		if !found {
			t.Errorf("getFilesPath() error want %v got %v", f, got)
		}
	}
}

// writeHello writes the "hello" content every Test_getFilesPath fixture file holds.
func writeHello(file string) error {
	return os.WriteFile(file, []byte("hello"), 0o600)
}

// mkSubdir creates a randomly named directory under parent, logged like the original fixtures did.
func mkSubdir(t *testing.T, parent string) (string, error) {
	dir := path.Join(parent, randomString(10))
	t.Logf("Creating directory %s", dir)
	return dir, os.Mkdir(dir, 0o744)
}

func filesPathEmptyDir(t *testing.T) ([]string, error) {
	dir, err := tempDir(t)
	return []string{dir}, err
}

func filesPathOneFile(t *testing.T) ([]string, error) {
	dir, err := tempDir(t)
	if err != nil {
		return nil, err
	}
	err = writeHello(path.Join(dir, "d1.yml"))
	return []string{dir}, err
}

func filesPathFileAndSubdir(t *testing.T) ([]string, error) {
	dir1, err := tempDir(t)
	if err != nil {
		return nil, err
	}
	if err = writeHello(path.Join(dir1, "d1.yml")); err != nil {
		return nil, err
	}
	dir2, err := mkSubdir(t, dir1)
	if err != nil {
		return nil, err
	}
	if err = writeHello(path.Join(dir2, "d2.yml")); err != nil {
		return nil, err
	}
	return []string{dir1, dir2}, err
}

// globstarTree builds dir1/d1.yml, dir1/dir2/d2.yml, dir1/dir2/d3.yml (d3 sits in dir2, not dir3, as it always did),
// dir1/dir2/dir3/ and dir1/dir2/dir3/dir4/d4.yml.
func globstarTree(t *testing.T) (dir1, dir2, dir3 string, err error) {
	if dir1, err = tempDir(t); err != nil {
		return "", "", "", err
	}
	if err = writeHello(path.Join(dir1, "d1.yml")); err != nil {
		return "", "", "", err
	}
	if dir2, err = mkSubdir(t, dir1); err != nil {
		return "", "", "", err
	}
	if err = writeHello(path.Join(dir2, "d2.yml")); err != nil {
		return "", "", "", err
	}
	if dir3, err = mkSubdir(t, dir2); err != nil {
		return "", "", "", err
	}
	if err = writeHello(path.Join(dir2, "d3.yml")); err != nil {
		return "", "", "", err
	}
	dir4, err := mkSubdir(t, dir3)
	if err != nil {
		return "", "", "", err
	}
	return dir1, dir2, dir3, writeHello(path.Join(dir4, "d4.yml"))
}

func filesPathGlobstars(t *testing.T) ([]string, error) {
	dir1, _, _, err := globstarTree(t)
	if err != nil {
		return nil, err
	}
	return []string{fmt.Sprintf("%s/**/*.yml", dir1)}, nil
}

func filesPathGlobstarsWithDuplicates(t *testing.T) ([]string, error) {
	dir1, dir2, dir3, err := globstarTree(t)
	if err != nil {
		return nil, err
	}
	return []string{dir2, dir3, fmt.Sprintf("%s/**/*.yml", dir1)}, nil
}

func Test_getFilesPath_files_order(t *testing.T) {
	dir1, err := tempDir(t)
	require.NoError(t, err)

	d1 := []byte("hello")
	require.NoError(t, os.WriteFile(path.Join(dir1, "a.yml"), d1, 0o600))

	d2 := []byte("hello")
	require.NoError(t, os.WriteFile(path.Join(dir1, "A.yml"), d2, 0o600))

	input := []string{dir1 + "/a.yml", dir1 + "/A.yml"}

	output, err := getFilesPath(input)
	require.NoError(t, err)
	require.Len(t, output, 2)
	t.Log(output)
	require.True(t, strings.HasSuffix(output[0], "a.yml"))
	require.True(t, strings.HasSuffix(output[1], "A.yml"))
}

func Test_extractLineNumbers(t *testing.T) {
	yaml := []byte(`name: HTTP testsuite
vars:
  input: "this my input"

testcases:
- name: get http testcase
  steps:
  - type: http
    method: GET
    url: https://example.com
    assertions:
    - result.statuscode ShouldEqual 200
    - result.body ShouldContainSubstring hello

- name: post http testcase
  steps:
  - type: http
    method: POST
    url: https://example.com/api
    assertions:
    - result.statuscode ShouldEqual 201
  - type: exec
    script: echo done
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldContainSubstring done
`)

	infos := extractLineNumbers(yaml)
	require.Len(t, infos, 2)

	// First testcase
	require.Equal(t, 6, infos[0].TestCaseLine)
	require.Len(t, infos[0].StepLines, 1)
	require.Equal(t, 8, infos[0].StepLines[0])
	require.Len(t, infos[0].AssertionLines, 1)
	require.Len(t, infos[0].AssertionLines[0], 2)
	require.Equal(t, 12, infos[0].AssertionLines[0][0])
	require.Equal(t, 13, infos[0].AssertionLines[0][1])

	// Second testcase
	require.Equal(t, 15, infos[1].TestCaseLine)
	require.Len(t, infos[1].StepLines, 2)
	require.Equal(t, 17, infos[1].StepLines[0])
	require.Equal(t, 22, infos[1].StepLines[1])
	require.Len(t, infos[1].AssertionLines, 2)
	require.Len(t, infos[1].AssertionLines[0], 1)
	require.Equal(t, 21, infos[1].AssertionLines[0][0])
	require.Len(t, infos[1].AssertionLines[1], 2)
	require.Equal(t, 25, infos[1].AssertionLines[1][0])
	require.Equal(t, 26, infos[1].AssertionLines[1][1])
}

func Test_findSourceLine(t *testing.T) {
	tc := TestCase{
		SourceLine:      6,
		StepSourceLines: []int{8, 17},
		AssertionSourceLines: [][]int{
			{12, 13},
			{21, 25, 26},
		},
	}

	// Steps are numbered from 1, the lines stored from 0.
	// Assertion line found
	require.Equal(t, 12, tc.findSourceLine(1, 0))
	require.Equal(t, 13, tc.findSourceLine(1, 1))
	require.Equal(t, 21, tc.findSourceLine(2, 0))
	require.Equal(t, 25, tc.findSourceLine(2, 1))
	require.Equal(t, 26, tc.findSourceLine(2, 2))

	// No assertion index -> step line
	require.Equal(t, 8, tc.findSourceLine(1, -1))
	require.Equal(t, 17, tc.findSourceLine(2, -1))

	// Out of bounds assertion -> step line
	require.Equal(t, 8, tc.findSourceLine(1, 5))
	require.Equal(t, 17, tc.findSourceLine(2, 10))

	// Out of bounds step -> testcase line
	require.Equal(t, 6, tc.findSourceLine(6, -1))

	// Step number below 1: no step or assertion line -> testcase line
	require.Equal(t, 6, tc.findSourceLine(0, 0))
	require.Equal(t, 6, tc.findSourceLine(-1, -1))

	// Empty testcase
	tc2 := TestCase{}
	require.Equal(t, 0, tc2.findSourceLine(1, 0))
}

// TestExtractLineNumbersCharacterisation pins extractLineNumbers before #2031 splits it (debt #1967 was an off-by-one
// here). Lines are 1-based and counted in the raw file, before interpolation. Every branch the function checks has a
// case: the parse, the document and root kinds, the testcases / steps / assertions lookups and their kinds.
func TestExtractLineNumbersCharacterisation(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []testCaseLineInfo
	}{
		// Nothing to read: nil, never an empty slice.
		{name: "invalid YAML returns nil", yaml: "name: [\n", want: nil},
		{name: "an empty document returns nil", yaml: "", want: nil},
		{name: "a document of comments and blank lines returns nil", yaml: "# a\n\n# b\n", want: nil},
		{name: "a root sequence returns nil", yaml: "- name: a\n  steps: []\n", want: nil},
		{name: "a root scalar returns nil", yaml: "testcases\n", want: nil},
		{name: "a document without testcases returns nil", yaml: "name: s\nvars:\n  a: b\n", want: nil},
		{name: "a testcases key nested under another key is not found", yaml: "vars:\n  testcases:\n  - name: a\n", want: nil},
		{name: "a null testcases returns nil", yaml: "name: s\ntestcases:\n", want: nil},
		{name: "a testcases mapping returns nil", yaml: "testcases:\n  name: a\n", want: nil},
		{name: "a testcases scalar returns nil", yaml: "testcases: a\n", want: nil},
		// quirk: an alias is an AliasNode, not a SequenceNode, although the engine expands it
		{name: "testcases given as an alias returns nil", yaml: "x: &tcs\n- name: a\ntestcases: *tcs\n", want: nil},
		{name: "a tab used as indentation is a parse error and returns nil", yaml: "testcases:\n\t- name: a\n", want: nil},

		// The testcases level.
		{name: "an empty testcases sequence returns an empty slice", yaml: "testcases: []\n", want: []testCaseLineInfo{}},
		{
			name: "a testcase without steps has only its line",
			yaml: "name: s\ntestcases:\n- name: a\n- name: b\n",
			want: []testCaseLineInfo{{TestCaseLine: 3}, {TestCaseLine: 4}},
		},
		{
			name: "a scalar testcase gives a zero entry that keeps the next index aligned",
			yaml: "testcases:\n- just a string\n- name: b\n",
			want: []testCaseLineInfo{{}, {TestCaseLine: 3}},
		},
		{
			name: "an aliased testcase gives a zero entry",
			yaml: "x: &tc\n  name: a\n  steps:\n  - type: exec\ntestcases:\n- *tc\n",
			want: []testCaseLineInfo{{}},
		},
		{
			name: "an anchor on the testcases sequence itself does not hide it",
			yaml: "testcases: &t\n- name: a\n",
			want: []testCaseLineInfo{{TestCaseLine: 2}},
		},
		{
			name: "testcases indented under their key, every level indented",
			yaml: "testcases:\n  - name: a\n    steps:\n      - type: exec\n        assertions:\n          - result.code ShouldEqual 0\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{4}, AssertionLines: [][]int{{6}}}},
		},
		{
			name: "the testcase line is its first key's line, whatever the key",
			yaml: "testcases:\n- steps:\n  - type: exec\n  name: a\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{3}, AssertionLines: [][]int{nil}}},
		},

		// The steps level.
		{name: "a null steps is ignored", yaml: "testcases:\n- name: a\n  steps:\n", want: []testCaseLineInfo{{TestCaseLine: 2}}},
		{name: "a steps mapping is ignored", yaml: "testcases:\n- name: a\n  steps:\n    type: exec\n", want: []testCaseLineInfo{{TestCaseLine: 2}}},
		{name: "a steps scalar is ignored", yaml: "testcases:\n- name: a\n  steps: exec\n", want: []testCaseLineInfo{{TestCaseLine: 2}}},
		{
			name: "an empty steps sequence gives empty, non-nil slices",
			yaml: "testcases:\n- name: a\n  steps: []\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{}, AssertionLines: [][]int{}}},
		},
		{
			name: "a scalar step and a nested-sequence step keep their line and have no assertions",
			yaml: "testcases:\n- name: a\n  steps:\n  - just a string\n  - - type: exec\n  - type: exec\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{4, 5, 6}, AssertionLines: [][]int{nil, nil, nil}}},
		},
		{
			name: "a step without assertions has a nil entry",
			yaml: "testcases:\n- name: a\n  steps:\n  - type: exec\n    script: echo\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{4}, AssertionLines: [][]int{nil}}},
		},
		// quirk: the engine expands the alias and its assertions; only the alias's own line is known here
		{
			name: "an aliased step records the alias line but not the anchored assertions",
			yaml: "x: &s\n  type: exec\n  assertions:\n  - a\ntestcases:\n- name: a\n  steps:\n  - *s\n",
			want: []testCaseLineInfo{{TestCaseLine: 6, StepLines: []int{8}, AssertionLines: [][]int{nil}}},
		},
		// quirk: same for assertions brought in by a merge key
		{
			name: "a step with a merge key does not see the merged assertions",
			yaml: "x: &s\n  type: exec\n  assertions:\n  - a\ntestcases:\n- name: a\n  steps:\n  - <<: *s\n    script: echo\n",
			want: []testCaseLineInfo{{TestCaseLine: 6, StepLines: []int{8}, AssertionLines: [][]int{nil}}},
		},

		// The assertions level.
		{
			name: "assertions null, scalar and mapping are ignored",
			yaml: "testcases:\n- name: a\n  steps:\n  - assertions:\n  - assertions: result.code ShouldEqual 0\n  - assertions:\n      a: b\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{4, 5, 6}, AssertionLines: [][]int{nil, nil, nil}}},
		},
		{
			name: "an empty assertions sequence gives an empty, non-nil slice",
			yaml: "testcases:\n- name: a\n  steps:\n  - type: exec\n    assertions: []\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{4}, AssertionLines: [][]int{{}}}},
		},
		{
			name: "assertions before the type key are still found",
			yaml: "testcases:\n- name: a\n  steps:\n  - assertions:\n    - a\n    - b\n    type: exec\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{4}, AssertionLines: [][]int{{5, 6}}}},
		},
		{
			name: "block scalar assertions are placed on their indicator line",
			yaml: "testcases:\n- name: a\n  steps:\n  - type: exec\n    script: |\n      echo 1\n      echo 2\n" +
				"    assertions:\n    - >\n      result.code\n      ShouldEqual 0\n    - |-\n      result.systemout ShouldEqual 1\n" +
				"    - \"result.err\n      ShouldBeEmpty\"\n    - result.systemerr\n      ShouldBeEmpty\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{4}, AssertionLines: [][]int{{9, 12, 14, 16}}}},
		},

		// Flow style, keys, comments, encodings.
		{
			name: "flow sequences put every node on line 1",
			yaml: "testcases: [{name: a, steps: [{type: exec, assertions: [x, y]}, {type: exec}]}, {name: b}]\n",
			want: []testCaseLineInfo{
				{TestCaseLine: 1, StepLines: []int{1, 1}, AssertionLines: [][]int{{1, 1}, nil}},
				{TestCaseLine: 1},
			},
		},
		{
			name: "flow assertions inside a block step share the key's line",
			yaml: "testcases:\n- name: a\n  steps:\n  - type: exec\n    assertions: [x, y]\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{4}, AssertionLines: [][]int{{5, 5}}}},
		},
		{
			name: "comments, blank lines and a document start marker are counted",
			yaml: "--- # start\n\n# comment\ntestcases:\n\n  # comment\n  - name: a\n    # comment\n\n    steps:\n    - type: exec\n" +
				"      assertions:\n      # comment\n      - a\n\n      - b\n",
			want: []testCaseLineInfo{{TestCaseLine: 7, StepLines: []int{11}, AssertionLines: [][]int{{14, 16}}}},
		},
		{
			name: "keys that look like '- name:' inside a step or a testcase are not steps nor testcases",
			yaml: "testcases:\n- name: a\n  vars:\n    list:\n    - name: v\n  steps:\n  - type: http\n    headers:\n    - name: h\n" +
				"    body:\n      steps:\n      - name: s\n    assertions:\n    - a\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{7}, AssertionLines: [][]int{{14}}}},
		},
		// quirk: the engine refuses a duplicate key, so this never reaches the lines in a run
		{
			name: "only the first testcases, steps and assertions keys are read",
			yaml: "testcases:\n- name: a\n  steps:\n  - assertions:\n    - a\n    assertions:\n    - b\n    - c\n  steps: []\n" +
				"testcases:\n- name: b\n- name: c\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{4}, AssertionLines: [][]int{{5}}}},
		},
		{
			name: "only the first document is read",
			yaml: "testcases:\n- name: a\n---\ntestcases:\n- name: b\n- name: c\n",
			want: []testCaseLineInfo{{TestCaseLine: 2}},
		},
		{name: "a first document without testcases returns nil", yaml: "---\nname: x\n---\ntestcases:\n- name: b\n", want: nil},
		{
			name: "CRLF line endings count like LF",
			yaml: "name: s\r\ntestcases:\r\n- name: a\r\n  steps:\r\n  - type: exec\r\n    assertions:\r\n    - result.code ShouldEqual 0\r\n",
			want: []testCaseLineInfo{{TestCaseLine: 3, StepLines: []int{5}, AssertionLines: [][]int{{7}}}},
		},
		{name: "a quoted testcases key is found", yaml: "\"testcases\":\n- name: a\n", want: []testCaseLineInfo{{TestCaseLine: 2}}},
		{name: "a byte order mark does not shift lines", yaml: "\xef\xbb\xbftestcases:\n- name: a\n", want: []testCaseLineInfo{{TestCaseLine: 2}}},
		{
			name: "number and null keys before testcases are skipped",
			yaml: "1: x\n~: y\ntestcases:\n- name: a\n",
			want: []testCaseLineInfo{{TestCaseLine: 4}},
		},
		{
			name: "a tab inside a quoted value is not indentation",
			yaml: "testcases:\n- name: \"a\tb\"\n  steps:\n  - type: exec\n",
			want: []testCaseLineInfo{{TestCaseLine: 2, StepLines: []int{4}, AssertionLines: [][]int{nil}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractLineNumbers([]byte(tt.yaml)))
		})
	}
}

// writeSuiteFile writes a synthetic suite file in dir and returns its path.
func writeSuiteFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

// suiteVarsWithoutRunValues returns the suite's variables without the venom.* ones readFiles adds (checked apart by
// TestReadFilesFillsTheTestSuite).
func suiteVarsWithoutRunValues(vars H) H {
	out := H{}
	for k, v := range vars {
		if !strings.HasPrefix(k, "venom.") {
			out[k] = v
		}
	}
	return out
}

// TestReadFilesFillsTheTestSuite pins every field readFiles sets on the TestSuite it appends, before #2031 splits it.
func TestReadFilesFillsTheTestSuite(t *testing.T) {
	InitTestLogger(t)
	dir := t.TempDir()
	p := writeSuiteFile(t, dir, "synthetic.suite.yml", `name: synthetic suite
description: a suite for the characterisation tests
vars:
  greeting: hello
secrets:
- greeting
testcases:
- name: case {{.greeting}}
  steps:
  - type: exec
    script: echo {{.greeting}}
    assertions:
    - result.code ShouldEqual 0
- name: second
`)

	v := New()
	before := time.Now().Truncate(time.Second)
	require.NoError(t, v.readFiles(context.Background(), []string{p}))
	after := time.Now()

	require.Len(t, v.Tests.TestSuites, 1)
	ts := v.Tests.TestSuites[0]
	assert.Equal(t, "synthetic suite", ts.Name)
	assert.Equal(t, "a suite for the characterisation tests", ts.Description)
	assert.Equal(t, []string{"greeting"}, ts.Secrets)
	assert.Equal(t, p, ts.Filepath)
	assert.Equal(t, "synthetic.suite.yml", ts.Filename)
	assert.Equal(t, "synthetic.suite", ts.ShortName, "only the last extension is trimmed")
	assert.Equal(t, dir, ts.WorkDir)
	assert.Empty(t, ts.Status)
	assert.Nil(t, ts.ComputedVars)

	// Lines come from the raw file; names and steps from the interpolated one.
	assert.Equal(t, []TestCase{
		{
			TestCaseInput: TestCaseInput{
				Name:         "case hello",
				RawTestSteps: []json.RawMessage{json.RawMessage(`{"assertions":["result.code ShouldEqual 0"],"script":"echo hello","type":"exec"}`)},
			},
			SourceLine:           8,
			StepSourceLines:      []int{10},
			AssertionSourceLines: [][]int{{13}},
		},
		{TestCaseInput: TestCaseInput{Name: "second"}, SourceLine: 14},
	}, ts.TestCases)

	// The two time values are the only ones that move between runs.
	rawDatetime, ok := ts.Vars["venom.datetime"].(string)
	require.True(t, ok, "venom.datetime is a string")
	datetime, err := time.Parse(time.RFC3339, rawDatetime)
	require.NoError(t, err)
	assert.False(t, datetime.Before(before) || datetime.After(after), "venom.datetime %v not in [%v, %v]", datetime, before, after)
	rawTimestamp, ok := ts.Vars["venom.timestamp"].(string)
	require.True(t, ok, "venom.timestamp is a string")
	timestamp, err := strconv.ParseInt(rawTimestamp, 10, 64)
	require.NoError(t, err)
	assert.True(t, timestamp >= before.Unix() && timestamp <= after.Unix(), "venom.timestamp %d", timestamp)
	delete(ts.Vars, "venom.datetime")
	delete(ts.Vars, "venom.timestamp")

	// quirk: ts.Vars replaces the suite's own vars (TestSuiteInput.Vars) with their dumped strings, dump metadata included
	assert.Equal(t, H{
		"greeting":                  "hello",
		"__Len__":                   "1",
		"__Type__":                  "Map",
		"venom.testsuite.workdir":   dir,
		"venom.testsuite.name":      "synthetic suite",
		"venom.testsuite.shortName": "synthetic.suite",
		"venom.testsuite.filename":  "synthetic.suite.yml",
		"venom.testsuite.filepath":  p,
	}, ts.Vars)
}

// TestReadFilesRunSecrets proves that the secret names the run declares (AddRunSecrets, from .venomrc) join each
// suite's own list when the files are read: the suite's names first, then the run names not already there, once
// each; without a run secret the suite's list is left exactly as the file declares it (#2125).
func TestReadFilesRunSecrets(t *testing.T) {
	tests := []struct {
		name    string
		content string
		run     []string
		want    []string
	}{
		{
			name:    "a run secret reaches a suite that declares none",
			content: "name: s\ntestcases:\n- name: a\n",
			run:     []string{"pwd"},
			want:    []string{"pwd"},
		},
		{
			name:    "a name declared by the suite and the run is listed once",
			content: "name: s\nsecrets:\n- pwd\ntestcases:\n- name: a\n",
			run:     []string{"pwd"},
			want:    []string{"pwd"},
		},
		{
			name:    "suite names first, then the run names not there yet, deduplicated",
			content: "name: s\nsecrets:\n- a\ntestcases:\n- name: a\n",
			run:     []string{"b", "a", "b"},
			want:    []string{"a", "b"},
		},
		{
			name:    "no run secret: the suite's list is unchanged",
			content: "name: s\nsecrets:\n- a\n- a\ntestcases:\n- name: a\n",
			want:    []string{"a", "a"},
		},
		{
			name:    "no run secret and none in the suite: the list stays nil",
			content: "name: s\ntestcases:\n- name: a\n",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitTestLogger(t)
			p := writeSuiteFile(t, t.TempDir(), "s.yml", tt.content)

			v := New()
			v.AddRunSecrets(tt.run)
			require.NoError(t, v.readFiles(context.Background(), []string{p}))

			require.Len(t, v.Tests.TestSuites, 1)
			assert.Equal(t, tt.want, v.Tests.TestSuites[0].Secrets)
		})
	}
}

// TestReadFilesSuiteVars pins how readFiles merges the run's variables (AddVariables) with the suite's vars block.
func TestReadFilesSuiteVars(t *testing.T) {
	tests := []struct {
		name      string
		global    map[string]interface{}
		content   string
		wantVars  H
		wantCases []string
	}{
		{
			name:      "no variables at all: only the venom ones",
			content:   "name: s\ntestcases:\n- name: a\n",
			wantVars:  H{},
			wantCases: []string{"a"},
		},
		{
			name:      "a suite var is added and interpolated into the file",
			content:   "name: s\nvars:\n  a: x\ntestcases:\n- name: \"{{.a}}\"\n",
			wantVars:  H{"a": "x", "__Len__": "1", "__Type__": "Map"},
			wantCases: []string{"x"},
		},
		{
			name:      "a run variable without a suite vars block adds no dump metadata",
			global:    map[string]interface{}{"a": "G"},
			content:   "name: s\ntestcases:\n- name: \"{{.a}}\"\n",
			wantVars:  H{"a": "G"},
			wantCases: []string{"G"},
		},
		{
			name:      "a run variable wins over the suite var of the same name",
			global:    map[string]interface{}{"a": "G"},
			content:   "name: s\nvars:\n  a: x\n  b: y\ntestcases:\n- name: \"{{.a}}-{{.b}}\"\n",
			wantVars:  H{"a": "G", "b": "y", "__Len__": "2", "__Type__": "Map"},
			wantCases: []string{"G-y"},
		},
		{
			name:    "suite vars become strings and complex ones are flattened",
			content: "name: s\nvars:\n  n: 1\n  b: true\n  l: [1, 2]\n  m: {k: v}\ntestcases:\n- name: \"{{.n}} {{.b}} {{.l}} {{.m.k}} {{.l.l1}}\"\n",
			wantVars: H{
				"n": "1", "b": "true",
				"l": "[1,2]", "l.__Len__": "2", "l.__Type__": "Array", "l.l0": "1", "l.l1": "2",
				"m": `{"k":"v"}`, "m.__Len__": "1", "m.__Type__": "Map", "m.k": "v",
				"__Len__": "4", "__Type__": "Map",
			},
			wantCases: []string{"1 true [1,2] v 2"},
		},
		{
			name:      "a suite var referencing another suite var is resolved",
			content:   "name: s\nvars:\n  n: 1\n  ref: \"{{.n}}-x\"\ntestcases:\n- name: \"{{.ref}}\"\n",
			wantVars:  H{"n": "1", "ref": "1-x", "__Len__": "2", "__Type__": "Map"},
			wantCases: []string{"1-x"},
		},
		// quirk: suite vars are interpolated with the suite vars only, and the file in a single pass
		{
			name:      "a suite var referencing a run variable is left unresolved",
			global:    map[string]interface{}{"glob": "G"},
			content:   "name: s\nvars:\n  g: \"{{.glob}}\"\ntestcases:\n- name: \"{{.g}}\"\n",
			wantVars:  H{"glob": "G", "g": "{{.glob}}", "__Len__": "1", "__Type__": "Map"},
			wantCases: []string{"{{.glob}}"},
		},
		// quirk: DumpStringPreserveCase drops the empty name, so readFiles' own `k == ""` guard never fires
		{
			name:      "a suite var with an empty name is dropped",
			content:   "name: s\nvars:\n  \"\": empty\n  a: x\ntestcases:\n- name: a\n",
			wantVars:  H{"a": "x", "__Len__": "1", "__Type__": "Map"},
			wantCases: []string{"a"},
		},
		// quirk: a run variable holding "{}" while the run's __Len__ is "0" counts as unset
		{
			name:      "a run variable '{}' with a run __Len__ of '0' is replaced by the suite var",
			global:    map[string]interface{}{"m": "{}", "__Len__": "0"},
			content:   "name: s\nvars:\n  m: x\ntestcases:\n- name: \"{{.m}}\"\n",
			wantVars:  H{"m": "x", "__Len__": "0", "__Type__": "Map"},
			wantCases: []string{"x"},
		},
		{
			name:      "a run variable '{}' without a run __Len__ of '0' wins",
			global:    map[string]interface{}{"m": "{}"},
			content:   "name: s\nvars:\n  m: x\ntestcases:\n- name: \"{{.m}}\"\n",
			wantVars:  H{"m": "{}", "__Len__": "1", "__Type__": "Map"},
			wantCases: []string{"{}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitTestLogger(t)
			v := New()
			v.AddVariables(tt.global)
			globalBefore := v.variables.Clone()
			p := writeSuiteFile(t, t.TempDir(), "suite.yml", tt.content)

			require.NoError(t, v.readFiles(context.Background(), []string{p}))

			require.Len(t, v.Tests.TestSuites, 1)
			ts := v.Tests.TestSuites[0]
			assert.Equal(t, tt.wantVars, suiteVarsWithoutRunValues(ts.Vars))
			names := make([]string, 0, len(ts.TestCases))
			for _, tc := range ts.TestCases {
				names = append(names, tc.Name)
			}
			assert.Equal(t, tt.wantCases, names)
			assert.Equal(t, globalBefore, v.variables, "the run variables are not modified")
		})
	}
}

// TestReadFilesSeveralFiles pins that readFiles appends one suite per path, in order, after the suites already there,
// with no deduplication of names or paths.
func TestReadFilesSeveralFiles(t *testing.T) {
	InitTestLogger(t)
	dir := t.TempDir()
	first := writeSuiteFile(t, dir, "first.yml", "name: same name\n")
	second := writeSuiteFile(t, dir, "second.yaml", "name: same name\n")
	third := writeSuiteFile(t, dir, "third.yml", "name: third\n")

	v := New()
	v.Tests.TestSuites = []TestSuite{{Name: "already there"}}
	require.NoError(t, v.readFiles(context.Background(), []string{first, second, first, third}))

	got := make([]string, 0, len(v.Tests.TestSuites))
	for _, ts := range v.Tests.TestSuites {
		got = append(got, ts.Name+" | "+ts.Filename+" | "+ts.ShortName)
	}
	assert.Equal(t, []string{
		"already there |  | ",
		"same name | first.yml | first",
		"same name | second.yaml | second",
		"same name | first.yml | first",
		"third | third.yml | third",
	}, got)
	assert.Empty(t, v.Tests.TestSuites[1].TestCases)
	assert.NotNil(t, v.Tests.TestSuites[1].TestCases, "a suite without testcases gets an empty, non-nil slice")

	t.Run("no path at all appends nothing", func(t *testing.T) {
		v := New()
		require.NoError(t, v.readFiles(context.Background(), nil))
		assert.Nil(t, v.Tests.TestSuites)
	})
}

// TestReadFilesRelativePath pins that the file path is kept as given while the workdir is made absolute.
func TestReadFilesRelativePath(t *testing.T) {
	InitTestLogger(t)
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))
	writeSuiteFile(t, filepath.Join(dir, "sub"), "rel.yml", "name: rel\n")
	t.Chdir(dir)

	v := New()
	require.NoError(t, v.readFiles(context.Background(), []string{"sub/rel.yml"}))

	ts := v.Tests.TestSuites[0]
	assert.Equal(t, "sub/rel.yml", ts.Filepath)
	assert.Equal(t, "sub/rel.yml", ts.Vars["venom.testsuite.filepath"])
	assert.Equal(t, filepath.Join(dir, "sub"), ts.WorkDir)
	assert.Equal(t, filepath.Join(dir, "sub"), ts.Vars["venom.testsuite.workdir"])
}

// TestReadFilesErrors pins the exact error of each failure path; $FILE in want stands for the quoted file path.
func TestReadFilesErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "an empty file", content: "", want: `error while unmarshal file $FILE: EOF`},
		{
			name:    "invalid YAML",
			content: "name: [\n",
			want:    `error while unmarshal file $FILE: error converting YAML to JSON: yaml: line 1: did not find expected node content`,
		},
		{
			name:    "a tab used as indentation",
			content: "name: s\ntestcases:\n\t- name: a\n",
			want:    `error while unmarshal file $FILE: error converting YAML to JSON: yaml: line 3: found character that cannot start any token`,
		},
		{
			name:    "a duplicate key",
			content: "name: s\ntestcases:\n- name: a\ntestcases:\n- name: b\n",
			want: "error while unmarshal file $FILE: error converting YAML to JSON: yaml: unmarshal errors:\n" +
				`  line 4: mapping key "testcases" already defined at line 2`,
		},
		{
			name:    "an invalid vars block",
			content: "vars:\n  a: [\n",
			want: `unable to get vars from file $FILE: error while unmarshal - see venom.log: ` +
				`error converting YAML to JSON: yaml: line 2: did not find expected node content`,
		},
		{
			name:    "a suite var that is not a valid template",
			content: "name: s\nvars:\n  z: \"{{ .x\"\n",
			want:    `unable to parse variable "z": invalid template format "{{ .x": template: input:1: unclosed action`,
		},
		// quirk: the only error without the file path, and it carries the whole file content
		{
			name:    "a file that is not a valid template",
			content: "name: \"{{ .x\"\n",
			want:    "invalid template format \"name: \"{{ .x\"\n\": template: input:1: bad character U+0022 '\"'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitTestLogger(t)
			p := writeSuiteFile(t, t.TempDir(), "broken.yml", tt.content)
			v := New()
			err := v.readFiles(context.Background(), []string{p})
			require.Error(t, err)
			assert.Equal(t, strings.ReplaceAll(tt.want, "$FILE", strconv.Quote(p)), err.Error())
			assert.Empty(t, v.Tests.TestSuites)
		})
	}

	t.Run("a missing file", func(t *testing.T) {
		InitTestLogger(t)
		p := filepath.Join(t.TempDir(), "missing.yml")
		err := New().readFiles(context.Background(), []string{p})
		require.Error(t, err)
		assert.Equal(t, fmt.Sprintf("unable to read file %q: open %s: no such file or directory", p, p), err.Error())
	})

	t.Run("the first failing file stops the loop and keeps the suites read before it", func(t *testing.T) {
		InitTestLogger(t)
		dir := t.TempDir()
		good := writeSuiteFile(t, dir, "good.yml", "name: good\n")
		bad := writeSuiteFile(t, dir, "bad.yml", "name: [\n")
		never := writeSuiteFile(t, dir, "never.yml", "name: never\n")
		v := New()
		require.Error(t, v.readFiles(context.Background(), []string{good, bad, never}))
		require.Len(t, v.Tests.TestSuites, 1)
		assert.Equal(t, "good", v.Tests.TestSuites[0].Name)
	})
}

// TestReadFilesParseErrorLog proves #2037 AC 2: when a suite file does not parse, venom.log says why its content is not
// logged instead of dumping it, since that content may hold secrets; the returned error is the same as before. The
// secret is outputSecret (venom_output_test.go); $FILE in want and notice stands for the quoted file path.
func TestReadFilesParseErrorLog(t *testing.T) {
	tests := []struct {
		name    string
		global  map[string]interface{}
		content string
		want    string
		notice  string
	}{
		{
			name:    "a secret run variable makes the file invalid YAML once interpolated",
			global:  map[string]interface{}{"pwd": outputSecret + ": ["},
			content: "name: s\nsecrets: [pwd]\ntestcases:\n- name: {{.pwd}}\n",
			want:    `error while unmarshal file $FILE: error converting YAML to JSON: yaml: line 4: mapping values are not allowed in this context`,
			notice:  `file $FILE is not valid YAML after interpolation; its content is not logged as it may hold secrets`,
		},
		{
			name:    "an invalid vars block holds a literal secret",
			content: "name: s\nsecrets: [pwd]\nvars:\n  pwd: " + outputSecret + "\n  other: [\n",
			want: `unable to get vars from file $FILE: error while unmarshal - see venom.log: ` +
				`error converting YAML to JSON: yaml: line 3: did not find expected node content`,
			notice: `the vars block is not valid YAML; its content is not logged as it may hold secrets`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitTestLogger(t)
			var buf bytes.Buffer
			logger.Logger.SetOutput(&buf)
			defer logger.Logger.SetOutput(os.Stdout)

			p := writeSuiteFile(t, t.TempDir(), "broken.yml", tt.content)
			v := New()
			v.AddVariables(tt.global)
			err := v.readFiles(context.Background(), []string{p})
			require.Error(t, err)
			quoted := strconv.Quote(p)
			assert.Equal(t, strings.ReplaceAll(tt.want, "$FILE", quoted), err.Error(), "the returned error does not change")
			assert.Contains(t, buf.String(), strconv.Quote(strings.ReplaceAll(tt.notice, "$FILE", quoted)), "the log says why the content is missing")
			assert.NotContains(t, buf.String(), outputSecret, "the log does not hold the secret")
		})
	}
}
