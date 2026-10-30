package mobile

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// gestureArgsSent returns the arguments of the first gesture the fake received.
func gestureArgsSent(t *testing.T, f *fakeServer) map[string]interface{} {
	t.Helper()
	list, ok := f.first(t, http.MethodPost, "/session/{id}/execute/sync")["args"].([]interface{})
	require.True(t, ok)
	args, ok := list[0].(map[string]interface{})
	require.True(t, ok)
	return args
}

// number reads a JSON number argument.
func number(t *testing.T, args map[string]interface{}, key string) float64 {
	t.Helper()
	n, ok := args[key].(float64)
	require.True(t, ok, "%s is not a number: %v", key, args[key])
	return n
}

func TestBug3240_GestureAreaStaysAwayFromTheScreenEdges(t *testing.T) {
	for _, size := range [][2]int{{1080, 2400}, {100, 200}} {
		for _, action := range []string{"swipe", "scroll"} {
			for _, direction := range []string{"up", "down", "left", "right"} {
				t.Run(fmt.Sprintf("%dx%d/%s/%s", size[0], size[1], action, direction), func(t *testing.T) {
					ctx, f := caseOn(t, nil)
					f.on(rectRoute, reply(http.StatusOK, fmt.Sprintf(`{"x":0,"y":0,"width":%d,"height":%d}`, size[0], size[1])))
					f.on(execRoute, reply(http.StatusOK, `true`))

					assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": action, "direction": direction}).Err)
					args := gestureArgsSent(t, f)
					left, top := number(t, args, "left"), number(t, args, "top")
					width, height := number(t, args, "width"), number(t, args, "height")
					assert.Greater(t, left, 0.0)
					assert.Greater(t, top, 0.0)
					assert.Greater(t, width, 0.0)
					assert.Greater(t, height, 0.0)
					assert.Less(t, left+width, float64(size[0]))
					assert.Less(t, top+height, float64(size[1]))
				})
			}
		}
	}
}

func TestBug3240_AnInsetAreaOfA1080x2400WindowIsTenPercentOnEachSide(t *testing.T) {
	ctx, f := caseOn(t, nil)
	f.on(rectRoute, reply(http.StatusOK, `{"x":0,"y":0,"width":1080,"height":2400}`))
	f.on(execRoute, reply(http.StatusOK, `null`))

	assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": "swipe", "direction": "up"}).Err)
	args := gestureArgsSent(t, f)
	assert.InDelta(t, 108, args["left"], 1e-9)
	assert.InDelta(t, 240, args["top"], 1e-9)
	assert.InDelta(t, 864, args["width"], 1e-9)
	assert.InDelta(t, 1920, args["height"], 1e-9)
	assert.InDelta(t, 0.75, args["percent"], 1e-9)
}

func TestBug3240_AGestureInsideAnElementSendsNoArea(t *testing.T) {
	for _, action := range []string{"swipe", "scroll"} {
		ctx, f := caseOn(t, nil)
		found(f)
		f.on(execRoute, reply(http.StatusOK, `true`))

		assert.Empty(t, runWith(ctx, t, venom.TestStep{"action": action, "id": "list", "direction": "down"}).Err)
		args := gestureArgsSent(t, f)
		assert.Equal(t, "el-0", args["elementId"])
		for _, key := range []string{"left", "top", "width", "height"} {
			assert.NotContains(t, args, key)
		}
	}
}
