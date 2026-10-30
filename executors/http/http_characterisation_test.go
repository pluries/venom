package http

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig/tlstest"
)

// Characterisation tests for (Executor).Run, getRequest and TLSOptions (#2031, session C). They pin what the executor
// does today, odd parts included; the discrepancies are listed in docs/feat/2031/plan.md (D74 onwards), never fixed here.
// Every server is a local httptest server; no test reaches the network.

const digicertRootCA = "../../tests/http/tls/digicert-root-ca.crt"

// httpRun runs one step and returns its Result; a nil result (an error before the request was sent) gives a zero Result
// and ok=false.
func httpRun(ctx context.Context, t *testing.T, step venom.TestStep) (Result, bool, error) {
	t.Helper()
	venom.InitTestLogger(t)
	got, err := Executor{}.Run(ctx, step)
	if got == nil {
		return Result{}, false, err
	}
	res, ok := got.(Result)
	require.Truef(t, ok, "want a Result, got %T", got)
	return res, true, err
}

func httpWorkdirCtx(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), dir)
}

// echoHandler answers with a JSON description of the request it received.
func echoHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body) //nolint:errcheck // test server: a short read only shows up as a smaller echoed body
	w.Header().Set("Content-Type", "application/json")
	//nolint:errcheck // test server: the client side asserts on what it received
	json.NewEncoder(w).Encode(map[string]string{
		"method": r.Method,
		"host":   r.Host,
		"uri":    r.RequestURI,
		"auth":   r.Header.Get("Authorization"),
		"custom": r.Header.Get("X-Custom"),
		"body":   string(body),
	})
}

// fixedServer answers every request with the given content type and body, plus two multi-valued headers.
func fixedServer(t *testing.T, ct, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", ct)
		w.Header().Add("Set-Cookie", "a=1")
		w.Header().Add("Set-Cookie", "b=2")
		w.Header().Add("X-Multi", "one")
		w.Header().Add("X-Multi", "two")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", echoHandler)
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/echo", http.StatusSeeOther)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func resultInfo(method, u, body string) string {
	return "===== Result Info =====\n\t\tMethod:     " + method + "\n\t\tURL:        " + u + "\n\t\tBody:       " + body + "\n\t\t======================"
}

func TestRunSendsTheRequestAsConfigured(t *testing.T) {
	srv := newTestServer(t)
	host := strings.TrimPrefix(srv.URL, "http://")
	echo := func(method, h, uri, auth, custom, body string) map[string]interface{} {
		return map[string]interface{}{"method": method, "host": h, "uri": uri, "auth": auth, "custom": custom, "body": body}
	}
	cases := []struct {
		name string
		step venom.TestStep
		want map[string]interface{}
	}{
		{"the method defaults to GET", venom.TestStep{"url": srv.URL, "path": "/echo"}, echo("GET", host, "/echo", "", "", "")},
		{"a method and a body are sent", venom.TestStep{"method": "PUT", "url": srv.URL, "path": "/echo", "body": "hello"}, echo("PUT", host, "/echo", "", "", "hello")},
		{"headers are set", venom.TestStep{"url": srv.URL, "path": "/echo", "headers": map[string]string{"X-Custom": "v"}}, echo("GET", host, "/echo", "", "v", "")},
		{"a Host header overrides the request host", venom.TestStep{"url": srv.URL, "path": "/echo", "headers": map[string]string{"host": "virtual.test"}}, echo("GET", "virtual.test", "/echo", "", "", "")},
		{"query parameters are encoded and sorted", venom.TestStep{"url": srv.URL + "/echo", "query_parameters": map[string]string{"b": "x y", "a": "1"}}, echo("GET", host, "/echo?a=1&b=x+y", "", "", "")},
		{"query parameters replace the query of the url (D74)", venom.TestStep{"url": srv.URL + "/echo?keep=me", "query_parameters": map[string]string{"a": "1"}}, echo("GET", host, "/echo?a=1", "", "", "")},
		{"basic auth is sent with only a user", venom.TestStep{"url": srv.URL, "path": "/echo", "basic_auth_user": "user"}, echo("GET", host, "/echo", "Basic dXNlcjo=", "", "")},
		{"a body_file key is not the bodyfile attribute and is ignored (D82)", venom.TestStep{"url": srv.URL, "path": "/echo", "method": "POST", "body_file": digicertRootCA}, echo("POST", host, "/echo", "", "", "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _, err := httpRun(context.Background(), t, tc.step)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Equal(t, tc.want, res.BodyJSON)
		})
	}
}

