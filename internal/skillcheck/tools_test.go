package skillcheck

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// inCI reports whether the build must fail, not skip, on a missing tool (FR-019).
func inCI() bool {
	return os.Getenv("CI") != "" || os.Getenv("JENKINS_URL") != ""
}

// skipOrFail skips on a developer machine and fails in CI, so a build cannot go green without proving the checks.
func skipOrFail(t *testing.T, format string, args ...any) {
	t.Helper()
	msg := fmt.Sprintf(format, args...)
	if inCI() {
		t.Fatal(msg)
	}
	t.Skip(msg)
}

// repoRoot walks up from the test's folder to the folder holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root (go.mod) not found above the test folder")
		}
		dir = parent
	}
}

// skillFile returns the path of a file of the skill and fails when it does not exist yet.
func skillFile(t *testing.T, rel string) string {
	t.Helper()
	p := filepath.Join(repoRoot(t), "skills", "venom-pluries", filepath.FromSlash(rel))
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("skills/venom-pluries/%s does not exist yet: %v", rel, err)
	}
	return p
}

var (
	bashOnce   sync.Once
	bashFound  string
	bashReason string
)

// findBash resolves the shell the scripts run in: SKILLCHECK_BASH, then Git Bash next to `git --exec-path` on
// Windows, then `bash` from PATH. The WSL launcher in System32 is never used.
func findBash() (string, string) {
	if p := os.Getenv("SKILLCHECK_BASH"); p != "" {
		if _, err := os.Stat(p); err != nil { //nolint:gosec // candidate paths built from the Git for Windows folder
			return "", "SKILLCHECK_BASH names a file that does not exist: " + p
		}
		return p, ""
	}
	if runtime.GOOS == "windows" {
		if p := gitForWindowsBash(); p != "" {
			return p, ""
		}
	}
	p, err := exec.LookPath("bash")
	if err != nil {
		return "", "bash not found (set SKILLCHECK_BASH or install Git Bash)"
	}
	if isWSLLauncher(p) {
		return "", "the only bash on PATH is the WSL launcher in System32 (set SKILLCHECK_BASH to Git Bash)"
	}
	return p, ""
}

func isWSLLauncher(p string) bool {
	return strings.Contains(strings.ToLower(filepath.ToSlash(p)), "/system32/")
}

func gitForWindowsBash() string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "--exec-path").Output()
	if err != nil {
		return ""
	}
	execPath := filepath.FromSlash(strings.TrimSpace(string(out)))
	root := filepath.Dir(filepath.Dir(filepath.Dir(execPath)))
	for _, rel := range []string{`bin\bash.exe`, `usr\bin\bash.exe`} {
		p := filepath.Join(root, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// bashPath returns the shell to run the scripts with, or skips (fails in CI) when there is none.
func bashPath(t *testing.T) string {
	t.Helper()
	bashOnce.Do(func() { bashFound, bashReason = findBash() })
	if bashFound == "" {
		skipOrFail(t, "%s", bashReason)
	}
	return bashFound
}

// requireTools applies the skip-or-fail rule to the tools a test needs: "openssl" (version 3 or later),
// "node" (looked up on PATH) and any other name (looked up inside bash, where the scripts run).
func requireTools(t *testing.T, names ...string) {
	t.Helper()
	bash := bashPath(t)
	for _, name := range names {
		switch name {
		case "openssl":
			requireOpenSSL3(t)
		case "node":
			requireOnPath(t, name)
		default:
			requireInBash(t, bash, name)
		}
	}
}

func requireOnPath(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		skipOrFail(t, "%s not found on PATH", name)
	}
}

func requireInBash(t *testing.T, bash, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bash, "-c", `command -v "$1"`, "_", name)
	if err := cmd.Run(); err != nil {
		skipOrFail(t, "%s not found by bash", name)
	}
}

