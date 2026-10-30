package mobile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const elementsRoute = "POST /session/{id}/elements"

// elementList is the JSON text of n element references.
func elementList(n int) string {
	out := "["
	for i := 0; i < n; i++ {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf(`{%q:"el-%d"}`, elementKey, i)
	}
	return out + "]"
}

// openSessionOf opens a session on the fake server and returns it with a context for its calls.
func openSessionOf(t *testing.T, f *fakeServer) (context.Context, *session) {
	t.Helper()
	ctx := openCase(t, f.baseVars())
	s := getSession(ctx)
	require.NotNil(t, s)
	return ctx, s
}

// lastQuery decodes the body of the n-th /elements request.
func lastQuery(t *testing.T, f *fakeServer) map[string]string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.reqs) - 1; i >= 0; i-- {
		if f.reqs[i].Path == "/session/{id}/elements" {
			var out map[string]string
			require.NoError(t, json.Unmarshal([]byte(f.reqs[i].Body), &out))
			return out
		}
	}
	t.Fatal("no /elements request received")
	return nil
}

func TestFR07_EachLocatorKindReachesTheServerWithItsStrategy(t *testing.T) {
	cases := []struct {
		name  string
		step  Executor
		using string
		value string
	}{
		{"accessibility id", Executor{AccessibilityID: "login"}, "accessibility id", "login"},
		{"text", Executor{Text: "Sign in"}, "-android uiautomator", `new UiSelector().text("Sign in")`},
		{"id", Executor{ID: "com.example.app:id/ok"}, "id", "com.example.app:id/ok"},
		{"xpath", Executor{XPath: "//android.widget.Button"}, "xpath", "//android.widget.Button"},
		{"uiautomator", Executor{UIAutomator: "new UiSelector().resourceId(\"x\")"}, "-android uiautomator", "new UiSelector().resourceId(\"x\")"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeServer(t)
			f.on(elementsRoute, reply(http.StatusOK, elementList(1)))
			ctx, s := openSessionOf(t, f)

			loc, err := tc.step.locator()
			require.NoError(t, err)
			ref, err := s.findElement(ctx, loc, time.Second)
			require.NoError(t, err)
			assert.Equal(t, elementRef("el-0"), ref)
			assert.Equal(t, map[string]string{"using": tc.using, "value": tc.value}, lastQuery(t, f))
		})
	}
}

