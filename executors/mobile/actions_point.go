package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"
)

// Values of the step field unit, for a point given by x and y (FR-11).
const (
	unitPixels  = "pixels"
	unitPixelsX = "px"
	unitPercent = "percent"
)

// point is a position on the screen, in the unit of the step.
type point struct{ x, y float64 }

// hasPoint tells whether the step names a point by coordinates.
func (e Executor) hasPoint() bool {
	return e.X != nil || e.Y != nil || e.ToX != nil || e.ToY != nil
}

// refuseStrayPoint refuses coordinates on an action that does not take them.
func refuseStrayPoint(step Executor) error {
	if step.hasPoint() && step.Action != actionTap && step.Action != actionSwipe {
		return usagef("x, y, toX and toY are for tap and swipe, not for %s", step.Action)
	}
	return nil
}

// isPercent reads the unit of the step: pixels (also px, the default) or percent.
func (e Executor) isPercent() (bool, error) {
	switch strings.ToLower(e.Unit) {
	case "", unitPixels, unitPixelsX:
		return false, nil
	case unitPercent:
		return true, nil
	default:
		return false, usagef("unit must be %s, %s or %s, got %q", unitPixels, unitPixelsX, unitPercent, e.Unit)
	}
}

// pairOf makes a point of two fields that come together; none is no point.
func pairOf(nameX string, x *float64, nameY string, y *float64) (*point, error) {
	switch {
	case x == nil && y == nil:
		return nil, nil
	case x == nil:
		return nil, usagef("%s needs %s", nameY, nameX)
	case y == nil:
		return nil, usagef("%s needs %s", nameX, nameY)
	}
	return &point{x: *x, y: *y}, nil
}

// maxPixels bounds a pixel coordinate, far beyond any screen and safe to convert to an int.
const maxPixels = 1e7

// checkRange refuses NaN and infinities, a negative coordinate, a percentage above 100 and a pixel value above maxPixels.
func checkRange(name string, v *float64, percent bool) error {
	if v == nil {
		return nil
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) {
		return usagef("%s must be a finite number", name)
	}
	if !percent && *v > maxPixels {
		return usagef("%s is too large for a pixel coordinate", name)
	}
	if *v < 0 || (percent && *v > 100) {
		if percent {
			return usagef("%s must be between 0 and 100 percent, got %v", name, *v)
		}
		return usagef("%s must not be negative, got %v", name, *v)
	}
	return nil
}

// checkRanges checks the four coordinates of the step.
func checkRanges(step Executor, percent bool) error {
	for _, c := range []struct {
		name string
		v    *float64
	}{{"x", step.X}, {"y", step.Y}, {"toX", step.ToX}, {"toY", step.ToY}} {
		if err := checkRange(c.name, c.v, percent); err != nil {
			return err
		}
	}
	return nil
}

// pointsOf checks the coordinates of a tap or a swipe, before any request, and returns the one or two points.
func pointsOf(step Executor, action string) ([]point, error) {
	start, err := pairOf("x", step.X, "y", step.Y)
	if err != nil {
		return nil, err
	}
	end, err := pairOf("toX", step.ToX, "toY", step.ToY)
	if err != nil {
		return nil, err
	}
	if step.hasLocator() {
		return nil, usagef("give coordinates or a locator, not both")
	}
	percent, err := step.isPercent()
	if err != nil {
		return nil, err
	}
	if err := checkRanges(step, percent); err != nil {
		return nil, err
	}
	if start == nil {
		return nil, usagef("%s from a point needs x and y", action)
	}
	if action == actionTap {
		return tapPoints(start, end)
	}
	return swipePoints(step, start, end)
}

func tapPoints(start, end *point) ([]point, error) {
	if end != nil {
		return nil, usagef("toX and toY are for swipe, not for tap")
	}
	return []point{*start}, nil
}

func swipePoints(step Executor, start, end *point) ([]point, error) {
	if end == nil {
		return nil, usagef("swipe from a point needs toX and toY")
	}
	if step.Direction != "" || step.Percent != nil {
		return nil, usagef("swipe from a point to a point takes no direction or percent")
	}
	return []point{*start, *end}, nil
}

// rawRect is the whole window of the device, as the server reports it.
type rawRect struct{ X, Y, Width, Height float64 }

// windowBounds asks the server for the whole window, edges included.
func (s *session) windowBounds(ctx context.Context) (rawRect, error) {
	raw, err := s.call(ctx, http.MethodGet, "/window/rect", nil)
	if err != nil {
		return rawRect{}, err
	}
	var rect struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	}
	if err := json.Unmarshal(raw, &rect); err != nil || rect.Width <= 0 || rect.Height <= 0 {
		return rawRect{}, errors.New("the automation server answered /window/rect without a usable area")
	}
	return rawRect{rect.X, rect.Y, rect.Width, rect.Height}, nil
}

// toPixels converts the points of a step to pixels. Pixels are kept as given and ask the server nothing; a percentage
// is of the whole window rect, offset by its origin.
func (s *session) toPixels(ctx context.Context, step Executor, pts []point) ([]point, error) {
	percent, err := step.isPercent()
	if err != nil || !percent {
		return pts, err
	}
	r, err := s.windowBounds(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]point, len(pts))
	for i, p := range pts {
		out[i] = point{x: r.X + p.x/100*r.Width, y: r.Y + p.y/100*r.Height}
	}
	return out, nil
}

// pixelPoints checks and converts the points of the step, and returns them in pixels.
func (s *session) pixelPoints(ctx context.Context, step Executor, action string) ([]point, error) {
	pts, err := pointsOf(step, action)
	if err != nil {
		return nil, err
	}
	return s.toPixels(ctx, step, pts)
}

func pixel(v float64) int { return int(math.Round(v)) }

// tapPoint taps at the coordinates of the step.
func tapPoint(ctx context.Context, s *session, step Executor) error {
	if s.ios() {
		return iosTapPoint(ctx, s, step)
	}
	pts, err := s.pixelPoints(ctx, step, actionTap)
	if err != nil {
		return err
	}
	args := map[string]interface{}{"x": pixel(pts[0].x), "y": pixel(pts[0].y)}
	_, err = s.call(ctx, http.MethodPost, "/execute/sync", executeBody("mobile: clickGesture", args))
	return err
}

// swipePoint drags from one point of the step to the other.
func swipePoint(ctx context.Context, s *session, step Executor) error {
	pts, err := s.pixelPoints(ctx, step, actionSwipe)
	if err != nil {
		return err
	}
	args := map[string]interface{}{
		"startX": pixel(pts[0].x), "startY": pixel(pts[0].y),
		"endX": pixel(pts[1].x), "endY": pixel(pts[1].y),
	}
	_, err = s.call(ctx, http.MethodPost, "/execute/sync", executeBody("mobile: dragGesture", args))
	return err
}

// The two actions that take coordinates.
const (
	actionTap   = "tap"
	actionSwipe = "swipe"
)
