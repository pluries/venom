package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

// pollInterval is the pause between two attempts to find an element (FR-08, NFR-08). A package variable so that tests
// can shorten it.
var pollInterval = 500 * time.Millisecond

// elementKey is the key of an element reference in a W3C answer.
const elementKey = "element-6066-11e4-a52e-4f735466cecf"

// W3C location strategies of UIAutomator2.
const (
	strategyAccessibilityID = "accessibility id"
	strategyID              = "id"
	strategyXPath           = "xpath"
	strategyUIAutomator     = "-android uiautomator"
	strategyCSS             = "css selector"
	strategyName            = "name"
	strategyIOSPredicate    = "-ios predicate string"
	strategyIOSClassChain   = "-ios class chain"

	// nativeContext is the context name of the native app; a CSS selector has nothing to search there.
	nativeContext = "NATIVE_APP"
)

// Step fields that name a locator, in the order the failure messages list them.
const (
	locAccessibilityID = "accessibilityId"
	locText            = "text"
	locID              = "id"
	locXPath           = "xpath"
	locUIAutomator     = "uiautomator"
	locCSS             = "css"
	locName            = "name"       // iOS only
	locPredicate       = "predicate"  // iOS only
	locClassChain      = "classChain" // iOS only
)

// The kinds each platform accepts; the "give exactly one of" lists are these.
var (
	androidKinds = []string{locAccessibilityID, locText, locID, locXPath, locUIAutomator, locCSS}
	iosKinds     = []string{locAccessibilityID, locText, locName, locPredicate, locClassChain, locXPath, locCSS}
)

// locator is the one way a step names an element (FR-07).
type locator struct {
	kind  string // the step field it came from
	value string
}

func (l locator) String() string { return fmt.Sprintf("%s %q", l.kind, l.value) }

// strategy returns the W3C strategy and selector this locator sends.
func (l locator) strategy() (using, value string) {
	switch l.kind {
	case locAccessibilityID:
		return strategyAccessibilityID, l.value
	case locText:
		return strategyUIAutomator, `new UiSelector().text("` + selectorEscaper.Replace(l.value) + `")`
	case locID:
		return strategyID, l.value
	case locXPath:
		return strategyXPath, l.value
	case locCSS:
		return strategyCSS, l.value
	default:
		return strategyUIAutomator, l.value
	}
}

// strategyOn returns the strategy and selector this locator sends on a platform.
func (l locator) strategyOn(ios bool) (using, value string) {
	if ios {
		return l.iosStrategy()
	}
	return l.strategy()
}

// iosStrategy is the XCUITest mapping. The predicate of text is built here and the suite text is escaped, so it cannot
// extend the predicate it sits in.
func (l locator) iosStrategy() (using, value string) {
	switch l.kind {
	case locText:
		return strategyIOSPredicate, `label == "` + selectorEscaper.Replace(l.value) + `"`
	case locPredicate:
		return strategyIOSPredicate, l.value
	case locClassChain:
		return strategyIOSClassChain, l.value
	case locName:
		return strategyName, l.value
	case locAccessibilityID:
		return strategyAccessibilityID, l.value
	case locCSS:
		return strategyCSS, l.value
	default:
		return strategyXPath, l.value
	}
}

// wrongPlatform is the usage error for a locator kind the platform does not have.
func wrongPlatform(kind string, ios bool) error {
	switch {
	case kind == locID && ios:
		return usagef("id is the Android resource id: on iOS use accessibilityId or name")
	case kind == locUIAutomator && ios:
		return usagef("uiautomator is Android only")
	default:
		return usagef("%s is iOS only: on Android use accessibilityId, text, id, xpath or uiautomator", kind)
	}
}