func TestAC4_TwoLocatorsOrNoneAreRefusedBeforeAnyRequest(t *testing.T) {
	f := newFakeServer(t)
	f.on(elementsRoute, reply(http.StatusOK, elementList(1)))
	ctx := openCase(t, f.baseVars())
	before := f.total()
	withAction(t, "probe", func(_ context.Context, _ *session, step Executor, _ *Result) error {
		_, err := step.locator()
		return err
	})

	for name, tc := range map[string]struct {
		step Executor
		want []string
	}{
		"two":   {Executor{AccessibilityID: "a", XPath: "//x"}, []string{"accessibilityId", "xpath"}},
		"three": {Executor{Text: "t", ID: "i", UIAutomator: "u"}, []string{"text", "id", "uiautomator"}},
		"none":  {Executor{}, []string{"no locator", "accessibilityId", "uiautomator"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := tc.step.locator()
			var misuse *usageError
			require.ErrorAs(t, err, &misuse)
			for _, w := range tc.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}

	// through Run: a Go error, not a Result
	out, err := Executor{}.Run(ctx, map[string]interface{}{"action": "probe", "id": "a", "xpath": "//x"})
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Equal(t, before, f.total(), "no request after the session opened")
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/elements"))
}

func TestFR07_StepFieldsDecode(t *testing.T) {
	f := newFakeServer(t)
	f.on(elementsRoute, reply(http.StatusOK, elementList(1)))
	ctx := openCase(t, f.baseVars())
	var seen Executor
	withAction(t, "probe", func(_ context.Context, _ *session, step Executor, _ *Result) error {
		seen = step
		return nil
	})
	_, err := Executor{}.Run(ctx, map[string]interface{}{"action": "probe", "accessibilityId": "a", "syncTimeout": 3})
	require.NoError(t, err)
	assert.Equal(t, "a", seen.AccessibilityID)
	require.NotNil(t, seen.SyncTimeout)
	assert.InDelta(t, 3, *seen.SyncTimeout, 0)
}

func TestFR08_ElementAppearingAfterNPollsIsFound(t *testing.T) {
	shorten(t, &pollInterval, 5*time.Millisecond)
	f := newFakeServer(t)
	var polls atomic.Int32
	f.on(elementsRoute, func(w http.ResponseWriter, r *http.Request, body []byte) {
		if polls.Add(1) < 4 {
			reply(http.StatusOK, `[]`)(w, r, body)
			return
		}
		reply(http.StatusOK, elementList(1))(w, r, body)
	})
	ctx, s := openSessionOf(t, f)

	wait, err := s.waitOf(Executor{})
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, wait, "the session default")
	ref, err := s.findElement(ctx, locator{locText, "Late"}, wait)
	require.NoError(t, err)
	assert.Equal(t, elementRef("el-0"), ref)
	assert.EqualValues(t, 4, polls.Load())
}

func TestAC5_NeverFoundFailsNamingTheLocatorAndTheWait(t *testing.T) {
	shorten(t, &pollInterval, 5*time.Millisecond)
	f := newFakeServer(t)
	f.on(elementsRoute, reply(http.StatusOK, `[]`))
	ctx, s := openSessionOf(t, f)

	start := time.Now()
	_, err := s.findElement(ctx, locator{locAccessibilityID, "ghost"}, 60*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `accessibilityId "ghost"`)
	assert.Contains(t, err.Error(), "60ms")
	assert.GreaterOrEqual(t, time.Since(start), 60*time.Millisecond)
	assert.Less(t, time.Since(start), 2*time.Second)
	assert.Greater(t, f.count(http.MethodPost, "/session/{id}/elements"), 1)

	var misuse *usageError
	assert.NotErrorAs(t, err, &misuse, "a server-style failure, not a usage error")
}

func TestAC5_NeverFoundLandsInResultErr(t *testing.T) {
	shorten(t, &pollInterval, 5*time.Millisecond)
	f := newFakeServer(t)
	f.on(elementsRoute, reply(http.StatusOK, `[]`))
	ctx := openCase(t, f.baseVars())
	withAction(t, "probe", func(ctx context.Context, s *session, step Executor, _ *Result) error {
		loc, err := step.locator()
		if err != nil {
			return err
		}
		wait, err := s.waitOf(step)
		if err != nil {
			return err
		}
		_, err = s.findElement(ctx, loc, wait)
		return err
	})
	out, err := Executor{}.Run(ctx, map[string]interface{}{"action": "probe", "id": "gone", "syncTimeout": 0.05})
	require.NoError(t, err)
	result, ok := out.(Result)
	require.True(t, ok)
	assert.Contains(t, result.Err, `id "gone"`)
	assert.Contains(t, result.Err, "50ms")
}

func TestFR08_StepSyncTimeoutOverridesAndIsChecked(t *testing.T) {
	s := &session{syncTimeout: 10 * time.Second}
	half := 0.5
	wait, err := s.waitOf(Executor{SyncTimeout: &half})
	require.NoError(t, err)
	assert.Equal(t, 500*time.Millisecond, wait)

	neg := -1.0
	_, err = s.waitOf(Executor{SyncTimeout: &neg})
	var misuse *usageError
	require.ErrorAs(t, err, &misuse)
}

func TestNFR08_FirstPollHitAddsNoDelay(t *testing.T) {
	shorten(t, &pollInterval, 2*time.Second) // a delay would be obvious
	f := newFakeServer(t)
	f.on(elementsRoute, reply(http.StatusOK, elementList(2)))
	ctx, s := openSessionOf(t, f)

	within(t, time.Second, func() {
		refs, err := s.findElements(ctx, locator{locID, "x"}, 10*time.Second)
		assert.NoError(t, err)
		assert.Len(t, refs, 2)
	})
	assert.Equal(t, 1, f.count(http.MethodPost, "/session/{id}/elements"))
}

func TestFR08_EveryFindAsksTheServerAgain(t *testing.T) {
	f := newFakeServer(t)
	f.on(elementsRoute, reply(http.StatusOK, elementList(1)))
	ctx, s := openSessionOf(t, f)
	for i := 0; i < 3; i++ {
		_, err := s.findElement(ctx, locator{locID, "x"}, time.Second)
		require.NoError(t, err)
	}
	assert.Equal(t, 3, f.count(http.MethodPost, "/session/{id}/elements"))
}

func TestFR08_AbsentVariantAndCountDoNotFail(t *testing.T) {
	shorten(t, &pollInterval, 5*time.Millisecond)
	f := newFakeServer(t)
	f.on(elementsRoute, reply(http.StatusOK, `[]`))
	ctx, s := openSessionOf(t, f)

	refs, err := s.findElements(ctx, locator{locID, "x"}, 30*time.Millisecond)
	require.NoError(t, err)
	assert.Empty(t, refs)

	n, err := s.countElements(ctx, locator{locID, "x"})
	require.NoError(t, err)
	assert.Zero(t, n)

	f.on(elementsRoute, reply(http.StatusOK, elementList(3)))
	n, err = s.countElements(ctx, locator{locID, "x"})
	require.NoError(t, err)
	assert.Equal(t, 3, n)
}

func TestFR07_TextIsEscapedInTheSelector(t *testing.T) {
	_, value := locator{locText, `say "hi" \ now` + "\n"}.strategy()
	assert.Equal(t, `new UiSelector().text("say \"hi\" \\ now\n")`, value)

	// the text cannot close the string and add a clause
	_, value = locator{locText, `x").clickable(true).text("y`}.strategy()
	assert.Equal(t, `new UiSelector().text("x\").clickable(true).text(\"y")`, value)
}

func TestFR08_ServerFailureAndBadAnswerStopThePoll(t *testing.T) {
	shorten(t, &pollInterval, 5*time.Millisecond)
	f := newFakeServer(t)
	ctx, s := openSessionOf(t, f)

	for name, h := range map[string]handler{
		"server error":   fail(http.StatusNotFound, "invalid session id", "gone"),
		"not a list":     reply(http.StatusOK, `{}`),
		"no reference":   reply(http.StatusOK, `[{"other":"x"}]`),
		"malformed list": reply(http.StatusOK, `"text"`),
	} {
		t.Run(name, func(t *testing.T) {
			f.on(elementsRoute, h)
			_, err := s.findElements(ctx, locator{locID, "x"}, 10*time.Second)
			require.Error(t, err)
		})
	}
}

func TestFR08_CancelledContextStopsPolling(t *testing.T) {
	shorten(t, &pollInterval, 20*time.Millisecond)
	f := newFakeServer(t)
	f.on(elementsRoute, reply(http.StatusOK, `[]`))
	ctx, s := openSessionOf(t, f)

	cctx, cancel := context.WithCancel(ctx)
	time.AfterFunc(60*time.Millisecond, cancel)
	within(t, 3*time.Second, func() {
		_, err := s.findElements(cctx, locator{locID, "x"}, time.Minute)
		assert.Error(t, err)
	})
}
