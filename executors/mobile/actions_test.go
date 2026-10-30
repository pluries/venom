package mobile

import (
	"context"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

const (
	clickRoute = "POST /session/{id}/element/el-0/click"
	clearRoute = "POST /session/{id}/element/el-0/clear"
	valueRoute = "POST /session/{id}/element/el-0/value"
	textRoute  = "GET /session/{id}/element/el-0/text"
	execRoute  = "POST /session/{id}/execute/sync"
	rectRoute  = "GET /session/{id}/window/rect"
	shownRoute = "GET /session/{id}/element/el-0/displayed"
)

// found makes the fake find one element for any locator.
func found(f *fakeServer) { f.on(elementsRoute, reply(http.StatusOK, elementList(1))) }

// runWith runs a step with the given fields and returns its Result.
func runWith(ctx context.Context, t *testing.T, step venom.TestStep) Result {
	t.Helper()
	if _, set := step["syncTimeout"]; !set {
		step["syncTimeout"] = 0.05 // the session's whole seconds are too coarse for a test
	}
	out, err := Executor{}.Run(ctx, step)
	require.NoError(t, err)
	result, ok := out.(Result)
	require.True(t, ok, "Run returned %T", out)
	return result
}

// usageOf runs a step that must be refused as a misuse.
func usageOf(ctx context.Context, t *testing.T, step venom.TestStep) error {
	t.Helper()
	_, err := Executor{}.Run(ctx, step)
	var misuse *usageError
	require.ErrorAs(t, err, &misuse)
	return err
}

// caseOn opens a test case on a fresh fake server with a short poll interval.
func caseOn(t *testing.T, extra map[string]interface{}) (context.Context, *fakeServer) {
	t.Helper()
	shorten(t, &pollInterval, 5*time.Millisecond)
	f := newFakeServer(t)
	vars := map[string]interface{}{}
	for k, v := range extra {
		vars[k] = v
	}
	return openCase(t, f.withVars(vars)), f
}

func TestFR09_TheTableHoldsExactlyTheSeventeenActions(t *testing.T) {
	names := make([]string, 0, len(actions))
	for name := range actions {
		names = append(names, name)
	}
	sort.Strings(names)
	assert.Equal(t, []string{"alert", "back", "clear", "context", "count", "fill", "hideKeyboard", "launch", "openURL", "read", "screenshot", "scroll", "source", "swipe", "tap", "terminate", "waitFor"}, names)

	ctx, _ := caseOn(t, nil)
	for _, unknown := range []string{"pinch", "tapAt"} {
		err := usageOf(ctx, t, venom.TestStep{"action": unknown})
		assert.Contains(t, err.Error(), "supported actions: alert, back, clear, context")
	}
}

func TestAC6_TapClicksTheElement(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(clickRoute, reply(http.StatusOK, `null`))

	result := runWith(ctx, t, venom.TestStep{"action": "tap", "accessibilityId": "login"})
	assert.Empty(t, result.Err)
	assert.True(t, result.Found)
	assert.Equal(t, 1, f.count(http.MethodPost, "/session/{id}/element/el-0/click"))
}

func TestAC6_TapOnAMissingElementFailsNamingTheLocator(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(elementsRoute, reply(http.StatusOK, `[]`))

	result := runWith(ctx, t, venom.TestStep{"action": "tap", "id": "gone"})
	assert.Contains(t, result.Err, `id "gone"`)
	assert.False(t, result.Found)
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/element/el-0/click"))
}

func TestAC6_FillClearsThenTypesWithBothKeys(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(clearRoute, reply(http.StatusOK, `null`))
	f.on(valueRoute, reply(http.StatusOK, `null`))

	result := runWith(ctx, t, venom.TestStep{"action": "fill", "id": "name", "value": "Ada", "clear": true})
	assert.Empty(t, result.Err)
	assert.Empty(t, result.Text, "the typed text is not echoed")
	assert.Empty(t, result.Value)
	body := f.first(t, http.MethodPost, "/session/{id}/element/el-0/value")
	assert.Equal(t, "Ada", body["text"])
	assert.Equal(t, []interface{}{"A", "d", "a"}, body["value"])
	assert.Equal(t, 1, f.count(http.MethodPost, "/session/{id}/element/el-0/clear"))
}

func TestAC6_FillWithoutClearDoesNotClear(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(valueRoute, reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "fill", "id": "name", "value": "x"}).Err)
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/element/el-0/clear"))
}