func TestRunRecordsTheRequestItSent(t *testing.T) {
	srv := newTestServer(t)
	bodyfile := filepath.Join(t.TempDir(), "body.json")
	require.NoError(t, os.WriteFile(bodyfile, []byte(`{"k":"v"}`), 0o600))
	withType := func(ct string) venom.TestStep {
		return venom.TestStep{"method": "POST", "url": srv.URL, "path": "/echo", "body": "payload", "headers": map[string]string{"Content-Type": ct}}
	}
	cases := []struct {
		name       string
		step       venom.TestStep
		wantBody   string
		wantHeader http.Header
	}{
		{"a body without content type is kept", venom.TestStep{"method": "POST", "url": srv.URL, "path": "/echo", "body": "payload"}, "payload", http.Header{}},
		{"a JSON body is kept", withType("application/json"), "payload", http.Header{"Content-Type": {"application/json"}}},
		{"a text body is kept", withType("text/plain; charset=utf-8"), "payload", http.Header{"Content-Type": {"text/plain; charset=utf-8"}}},
		{"a binary body is dropped", withType("application/octet-stream"), "", http.Header{"Content-Type": {"application/octet-stream"}}},
		{"an image body is dropped", withType("image/png"), "", http.Header{"Content-Type": {"image/png"}}},
		{"a vendor type body is dropped", withType("application/vnd.ms-excel"), "", http.Header{"Content-Type": {"application/vnd.ms-excel"}}},
		{"a vendor +json body is kept, the suffix wins", withType("application/vnd.api+json"), "payload", http.Header{"Content-Type": {"application/vnd.api+json"}}},
		{"a multipart content type drops the body", withType("multipart/form-data; boundary=x"), "", http.Header{"Content-Type": {"multipart/form-data; boundary=x"}}},
		{"a preserved body file is dropped", venom.TestStep{"method": "POST", "url": srv.URL, "path": "/echo", "bodyfile": bodyfile, "preserve_bodyfile": true}, "", http.Header{}},
		{"an interpolated body file is kept", venom.TestStep{"method": "POST", "url": srv.URL, "path": "/echo", "bodyfile": bodyfile}, `{"k":"v"}`, http.Header{}},
		{"basic auth shows in the recorded headers", venom.TestStep{"url": srv.URL, "path": "/echo", "basic_auth_user": "u", "basic_auth_password": "synthetic"}, "", http.Header{"Authorization": {"Basic dTpzeW50aGV0aWM="}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _, err := httpRun(context.Background(), t, tc.step)
			require.NoError(t, err)
			assert.Equal(t, srv.URL+"/echo", res.Request.URL)
			assert.Equal(t, tc.wantBody, res.Request.Body)
			assert.Equal(t, tc.wantHeader, res.Request.Header)
			assert.Nil(t, res.Request.Form)
			assert.Nil(t, res.Request.PostForm)
			assert.Greater(t, res.TimeSeconds, 0.0)
		})
	}
}

