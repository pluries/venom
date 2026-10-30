package exec

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Characterisation tests for (Executor).Run (#2031, session C). They pin what the executor does today, odd parts
// included; the discrepancies are listed in docs/feat/2031/plan.md (D70 onwards), never fixed here.

// execRun runs one step with a logger installed (venom.Debug panics on a nil logger).
func execRun(ctx context.Context, t *testing.T, step venom.TestStep) (interface{}, error) {
	t.Helper()
	venom.InitTestLogger(t)
	return Executor{}.Run(ctx, step)
}

// execResult runs a step that must succeed and returns its Result with TimeSeconds checked then zeroed, so a whole
// struct can be compared.
func execResult(ctx context.Context, t *testing.T, step venom.TestStep) Result {
	t.Helper()
	got, err := execRun(ctx, t, step)
	require.NoError(t, err)
	res, ok := got.(Result)
	require.Truef(t, ok, "want a Result, got %T: %#v", got, got)
	assert.Greater(t, res.TimeSeconds, 0.0)
	res.TimeSeconds = 0
	return res
}

func execWorkdirCtx(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), dir)
}

func TestExecutorDeclaresItsDefaults(t *testing.T) {
	assert.Equal(t, "exec", Name)
	assert.IsType(t, &Executor{}, New())
	assert.Equal(t, Result{}, Executor{}.ZeroValueResult())
	assert.Equal(t, []venom.Assertion{"result.code ShouldEqual 0"}, Executor{}.GetDefaultAssertions().Assertions)
}

func TestRunRejectsAStepWithoutExactlyOneOfScriptAndCommand(t *testing.T) {
	cases := []struct {
		name string
		step venom.TestStep
		want string
	}{
		{"neither script nor command", venom.TestStep{}, "invalid command"},
		{"an empty script and no command", venom.TestStep{"script": ""}, "invalid command"},
		{"an empty command list", venom.TestStep{"command": []string{}}, "invalid command"},
		{"both script and command", venom.TestStep{"script": "echo a", "command": []string{"echo", "b"}}, "cannot use both 'script' and 'command'"},
		{"a command given as a string", venom.TestStep{"command": "echo hi"}, "1 error(s) decoding:\n\n* 'Command': source data must be an array or slice, got string"},
		{"a script given as a number", venom.TestStep{"script": 42}, "1 error(s) decoding:\n\n* 'Script' expected type 'string', got unconvertible type 'int', value: '42'"},
		{
			"a command with non-string items", venom.TestStep{"command": []interface{}{"echo", 1, true}},
			"2 error(s) decoding:\n\n* 'Command[1]' expected type 'string', got unconvertible type 'int', value: '1'\n" +
				"* 'Command[2]' expected type 'string', got unconvertible type 'bool', value: 'true'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := execRun(context.Background(), t, tc.step)
			require.EqualError(t, err, tc.want)
			assert.Nil(t, got)
		})
	}
}

func TestRunScriptCapturesOutputAndExitCode(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   Result
	}{
		{"stdout of a one-line script", "echo hello", Result{Systemout: "hello", Code: "0"}},
		{"a multi-line script runs every line", "echo one\necho two\n", Result{Systemout: "one\ntwo", Code: "0"}},
		{"stderr is captured apart from stdout", "echo out; echo err >&2", Result{Systemout: "out", Systemerr: "err", Code: "0"}},
		{"the exit code is a string", "echo partial; exit 3", Result{Systemout: "partial", Code: "3"}},
		{"a process killed by a signal reports -1", "kill -9 $$", Result{Code: "-1"}},
		{"only trailing newlines are trimmed, a carriage return stays", `printf '\nx\r\n\n\n'`, Result{Systemout: "\nx\r", Code: "0"}},
		{"trailing spaces and tabs stay", `printf 'x \t'`, Result{Systemout: "x \t", Code: "0"}},
		{"non-printable characters become spaces", `printf 'a\001b\033c'; printf 'd\002' >&2`, Result{Systemout: "a b c", Systemerr: "d ", Code: "0"}},
		{"a script with no output", "true", Result{Code: "0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, execResult(context.Background(), t, venom.TestStep{"script": tc.script}))
		})
	}
}

func TestRunDecodesJSONOutputWithNumbersKeptAsText(t *testing.T) {
	cases := []struct {
		name    string
		script  string
		wantOut interface{}
		wantErr interface{}
	}{
		{"an object keeps number formatting", `echo '{"a":1.50,"b":[1,2]}'`, map[string]interface{}{"a": json.Number("1.50"), "b": []interface{}{json.Number("1"), json.Number("2")}}, nil},
		{"stderr is decoded too", `echo '[1]' >&2`, nil, []interface{}{json.Number("1")}},
		{"a bare number is JSON", "echo 42", json.Number("42"), nil},
		{"a quoted string is JSON", `echo '"s"'`, "s", nil},
		{"only the first JSON value is read, trailing text ignored (D73)", `echo '{"a":1} trailing'`, map[string]interface{}{"a": json.Number("1")}, nil},
		{"plain text is not JSON", "echo not json", nil, nil},
		{"no output is not JSON", "true", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := execResult(context.Background(), t, venom.TestStep{"script": tc.script})
			assert.Equal(t, tc.wantOut, res.SystemoutJSON)
			assert.Equal(t, tc.wantErr, res.SystemerrJSON)
		})
	}
}

