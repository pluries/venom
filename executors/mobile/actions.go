package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Values of the step field state, for waitFor.
const (
	statePresent = "present"
	stateAbsent  = "absent"
)

// maxScreenshotBytes caps a decoded screenshot (T8). A package variable so that tests can shorten it.
var maxScreenshotBytes = 12 << 20

// actions is the dispatch table of the step actions (FR-09, FR-10, FR-12). alert answers an iOS system alert and is
// refused on Android.
var actions = map[string]actionFunc{
	actionTap:      actTap,
	"fill":         actFill,
	"clear":        actClear,
	"read":         actRead,
	"count":        actCount,
	"waitFor":      actWaitFor,
	actionSwipe:    actSwipe,
	"scroll":       actScroll,
	"back":         actBack,
	"hideKeyboard": actHideKeyboard,
	"launch":       actLaunch,
	"terminate":    actTerminate,
	"screenshot":   actScreenshot,
	"source":       actSource,
	"context":      actContext,
	"openURL":      actOpenURL,
	"alert":        actAlert,
}

// emptyBody is the body of a W3C command that takes no argument: the protocol wants an object, not nothing.
func emptyBody() map[string]interface{} { return map[string]interface{}{} }

// targetElement finds the element a step names, waiting as FR-08 says, and marks the result as found.
func targetElement(ctx context.Context, s *session, step Executor, result *Result) (elementRef, error) {
	loc, err := step.locatorOn(s.ios())
	if err != nil {
		return "", err
	}
	wait, err := s.waitOf(step)
	if err != nil {
		return "", err
	}
	ref, err := s.findElement(ctx, loc, wait)
	if err != nil {
		return "", err
	}
	result.Found = true
	return ref, nil
}

// elementPath is the path of a command on one element.
func elementPath(ref elementRef, command string) string {
	return "/element/" + url.PathEscape(string(ref)) + "/" + command
}

// textOf decodes a W3C value that is a string.
func textOf(raw json.RawMessage) (string, error) {
	var out string
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", errors.New("the automation server answered with something else than text")
	}
	return out, nil
}

func actTap(ctx context.Context, s *session, step Executor, result *Result) error {
	if step.hasPoint() {
		return tapPoint(ctx, s, step)
	}
	ref, err := targetElement(ctx, s, step, result)
	if err != nil {
		return err
	}
	_, err = s.call(ctx, http.MethodPost, elementPath(ref, "click"), emptyBody())
	return err
}

// actFill types step.Value into the element, after clearing it when step.Clear is set. An empty value is allowed and
// types nothing: the element is still found (a missing one fails) and cleared on request, but no value command is
// sent. The typed text is never echoed in the result (NFR-09).
func actFill(ctx context.Context, s *session, step Executor, result *Result) error {
	ref, err := targetElement(ctx, s, step, result)
	if err != nil {
		return err
	}
	if step.Clear {
		if _, err := s.call(ctx, http.MethodPost, elementPath(ref, "clear"), emptyBody()); err != nil {
			return err
		}
	}
	if step.Value == "" {
		return nil
	}
	chars := make([]string, 0, len(step.Value))
	for _, r := range step.Value {
		chars = append(chars, string(r))
	}
	// both keys: UiAutomator2 reads text, the W3C protocol names value
	_, err = s.call(ctx, http.MethodPost, elementPath(ref, "value"), map[string]interface{}{"text": step.Value, "value": chars})
	return err
}

func actClear(ctx context.Context, s *session, step Executor, result *Result) error {
	ref, err := targetElement(ctx, s, step, result)
	if err != nil {
		return err
	}
	_, err = s.call(ctx, http.MethodPost, elementPath(ref, "clear"), emptyBody())
	return err
}

// actRead returns the element's text in result.text, or the attribute step.Attribute in result.value.
func actRead(ctx context.Context, s *session, step Executor, result *Result) error {
	ref, err := targetElement(ctx, s, step, result)
	if err != nil {
		return err
	}
	command := "text"
	if step.Attribute != "" {
		command = "attribute/" + url.PathEscape(step.Attribute)
	}
	raw, err := s.call(ctx, http.MethodGet, elementPath(ref, command), nil)
	if err != nil {
		return err
	}
	// an attribute the element does not have comes back as null: it reads as an empty value
	var text string
	if string(raw) != "null" {
		if text, err = textOf(raw); err != nil {
			return err
		}
	}
	if step.Attribute != "" {
		result.Value = text
	} else {
		result.Text = text
	}
	return nil
}

// actCount counts the matches right now. Zero is an answer, not a failure.
func actCount(ctx context.Context, s *session, step Executor, result *Result) error {
	loc, err := step.locatorOn(s.ios())
	if err != nil {
		return err
	}
	n, err := s.countElements(ctx, loc)
	if err != nil {
		return err
	}
	result.Count = n
	result.Found = n > 0
	return nil
}