func TestRunReadsTheResponseBodyByContentType(t *testing.T) {
	obj := `{"b":1,"a":2.50}`
	cases := []struct {
		name     string
		ct       string
		body     string
		skipBody bool
		wantBody string
		wantJSON interface{}
		wantOut  string // Result Info body; "-" when Systemout stays empty
	}{
		{"JSON is decoded with numbers kept as text", "application/json", obj, false, obj, map[string]interface{}{"a": json.Number("2.50"), "b": json.Number("1")}, `{"a":2.50,"b":1}`},
		{"a +json type is decoded", "application/problem+json", "[1]", false, "[1]", []interface{}{json.Number("1")}, "[1]"},
		{"invalid JSON keeps the body only", "application/json", "{oops", false, "{oops", nil, "{oops"},
		{"JSON sent as text is not decoded", "text/plain", obj, false, obj, nil, obj},
		{"an image body is not read", "image/png", "png", false, "", nil, "-"},
		{"a PDF body is not read", "application/pdf", "pdf", false, "", nil, "-"},
		{"skip_body reads nothing", "application/json", obj, true, "", nil, "-"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := fixedServer(t, tc.ct, tc.body)
			res, _, err := httpRun(context.Background(), t, venom.TestStep{"url": srv.URL, "path": "/r", "skip_body": tc.skipBody})
			require.NoError(t, err)
			wantOut := ""
			if tc.wantOut != "-" {
				wantOut = resultInfo("GET", srv.URL+"/r", tc.wantOut)
			}
			tc.wantOut = wantOut
			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Equal(t, tc.wantBody, res.Body)
			assert.Equal(t, tc.wantJSON, res.BodyJSON)
			assert.Equal(t, tc.wantOut, res.Systemout)
		})
	}
}

func TestRunReportsResponseHeaders(t *testing.T) {
	srv := fixedServer(t, "text/plain", "x")
	step := venom.TestStep{"url": srv.URL}

	res, _, err := httpRun(context.Background(), t, step)
	require.NoError(t, err)
	assert.Equal(t, "a=1; b=2", res.Headers["Set-Cookie"])
	assert.Equal(t, "one, two", res.Headers["X-Multi"])
	assert.Equal(t, "text/plain", res.Headers["Content-Type"])

	step["skip_headers"] = true
	res, _, err = httpRun(context.Background(), t, step)
	require.NoError(t, err)
	assert.Nil(t, res.Headers)
	assert.Equal(t, "x", res.Body)
}

// The cookie jar across one call's redirects is pinned by TestCookieRedirect.
func TestRunFollowsRedirectsUnlessTold(t *testing.T) {
	srv := newTestServer(t)
	step := venom.TestStep{"method": "POST", "url": srv.URL, "path": "/redirect", "body": "hello"}

	t.Run("followed: the request is the first one, Systemout shows the last one (D75)", func(t *testing.T) {
		res, _, err := httpRun(context.Background(), t, step)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, "POST", res.Request.Method)
		assert.Equal(t, srv.URL+"/redirect", res.Request.URL)
		assert.Equal(t, "hello", res.Request.Body)
		echoed, ok := res.BodyJSON.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "GET", echoed["method"])
		assert.Empty(t, echoed["body"])
		marshalled, err := json.Marshal(echoed)
		require.NoError(t, err)
		assert.Equal(t, resultInfo("GET", srv.URL+"/echo", string(marshalled)), res.Systemout)
	})
	t.Run("no_follow_redirect returns the redirect itself", func(t *testing.T) {
		noFollow := venom.TestStep{"no_follow_redirect": true}
		for k, v := range step {
			noFollow[k] = v
		}
		res, _, err := httpRun(context.Background(), t, noFollow)
		require.NoError(t, err)
		assert.Equal(t, http.StatusSeeOther, res.StatusCode)
		assert.Equal(t, "/echo", res.Headers["Location"])
		assert.Equal(t, resultInfo("POST", srv.URL+"/redirect", ""), res.Systemout)
	})
}

func tcpPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	addr, ok := srv.Listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return addr.Port
}

func echoedHost(t *testing.T, res Result) interface{} {
	t.Helper()
	echoed, ok := res.BodyJSON.(map[string]interface{})
	require.Truef(t, ok, "not an echo: %#v", res.BodyJSON)
	return echoed["host"]
}