func TestAC6_FillWithAnEmptyValueTypesNothing(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(clearRoute, reply(http.StatusOK, `null`))

	result := runWith(ctx, t, venom.TestStep{"action": "fill", "id": "name", "clear": true})
	assert.Empty(t, result.Err)
	assert.Equal(t, 1, f.count(http.MethodPost, "/session/{id}/element/el-0/clear"), "clear still runs")
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/element/el-0/value"))
}

func TestAC6_FillOnAMissingElementFails(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(elementsRoute, reply(http.StatusOK, `[]`))

	result := runWith(ctx, t, venom.TestStep{"action": "fill", "id": "nope", "value": "x"})
	assert.Contains(t, result.Err, `id "nope"`)
}

func TestAC6_ClearClearsTheElement(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(clearRoute, reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "clear", "id": "name"}).Err)
	assert.Equal(t, 1, f.count(http.MethodPost, "/session/{id}/element/el-0/clear"))
}

func TestAC6_ClearReportsAServerRefusal(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(clearRoute, fail(http.StatusBadRequest, "invalid element state", "not editable"))

	result := runWith(ctx, t, venom.TestStep{"action": "clear", "id": "label"})
	assert.Contains(t, result.Err, "invalid element state")
	assert.NotContains(t, result.Err, "secret.Frame", "the stack trace is dropped")
}

func TestAC6_ReadReturnsTheTextAndTheAttribute(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(textRoute, reply(http.StatusOK, `"Welcome"`))
	f.on("GET /session/{id}/element/el-0/attribute/checked", reply(http.StatusOK, `"true"`))

	text := runWith(ctx, t, venom.TestStep{"action": "read", "id": "title"})
	assert.Equal(t, "Welcome", text.Text)
	assert.Empty(t, text.Value)

	attr := runWith(ctx, t, venom.TestStep{"action": "read", "id": "box", "attribute": "checked"})
	assert.Equal(t, "true", attr.Value)
	assert.Empty(t, attr.Text)
}

func TestAC6_ReadOfEmptyTextAndMissingAttributeIsEmptyNotAFailure(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(textRoute, reply(http.StatusOK, `""`))
	f.on("GET /session/{id}/element/el-0/attribute/hint", reply(http.StatusOK, `null`))

	empty := runWith(ctx, t, venom.TestStep{"action": "read", "id": "title"})
	assert.Empty(t, empty.Err)
	assert.True(t, empty.Found)
	assert.Empty(t, empty.Text)
	none := runWith(ctx, t, venom.TestStep{"action": "read", "id": "title", "attribute": "hint"})
	assert.Empty(t, none.Err)
	assert.Empty(t, none.Value)
}

func TestAC6_ReadOfNonTextFails(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(textRoute, reply(http.StatusOK, `{"a":1}`))

	assert.Contains(t, runWith(ctx, t, venom.TestStep{"action": "read", "id": "title"}).Err, "something else than text")
}

func TestAC6_CountReturnsTheNumberOfMatches(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(elementsRoute, reply(http.StatusOK, elementList(3)))

	result := runWith(ctx, t, venom.TestStep{"action": "count", "xpath": "//row"})
	assert.Empty(t, result.Err)
	assert.Equal(t, 3, result.Count)
	assert.True(t, result.Found)
}

func TestAC6_CountOfZeroIsNotAFailureAndDoesNotWait(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(elementsRoute, reply(http.StatusOK, `[]`))

	within(t, 2*time.Second, func() {
		result := runWith(ctx, t, venom.TestStep{"action": "count", "xpath": "//row"})
		assert.Empty(t, result.Err)
		assert.Zero(t, result.Count)
		assert.False(t, result.Found)
	})
	assert.Equal(t, 1, f.count(http.MethodPost, "/session/{id}/elements"))
}

