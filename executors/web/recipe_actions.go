package web

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/mxschmitt/playwright-go"

	"github.com/ovh/venom"
)

// The field names the new actions share.
const (
	fieldFind        = "find"
	fieldSyncTimeout = "syncTimeout"
	actionPress      = "press"
)

// defaultWaitSeconds is the timeout of the context a step without syncTimeout waits for.
const defaultWaitSeconds = 30

// waitStates are the states waitFor takes; the first is the default.
var waitStates = []string{"visible", "hidden", "attached", "detached"}

// existingActions are the actions of performAction that come before the new ones, in its order.
var existingActions = []string{
	"click", "fill", fieldFind, "navigate", "wait", "confirmPopup", "cancelPopup", "select", "uploadFile", "selectFrame",
	"selectRootFrame", "nextWindow", "historyAction", "execute",
}

// newActionSpec is what a step may set under one new action: its fields, those it must set, and a hint added when a
// required one is missing.
type newActionSpec struct {
	name     string
	fields   []string
	required []string
	hint     string
}

// newActionSpecs are the new actions in performAction's order.
var newActionSpecs = []newActionSpec{
	{name: "waitFor", fields: []string{fieldFind, "state", fieldSyncTimeout}, required: []string{fieldFind}},
	{name: "hover", fields: []string{fieldFind, fieldSyncTimeout}, required: []string{fieldFind}},
	{name: "dragAndDrop", fields: []string{"from", "to", fieldSyncTimeout}, required: []string{"from", "to"}},
	{name: actionPress, fields: []string{"key", fieldFind, fieldSyncTimeout}, required: []string{"key"}, hint: "press takes one key or one chord"},
}

// validateNewActions refuses, before any browser action, a malformed waitFor, hover, dragAndDrop or press on the raw
// step: it is the one that would run, no earlier action of performAction's order being set.
func validateNewActions(step venom.TestStep) error {
	raw, _ := lookupField(step, "action")
	fields, ok := asMap(raw)
	if !ok || existingActionSet(fields) {
		return nil
	}
	for _, spec := range newActionSpecs {
		if v, _ := lookupField(fields, spec.name); v != nil {
			return spec.validate(v)
		}
	}
	return nil
}

// lookupField reads a field the way mapstructure matches it: case-insensitively.
func lookupField(m map[string]interface{}, name string) (interface{}, bool) {
	if v, ok := m[name]; ok {
		return v, true
	}
	for k, v := range m {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return nil, false
}

// asMap reads a YAML mapping, whichever key type the decoder gave it.
func asMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	case venom.H:
		return m, true
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out, true
	}
	return nil, false
}

// existingActionSet reports whether an action that performAction runs before the new ones is set: present and not the
// zero value of its type, as the decode and performAction read it; find is an interface{}, set whatever its value.
func existingActionSet(fields map[string]interface{}) bool {
	for _, name := range existingActions {
		if v, _ := lookupField(fields, name); v != nil && (name == fieldFind || !reflect.ValueOf(v).IsZero()) {
			return true
		}
	}
	return false
}

// validate refuses a value that is not a map, a field the action does not take, a missing required field and, for
// waitFor, an unknown state. A mistyped syncTimeout is left to the decode.
func (s newActionSpec) validate(raw interface{}) error {
	allowed := strings.Join(s.fields, ", ")
	fields, ok := asMap(raw)
	if !ok {
		return fmt.Errorf("%s: must be a map with the fields %s, not %s", s.name, allowed, s.describe(raw))
	}
	for _, k := range slices.Sorted(maps.Keys(fields)) {
		if !slices.ContainsFunc(s.fields, func(f string) bool { return strings.EqualFold(f, k) }) {
			return fmt.Errorf("%s: unknown field '%s' (allowed: %s)", s.name, k, allowed)
		}
	}
	for _, name := range s.required {
		if v, _ := lookupField(fields, name); v == nil {
			return s.missing(name, allowed)
		}
	}
	switch s.name {
	case "waitFor":
		return checkWaitState(fields)
	case actionPress:
		return checkPressKey(fields)
	}
	return nil
}

// checkPressKey refuses a key that is not a text before the decode, which would echo it.
func checkPressKey(fields map[string]interface{}) error {
	v, _ := lookupField(fields, "key")
	if _, ok := v.(string); ok {
		return nil
	}
	return fmt.Errorf("press: invalid 'key': a value of type %T, not a text; press takes one key or one chord (write a digit in quotes: key: \"5\")", v)
}

