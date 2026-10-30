package web

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Unit tests of FR-07, the value an execute script returns (docs/feat/2048/research.md R6, R7; validation.md V5, V6),
// on the values playwright-go v0.6201.1 decodes a page's value into, and through Run on the fake seam. Their
// real-browser counterparts are in web_playwright_test.go.

// selfReferences are values that contain themselves, as playwright-go decodes a JavaScript object that refers to
// itself: the same Go map again inside it.
func selfReferences() map[string]any {
	object := map[string]any{"name": "Ada"}
	object["self"] = object
	throughList := map[string]any{}
	throughList["list"] = []any{"a", throughList}
	list := []any{nil}
	list[0] = list
	typed := [][]any{nil}
	typed[0] = []any{typed}
	return map[string]any{
		"an object that refers to itself":           object,
		"an object inside a list inside the object": throughList,
		"a list that holds itself":                  list,
		"a typed slice reached again":               typed,
		"a cycle inside an array":                   [1]any{object},
		"a cycle below a sound element":             []any{1, map[string]any{"deep": object}},
	}
}

func TestJSONValue(t *testing.T) {
	// R7: the types encoding/json decodes a document into, whatever playwright-go handed back.
	date := time.Date(2026, time.September, 26, 12, 30, 0, 500_000_000, time.FixedZone("CEST", 2*60*60))
	bigInt, ok := new(big.Int).SetString("12345678901234567890", 10)
	require.True(t, ok)
	link, err := url.Parse("https://example.test/users?name=Ada#top")
	require.NoError(t, err)
	shared := map[string]any{"name": "Ada"}
	sharedList := []any{"a"}
	tests := []struct {
		name string
		in   any
		want any
	}{
		{"undefined and null", nil, nil},
		{"an integral number", 42, 42.0},
		{"a number", 4.5, 4.5},
		{"NaN", math.NaN(), "NaN"},
		{"Infinity", math.Inf(1), "Infinity"},
		{"-Infinity", math.Inf(-1), "-Infinity"},
		{"a string", "Ada", "Ada"},
		{"an empty string", "", ""},
		{"a boolean", true, true},
		{"a Date, in UTC", date, "2026-09-26T10:30:00.5Z"},
		{"a BigInt, every digit", bigInt, "12345678901234567890"},
		{"a URL, not a Go struct", link, "https://example.test/users?name=Ada#top"},
		{"a RegExp", regexp.MustCompile("(?i)^ada$"), "(?i)^ada$"},
		{"an Error", fmt.Errorf("synthetic: %w", errors.New("failure")), "synthetic: failure"},
		{"a Float64Array", []float64{1, 2.5}, []any{1.0, 2.5}},
		{"a Float32Array", []float32{0.5}, []any{0.5}},
		{"a Uint8Array, not base64", []uint8{1, 2}, []any{1.0, 2.0}},
		{"an Int8Array", []int8{-1, 1}, []any{-1.0, 1.0}},
		{"a BigInt64Array and a BigUint64Array", []any{[]int64{-7}, []uint64{7}}, []any{[]any{-7.0}, []any{7.0}}},
		{"a typed array with no element", []uint16{}, []any{}},
		{"a Go array", [2]int{1, 2}, []any{1.0, 2.0}},
		{"a list", []any{1, "a", nil, false}, []any{1.0, "a", nil, false}},
		{"an empty list", []any{}, []any{}},
		{
			"an object, element by element",
			map[string]any{"user": map[string]any{"name": "Ada", "age": 36, "tags": []any{"x"}}},
			map[string]any{"user": map[string]any{"name": "Ada", "age": 36.0, "tags": []any{"x"}}},
		},
		{"an empty object: a Map or a Set", map[string]any{}, map[string]any{}},
		{
			"an object referenced twice is no cycle",
			[]any{shared, map[string]any{"again": shared}},
			[]any{map[string]any{"name": "Ada"}, map[string]any{"again": map[string]any{"name": "Ada"}}},
		},
		{"a list referenced twice is no cycle", []any{sharedList, sharedList}, []any{[]any{"a"}, []any{"a"}}},
		{"a DOM node", "ref: <Node>", "ref: <Node>"},
		{"any other value, as its text", struct{ ID int }{7}, "{7}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonValue(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("-0 is a number, its sign kept", func(t *testing.T) {
		// assert.Equal cannot tell -0 from 0.
		got, err := jsonValue(math.Copysign(0, -1))
		require.NoError(t, err)
		f, isFloat := got.(float64)
		require.True(t, isFloat)
		assert.True(t, math.Signbit(f))
	})

	for name, v := range selfReferences() {
		t.Run(name+" is refused", func(t *testing.T) {
			got, err := jsonValue(v)
			require.ErrorIs(t, err, errSelfReference)
			assert.Nil(t, got)
		})
	}

	// playwright-go drops url.Parse's error: a URL the page accepts and Go refuses, as https://a.test/x#50%off, is a nil
	// *url.URL, whose String method dereferences it.
	for name, v := range map[string]any{
		"alone":            (*url.URL)(nil),
		"inside an object": map[string]any{"link": (*url.URL)(nil)},
		"inside a list":    []any{"a", (*url.URL)(nil)},
	} {
		t.Run("an undecoded URL "+name+" is refused", func(t *testing.T) {
			got, err := jsonValue(v)
			require.ErrorIs(t, err, errUndecoded)
			assert.EqualError(t, err, "the script's return value holds a value playwright-go could not decode: a nil *url.URL")
			assert.Nil(t, got)
		})
	}
}

