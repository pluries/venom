package venom

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// profileIsReadable checks that the file holds something `go tool pprof` could open. venom writes
// two different shapes and both are valid, so the check has to know which is which:
//
//   - CPU profiles come from pprof.StartCPUProfile: gzip-compressed protobuf. Decompressing proves
//     the writer ran to completion, which a zero-byte or truncated file does not.
//   - Memory profiles come from Profile.WriteTo(f, 1), whose debug=1 writes the legacy *text*
//     format beginning "heap profile:". Verified that `go tool pprof` reads it.
//
// Full protobuf parsing would mean making github.com/google/pprof a direct dependency for one
// assertion; the end-to-end check in docs/feat/2024/validation.md runs the real `go tool pprof`.
func profileIsReadable(t *testing.T, path string) {
	t.Helper()
	name := filepath.Base(path)
	b, err := os.ReadFile(path)
	require.NoError(t, err, "profile %s was not written at all", name)
	require.NotEmpty(t, b,
		"profile %s is empty — pprof cannot open it, which is exactly what a silently failed "+
			"StartCPUProfile leaves behind", name)

	if bytes.HasPrefix(b, []byte("heap profile:")) {
		return // legacy text heap profile, written by WriteTo(f, 1)
	}

	zr, err := gzip.NewReader(bytes.NewReader(b))
	require.NoError(t, err, "profile %s is neither a text heap profile nor a gzip stream, so pprof cannot read it", name)
	defer zr.Close()
	// Bounded: this only has to prove the stream decompresses, and an unbounded copy from a
	// reader is a decompression bomb by gosec's reckoning (G110). No venom profile comes near it.
	const maxProfile = 64 << 20
	n, err := io.Copy(io.Discard, io.LimitReader(zr, maxProfile))
	require.NoError(t, err, "profile %s is a truncated gzip stream", name)
	require.NotZero(t, n, "profile %s decompresses to nothing", name)
}

// TestVerbose3WritesReadableProfiles pins the bug: with -vvv every test file must get a CPU and a
// memory profile that can actually be opened. Before the fix the CPU one was 0 bytes, because
// cmd/venom/run had already started a process-wide CPU profile and runTestSuite's own
// StartCPUProfile failed with its error unchecked.
func TestVerbose3WritesReadableProfiles(t *testing.T) {
	InitTestLogger(t)

	dir := t.TempDir()
	v := New()
	v.Verbose = 3
	v.OutputDir = dir
	v.RegisterExecutorBuiltin("sleeper", sleeperExecutor{})
	v.PrintFunc = func(string, ...interface{}) (int, error) { return 0, nil }

	v.Tests.TestSuites = waitingSuites(t, 2, 5*time.Millisecond)
	for i := range v.Tests.TestSuites {
		v.Tests.TestSuites[i].Filename = fmt.Sprintf("suite-%02d.yml", i)
	}

	require.NoError(t, v.Process(context.Background(), nil))

	for i := range v.Tests.TestSuites {
		name := v.Tests.TestSuites[i].Filename
		profileIsReadable(t, filepath.Join(dir, "pprof_cpu_profile_"+name+".prof"))
		profileIsReadable(t, filepath.Join(dir, "pprof_mem_profile_"+name+".prof"))
	}
}

// TestVerbose3ReportsAProfilerAlreadyRunning pins the other half: when something else in the
// process already holds the CPU profiler, venom must say so rather than leave a zero-byte file
// behind. That silence is what made #2024 hard to notice — the run reported success and the
// profile looked present until someone tried to open it.
func TestVerbose3ReportsAProfilerAlreadyRunning(t *testing.T) {
	InitTestLogger(t)

	// Hold the process-wide profiler, exactly as cmd/venom/run used to while the suites ran.
	var held bytes.Buffer
	require.NoError(t, pprof.StartCPUProfile(&held))
	defer pprof.StopCPUProfile()

	dir := t.TempDir()
	v := New()
	v.Verbose = 3
	v.OutputDir = dir
	v.RegisterExecutorBuiltin("sleeper", sleeperExecutor{})
	v.PrintFunc = func(string, ...interface{}) (int, error) { return 0, nil }
	v.Tests.TestSuites = waitingSuites(t, 1, 0)
	v.Tests.TestSuites[0].Filename = "busy.yml"

	err := v.Process(context.Background(), nil)
	require.Error(t, err, "venom must report that the CPU profiler was unavailable")
	require.Contains(t, err.Error(), "cpu profil",
		"the error should name the profiler, so the reader knows what to do; got %q", err)

	// And it must not leave the misleading empty file behind.
	if b, readErr := os.ReadFile(filepath.Join(dir, "pprof_cpu_profile_busy.yml.prof")); readErr == nil {
		require.NotEmpty(t, b, "an unreadable zero-byte profile was left on disk")
	}
}

// There is deliberately no descriptor-leak test here. The reviewer who found the missing Close
// described it as two descriptors held per test file "until process exit"; that is not quite what
// happens. os.File registers a finalizer (os/file_unix.go:225, SetFinalizer((*file).close)), so the
// runtime closes an unreachable File on the next collection. A /proc/self/fd counting test was
// written, and it passed with the Close calls removed — it pinned nothing, so it was deleted rather
// than kept for the comfort of a green line.
//
// The explicit Close is still right: release belongs on every path (stack-golang.md), and relying
// on the collector makes the release nondeterministic and exhaustible under pressure. It is simply
// not something this package can assert, so the guarantee lives in the code, not in a test.