func TestAC6_WaitForPresentIsTheDefaultAndFindsTheElement(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)

	assert.True(t, runWith(ctx, t, venom.TestStep{"action": "waitFor", "id": "home"}).Found)
	assert.True(t, runWith(ctx, t, venom.TestStep{"action": "waitFor", "id": "home", "state": "present"}).Found)
}

func TestAC6_WaitForPresentThatNeverComesFailsNamingLocatorAndWait(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(elementsRoute, reply(http.StatusOK, `[]`))

	result := runWith(ctx, t, venom.TestStep{"action": "waitFor", "id": "home"})
	assert.Contains(t, result.Err, `id "home"`)
	assert.Contains(t, result.Err, "50ms")
}

func TestAC6_WaitForAbsentSucceedsOnceTheElementIsGone(t *testing.T) {
	ctx, f := caseOn(t, nil)
	calls := 0
	f.on(elementsRoute, func(w http.ResponseWriter, r *http.Request, body []byte) {
		calls++
		if calls < 3 {
			reply(http.StatusOK, elementList(1))(w, r, body)
			return
		}
		reply(http.StatusOK, `[]`)(w, r, body)
	})

	result := runWith(ctx, t, venom.TestStep{"action": "waitFor", "id": "spinner", "state": "absent", "syncTimeout": 5})
	assert.Empty(t, result.Err)
	assert.Equal(t, 3, f.count(http.MethodPost, "/session/{id}/elements"))
}

func TestAC6_WaitForAbsentThatStaysFailsNamingLocatorAndWait(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)

	result := runWith(ctx, t, venom.TestStep{"action": "waitFor", "id": "spinner", "state": "absent"})
	assert.Contains(t, result.Err, `id "spinner" is still present after 50ms`)
}

func TestAC6_WaitForRefusesAnUnknownState(t *testing.T) {
	ctx, _ := caseOn(t, nil)
	err := usageOf(ctx, t, venom.TestStep{"action": "waitFor", "id": "x", "state": "gone"})
	assert.Contains(t, err.Error(), `"gone"`)
}

func TestAC6_SwipeInTheVisibleAreaSendsDirectionPercentAndRect(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(rectRoute, reply(http.StatusOK, `{"x":0,"y":0,"width":1080,"height":2400}`))
	f.on(execRoute, reply(http.StatusOK, `null`))

	result := runWith(ctx, t, venom.TestStep{"action": "swipe", "direction": "left"})
	assert.Empty(t, result.Err)
	body := f.first(t, http.MethodPost, "/session/{id}/execute/sync")
	assert.Equal(t, "mobile: swipeGesture", body["script"])
	args, ok := body["args"].([]interface{})[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "left", args["direction"])
	assert.InDelta(t, 0.75, args["percent"], 1e-9)
	assert.InDelta(t, 864, args["width"], 1e-9)
	assert.InDelta(t, 1920, args["height"], 1e-9)
	assert.NotContains(t, args, "elementId")
}

func TestAC6_SwipeInsideAnElementSendsItsIdAndTheGivenPercent(t *testing.T) {
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(execRoute, reply(http.StatusOK, `null`))

	result := runWith(ctx, t, venom.TestStep{"action": "swipe", "id": "pager", "direction": "right", "percent": 0.5})
	assert.Empty(t, result.Err)
	args, ok := f.first(t, http.MethodPost, "/session/{id}/execute/sync")["args"].([]interface{})[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "el-0", args["elementId"])
	assert.InDelta(t, 0.5, args["percent"], 1e-9)
	assert.Zero(t, f.count(http.MethodGet, "/session/{id}/window/rect"), "no area is asked for when an element is named")
}

