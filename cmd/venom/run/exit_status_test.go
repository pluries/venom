package run

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	helperArgsEnv = "VENOM_TEST_RUN_ARGS"
	helperNoExit  = 99
)

// TestRunHelperProcess is not a test: TestRunExitStatus starts the test binary again with VENOM_TEST_RUN_ARGS set, and
// this function then runs the real `venom run` command. The command ends in os.Exit, which cannot be observed in the
// process that runs the tests (venom.OSExit only records the code when venom.IsTest is set, and the command goes on to
// call it a second time), so the exit status is read from a child process.
func TestRunHelperProcess(t *testing.T) {
	args := os.Getenv(helperArgsEnv)
	if args == "" {
		t.Skip("helper process of TestRunExitStatus")
	}
	Cmd.SetArgs(strings.Split(args, "\x1f"))
	if err := Cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(helperNoExit)
}

const (
	suitePass = `name: pass
testcases:
- name: ok
  steps:
  - type: exec
    script: echo hello
    assertions:
    - result.code ShouldEqual 0
`
	suiteFail = `name: fail
testcases:
- name: ko
  steps:
  - type: exec
    script: echo hello
    assertions:
    - result.systemout ShouldEqual nothing
`
	suiteSkipped = `name: skipped
testcases:
- name: skipped
  skip:
  - 1 ShouldEqual 1
  steps:
  - type: exec
    script: echo hello
    assertions:
    - result.code ShouldEqual 0
`
	suiteUnknownVar = `name: unknown
testcases:
- name: ok
  steps:
  - type: exec
    script: echo {{.nope}}
`
)

// runVenom runs `venom run args...` in a child process started in a clean working folder and returns its exit status and
// what it printed.
func runVenom(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	// The test binary is started again as it is: nothing here comes from outside the test.
	cmd := &exec.Cmd{
		Path: self,
		Args: []string{self, "-test.run=^TestRunHelperProcess$"},
		Dir:  dir,
		Env:  append(os.Environ(), "HOME="+t.TempDir(), helperArgsEnv+"="+strings.Join(args, "\x1f")),
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	if err == nil {
		return 0, out.String()
	}
	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr), "the child did not start: %v", err)
	return exitErr.ExitCode(), out.String()
}

// TestRunExitStatus pins what `venom run` exits with and the final status line it prints (RULE-029): 0 when every
// case passes, 2 for a failing case, and 2 as well when every case is skipped (the latter is an open maintainer
// question: it is pinned as it is today, not endorsed).
func TestRunExitStatus(t *testing.T) {
	tests := []struct {
		name     string
		suite    string
		args     []string
		wantExit int
		wantLine string
	}{
		{name: "every case passes", suite: suitePass, wantExit: 0, wantLine: "final status: "},
		{name: "a case fails", suite: suiteFail, wantExit: 2, wantLine: "final status: "},
		{name: "every case is skipped", suite: suiteSkipped, wantExit: 2, wantLine: "final status: "},
		{name: "an unknown variable with --strict-variables", suite: suiteUnknownVar, args: []string{"--strict-variables"}, wantExit: 2, wantLine: "final status: "},
		{name: "an invalid --parallel value stops at startup", suite: suitePass, args: []string{"--parallel", "0"}, wantExit: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "suite.yml"), []byte(tt.suite), 0o600))
			args := append([]string{"--format=json", "--output-dir", dir}, tt.args...)
			exit, out := runVenom(t, dir, append(args, "suite.yml")...)
			assert.Equal(t, tt.wantExit, exit, out)
			if tt.wantLine == "" {
				assert.NotContains(t, out, "final status:")
				return
			}
			assert.Contains(t, out, tt.wantLine)
		})
	}
}

// TestRunFinalStatusWord pins the word after "final status:" for a pass and for a failure.
func TestRunFinalStatusWord(t *testing.T) {
	for suite, want := range map[string]string{suitePass: "PASS", suiteFail: "FAIL"} {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "suite.yml"), []byte(suite), 0o600))
		_, out := runVenom(t, dir, "--format=json", "--output-dir", dir, "suite.yml")
		assert.Regexp(t, "final status: .*"+want, out)
	}
}

// TestInitArgsPrecedence pins the order a setting is read in (RULE-030): the environment, then the .venomrc file
// (working folder before home, which Test_initFromConfigFile_homeFallback pins), then the command-line flag, each one
// overriding the one before.
func TestInitArgsPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		file       string
		flag       string
		wantFormat string
	}{
		{name: "nothing set keeps the default", wantFormat: "xml"},
		{name: "the environment alone", env: "json", wantFormat: "json"},
		{name: "the file overrides the environment", env: "json", file: "tap", wantFormat: "tap"},
		{name: "the flag overrides the file and the environment", env: "json", file: "tap", flag: "yaml", wantFormat: "yaml"},
		{name: "the flag overrides the default", flag: "json", wantFormat: "json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateOptions(t, defaultOptions)
			t.Setenv("HOME", t.TempDir())
			t.Chdir(t.TempDir())
			if tt.env != "" {
				t.Setenv("VENOM_FORMAT", tt.env)
			}
			if tt.file != "" {
				require.NoError(t, os.WriteFile(".venomrc", []byte("format: "+tt.file+"\n"), 0o600))
			}
			c := &cobra.Command{Use: "run"}
			formatFlag = c.Flags().String("format", "xml", "")
			if tt.flag != "" {
				require.NoError(t, c.Flags().Set("format", tt.flag))
			}

			initArgs(c)

			assert.Equal(t, tt.wantFormat, format)
		})
	}
}