func TestRunCommandRunsWithoutAShell(t *testing.T) {
	stdin := "from stdin\n"
	cases := []struct {
		name string
		step venom.TestStep
		want Result
	}{
		{"arguments are passed literally", venom.TestStep{"command": []string{"echo", "$HOME", "a b"}}, Result{Systemout: "$HOME a b", Code: "0"}},
		{"a single-item command", venom.TestStep{"command": []string{"pwd"}}, Result{Systemout: mustGetwd(t), Code: "0"}},
		{"stdin is fed to the process", venom.TestStep{"command": []string{"cat"}, "stdin": stdin}, Result{Systemout: "from stdin", Code: "0"}},
		{"without stdin the process reads nothing", venom.TestStep{"command": []string{"cat"}}, Result{Code: "0"}},
		{"an empty script next to a command is ignored", venom.TestStep{"script": "", "command": []string{"echo", "x"}}, Result{Systemout: "x", Code: "0"}},
		{"a failing command reports its exit code", venom.TestStep{"command": []string{"sh", "-c", "exit 7"}}, Result{Code: "7"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, execResult(context.Background(), t, tc.step))
		})
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	return wd
}

func TestRunScriptPicksTheInterpreterFromTheShebang(t *testing.T) {
	// /proc/$$/exe names the interpreter actually running the script (Linux).
	t.Run("without a shebang the script runs under /bin/sh", func(t *testing.T) {
		res := execResult(context.Background(), t, venom.TestStep{"script": "readlink /proc/$$/exe"})
		assert.Equal(t, Result{Systemout: realPath(t, "/bin/sh"), Code: "0"}, res)
	})
	t.Run("a shebang selects the interpreter, trailing blanks trimmed", func(t *testing.T) {
		if _, err := os.Stat("/bin/bash"); err != nil {
			t.Skip("/bin/bash not installed")
		}
		res := execResult(context.Background(), t, venom.TestStep{"script": "#!/bin/bash \t\r\nreadlink /proc/$$/exe"})
		assert.Equal(t, Result{Systemout: realPath(t, "/bin/bash"), Code: "0"}, res)
	})
	t.Run("a shebang with an argument is taken as one binary name and fails to start (D71)", func(t *testing.T) {
		script := "#!/bin/sh -e\necho x"
		got, err := execRun(context.Background(), t, venom.TestStep{"script": script})
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"executor.script": script, "executor.stdin": ""}, got)
	})
}

func realPath(t *testing.T, path string) string {
	t.Helper()
	target, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return target
}

func TestRunScriptWritesATemporaryFileAndRemovesIt(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	res := execResult(context.Background(), t, venom.TestStep{"script": `echo "$0"; test -x "$0" && echo executable`})
	lines := strings.Split(res.Systemout, "\n")
	require.Len(t, lines, 2)
	assert.True(t, strings.HasPrefix(lines[0], filepath.Join(tmp, "venom-")), lines[0])
	assert.Equal(t, "executable", lines[1])
	assert.NoFileExists(t, lines[0])

	t.Run("also when the interpreter fails to start", func(t *testing.T) {
		_, err := execRun(context.Background(), t, venom.TestStep{"script": "#!/nonexistent/interpreter\necho x"})
		require.NoError(t, err)
		entries, err := os.ReadDir(tmp)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})
}

func TestRunUsesTheSuiteWorkdirAndTheProcessEnvironment(t *testing.T) {
	dir := t.TempDir()
	realDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	t.Setenv("VENOM_EXEC_CHARACTERISATION", "synthetic-value")

	cases := []struct {
		name string
		ctx  context.Context
		step venom.TestStep
		want string
	}{
		{"the workdir from the context is the process directory", execWorkdirCtx(dir), venom.TestStep{"script": "pwd -P"}, realDir},
		{"without a workdir the process runs in the current directory", context.Background(), venom.TestStep{"script": "pwd -P"}, mustGetwd(t)},
		{"the environment is inherited", context.Background(), venom.TestStep{"script": `echo "$VENOM_EXEC_CHARACTERISATION"`}, "synthetic-value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := execResult(tc.ctx, t, tc.step)
			assert.Equal(t, tc.want, res.Systemout)
		})
	}
}

// A process that cannot start returns the dump of the step's own input, not a Result, and no error: result.code is
// absent, the "127" and the start error are dropped (D70).
func TestRunReturnsTheInputDumpWhenTheProcessCannotStart(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	missingDir := execWorkdirCtx(filepath.Join(t.TempDir(), "missing"))

	cases := []struct {
		name string
		ctx  context.Context
		step venom.TestStep
		want map[string]interface{}
	}{
		{
			"a binary not on PATH", context.Background(), venom.TestStep{"command": []string{"venom-no-such-binary", "a"}},
			map[string]interface{}{"executor.command.command0": "venom-no-such-binary", "executor.command.command1": "a", "executor.script": "", "executor.stdin": ""},
		},
		{
			"a missing workdir", missingDir, venom.TestStep{"command": []string{"true"}, "stdin": "x"},
			map[string]interface{}{"executor.command.command0": "true", "executor.script": "", "executor.stdin": "x"},
		},
		{
			"an already cancelled context", cancelled, venom.TestStep{"command": []string{"true"}},
			map[string]interface{}{"executor.command.command0": "true", "executor.script": "", "executor.stdin": ""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := execRun(tc.ctx, t, tc.step)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// A context that ends while the process runs kills it: the step reports code -1 like any signal, with no error and
// no Err text (D72).
func TestRunKillsTheProcessWhenTheContextEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	res := execResult(ctx, t, venom.TestStep{"command": []string{"sleep", "5"}})
	assert.Less(t, time.Since(start), 4*time.Second)
	assert.Equal(t, Result{Code: "-1"}, res)
}
