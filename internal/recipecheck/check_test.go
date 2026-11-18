package recipecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalSuite = "name: Minimal\ntestcases:\n- name: US1-AS1 one\n  steps:\n  - type: web\n    action:\n      find: h1\n    assertions:\n    - result.find ShouldEqual 1\n"

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

// US3-AS8: the JSON of a finding carries exactly the contract's fields.
func TestFindingJSONFields(t *testing.T) {
	f := Finding{
		File:     "tests/site/site-languages.yml",
		Suite:    "Site languages in a browser",
		Testcase: "Chinese home loads",
		Rule:     "RC01",
		Message:  "the name has no scenario id",
		Line:     12,
	}
	b, err := json.Marshal(f)
	require.NoError(t, err)
	assert.JSONEq(t, `{"file":"tests/site/site-languages.yml","suite":"Site languages in a browser","testcase":"Chinese home loads","rule":"RC01","message":"the name has no scenario id","line":12}`, string(b))
}

// US3-AS8: a zero line and empty names are written, not omitted.
func TestFindingJSONKeepsZeroValues(t *testing.T) {
	b, err := json.Marshal(Finding{Rule: "RC06"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"file":"","suite":"","testcase":"","rule":"RC06","message":"","line":0}`, string(b))
}

// US3-AS8: the report is one object with files and findings.
func TestReportJSONFields(t *testing.T) {
	r := Report{Files: 2, Findings: []Finding{{Rule: "RC05", Line: 3}}}
	b, err := json.Marshal(r)
	require.NoError(t, err)
	assert.JSONEq(t, `{"files":2,"findings":[{"file":"","suite":"","testcase":"","rule":"RC05","message":"","line":3}]}`, string(b))
}

// US3-AS8: the report counts the files it read.
func TestCheckCountsFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "b.yml", minimalSuite)
	writeFile(t, dir, "a.yaml", minimalSuite)
	writeFile(t, dir, "notes.txt", "not a suite")

	report, err := Check([]string{dir}, Options{})

	require.NoError(t, err)
	assert.Equal(t, 2, report.Files)
}

// US3-AS7: no path is unusable input.
func TestCheckNoPathIsUnusable(t *testing.T) {
	_, err := Check(nil, Options{})
	assert.ErrorIs(t, err, ErrUnusable)
}

// US3-AS7: a path that does not exist is unusable input.
func TestCheckMissingPathIsUnusable(t *testing.T) {
	_, err := Check([]string{filepath.Join(t.TempDir(), "absent.yml")}, Options{})
	assert.ErrorIs(t, err, ErrUnusable)
}

// US3-AS7: an unparsable file is unusable input and is also reported as RC10 with its path.
func TestCheckUnparsableFileIsRC10AndUnusable(t *testing.T) {
	dir := t.TempDir()
	bad := writeFile(t, dir, "bad.yml", "name: [unclosed\n")

	report, err := Check([]string{bad}, Options{})

	assert.ErrorIs(t, err, ErrUnusable)
	require.Len(t, report.Findings, 1)
	assert.Equal(t, "RC10", report.Findings[0].Rule)
	assert.Equal(t, bad, report.Findings[0].File)
}

// aliasBomb is a file of about 500 bytes whose variable g expands to ten million scalars through seven nested aliases
// of ten entries each (venom run says "document contains excessive aliasing").
func aliasBomb() string {
	const row = "[%s,%s,%s,%s,%s,%s,%s,%s,%s,%s]"
	text := "name: Bomb\nvars:\n  a: &a [x,x,x,x,x,x,x,x,x,x]\n"
	prev := "*a"
	for _, name := range []string{"b", "c", "d", "e", "f", "g"} {
		text += "  " + name + ": &" + name + " " + strings.NewReplacer("%s", prev).Replace(row) + "\n"
		prev = "*" + name
	}
	return text + "testcases:\n" + strings.TrimPrefix(minimalSuite, "name: Minimal\ntestcases:\n")
}

// US3-AS7, RC10: an alias bomb is rejected as RC10 quickly, as venom run rejects it, instead of being walked. The check
// runs in a goroutine so that a check that walks it fails on the time bound instead of hanging the test binary.
func TestCheckRejectsAnAliasBombAsRC10Quickly(t *testing.T) {
	bomb := writeFile(t, t.TempDir(), "bomb.yml", aliasBomb())
	require.Less(t, len(aliasBomb()), 600)
	type result struct {
		report Report
		err    error
	}
	done := make(chan result, 1)

	start := time.Now()
	go func() {
		report, err := Check([]string{bomb}, Options{})
		done <- result{report, err}
	}()

	select {
	case res := <-done:
		assert.Less(t, time.Since(start), 2*time.Second)
		assert.ErrorIs(t, res.err, ErrUnusable)
		require.NotEmpty(t, res.report.Findings)
		assert.Equal(t, "RC10", res.report.Findings[0].Rule)
		assert.Equal(t, bomb, res.report.Findings[0].File)
	case <-time.After(2 * time.Second):
		t.Fatal("the check was still walking the alias bomb after 2 seconds")
	}
}