func TestScriptValue(t *testing.T) {
	// R6: a string as is; any other value as its JSON text (keys sorted, HTML as written) and typed; nothing for nil.
	tests := []struct {
		name      string
		in        any
		wantText  string
		wantTyped any
	}{
		{"an integral number", 42, "42", 42.0},
		{"a number", 4.5, "4.5", 4.5},
		{"zero", 0, "0", 0.0},
		{"true", true, "true", true},
		{"false", false, "false", false},
		{"a string, as is", "s", "s", "s"},
		{"a JSON string stays text", `{"a":1}`, `{"a":1}`, `{"a":1}`},
		{"an empty string", "", "", ""},
		{"a list", []any{1, "a"}, `[1,"a"]`, []any{1.0, "a"}},
		{
			"an object, its keys sorted",
			map[string]any{"user": map[string]any{"name": "Ada"}, "id": 7},
			`{"id":7,"user":{"name":"Ada"}}`,
			map[string]any{"user": map[string]any{"name": "Ada"}, "id": 7.0},
		},
		{"HTML is not escaped, as JSON.stringify", map[string]any{"html": "<b>&</b>"}, `{"html":"<b>&</b>"}`, map[string]any{"html": "<b>&</b>"}},
		{"a Uint8Array", []uint8{1, 2}, "[1,2]", []any{1.0, 2.0}},
		{"NaN", math.NaN(), "NaN", "NaN"},
		{"NaN in a list", []any{math.NaN(), math.Inf(-1)}, `["NaN","-Infinity"]`, []any{"NaN", "-Infinity"}},
		{"undefined and null", nil, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, typed, err := scriptValue(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.wantText, text)
			assert.Equal(t, tt.wantTyped, typed)
		})
	}

	t.Run("a value that contains itself is neither", func(t *testing.T) {
		for name, v := range selfReferences() {
			text, typed, err := scriptValue(v)
			require.ErrorIs(t, err, errSelfReference, name)
			assert.Empty(t, text, name)
			assert.Nil(t, typed, name)
		}
	})
}

