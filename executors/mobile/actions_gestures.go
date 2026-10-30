package mobile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// maxScrollAttempts bounds scroll with until (FR-09). A package variable so that tests can shorten it.
var maxScrollAttempts = 10

// defaultPercent is the part of the area a swipe or scroll covers when the step gives no percent.
const defaultPercent = 0.75

var directions = map[string]bool{"up": true, "down": true, "left": true, "right": true}

// gestureArgs checks direction and percent of a gesture step and returns the arguments they make.
func gestureArgs(step Executor) (map[string]interface{}, error) {
	direction := strings.ToLower(step.Direction)
	if !directions[direction] {
		return nil, usagef("direction must be up, down, left or right, got %q", step.Direction)
	}
	percent := defaultPercent
	if step.Percent != nil {
		percent = *step.Percent
	}
	if !(percent > 0 && percent <= 1) {
		return nil, usagef("percent must be above 0 and at most 1, got %v", percent)
	}
	return map[string]interface{}{"direction": direction, "percent": percent}, nil
}

// windowRect is the visible area of the device without its edges, used for a gesture that names no element.
func (s *session) windowRect(ctx context.Context) (map[string]interface{}, error) {
	rect, err := s.windowBounds(ctx)
	if err != nil {
		return nil, err
	}
	return insetArea(int(rect.X), int(rect.Y), int(rect.Width), int(rect.Height)), nil
}

// edgeInsetPercent is the part of each side of the window that a gesture without an element keeps away from: a
// gesture that touches the screen edge is taken by the system (back, home, recents) instead of the app.
const edgeInsetPercent = 10

// insetArea is the window shrunk by edgeInsetPercent of the respective side on all four edges.
func insetArea(x, y, width, height int) map[string]interface{} {
	dx, dy := width*edgeInsetPercent/100, height*edgeInsetPercent/100
	return map[string]interface{}{"left": x + dx, "top": y + dy, "width": width - 2*dx, "height": height - 2*dy}
}

// gesture sends a "mobile: swipeGesture" or "mobile: scrollGesture". It acts inside the element ref when there is one,
// else inside the visible area of the device. It returns the server's value.
func (s *session) gesture(ctx context.Context, script string, step Executor, ref elementRef) (json.RawMessage, error) {
	args, err := gestureArgs(step)
	if err != nil {
		return nil, err
	}
	if ref != "" {
		args["elementId"] = string(ref)
	} else {
		area, err := s.windowRect(ctx)
		if err != nil {
			return nil, err
		}
		for k, v := range area {
			args[k] = v
		}
	}
	return s.call(ctx, http.MethodPost, "/execute/sync", executeBody(script, args))
}

// gestureTarget runs a gesture inside the element the step names, if it names one.
func gestureTarget(ctx context.Context, s *session, step Executor, result *Result, script string) (json.RawMessage, error) {
	if _, err := gestureArgs(step); err != nil {
		return nil, err
	}
	var ref elementRef
	if step.hasLocator() {
		var err error
		if ref, err = targetElement(ctx, s, step, result); err != nil {
			return nil, err
		}
	}
	return s.gesture(ctx, script, step, ref)
}

// actSwipe swipes in step.Direction, inside the named element or the visible area.
func actSwipe(ctx context.Context, s *session, step Executor, result *Result) error {
	if s.ios() {
		return iosSwipe(ctx, s, step, result)
	}
	if step.hasPoint() {
		return swipePoint(ctx, s, step)
	}
	_, err := gestureTarget(ctx, s, step, result, "mobile: swipeGesture")
	return err
}

// actScroll scrolls in step.Direction, inside the named element or the visible area; with until it scrolls the visible
// area until the named element is on screen.
func actScroll(ctx context.Context, s *session, step Executor, result *Result) error {
	if s.ios() {
		return iosScroll(ctx, s, step, result)
	}
	if step.Until {
		return scrollUntil(ctx, s, step, result)
	}
	_, err := gestureTarget(ctx, s, step, result, "mobile: scrollGesture")
	return err
}

// scrollOnce scrolls the visible area once and says whether the server thinks there is more to scroll. An answer it
// cannot read counts as "more".
func scrollOnce(ctx context.Context, s *session, step Executor) (bool, error) {
	if s.ios() {
		return iosScrollOnce(ctx, s, step)
	}
	raw, err := s.gesture(ctx, "mobile: scrollGesture", step, "")
	if err != nil {
		return false, err
	}
	var more bool
	if json.Unmarshal(raw, &more) != nil {
		return true, nil
	}
	return more, nil
}

// isVisible tells whether the locator matches an element that is displayed, without waiting.
func (s *session) isVisible(ctx context.Context, loc locator) (bool, error) {
	refs, err := s.queryElements(ctx, loc)
	if err != nil || len(refs) == 0 {
		return false, err
	}
	raw, err := s.call(ctx, http.MethodGet, elementPath(refs[0], "displayed"), nil)
	if err != nil {
		return false, err
	}
	var shown bool
	return json.Unmarshal(raw, &shown) == nil && shown, nil
}

// roundOutcome is what one round of scrolling until an element is visible found out.
type roundOutcome int

const (
	roundContinue roundOutcome = iota // not visible yet, scroll again
	roundVisible                      // the element is on screen
	roundEnd                          // not visible and nothing left to scroll
)

func scrollUntil(ctx context.Context, s *session, step Executor, result *Result) error {
	loc, err := step.locatorOn(s.ios())
	if err != nil {
		return err
	}
	if _, err := gestureArgs(step); err != nil {
		return err
	}
	for attempt := 0; attempt <= maxScrollAttempts; attempt++ {
		outcome, err := scrollRound(ctx, s, step, loc, attempt == maxScrollAttempts)
		if err != nil {
			return err
		}
		if outcome == roundVisible {
			result.Found = true
			return nil
		}
		if outcome == roundEnd {
			break
		}
	}
	return fmt.Errorf("%s is not visible after scrolling %s (at most %d scrolls)", loc, strings.ToLower(step.Direction), maxScrollAttempts)
}

// scrollRound looks for the element, then scrolls once unless it is the last round. At the end of the list it looks
// one more time.
func scrollRound(ctx context.Context, s *session, step Executor, loc locator, last bool) (roundOutcome, error) {
	shown, err := s.isVisible(ctx, loc)
	if err != nil {
		return roundEnd, err
	}
	if shown {
		return roundVisible, nil
	}
	if last {
		return roundEnd, nil
	}
	more, err := scrollOnce(ctx, s, step)
	if err != nil {
		return roundEnd, err
	}
	if more {
		return roundContinue, nil
	}
	if shown, err = s.isVisible(ctx, loc); err != nil {
		return roundEnd, err
	}
	if shown {
		return roundVisible, nil
	}
	return roundEnd, nil
}