func TestAC6_SwipeRefusesABadDirectionAndABadPercent(t *testing.T) {
	ctx, f := caseOn(t, nil)
	assert.Contains(t, usageOf(ctx, t, venom.TestStep{"action": "swipe"}).Error(), "direction")
	assert.Contains(t, usageOf(ctx, t, venom.TestStep{"action": "swipe", "direction": "diagonal"}).Error(), `"diagonal"`)
	assert.Contains(t, usageOf(ctx, t, venom.TestStep{"action": "swipe", "direction": "up", "percent": 1.5}).Error(), "percent")
	assert.Contains(t, usageOf(ctx, t, venom.TestStep{"action": "swipe", "direction": "up", "percent": 0.0}).Error(), "percent")
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/execute/sync"))
}

func TestAC6_SwipeWithAnUnusableWindowRectFails(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(rectRoute, reply(http.StatusOK, `{"x":0,"y":0,"width":0,"height":0}`))

	assert.Contains(t, runWith(ctx, t, venom.TestStep{"action": "swipe", "direction": "up"}).Err, "usable area")
}

func TestAC6_ScrollByDirectionSendsAScrollGesture(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(rectRoute, reply(http.StatusOK, `{"x":0,"y":0,"width":1080,"height":2400}`))
	f.on(execRoute, reply(http.StatusOK, `true`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "scroll", "direction": "down"}).Err)
	assert.Equal(t, "mobile: scrollGesture", f.first(t, http.MethodPost, "/session/{id}/execute/sync")["script"])
}

func TestAC6_ScrollUntilStopsAsSoonAsTheElementIsVisible(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(rectRoute, reply(http.StatusOK, `{"x":0,"y":0,"width":100,"height":200}`))
	f.on(execRoute, reply(http.StatusOK, `true`))
	f.on(shownRoute, reply(http.StatusOK, `true`))
	queries := 0
	f.on(elementsRoute, func(w http.ResponseWriter, r *http.Request, body []byte) {
		queries++
		if queries < 3 {
			reply(http.StatusOK, `[]`)(w, r, body)
			return
		}
		reply(http.StatusOK, elementList(1))(w, r, body)
	})

	result := runWith(ctx, t, venom.TestStep{"action": "scroll", "text": "Terms", "direction": "down", "until": true})
	assert.Empty(t, result.Err)
	assert.True(t, result.Found)
	assert.Equal(t, 2, f.count(http.MethodPost, "/session/{id}/execute/sync"))
}

func TestAC6_ScrollUntilANeverFoundElementFailsAfterTheBoundNamingTheLocator(t *testing.T) {
	shorten(t, &maxScrollAttempts, 3)
	ctx, f := caseOn(t, nil)
	f.on(elementsRoute, reply(http.StatusOK, `[]`))
	f.on(rectRoute, reply(http.StatusOK, `{"x":0,"y":0,"width":100,"height":200}`))
	f.on(execRoute, reply(http.StatusOK, `true`))

	result := runWith(ctx, t, venom.TestStep{"action": "scroll", "text": "Nowhere", "direction": "down", "until": true})
	assert.Contains(t, result.Err, `text "Nowhere"`)
	assert.Contains(t, result.Err, "at most 3 scrolls")
	assert.Equal(t, 3, f.count(http.MethodPost, "/session/{id}/execute/sync"))
}

func TestAC6_ScrollUntilStopsAtTheEndOfTheList(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(elementsRoute, reply(http.StatusOK, `[]`))
	f.on(rectRoute, reply(http.StatusOK, `{"x":0,"y":0,"width":100,"height":200}`))
	f.on(execRoute, reply(http.StatusOK, `false`))

	result := runWith(ctx, t, venom.TestStep{"action": "scroll", "text": "Nowhere", "direction": "down", "until": true})
	assert.Contains(t, result.Err, `text "Nowhere"`)
	assert.Equal(t, 1, f.count(http.MethodPost, "/session/{id}/execute/sync"), "no scroll after the server says there is no more")
}