func TestRun_executeValue(t *testing.T) {
	// FR-07: what the script returns is result.value, text, and result.valuejson, typed, as http's body and bodyjson.
	const command = "return window.answer;"
	call := executeCall("page1", command)
	withValue := func(text string, typed any) Result {
		r := pageResult()
		r.Value, r.ValueJSON = text, typed
		return r
	}
	tests := []struct {
		name     string
		returned any
		want     Result
	}{
		{
			"an object is its JSON text and a map",
			map[string]any{"user": map[string]any{"name": "Ada"}, "count": 2},
			withValue(`{"count":2,"user":{"name":"Ada"}}`, map[string]any{"user": map[string]any{"name": "Ada"}, "count": 2.0}),
		},
		{"a number", 42, withValue("42", 42.0)},
		{"a string is itself in both", "Ada", withValue("Ada", "Ada")},
		{"false is typed, not absent", false, withValue("false", false)},
		{"an empty string is only typed", "", withValue("", "")},
		{"a list", []any{1, "a"}, withValue(`[1,"a"]`, []any{1.0, "a"})},
		{"undefined or null adds nothing", nil, pageResult()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw := withFake(t)
			ctx := openFake(t, pw)
			pw.results[call] = tt.returned
			got, err := webRun(ctx, t, executeStep(command))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, settled(call), pw.seen())
		})
	}

	t.Run("venom flattens valuejson into the fields a suite asserts on", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.results[call] = map[string]any{"user": map[string]any{"name": "Ada"}, "count": 2}
		got, err := Executor{}.Run(ctx, executeStep(command))
		require.NoError(t, err)
		flat := venom.GetExecutorResult(got)
		assert.Equal(t, "Ada", flat["result.valuejson.user.name"])
		assert.Equal(t, 2.0, flat["result.valuejson.count"])
	})

	t.Run("a value that contains itself leaves both empty, warns, and the step passes", func(t *testing.T) {
		// An object that refers to itself, returned today, passes: go-dump would recurse without end on it.
		object := map[string]any{"name": "Ada"}
		object["self"] = object
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.results[call] = object
		var got Result
		var err error
		logged := stderrOf(t, func() { got, err = webRun(ctx, t, executeStep(command)) })
		require.NoError(t, err)
		assert.Equal(t, pageResult(), got)
		assert.Equal(t, settled(call), pw.seen())
		assert.Contains(t, logged,
			"web - execute: the script's return value contains itself: result.value and result.valuejson are left empty")
	})

	t.Run("an undecoded URL leaves both empty, warns, and the step passes", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.results[call] = map[string]any{"link": (*url.URL)(nil)}
		var got Result
		var err error
		logged := stderrOf(t, func() { got, err = webRun(ctx, t, executeStep(command)) })
		require.NoError(t, err)
		assert.Equal(t, pageResult(), got)
		assert.Contains(t, logged, "web - execute: the script's return value holds a value playwright-go could not decode: "+
			"a nil *url.URL: result.value and result.valuejson are left empty")
	})

	t.Run("a script that opens a dialog passes with no value; the dialog is answered later", func(t *testing.T) {
		// As any command that opens one: the script has done its part (R6). What it returns once answered is lost.
		pw := withFake(t)
		ctx := openFake(t, pw)
		finished := newFlag()
		pw.hooks[call] = pw.blockOnDialog("alert", 1, finished)
		pw.results[call] = 42
		got, err := webRun(ctx, t, executeStep(command))
		require.NoError(t, err)
		assert.Equal(t, Result{}, got, "no value; the title and the url are empty while the dialog is open")
		assert.Equal(t, []string{call}, pw.seen(), "no settle round, no page read")
		assert.False(t, finished.isSet(), "the script is still in flight")

		pw.reset()
		got, err = webRun(ctx, t, action(map[string]interface{}{"confirmPopup": true}))
		require.NoError(t, err)
		assert.True(t, finished.isSet())
		assert.Equal(t, pageResult(), got, "the answer's step has no value either")
		assert.Equal(t, settled("Accept alert1"), pw.seen())
	})

	t.Run("a failing script fails the step with no value", func(t *testing.T) {
		pw := withFake(t)
		ctx := openFake(t, pw)
		pw.results[call] = 42
		pw.errs[call] = errors.New("synthetic failure")
		_, err := webRun(ctx, t, executeStep(command))
		require.EqualError(t, err, "synthetic failure")
	})
}