// selectorEscaper makes suite text a Java string literal, so that it cannot close the selector it sits in.
var selectorEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`)

// candidates lists every locator field of the step, set or not. It is the one list the locator rules read: a new
// locator kind is added here and nowhere else.
func (e Executor) candidates() []locator {
	return []locator{
		{locAccessibilityID, e.AccessibilityID},
		{locText, e.Text},
		{locID, e.ID},
		{locXPath, e.XPath},
		{locUIAutomator, e.UIAutomator},
		{locCSS, e.CSS},
		{locName, e.Name},
		{locPredicate, e.Predicate},
		{locClassChain, e.ClassChain},
	}
}

// hasLocator tells whether the step names an element at all.
func (e Executor) hasLocator() bool {
	for _, c := range e.candidates() {
		if c.value != "" {
			return true
		}
	}
	return false
}

// kindsOn lists the locator kinds a platform accepts, in the order the failure messages name them.
func kindsOn(ios bool) []string {
	if ios {
		return iosKinds
	}
	return androidKinds
}

// locator returns the single locator of an Android step. Kept for the code that has no session at hand.
func (e Executor) locator() (locator, error) { return e.locatorOn(false) }

// locatorOn returns the single locator of the step on a platform. The platform is a parameter, read by the callers from
// the session (s.ios()), so that the Android path is the same code with ios == false. A kind of the other platform is
// refused before any request; zero or several is a usage error naming the platform's fields.
func (e Executor) locatorOn(ios bool) (locator, error) {
	names := kindsOn(ios)
	var set []locator
	for _, c := range e.candidates() {
		if c.value == "" {
			continue
		}
		if !slices.Contains(names, c.kind) {
			return locator{}, wrongPlatform(c.kind, ios)
		}
		set = append(set, c)
	}
	if len(set) == 1 {
		return set[0], nil
	}
	if len(set) == 0 {
		return locator{}, usagef("no locator: give exactly one of %s", strings.Join(names, ", "))
	}
	given := make([]string, len(set))
	for i, c := range set {
		given[i] = c.kind
	}
	return locator{}, usagef("one locator per step: got %s, give exactly one of %s", strings.Join(given, " and "), strings.Join(names, ", "))
}

// waitOf is the implicit wait of a step: its syncTimeout in seconds, else the session's.
func (s *session) waitOf(step Executor) (time.Duration, error) {
	if step.SyncTimeout == nil {
		return s.syncTimeout, nil
	}
	if *step.SyncTimeout < 0 {
		return 0, usagef("syncTimeout must not be negative, got %v", *step.SyncTimeout)
	}
	return time.Duration(*step.SyncTimeout * float64(time.Second)), nil
}

// elementRef is the id the server gave an element. It is valid for the current find only: every call looks again.
type elementRef string

// checkContext refuses a css locator while the session is in the native context. Only css asks; the other locators
// make no extra call. A failing GET /context comes back as it is: the executor does not guess the context.
func (s *session) checkContext(ctx context.Context, loc locator) error {
	if loc.kind != locCSS {
		return nil
	}
	raw, err := s.call(ctx, http.MethodGet, "/context", nil)
	if err != nil {
		return err
	}
	if current, err := textOf(raw); err == nil && current == nativeContext {
		return usagef("css locators search a web view: switch to one with the context action first, the session is in %s", nativeContext)
	}
	return nil
}

// queryElements asks the server once for the elements of a locator.
func (s *session) queryElements(ctx context.Context, loc locator) ([]elementRef, error) {
	if err := s.checkContext(ctx, loc); err != nil {
		return nil, err
	}
	return s.postElements(ctx, loc)
}

// postElements sends the search of a locator, without any check.
func (s *session) postElements(ctx context.Context, loc locator) ([]elementRef, error) {
	using, value := loc.strategyOn(s.ios())
	raw, err := s.call(ctx, http.MethodPost, "/elements", map[string]string{"using": using, "value": value})
	if err != nil {
		return nil, err
	}
	var entries []map[string]string
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, errors.New("the automation server answered /elements with something else than a list")
	}
	refs := make([]elementRef, 0, len(entries))
	for _, entry := range entries {
		id := entry[elementKey]
		if id == "" {
			return nil, errors.New("the automation server answered an element without a reference")
		}
		refs = append(refs, elementRef(id))
	}
	return refs, nil
}

// findElements looks for the elements of a locator until there is one or the wait runs out (FR-08). Finding nothing is
// not a failure: it returns an empty list, for the callers that wait for absence. The first attempt is immediate, so
// an element that is already there costs no delay (NFR-08).
func (s *session) findElements(ctx context.Context, loc locator, wait time.Duration) ([]elementRef, error) {
	if err := s.checkContext(ctx, loc); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		refs, err := s.postElements(ctx, loc)
		if err != nil || len(refs) > 0 {
			return refs, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, nil
		}
		timer := time.NewTimer(min(pollInterval, remaining))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, errors.New("wait cancelled")
		case <-timer.C:
		}
	}
}

// findElement returns the first element of a locator, or fails naming the locator and the wait (AC-5).
func (s *session) findElement(ctx context.Context, loc locator, wait time.Duration) (elementRef, error) {
	refs, err := s.findElements(ctx, loc, wait)
	if err != nil {
		return "", err
	}
	if len(refs) == 0 {
		return "", fmt.Errorf("no element found for %s within %s", loc, wait)
	}
	return refs[0], nil
}

// countElements is the number of elements a locator matches right now, without waiting.
func (s *session) countElements(ctx context.Context, loc locator) (int, error) {
	refs, err := s.queryElements(ctx, loc)
	return len(refs), err
}