// describe is how a refusal names a value that is not a map: a text by what describeValue allows, a press value of any
// other type by its type, since either may hold a secret.
func (s newActionSpec) describe(raw interface{}) string {
	if text, ok := raw.(string); ok {
		return describeValue(text)
	}
	if s.name == actionPress {
		return fmt.Sprintf("a value of type %T", raw)
	}
	return fmt.Sprint(raw)
}

func (s newActionSpec) missing(field, allowed string) error {
	msg := fmt.Sprintf("%s: the field '%s' is required (allowed: %s)", s.name, field, allowed)
	if s.hint != "" {
		msg += ": " + s.hint
	}
	return errors.New(msg)
}

func checkWaitState(fields map[string]interface{}) error {
	v, _ := lookupField(fields, "state")
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok && (s == "" || slices.Contains(waitStates, s)) {
		return nil
	}
	return fmt.Errorf("waitFor: invalid 'state' %v (allowed: %s)", v, strings.Join(waitStates, ", "))
}

// reachesFrame reports whether one of the new actions is set: each starts with a call to the current frame.
func (a Action) reachesFrame() bool {
	return a.WaitFor != nil || a.Hover != nil || a.DragAndDrop != nil || a.Press != nil
}

// waitStateOf is Playwright's constant for a state name; no name means visible.
func waitStateOf(state string) *playwright.WaitForSelectorState {
	switch state {
	case "hidden":
		return playwright.WaitForSelectorStateHidden
	case "attached":
		return playwright.WaitForSelectorStateAttached
	case "detached":
		return playwright.WaitForSelectorStateDetached
	}
	return playwright.WaitForSelectorStateVisible
}

// waitFor waits for the first matching element to reach the state, within syncTimeout.
func waitFor(ctx context.Context, webCtx *WebContext, w *WaitFor) error {
	elt, err := findOne(ctx, webCtx, w.Find)
	if err != nil {
		return err
	}
	state := w.State
	if state == "" {
		state = waitStates[0]
	}
	seconds := w.SyncTimeout
	if seconds <= 0 {
		seconds = defaultWaitSeconds
	}
	if err := webCtx.do(ownTimeout, func() error {
		return elt.WaitFor(playwright.LocatorWaitForOptions{State: waitStateOf(state), Timeout: stepTimeout(w.SyncTimeout)})
	}); err != nil {
		return fmt.Errorf("waitFor: the element %s did not become %s within %ds: %w", describeFind(w.Find), state, seconds, err)
	}
	return webCtx.settle()
}

// hover moves the pointer onto the first matching element.
func hover(ctx context.Context, webCtx *WebContext, h *Hover) error {
	elt, err := findOne(ctx, webCtx, h.Find)
	if err != nil {
		return err
	}
	if err := webCtx.do(ownTimeout, func() error {
		return elt.Hover(playwright.LocatorHoverOptions{Timeout: stepTimeout(h.SyncTimeout)})
	}); err != nil {
		return fmt.Errorf("hover: %s: %w", describeFind(h.Find), err)
	}
	return webCtx.settle()
}

// dragAndDrop drops the first element matching from on the first matching to.
func dragAndDrop(ctx context.Context, webCtx *WebContext, d *DragAndDrop) error {
	from, err := findOne(ctx, webCtx, d.From)
	if err != nil {
		return err
	}
	to, err := findOne(ctx, webCtx, d.To)
	if err != nil {
		return err
	}
	if err := webCtx.do(ownTimeout, func() error {
		return from.dragTo(to, playwright.LocatorDragToOptions{Timeout: stepTimeout(d.SyncTimeout)})
	}); err != nil {
		return fmt.Errorf("dragAndDrop: %s to %s: %w", describeFind(d.From), describeFind(d.To), err)
	}
	return webCtx.settle()
}

// press sends one key or one chord to the first matching element, or to the focused element of the page without find.
func press(ctx context.Context, webCtx *WebContext, p *Press) error {
	key, err := parsePress(ctx, p.Key)
	if err != nil {
		return err
	}
	if p.Find == nil {
		// Keyboard.Press takes no Playwright timeout: the Go-side limit keeps a stuck page from holding the step (R17).
		limit := elementRead("the key press", p.SyncTimeout)
		if err := webCtx.do(limit, func() error { return webCtx.page.pressKey(key) }); err != nil {
			return fmt.Errorf("press: %s: %w", key, err)
		}
		return webCtx.settle()
	}
	elt, err := findOne(ctx, webCtx, p.Find)
	if err != nil {
		return err
	}
	if err := webCtx.do(ownTimeout, func() error {
		return elt.Press(key, playwright.LocatorPressOptions{Timeout: stepTimeout(p.SyncTimeout)})
	}); err != nil {
		return fmt.Errorf("press: %s on %s: %w", key, describeFind(p.Find), err)
	}
	return webCtx.settle()
}
