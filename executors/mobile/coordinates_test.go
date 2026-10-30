package mobile

import (
	"math"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

const rect1080 = `{"x":0,"y":0,"width":1080,"height":2400}`

func TestFR11_ATapInPixelsSendsTheExactPointAndAsksNoRect(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(execRoute, reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "tap", "x": 120, "y": 340}).Err)
	body := f.first(t, http.MethodPost, "/session/{id}/execute/sync")
	assert.Equal(t, "mobile: clickGesture", body["script"])
	args := gestureArgsSent(t, f)
	assert.InDelta(t, 120, number(t, args, "x"), 1e-9)
	assert.InDelta(t, 340, number(t, args, "y"), 1e-9)
	assert.NotContains(t, args, "elementId")
	assert.Zero(t, f.count(http.MethodGet, "/session/{id}/window/rect"))
}

func TestFR11_ATapAtZeroIsAValidPoint(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(execRoute, reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "tap", "x": 0, "y": 0, "unit": "px"}).Err)
	assert.InDelta(t, 0, number(t, gestureArgsSent(t, f), "x"), 1e-9)
}

func TestFR11_ATapAtFiftyPercentOfA1080x2400WindowIs540x1200(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(rectRoute, reply(http.StatusOK, rect1080))
	f.on(execRoute, reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "tap", "x": 50, "y": 50, "unit": "percent"}).Err)
	args := gestureArgsSent(t, f)
	assert.InDelta(t, 540, number(t, args, "x"), 1e-9)
	assert.InDelta(t, 1200, number(t, args, "y"), 1e-9)
	assert.Equal(t, 1, f.count(http.MethodGet, "/session/{id}/window/rect"))
}

func TestFR11_APercentageIsOfTheRawRectOffsetByItsOrigin(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(rectRoute, reply(http.StatusOK, `{"x":10,"y":20,"width":1000,"height":2000}`))
	f.on(execRoute, reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "tap", "x": 50, "y": 25.04, "unit": "percent"}).Err)
	args := gestureArgsSent(t, f)
	assert.InDelta(t, 510, number(t, args, "x"), 1e-9)
	assert.InDelta(t, 521, number(t, args, "y"), 1e-9) // 20 + 500.8 rounds to 521
}

func TestFR11_ASwipeInPixelsDragsFromOnePointToTheOther(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(execRoute, reply(http.StatusOK, `null`))

	step := venom.TestStep{"action": "swipe", "x": 100, "y": 200, "toX": 300, "toY": 400}
	assert.Empty(t, runWith(ctx, t, step).Err)
	assert.Equal(t, "mobile: dragGesture", f.first(t, http.MethodPost, "/session/{id}/execute/sync")["script"])
	args := gestureArgsSent(t, f)
	assert.InDelta(t, 100, number(t, args, "startX"), 1e-9)
	assert.InDelta(t, 200, number(t, args, "startY"), 1e-9)
	assert.InDelta(t, 300, number(t, args, "endX"), 1e-9)
	assert.InDelta(t, 400, number(t, args, "endY"), 1e-9)
	assert.Zero(t, f.count(http.MethodGet, "/session/{id}/window/rect"))
}

func TestFR11_ASwipeInPercentConvertsBothPoints(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(rectRoute, reply(http.StatusOK, rect1080))
	f.on(execRoute, reply(http.StatusOK, `null`))

	step := venom.TestStep{"action": "swipe", "x": 50, "y": 80, "toX": 50, "toY": 20, "unit": "percent"}
	assert.Empty(t, runWith(ctx, t, step).Err)
	args := gestureArgsSent(t, f)
	assert.InDelta(t, 540, number(t, args, "startX"), 1e-9)
	assert.InDelta(t, 1920, number(t, args, "startY"), 1e-9)
	assert.InDelta(t, 540, number(t, args, "endX"), 1e-9)
	assert.InDelta(t, 480, number(t, args, "endY"), 1e-9)
	assert.Equal(t, 1, f.count(http.MethodGet, "/session/{id}/window/rect"))
}

func TestFR11_AServerFailureOfThePointTapLandsInResultErr(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(execRoute, fail(http.StatusBadRequest, "invalid argument", "the point is outside the screen"))

	result := runWith(ctx, t, venom.TestStep{"action": "tap", "x": 99999, "y": 1})
	assert.Contains(t, result.Err, "outside the screen")
}

func TestFR11_MisuseIsAUsageErrorBeforeAnyRequest(t *testing.T) {
	cases := map[string]venom.TestStep{
		"x without y":                   {"action": "tap", "x": 1},
		"y without x":                   {"action": "tap", "y": 1},
		"toX without toY":               {"action": "swipe", "x": 1, "y": 1, "toX": 2},
		"toY without toX":               {"action": "swipe", "x": 1, "y": 1, "toY": 2},
		"coordinates with a locator":    {"action": "tap", "x": 1, "y": 1, "id": "ok"},
		"swipe coordinates and locator": {"action": "swipe", "x": 1, "y": 1, "toX": 2, "toY": 2, "text": "ok"},
		"coordinates on read":           {"action": "read", "x": 1, "y": 1, "id": "ok"},
		"coordinates on back":           {"action": "back", "x": 1, "y": 1},
		"negative x":                    {"action": "tap", "x": -1, "y": 1},
		"negative toY":                  {"action": "swipe", "x": 1, "y": 1, "toX": 2, "toY": -2},
		"percent above 100":             {"action": "tap", "x": 101, "y": 1, "unit": "percent"},
		"x is NaN":                      {"action": "tap", "x": math.NaN(), "y": 1},
		"y is infinite":                 {"action": "tap", "x": 1, "y": math.Inf(1)},
		"toX is minus infinity":         {"action": "swipe", "x": 1, "y": 1, "toX": math.Inf(-1), "toY": 2},
		"pixels too large":              {"action": "tap", "x": 1e12, "y": 1},
		"percent NaN":                   {"action": "tap", "x": math.NaN(), "y": 1, "unit": "percent"},
		"unknown unit":                  {"action": "tap", "x": 1, "y": 1, "unit": "cm"},
		"swipe from a point only":       {"action": "swipe", "x": 1, "y": 1},
		"swipe to a point only":         {"action": "swipe", "toX": 1, "toY": 1},
		"swipe with a direction":        {"action": "swipe", "x": 1, "y": 1, "toX": 2, "toY": 2, "direction": "up"},
		"swipe with an extent":          {"action": "swipe", "x": 1, "y": 1, "toX": 2, "toY": 2, "percent": 0.5},
		"tap with an end point":         {"action": "tap", "x": 1, "y": 1, "toX": 2, "toY": 2},
	}
	for name, step := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, f := caseOn(t, nil)
			before := f.total()
			assert.Error(t, usageOf(ctx, t, step))
			assert.Equal(t, before, f.total())
		})
	}
}

func TestFR11_ALocatorLessTapWithoutCoordinatesKeepsItsError(t *testing.T) {
	ctx, f := caseOn(t, nil)
	before := f.total()
	err := usageOf(ctx, t, venom.TestStep{"action": "tap"})
	require.Contains(t, err.Error(), "no locator")
	assert.Equal(t, before, f.total())
}
