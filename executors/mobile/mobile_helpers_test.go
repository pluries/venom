package mobile

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

func TestMain(m *testing.M) {
	// Run and Setup log through venom's logger, which must exist
	venom.InitTestLogger(nil)
	os.Exit(m.Run())
}

// shorten sets a package variable for the length of the test.
func shorten[T any](t *testing.T, target *T, value T) {
	t.Helper()
	old := *target
	*target = value
	t.Cleanup(func() { *target = old })
}

// withAction registers a test action in the dispatch table for the length of the test.
func withAction(t *testing.T, name string, fn actionFunc) {
	t.Helper()
	actions[name] = fn
	t.Cleanup(func() { delete(actions, name) })
}

// openCase runs Setup as the engine does at the first step of a test case.
func openCase(t *testing.T, vars map[string]interface{}) context.Context {
	t.Helper()
	ctx, err := Executor{}.Setup(testCtx(vars), nil)
	require.NoError(t, err)
	return ctx
}

// runStep (the context comes first, as revive wants) runs one step and returns its Result.
func runStep(ctx context.Context, t *testing.T, action string) Result {
	t.Helper()
	out, err := Executor{}.Run(ctx, venom.TestStep{"action": action})
	require.NoError(t, err)
	result, ok := out.(Result)
	require.True(t, ok, "Run returned %T", out)
	return result
}

// callAction is an action that sends one command and fails with whatever the server answers.
func callAction(method, path string) actionFunc {
	return func(ctx context.Context, s *session, _ Executor, _ *Result) error {
		_, err := s.call(ctx, method, path, map[string]interface{}{})
		return err
	}
}

// within fails the test when fn does not return in time, so a regression does not hang the suite.
func within(t *testing.T, limit time.Duration, fn func()) {
	t.Helper()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		fn()
	}()
	select {
	case <-finished:
	case <-time.After(limit):
		t.Fatalf("still running after %s", limit)
	}
}

// requireUnixModes skips a test that asserts file modes: Windows does not report 0600.
func requireUnixModes(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not reported on windows")
	}
}

// capturedLogs reads what venom's logger wrote at debug level.
type capturedLogs struct{ hook *logtest.Hook }

func captureLogs(t *testing.T) *capturedLogs {
	t.Helper()
	v := venom.New()
	v.Verbose = 2
	v.OutputDir = t.TempDir()
	require.NoError(t, v.InitLogger())
	hook := logtest.NewGlobal()
	t.Cleanup(func() {
		if closer, ok := v.LogOutput.(io.Closer); ok {
			_ = closer.Close()
		}
		std := logrus.StandardLogger()
		std.ReplaceHooks(make(logrus.LevelHooks))
		std.SetOutput(os.Stderr)
		std.SetLevel(logrus.InfoLevel)
		std.SetFormatter(&logrus.TextFormatter{})
		venom.InitTestLogger(t)
	})
	return &capturedLogs{hook: hook}
}

// text is every entry as a reader of the log would see it.
func (c *capturedLogs) text() string {
	out := ""
	for _, e := range c.hook.AllEntries() {
		out += fmt.Sprintln(e.Message, e.Data)
	}
	return out
}