// actWaitFor waits for the element to be present (the default) or absent, within the step's wait.
func actWaitFor(ctx context.Context, s *session, step Executor, result *Result) error {
	switch strings.ToLower(step.State) {
	case "", statePresent:
		_, err := targetElement(ctx, s, step, result)
		return err
	case stateAbsent:
		return waitAbsent(ctx, s, step, result)
	default:
		return usagef("state must be %s or %s, got %q", statePresent, stateAbsent, step.State)
	}
}

func waitAbsent(ctx context.Context, s *session, step Executor, result *Result) error {
	loc, err := step.locatorOn(s.ios())
	if err != nil {
		return err
	}
	wait, err := s.waitOf(step)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(wait)
	for {
		n, err := s.countElements(ctx, loc)
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			result.Found = true
			return errors.New(loc.String() + " is still present after " + wait.String())
		}
		if err := pause(ctx, min(pollInterval, remaining)); err != nil {
			return err
		}
	}
}

// pause sleeps for d, or until the step is cancelled.
func pause(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return errors.New("wait cancelled")
	case <-timer.C:
		return nil
	}
}

func actBack(ctx context.Context, s *session, _ Executor, _ *Result) error {
	if s.ios() {
		return iosBack()
	}
	_, err := s.call(ctx, http.MethodPost, "/back", emptyBody())
	return err
}

// actHideKeyboard hides the soft keyboard. UiAutomator2 answers an error when none is shown; that is the state the
// step wants, so it is not a failure.
func actHideKeyboard(ctx context.Context, s *session, _ Executor, _ *Result) error {
	_, err := s.call(ctx, http.MethodPost, "/execute/sync", executeBody("mobile: hideKeyboard", map[string]interface{}{}))
	if err != nil && isKeyboardAlreadyHidden(err) {
		return nil
	}
	return err
}

func isKeyboardAlreadyHidden(err error) bool {
	var failure *serverError
	if !errors.As(err, &failure) {
		return false
	}
	msg := strings.ToLower(failure.Message)
	return strings.Contains(msg, "keyboard") && (strings.Contains(msg, "not present") || strings.Contains(msg, "not shown") || strings.Contains(msg, "already hidden"))
}

// executeBody is the body of an Appium extension command: "mobile: <name>" with one argument object.
func executeBody(script string, args map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"script": script, "args": []interface{}{args}}
}

// appOf is the package a launch or terminate step acts on: its appId, else the session's mobile.appId.
func appOf(s *session, step Executor) (string, error) {
	if step.AppID != "" {
		return step.AppID, nil
	}
	if s.facts.appID != "" {
		return s.facts.appID, nil
	}
	return "", usagef("no app id: give appId in the step or set mobile.appId")
}

func actLaunch(ctx context.Context, s *session, step Executor, _ *Result) error {
	return appCommand(ctx, s, step, "mobile: activateApp")
}

func actTerminate(ctx context.Context, s *session, step Executor, _ *Result) error {
	return appCommand(ctx, s, step, "mobile: terminateApp")
}

func appCommand(ctx context.Context, s *session, step Executor, script string) error {
	if s.ios() {
		return iosAppCommand(ctx, s, step, script)
	}
	appID, err := appOf(s, step)
	if err != nil {
		return err
	}
	_, err = s.call(ctx, http.MethodPost, "/execute/sync", executeBody(script, map[string]interface{}{"appId": appID}))
	return err
}

// actScreenshot takes a screenshot. With step.Path it writes the decoded PNG there (relative to the process's working
// directory, owner-only) and names the file in result.screenshot; without it nothing is written and the step only
// proves that the device renders.
func actScreenshot(ctx context.Context, s *session, step Executor, result *Result) error {
	raw, err := s.call(ctx, http.MethodGet, "/screenshot", nil)
	if err != nil {
		return err
	}
	data, err := decodeScreenshot(raw)
	if err != nil {
		return err
	}
	result.Found = true
	if step.Path == "" {
		return nil
	}
	if err := writeOwnerOnly(step.Path, data); err != nil {
		return err
	}
	result.Screenshot = step.Path
	return nil
}

// actSource returns the page source (the view hierarchy as XML) in result.source.
func actSource(ctx context.Context, s *session, _ Executor, result *Result) error {
	raw, err := s.call(ctx, http.MethodGet, "/source", nil)
	if err != nil {
		return err
	}
	source, err := textOf(raw)
	if err != nil {
		return err
	}
	if source == "" {
		return errors.New("the automation server returned an empty page source")
	}
	result.Source = source
	result.Found = true
	return nil
}