func TestAC6_ScrollUntilIgnoresAnElementThatIsNotDisplayed(t *testing.T) {
	shorten(t, &maxScrollAttempts, 2)
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(shownRoute, reply(http.StatusOK, `false`))
	f.on(rectRoute, reply(http.StatusOK, `{"x":0,"y":0,"width":100,"height":200}`))
	f.on(execRoute, reply(http.StatusOK, `true`))

	result := runWith(ctx, t, venom.TestStep{"action": "scroll", "text": "Hidden", "direction": "up", "until": true})
	assert.Contains(t, result.Err, "not visible")
	assert.False(t, result.Found)
}

func TestAC6_ScrollUntilNeedsALocator(t *testing.T) {
	ctx, _ := caseOn(t, nil)
	assert.Contains(t, usageOf(ctx, t, venom.TestStep{"action": "scroll", "direction": "down", "until": true}).Error(), "no locator")
}

func TestAC6_BackPressesBack(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on("POST /session/{id}/back", reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "back"}).Err)
	assert.Equal(t, 1, f.count(http.MethodPost, "/session/{id}/back"))
}

func TestAC6_HideKeyboardHidesIt(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(execRoute, reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "hideKeyboard"}).Err)
	assert.Equal(t, "mobile: hideKeyboard", f.first(t, http.MethodPost, "/session/{id}/execute/sync")["script"])
}

func TestAC6_HideKeyboardWhenAlreadyHiddenIsNotAFailure(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(execRoute, fail(http.StatusInternalServerError, "unknown error", "Soft keyboard not present, cannot hide keyboard"))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "hideKeyboard"}).Err)
}

func TestAC6_HideKeyboardStillReportsOtherFailures(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(execRoute, fail(http.StatusInternalServerError, "unknown error", "device offline"))

	assert.Contains(t, runWith(ctx, t, venom.TestStep{"action": "hideKeyboard"}).Err, "device offline")
}

func TestAC6_LaunchAndTerminateUseTheStepsOrTheSessionsAppId(t *testing.T) {
	ctx, f := caseOn(t, map[string]interface{}{"appId": "org.example.app"})
	f.on(execRoute, reply(http.StatusOK, `true`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "launch"}).Err)
	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "terminate", "appId": "org.example.other"}).Err)

	f.mu.Lock()
	defer f.mu.Unlock()
	var bodies []string
	for _, r := range f.reqs {
		if r.Path == "/session/{id}/execute/sync" {
			bodies = append(bodies, r.Body)
		}
	}
	require.Len(t, bodies, 2)
	assert.JSONEq(t, `{"script":"mobile: activateApp","args":[{"appId":"org.example.app"}]}`, bodies[0])
	assert.JSONEq(t, `{"script":"mobile: terminateApp","args":[{"appId":"org.example.other"}]}`, bodies[1])
}

func TestAC6_LaunchWithNoAppIdIsAUsageError(t *testing.T) {
	ctx, f := caseOn(t, nil)

	assert.Contains(t, usageOf(ctx, t, venom.TestStep{"action": "launch"}).Error(), "no app id")
	assert.Contains(t, usageOf(ctx, t, venom.TestStep{"action": "terminate"}).Error(), "no app id")
	assert.Zero(t, f.count(http.MethodPost, "/session/{id}/execute/sync"))
}

func TestAC6_ScreenshotWritesTheDecodedPngOwnerOnly(t *testing.T) {
	requireUnixModes(t)
	dir := t.TempDir()
	t.Chdir(dir)
	ctx, f := caseOn(t, nil)
	png := []byte("\x89PNG synthetic")
	f.on("GET /session/{id}/screenshot", reply(http.StatusOK, `"`+base64.StdEncoding.EncodeToString(png)+`"`))
	require.NoError(t, os.WriteFile("shot.png", []byte("old"), 0o644)) //nolint:gosec // G306: a looser file is the point, the step must narrow it

	result := runWith(ctx, t, venom.TestStep{"action": "screenshot", "path": "shot.png"})
	assert.Empty(t, result.Err)
	assert.Equal(t, "shot.png", result.Screenshot)
	got, err := os.ReadFile(filepath.Join(dir, "shot.png"))
	require.NoError(t, err)
	assert.Equal(t, png, got)
	info, err := os.Stat("shot.png")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "an existing 0644 file ends up 0600")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temporary file is left")
}