func TestRunDialsThroughResolve(t *testing.T) {
	srv := newTestServer(t)
	port := tcpPort(t, srv)
	target := fmt.Sprintf("http://example.test:%d/echo", port)
	good := fmt.Sprintf("example.test:%d:127.0.0.1", port)

	res, _, err := httpRun(context.Background(), t, venom.TestStep{"url": target, "resolve": []string{"other.test:1:127.0.0.2", good}})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, fmt.Sprintf("example.test:%d", port), echoedHost(t, res))

	cases := []struct {
		name    string
		resolve []string
	}{
		{"a tuple without three parts", []string{"bad"}},
		{"any invalid entry fails the dial, even after a matching one", []string{good, "bad"}},
		{"an IPv6 target splits into too many parts (D76)", []string{fmt.Sprintf("example.test:%d:::1", port)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := fmt.Sprintf("Get %q: invalid value for resolve attribute: %v", target, tc.resolve)
			res, ok, err := httpRun(context.Background(), t, venom.TestStep{"url": target, "resolve": tc.resolve})
			require.EqualError(t, err, want)
			require.True(t, ok, "the result is returned with the client error")
			assert.Equal(t, want, res.Err)
			assert.Equal(t, 0, res.StatusCode)
		})
	}
}

func TestRunDialsAUnixSocketWhateverTheURLHost(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "s.sock")
	l, err := net.Listen("unix", sock)
	require.NoError(t, err)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(echoHandler))
	require.NoError(t, srv.Listener.Close())
	srv.Listener = l
	srv.Start()
	t.Cleanup(srv.Close)

	res, _, err := httpRun(context.Background(), t, venom.TestStep{"url": "http://any.test", "path": "/echo", "unix_sock": sock})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, map[string]interface{}{"method": "GET", "host": "any.test", "uri": "/echo", "auth": "", "custom": "", "body": ""}, res.BodyJSON)
}

func TestRunSendsThroughAnExplicitProxy(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(echoHandler))
	t.Cleanup(proxy.Close)

	res, _, err := httpRun(context.Background(), t, venom.TestStep{"url": "http://target.test", "path": "/echo", "proxy": proxy.URL})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, map[string]interface{}{"method": "GET", "host": "target.test", "uri": "http://target.test/echo", "auth": "", "custom": "", "body": ""}, res.BodyJSON)
}

// Errors found before the request is sent return no result at all.
func TestRunReturnsNoResultWhenTheRequestCannotBeBuilt(t *testing.T) {
	srv := newTestServer(t)
	cases := []struct {
		name string
		step venom.TestStep
		want string
	}{
		{"resolve and unix_sock together", venom.TestStep{"url": srv.URL, "resolve": []string{"a:1:b"}, "unix_sock": "/nonexistent.sock"}, "you can't use resolve and unix_sock attributes in the same time"},
		{"a proxy without a scheme", venom.TestStep{"url": srv.URL, "proxy": "://bad"}, `parse "://bad": missing protocol scheme`},
		{"an unparsable url", venom.TestStep{"url": "http://[::1"}, `parse "http://[::1": missing ']' in host`},
		{"an unparsable url with query parameters", venom.TestStep{"url": "http://[::1", "query_parameters": map[string]string{"a": "b"}}, `unable to parse url http://[::1: parse "http://[::1": missing ']' in host`},
		{"an invalid method", venom.TestStep{"url": srv.URL, "method": "BAD METHOD"}, `net/http: invalid method "BAD METHOD"`},
		{"an unparsable client certificate", venom.TestStep{"url": srv.URL, "tls_client_cert": "not-a-cert", "tls_client_key": "not-a-key"}, "failed to parse x509 mTLS certificate or key: tls: failed to find any PEM data in certificate input"},
		{"a client certificate without its key", venom.TestStep{"url": srv.URL, "tls_client_cert": "not-a-cert"}, "tls_client_cert is set without tls_client_key"},
		{"a root CA path that does not exist is read as PEM content (D81)", venom.TestStep{"url": srv.URL, "tls_root_ca": "missing-ca.pem"}, "WithTLSRootCA: failed to add a certificate to the cert pool"},
		{"a multipart form that is not a map", venom.TestStep{"url": srv.URL, "multipart_form": []interface{}{"a"}}, "[]interface {} 'multipart_form' should be a map"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok, err := httpRun(context.Background(), t, tc.step)
			require.EqualError(t, err, tc.want)
			assert.False(t, ok)
		})
	}
}

