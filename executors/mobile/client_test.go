package mobile

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		value   string
		code    string
		message string
		other   string // an error that is not a serverError
	}{
		{name: "null value", status: 200, body: `{"value":null}`, value: `null`},
		{name: "object value", status: 200, body: `{"value":{"sessionId":"a"}}`, value: `{"sessionId":"a"}`},
		{name: "string value", status: 200, body: `{"value":"text"}`, value: `"text"`},
		{name: "element reference", status: 200, body: `{"value":{"element-6066-11e4-a52e-4f735466cecf":"e1"}}`, value: `{"element-6066-11e4-a52e-4f735466cecf":"e1"}`},
		{name: "error envelope", status: 404, body: `{"value":{"error":"no such element","message":"gone","stacktrace":"at x"}}`, code: "no such element", message: "gone"},
		{name: "error without message", status: 500, body: `{"value":{"error":"unknown error"}}`, code: "unknown error"},
		{name: "error envelope on a 200", status: 200, body: `{"value":{"error":"odd","message":"m"}}`, code: "odd", message: "m"},
		{name: "html error page", status: 502, body: `<html>bad gateway</html>`, code: "http 502"},
		{name: "empty error body", status: 500, body: ``, code: "http 500"},
		{name: "redirect", status: 302, body: ``, code: "http 302"},
		{name: "non-2xx value", status: 400, body: `{"value":{"x":1}}`, code: "http 400"},
		{name: "ok without value", status: 200, body: `{"other":1}`, other: "without a W3C value"},
		{name: "ok not json", status: 200, body: `nope`, other: "without a W3C value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value, err := decode(tc.status, []byte(tc.body))
			if tc.other != "" {
				require.ErrorContains(t, err, tc.other)
				return
			}
			if tc.code == "" {
				require.NoError(t, err)
				assert.JSONEq(t, tc.value, string(value))
				return
			}
			var failure *serverError
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, tc.code, failure.Code)
			assert.Equal(t, tc.message, failure.Message)
			assert.NotContains(t, err.Error(), "at x")
		})
	}
}

func TestDecodeTruncatesALongMessage(t *testing.T) {
	long := strings.Repeat("é", maxMessageRunes+50)
	_, err := decode(500, []byte(`{"value":{"error":"unknown error","message":"`+long+`"}}`))

	var failure *serverError
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, maxMessageRunes+3, len([]rune(failure.Message)))
	assert.True(t, strings.HasSuffix(failure.Message, "..."))
}

func TestClientCapsTheAnswerItReads(t *testing.T) {
	shorten(t, &maxResponseBytes, 20)
	f := newFakeServer(t)
	f.on("GET /big", reply(http.StatusOK, `"`+strings.Repeat("x", 100)+`"`))

	_, err := newClient(f.srv.URL).do(context.Background(), http.MethodGet, "/big", nil, time.Second)
	require.ErrorContains(t, err, "answer larger than 20 bytes")
}

func TestClientSendsJSONBodyAndPath(t *testing.T) {
	f := newFakeServer(t)
	f.on("POST /echo", reply(http.StatusOK, `"ok"`))

	value, err := newClient(f.srv.URL).do(context.Background(), http.MethodPost, "/echo", map[string]int{"n": 1}, time.Second)
	require.NoError(t, err)
	assert.JSONEq(t, `"ok"`, string(value))
	assert.Equal(t, map[string]interface{}{"n": float64(1)}, f.first(t, http.MethodPost, "/echo"))
}

func TestClientRefusesABodyItCannotEncode(t *testing.T) {
	_, err := newClient("http://127.0.0.1:1").do(context.Background(), http.MethodPost, "/x", make(chan int), time.Second)
	require.ErrorContains(t, err, "cannot encode")
}
