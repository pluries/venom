package exec

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fsamin/go-dump"
	"github.com/mitchellh/mapstructure"

	"github.com/ovh/venom"
)

// Name for test exec
const Name = "exec"

// New returns a new Test Exec
func New() venom.Executor {
	return &Executor{}
}

// Executor represents a Test Exec
type Executor struct {
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`
	Stdin   *string  `json:"stdin,omitempty" yaml:"stdin,omitempty"`
	Script  *string  `json:"script,omitempty" yaml:"script,omitempty"`
}

// Result represents a step result
type Result struct {
	Systemout     string      `json:"systemout,omitempty" yaml:"systemout,omitempty"`
	SystemoutJSON interface{} `json:"systemoutjson,omitempty" yaml:"systemoutjson,omitempty"`
	Systemerr     string      `json:"systemerr,omitempty" yaml:"systemerr,omitempty"`
	SystemerrJSON interface{} `json:"systemerrjson,omitempty" yaml:"systemerrjson,omitempty"`
	Err           string      `json:"err,omitempty" yaml:"err,omitempty"`
	Code          string      `json:"code,omitempty" yaml:"code,omitempty"`
	TimeSeconds   float64     `json:"timeseconds,omitempty" yaml:"timeseconds,omitempty"`
}

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return Result{}
}

// GetDefaultAssertions return default assertions for type exec
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.code ShouldEqual 0"}}
}

// Run execute TestStep of type exec
func (Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	if err := e.validate(); err != nil {
		return nil, err
	}

	command, opts := e.commandLine()
	if e.hasScript() {
		shell, shellOpts := scriptShell(*e.Script, opts)
		scriptPath, scriptOpts, err := writeScript(ctx, *e.Script, shellOpts)
		if err != nil {
			return nil, err
		}
		// Registered before the chmod, so a script that cannot be made executable is still removed.
		defer os.Remove(scriptPath)

		// Chmod file
		if err := os.Chmod(scriptPath, 0o700); err != nil {
			return nil, fmt.Errorf("cannot chmod script %s: %s", scriptPath, err)
		}

		command, opts = shell, scriptOpts
	}

	return e.runCommand(ctx, command, opts)
}

func (e Executor) hasScript() bool {
	return e.Script != nil && *e.Script != ""
}

// validate requires exactly one of 'script' and 'command'.
func (e Executor) validate() error {
	if !e.hasScript() && (len(e.Command) == 0) {
		return fmt.Errorf("invalid command")
	}
	if e.hasScript() && len(e.Command) != 0 {
		return fmt.Errorf("cannot use both 'script' and 'command'")
	}
	return nil
}

// commandLine splits 'command' into the binary and its arguments.
func (e Executor) commandLine() (string, []string) {
	var (
		command string
		opts    []string
	)

	if len(e.Command) != 0 {
		command = e.Command[0]
		if len(e.Command) > 1 {
			opts = e.Command[1:]
		}
	}
	return command, opts
}

// scriptShell picks the interpreter of a script: the whole shebang line when there is one (an argument
// in it is taken as part of the binary name), /bin/sh otherwise, PowerShell on windows.
func scriptShell(scriptContent string, opts []string) (string, []string) {
	// Default shell is sh
	shell := "/bin/sh"

	// If user wants a specific shell, use it
	if strings.HasPrefix(scriptContent, "#!") {
		t := strings.SplitN(scriptContent, "\n", 2)
		shell = strings.TrimPrefix(t[0], "#!")
		shell = strings.TrimRight(shell, " \t\r\n")
	}

	// except on windows where it's powershell
	if runtime.GOOS == "windows" {
		shell = "PowerShell"
		opts = append(opts, "-ExecutionPolicy", "Bypass", "-Command")
	}
	return shell, opts
}

// writeScript puts the script in a temporary file and returns its path and the interpreter arguments
// that run it. The caller removes the file.
func writeScript(ctx context.Context, scriptContent string, opts []string) (string, []string, error) {
	// Create a tmp file
	tmpscript, err := os.CreateTemp(os.TempDir(), "venom-")
	if err != nil {
		return "", nil, fmt.Errorf("cannot create tmp file: %s", err)
	}

	// Put script in file
	venom.Debug(ctx, "work with tmp file %s", tmpscript.Name())
	n, err := tmpscript.Write([]byte(scriptContent))
	if err != nil || n != len(scriptContent) {
		if err != nil {
			return "", nil, fmt.Errorf("cannot write script: %s", err)
		}
		return "", nil, fmt.Errorf("cannot write all script: %d/%d", n, len(scriptContent))
	}

	oldPath := tmpscript.Name()
	tmpscript.Close()
	var scriptPath string
	if runtime.GOOS == "windows" {
		// Remove all .txt Extensions, there is not always a .txt extension
		newPath := strings.ReplaceAll(oldPath, ".txt", "")
		// and add .PS1 extension
		newPath += ".PS1"
		if err := os.Rename(oldPath, newPath); err != nil {
			return "", nil, fmt.Errorf("cannot rename script to add powershell extension, aborting")
		}
		// This aims to stop a the very first error and return the right exit code
		psCommand := fmt.Sprintf("& { $ErrorActionPreference='Stop'; & %s ;exit $LastExitCode}", newPath)
		scriptPath = newPath
		opts = append(opts, psCommand)
	} else {
		scriptPath = oldPath
		opts = append(opts, scriptPath)
	}
	return scriptPath, opts, nil
}

// runCommand runs the command in the suite's workdir and collects its result. When the process cannot
// start, it returns the dump of the executor's input and no error.
func (e Executor) runCommand(ctx context.Context, command string, opts []string) (interface{}, error) {
	start := time.Now()

	cmd := exec.CommandContext(ctx, command, opts...)
	venom.Debug(ctx, "teststep exec '%s %s'", command, strings.Join(opts, " "))
	cmd.Dir = venom.StringVarFromCtx(ctx, "venom.testsuite.workdir")
	if e.Stdin != nil {
		cmd.Stdin = strings.NewReader(*e.Stdin)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("runScriptAction: Cannot get stdout pipe: %s", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("runScriptAction: Cannot get stderr pipe: %s", err)
	}

	result := Result{}
	outchan := make(chan bool)
	go readStdout(stdout, &result, outchan)

	errchan := make(chan bool)
	go readStderr(ctx, stderr, &result, errchan)

	if err := cmd.Start(); err != nil {
		result.Err = err.Error()
		result.Code = "127"
		venom.Debug(ctx, "error on cmd.Start: %v", err.Error())
		return dump.ToMap(e, dump.WithDefaultLowerCaseFormatter())
	}

	<-outchan
	<-errchan

	result.Code = exitCode(cmd.Wait())

	elapsed := time.Since(start)
	result.TimeSeconds = elapsed.Seconds()

	decodeOutputs(&result)

	return result, nil
}

// readStdout reads the whole standard output into result.Systemout, then closes done.
func readStdout(stdout io.Reader, result *Result, done chan bool) {
	var sb strings.Builder
	sb.Grow(1024 * 1024) // Pre-allocate 1MB

	// For efficiency, read in larger chunks
	buf := make([]byte, 64*1024) // 64KB buffer
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	result.Systemout = sb.String()
	close(done)
}

// readStderr reads the whole standard error into result.Systemerr, logging each chunk, then closes done.
func readStderr(ctx context.Context, stderr io.ReadCloser, result *Result, done chan bool) {
	var sb strings.Builder
	sb.Grow(64 * 1024) // Pre-allocate 64KB for stderr

	buf := make([]byte, 8*1024) // 8KB buffer for stderr
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			sb.Write(chunk)
			venom.Debug(ctx, "%s", string(chunk))
		}
		if err != nil {
			break
		}
	}

	result.Systemerr = sb.String()
	stderr.Close()
	close(done)
}

// exitCode is the process exit status, "0" when the wait error carries none (a killed process reports -1).
func exitCode(waitErr error) string {
	code := "0"
	if waitErr != nil {
		if exiterr, ok := waitErr.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				code = strconv.Itoa(status.ExitStatus())
			}
		}
	}
	return code
}

// decodeOutputs trims both outputs and fills their JSON twins when they decode.
func decodeOutputs(result *Result) {
	result.Systemout = venom.RemoveNotPrintableChar(strings.TrimRight(result.Systemout, "\n"))
	result.Systemerr = venom.RemoveNotPrintableChar(strings.TrimRight(result.Systemerr, "\n"))

	var outJSON interface{}
	if err := venom.JSONUnmarshal([]byte(result.Systemout), &outJSON); err == nil {
		result.SystemoutJSON = outJSON
	}

	var errJSON interface{}
	if err := venom.JSONUnmarshal([]byte(result.Systemerr), &errJSON); err == nil {
		result.SystemerrJSON = errJSON
	}
}