func TestAC6_ScreenshotWithoutAPathWritesNothing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	ctx, f := caseOn(t, nil)
	f.on("GET /session/{id}/screenshot", reply(http.StatusOK, `"`+base64.StdEncoding.EncodeToString([]byte("x"))+`"`))

	result := runWith(ctx, t, venom.TestStep{"action": "screenshot"})
	assert.Empty(t, result.Err)
	assert.True(t, result.Found)
	assert.Empty(t, result.Screenshot)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestAC6_ScreenshotRefusesBadBase64EmptyAndOversizeAnswers(t *testing.T) {
	t.Chdir(t.TempDir())
	ctx, f := caseOn(t, nil)

	f.on("GET /session/{id}/screenshot", reply(http.StatusOK, `"!!not base64!!"`))
	assert.Contains(t, runWith(ctx, t, venom.TestStep{"action": "screenshot", "path": "a.png"}).Err, "not valid base64")

	f.on("GET /session/{id}/screenshot", reply(http.StatusOK, `""`))
	assert.Contains(t, runWith(ctx, t, venom.TestStep{"action": "screenshot", "path": "a.png"}).Err, "empty screenshot")

	shorten(t, &maxScreenshotBytes, 3)
	f.on("GET /session/{id}/screenshot", reply(http.StatusOK, `"`+base64.StdEncoding.EncodeToString([]byte("four"))+`"`))
	assert.Contains(t, runWith(ctx, t, venom.TestStep{"action": "screenshot", "path": "a.png"}).Err, "larger than")

	_, err := os.Stat("a.png")
	assert.True(t, os.IsNotExist(err), "no file for a refused screenshot")
}

func TestAC6_ScreenshotToAMissingFolderFailsTheStep(t *testing.T) {
	t.Chdir(t.TempDir())
	ctx, f := caseOn(t, nil)
	f.on("GET /session/{id}/screenshot", reply(http.StatusOK, `"`+base64.StdEncoding.EncodeToString([]byte("x"))+`"`))

	result := runWith(ctx, t, venom.TestStep{"action": "screenshot", "path": "no/such/dir/a.png"})
	assert.Contains(t, result.Err, "cannot write")
	assert.Empty(t, result.Screenshot)
}

func TestAC6_SourceReturnsThePageSource(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on("GET /session/{id}/source", reply(http.StatusOK, `"<hierarchy/>"`))

	result := runWith(ctx, t, venom.TestStep{"action": "source"})
	assert.Empty(t, result.Err)
	assert.Equal(t, "<hierarchy/>", result.Source)
}

func TestAC6_EmptySourceFailsTheStep(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on("GET /session/{id}/source", reply(http.StatusOK, `""`))

	result := runWith(ctx, t, venom.TestStep{"action": "source"})
	assert.Contains(t, result.Err, "empty page source")
	assert.Empty(t, result.Source)
}

func TestNFR09_TypedTextNeverReachesTheLog(t *testing.T) {
	logs := captureLogs(t)
	ctx, f := caseOn(t, nil)
	found(f)
	f.on(valueRoute, reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "fill", "id": "pw", "value": "synthetic-typed-text"}).Err)
	assert.NotContains(t, logs.text(), "synthetic-typed-text")
}

func TestFR09_WriteOwnerOnlyReplacesAFileAndLeavesNoTemporaryOnFailure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.bin")
	require.NoError(t, writeOwnerOnly(target, []byte("one")))
	require.NoError(t, writeOwnerOnly(target, []byte("two")))
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "two", string(got))

	// the target is a directory: the rename fails and the temporary file goes
	blocked := filepath.Join(dir, "blocked")
	require.NoError(t, os.Mkdir(blocked, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(blocked, "keep"), nil, 0o600))
	require.Error(t, writeOwnerOnly(blocked, []byte("x")))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "out.bin and blocked only")
}
