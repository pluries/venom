package ovhapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// TestRun_ErrorResponseHasNoBody pins SA4031 (#2028): `res := new(interface{})` always returns a
// non-nil *interface{}, so `if res != nil` at the end of Run is always true and runs unconditionally
// — including after an API error, when go-ovh's UnmarshalResponse returns before ever touching *res
// (ovh.go: the error branch `return apiError` happens before the json.Decode call). The guard's
// evident intent — only build Body/BodyJSON when something was actually decoded — needs `*res != nil`
// instead. Before the fix: an API error left `r.Body` as the literal string "null" and `r.BodyJSON`
// nil, alongside `r.Err` carrying the real error message. Confirmed against the vendored
// github.com/ovh/go-ovh@v1.9.0 source before writing this test, not assumed.
func TestRun_ErrorResponseHasNoBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(`{"message":"boom"}`)); err != nil {
			t.Fatalf("test server: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	e := Executor{}
	res, err := e.Run(context.Background(), venom.TestStep{
		"endpoint":          srv.URL,
		"applicationKey":    "test-key",
		"applicationSecret": "test-secret",
		"noAuth":            true,
		"path":              "/",
	})
	require.NoError(t, err, "Run reports the API error through the result, not through err")

	result, ok := res.(Result)
	require.True(t, ok)

	require.Equal(t, http.StatusInternalServerError, result.StatusCode)
	require.Equal(t, "boom", result.Err, "the error message from the API must still come through")
	require.Empty(t, result.Body, "no body was decoded on the error path — Body must stay empty, not the string \"null\"")
	require.Nil(t, result.BodyJSON, "no body was decoded on the error path — BodyJSON must stay nil")
}

// TestRun_SuccessResponseHasBody is the companion case: on a 2xx response the body IS decoded into
// *res by go-ovh's UnmarshalResponse, so BodyJSON and Body must reflect it. Written alongside the
// error-path test so the fix (checking *res instead of res) is proven from both directions, not only
// the one that changes.
func TestRun_SuccessResponseHasBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
			t.Fatalf("test server: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	e := Executor{}
	res, err := e.Run(context.Background(), venom.TestStep{
		"endpoint":          srv.URL,
		"applicationKey":    "test-key",
		"applicationSecret": "test-secret",
		"noAuth":            true,
		"path":              "/",
	})
	require.NoError(t, err)

	result, ok := res.(Result)
	require.True(t, ok)

	require.Equal(t, 200, result.StatusCode)
	require.JSONEq(t, `{"ok":true}`, result.Body)
	require.NotNil(t, result.BodyJSON)
}
