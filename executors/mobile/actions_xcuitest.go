package mobile

import (
	"context"
	"net/http"
)

// The iOS side of the actions (FR-09, #2398). Each action in actions*.go starts with `if s.ios()` and hands over here;
// the Android bodies are untouched. The commands are the XCUITest driver's. Without a device to learn from they are
// the best-known calls: XCUITest, to be confirmed on a simulator in V8.
//
// Unchanged on iOS, because they are plain W3C commands: tap on an element, fill, clear, read, count, waitFor,
// screenshot, source, context, and hideKeyboard (mobile: hideKeyboard with no argument; the answer "keyboard not
// present" is already read as "hidden" by isKeyboardAlreadyHidden).

// iosExecute runs an XCUITest extension command.
func iosExecute(ctx context.Context, s *session, script string, args map[string]interface{}) error {
	_, err := s.call(ctx, http.MethodPost, "/execute/sync", executeBody(script, args))
	return err
}

// iosBack refuses back: iOS has no back key.
func iosBack() error {
	return usagef("back is Android only: iOS has no back key; use a tap on the app's back control")
}

// iosAppCommand runs mobile: activateApp or mobile: terminateApp, which name the app by bundleId.
func iosAppCommand(ctx context.Context, s *session, step Executor, script string) error {
	bundleID, err := appOf(s, step)
	if err != nil {
		return err
	}
	return iosExecute(ctx, s, script, map[string]interface{}{profileOf(platformIOS).appIDKey: bundleID})
}

// iosGestureArgs checks the direction of a swipe or scroll and refuses percent, which XCUITest has no argument for.
func iosGestureArgs(step Executor) (map[string]interface{}, error) {
	if step.Percent != nil {
		return nil, usagef("percent is Android only: mobile: swipe and mobile: scroll on iOS take a direction only")
	}
	args, err := gestureArgs(step)
	if err != nil {
		return nil, err
	}
	delete(args, "percent")
	return args, nil
}

// iosGesture sends mobile: swipe or mobile: scroll with a direction, on the named element or the whole app.
func iosGesture(ctx context.Context, s *session, step Executor, result *Result, script string) error {
	args, err := iosGestureArgs(step)
	if err != nil {
		return err
	}
	if step.hasLocator() {
		ref, err := targetElement(ctx, s, step, result)
		if err != nil {
			return err
		}
		args["elementId"] = string(ref)
	}
	return iosExecute(ctx, s, script, args)
}

// iosSwipe swipes by coordinates or by direction.
func iosSwipe(ctx context.Context, s *session, step Executor, result *Result) error {
	if step.hasPoint() {
		return iosSwipePoint(ctx, s, step)
	}
	return iosGesture(ctx, s, step, result, "mobile: swipe")
}

// iosScroll scrolls by direction; with until it repeats the shared visibility loop, whose single scroll is iosScrollOnce.
func iosScroll(ctx context.Context, s *session, step Executor, result *Result) error {
	if step.Until {
		if _, err := iosGestureArgs(step); err != nil {
			return err
		}
		return scrollUntil(ctx, s, step, result)
	}
	return iosGesture(ctx, s, step, result, "mobile: scroll")
}

// iosScrollOnce scrolls the app once. mobile: scroll answers nothing about the end of the list, so it always says
// "more": the bound maxScrollAttempts ends the loop.
func iosScrollOnce(ctx context.Context, s *session, step Executor) (bool, error) {
	args, err := iosGestureArgs(step)
	if err != nil {
		return false, err
	}
	return true, iosExecute(ctx, s, "mobile: scroll", args)
}

// iosTapPoint taps at a point in points (not pixels) with mobile: tap.
func iosTapPoint(ctx context.Context, s *session, step Executor) error {
	pts, err := s.pixelPoints(ctx, step, actionTap)
	if err != nil {
		return err
	}
	return iosExecute(ctx, s, "mobile: tap", map[string]interface{}{"x": pixel(pts[0].x), "y": pixel(pts[0].y)})
}

// iosDragSeconds is the time a swipe by coordinates takes on iOS.
const iosDragSeconds = 0.5

// iosSwipePoint drags from one point to the other with mobile: dragFromToForDuration. A percentage is of the window
// rect, which iOS reports in points.
func iosSwipePoint(ctx context.Context, s *session, step Executor) error {
	pts, err := s.pixelPoints(ctx, step, actionSwipe)
	if err != nil {
		return err
	}
	return iosExecute(ctx, s, "mobile: dragFromToForDuration", map[string]interface{}{
		"duration": iosDragSeconds,
		"fromX":    pixel(pts[0].x), "fromY": pixel(pts[0].y),
		"toX": pixel(pts[1].x), "toY": pixel(pts[1].y),
	})
}

// iosOpenURL opens a deep link with mobile: deepLink, restricted to the bundle id when it is known.
func iosOpenURL(ctx context.Context, s *session, step Executor) error {
	if err := checkLink(step.URL); err != nil {
		return err
	}
	args := map[string]interface{}{"url": step.URL}
	if bundleID, err := appOf(s, step); err == nil {
		args[profileOf(platformIOS).appIDKey] = bundleID
	}
	return iosExecute(ctx, s, "mobile: deepLink", args)
}