func requireOpenSSL3(t *testing.T) {
	t.Helper()
	path, err := exec.LookPath("openssl")
	if err != nil {
		skipOrFail(t, "openssl not found on PATH")
	}
	out, err := exec.Command(path, "version").Output()
	if err != nil {
		skipOrFail(t, "openssl version failed: %v", err)
	}
	if major := opensslMajor(string(out)); major < 3 {
		skipOrFail(t, "OpenSSL 3 or later is needed (the timestamp fixture), found %q", strings.TrimSpace(string(out)))
	}
}

// opensslMajor parses "OpenSSL 3.5.7 ..."; anything else (LibreSSL, garbage) counts as 0.
func opensslMajor(version string) int {
	fields := strings.Fields(version)
	if len(fields) < 2 || fields[0] != "OpenSSL" {
		return 0
	}
	major, _, _ := strings.Cut(fields[1], ".")
	n, err := strconv.Atoi(major)
	if err != nil {
		return 0
	}
	return n
}

// requireVerifyTools is what every test of verify-release.sh needs.
func requireVerifyTools(t *testing.T) {
	t.Helper()
	requireTools(t, "openssl", "sha256sum")
}

// requireInstallTools is what every test of install-venom.sh needs.
func requireInstallTools(t *testing.T) {
	t.Helper()
	requireTools(t, "openssl", "sha256sum", "curl")
}

// cmdResult is the outcome of one script run.
type cmdResult struct {
	code           int
	stdout, stderr string
	timedOut       bool
}

func (r cmdResult) combined() string { return r.stdout + r.stderr }

// bashRun runs the shell with the given environment and working folder (empty means the package folder).
func bashRun(t *testing.T, env []string, dir string, args ...string) cmdResult {
	t.Helper()
	return bashRunWith(t, bashPath(t), env, dir, args...)
}

// bashRunWith is bashRun with an explicit shell.
func bashRunWith(t *testing.T, shell string, env []string, dir string, args ...string) cmdResult {
	t.Helper()
	return bashRunTimeout(t, shell, 3*time.Minute, env, dir, args...)
}

// bashRunTimeout is bashRunWith with a limit; a run that exceeds it is killed and reported with timedOut set.
func bashRunTimeout(t *testing.T, shell string, limit time.Duration, env []string, dir string, args ...string) cmdResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Env = env
	cmd.Dir = dir
	cmd.WaitDelay = 2 * time.Second
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	res := cmdResult{stdout: so.String(), stderr: se.String(), timedOut: ctx.Err() != nil}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.code = exitErr.ExitCode()
	default:
		t.Fatalf("could not run bash: %v", err)
	}
	return res
}

// mergeEnv returns the process environment with the given KEY=value pairs replacing any same-named key.
func mergeEnv(kv ...string) []string {
	drop := map[string]bool{}
	for _, p := range kv {
		key, _, _ := strings.Cut(p, "=")
		drop[normalizeEnvKey(key)] = true
	}
	var env []string
	for _, p := range os.Environ() {
		key, _, _ := strings.Cut(p, "=")
		if !drop[normalizeEnvKey(key)] {
			env = append(env, p)
		}
	}
	return append(env, kv...)
}

func normalizeEnvKey(k string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(k)
	}
	return k
}

// slash gives a path in the form Git Bash and the scripts accept on every system.
func slash(p string) string { return filepath.ToSlash(p) }

// bareBashPath is the shell that keeps the PATH it is given. Git Bash's bin\bash.exe puts /mingw64/bin and
// /usr/bin in front of any PATH, which would defeat a test that removes a tool; usr\bin\bash.exe does not.
func bareBashPath(t *testing.T) string {
	t.Helper()
	bash := bashPath(t)
	if runtime.GOOS != "windows" {
		return bash
	}
	bare := filepath.Join(filepath.Dir(filepath.Dir(bash)), "usr", "bin", "bash.exe")
	if _, err := os.Stat(bare); err != nil {
		return bash
	}
	return bare
}