func TestRunReturnsTheClientErrorWithTheRequest(t *testing.T) {
	// A missing unix socket is the unreachable server: a closed TCP port can be reused by a parallel test's server.
	sock := filepath.Join(t.TempDir(), "missing.sock")
	u := "http://h.test"

	res, ok, err := httpRun(context.Background(), t, venom.TestStep{"url": u, "path": "/p", "body": "b", "unix_sock": sock})
	want := fmt.Sprintf("Get %q: dial unix %s: connect: no such file or directory", u+"/p", sock)
	require.EqualError(t, err, want)
	require.True(t, ok)
	assert.Equal(t, want, res.Err)
	assert.Equal(t, HTTPRequest{Method: "GET", URL: u + "/p", Header: http.Header{}, Body: "b"}, res.Request)
	assert.Zero(t, res.StatusCode)
	assert.Zero(t, res.TimeSeconds)
	assert.Empty(t, res.Systemout)
	assert.Nil(t, res.Headers)
}

func pemOf(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestRunVerifiesTLSUnlessToldNot(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(echoHandler))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caFile, pemOf(srv.Certificate().Raw), 0o600))

	_, _, err := httpRun(context.Background(), t, venom.TestStep{"url": srv.URL, "path": "/echo"})
	require.EqualError(t, err, fmt.Sprintf("Get %q: tls: failed to verify certificate: x509: certificate signed by unknown authority", srv.URL+"/echo"))

	cases := []struct {
		name string
		step venom.TestStep
	}{
		{"ignore_verify_ssl skips the check", venom.TestStep{"ignore_verify_ssl": true}},
		{"the server CA as an absolute root CA file", venom.TestStep{"tls_root_ca": caFile}},
		{"the server CA relative to the workdir", venom.TestStep{"tls_root_ca": "ca.pem"}},
		{"the server CA as PEM content", venom.TestStep{"tls_root_ca": string(pemOf(srv.Certificate().Raw))}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.step["url"] = srv.URL
			tc.step["path"] = "/echo"
			res, _, err := httpRun(httpWorkdirCtx(dir), t, tc.step)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, res.StatusCode)
		})
	}
}

// AC 5 of #2367 (ADR-0009 Precedence 1): skipping the server check does not drop the client certificate. The server
// requires a client certificate signed by the test CA and says whether it got tlstest's client certificate.
func TestRunSendsTheClientCertificateEvenWhenVerificationIsSkipped(t *testing.T) {
	pki := tlstest.New(t)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 || r.TLS.PeerCertificates[0].Subject.CommonName != "venom test client" {
			http.Error(w, "not the test client certificate", http.StatusForbidden)
			return
		}
		fmt.Fprint(w, "the test client certificate")
	}))
	srv.TLS = pki.ServerConfig(true)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0) // the refused handshake below is expected
	srv.StartTLS()
	t.Cleanup(srv.Close)

	pair := venom.TestStep{"tls_client_cert": string(pki.ClientCertPEM), "tls_client_key": string(pki.ClientKeyPEM)}
	cases := []struct {
		name  string
		extra venom.TestStep
	}{
		{"ignore_verify_ssl", venom.TestStep{"ignore_verify_ssl": true}},
		{"the server CA as root CA", venom.TestStep{"tls_root_ca": string(pki.CAPEM)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			step := venom.TestStep{"url": srv.URL}
			for _, m := range []venom.TestStep{tc.extra, pair} {
				for k, v := range m {
					step[k] = v
				}
			}
			res, ok, err := httpRun(context.Background(), t, step)
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, http.StatusOK, res.StatusCode)
			assert.Equal(t, "the test client certificate", res.Body)
		})
	}

	t.Run("without the client pair the server refuses the handshake", func(t *testing.T) {
		_, _, err := httpRun(context.Background(), t, venom.TestStep{"url": srv.URL, "ignore_verify_ssl": true})
		// Under TLS 1.3 the client learns of the refusal on its first read: the server's alert, or the reset after it.
		// The transport's read loop sometimes reads it before the request is written, and then names itself.
		require.Error(t, err)
		assert.Regexp(t, `^Get "`+regexp.QuoteMeta(srv.URL)+`": (readLoopPeekFailLocked: )?(remote error: tls: certificate required|read tcp [0-9.:]+->[0-9.:]+: read: connection reset by peer|EOF)$`, err.Error())
	})
}
