package ovhapi

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig/tlstest"
)

// Characterisation tests for (Executor).Run (#2031): they pin what the executor does today
// against a local fake of the OVHcloud API, odd parts included. Every credential is a synthetic
// placeholder; nothing here reaches the real API (named endpoints are only ever sent to a local
// proxy that refuses the tunnel).

const (
	fakeAppID      = "app-key-synthetic"
	fakeAppSigning = "app-signing-synthetic"
	fakeConsumerID = "consumer-key-synthetic"
	otherAppID     = "other-app-key-synthetic"
	fakeClientID   = "client-id-synthetic"
	fakeClientPair = "client-pair-synthetic"
)

var signaturePattern = regexp.MustCompile(`^\$1\$[0-9a-f]{40}$`)

// isolateOVHConfig keeps go-ovh from reading a developer's OVH_* variables or an ovh.conf in the
// working directory, and gives venom a logger.
func isolateOVHConfig(t *testing.T) {
	t.Helper()
	for _, name := range []string{"OVH_ENDPOINT", "OVH_APPLICATION_KEY", "OVH_APPLICATION_SECRET", "OVH_CONSUMER_KEY", "OVH_CLIENT_ID", "OVH_CLIENT_SECRET", "OVH_ACCESS_TOKEN"} {
		t.Setenv(name, "")
	}
	t.Chdir(t.TempDir())
	venom.InitTestLogger(t)
}

type apiReply struct {
	status  int
	body    string
	headers map[string][]string
}

type seenRequest struct {
	method     string
	path       string
	requestURI string
	host       string
	body       string
	header     http.Header
}

// fakeAPI records every request and answers /auth/time with the current time (or authTime when
// set) and anything else with reply.
type fakeAPI struct {
	*httptest.Server
	mu       sync.Mutex
	requests []seenRequest
}

func (f *fakeAPI) handle(t *testing.T, reply apiReply, authTime *apiReply) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("fake API: reading the request body: %v", err)
		}
		f.mu.Lock()
		f.requests = append(f.requests, seenRequest{method: r.Method, path: r.URL.Path, requestURI: r.RequestURI, host: r.Host, body: string(body), header: r.Header.Clone()})
		f.mu.Unlock()
		if r.URL.Path == "/auth/time" {
			if authTime != nil {
				writeReply(t, w, *authTime)
				return
			}
			writeReply(t, w, apiReply{body: strconv.FormatInt(time.Now().Unix(), 10)})
			return
		}
		writeReply(t, w, reply)
	}
}

func writeReply(t *testing.T, w http.ResponseWriter, reply apiReply) {
	for k, values := range reply.headers {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	if reply.status != 0 {
		w.WriteHeader(reply.status)
	}
	if _, err := io.WriteString(w, reply.body); err != nil {
		t.Errorf("fake API: writing the reply: %v", err)
	}
}

func newFakeAPI(t *testing.T, reply apiReply) *fakeAPI {
	t.Helper()
	f := &fakeAPI{}
	f.Server = httptest.NewServer(f.handle(t, reply, nil))
	t.Cleanup(f.Close)
	return f
}

func newFakeAPIWithAuthTime(t *testing.T, reply, authTime apiReply) *fakeAPI {
	t.Helper()
	f := &fakeAPI{}
	f.Server = httptest.NewServer(f.handle(t, reply, &authTime))
	t.Cleanup(f.Close)
	return f
}

func newFakeTLSAPI(t *testing.T, reply apiReply) *fakeAPI {
	t.Helper()
	f := &fakeAPI{}
	f.Server = httptest.NewTLSServer(f.handle(t, reply, nil))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeAPI) seen() []seenRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]seenRequest(nil), f.requests...)
}

func (f *fakeAPI) last(t *testing.T) seenRequest {
	t.Helper()
	all := f.seen()
	require.NotEmpty(t, all, "the fake API received no request")
	return all[len(all)-1]
}

func (f *fakeAPI) paths() []string {
	var paths []string
	for _, r := range f.seen() {
		paths = append(paths, r.path)
	}
	return paths
}

// baseStep is an unsigned call to /me with synthetic application credentials; extra overrides
// its keys, and a nil value removes one.
func baseStep(endpoint string, extra venom.TestStep) venom.TestStep {
	step := venom.TestStep{
		"endpoint":          endpoint,
		"applicationKey":    fakeAppID,
		"applicationSecret": fakeAppSigning,
		"noAuth":            true,
		"path":              "/me",
	}
	for k, v := range extra {
		if v == nil {
			delete(step, k)
			continue
		}
		step[k] = v
	}
	return step
}

func ctxWithVars(vars map[string]any) context.Context {
	ctx := context.Background()
	for k, v := range vars {
		ctx = context.WithValue(ctx, venom.ContextKey("var."+k), v)
	}
	return ctx
}

// runOVH runs the step and checks the shape of what comes back: a Result and no error, or no
// result and an error.
func runOVH(t *testing.T, step venom.TestStep, vars map[string]any) (Result, error) {
	t.Helper()
	res, err := Executor{}.Run(ctxWithVars(vars), step)
	if err != nil {
		require.Nil(t, res, "an error never comes with a result")
		return Result{}, err
	}
	r, ok := res.(Result)
	require.True(t, ok, "Run returns a Result value")
	return r, nil
}

func TestRun_sendsTheStepMethod(t *testing.T) {
	isolateOVHConfig(t)
	for _, tt := range []struct {
		name, method, want string
	}{
		{name: "no method defaults to GET", method: "", want: http.MethodGet},
		{name: "GET", method: http.MethodGet, want: http.MethodGet},
		{name: "POST", method: http.MethodPost, want: http.MethodPost},
		{name: "PUT", method: http.MethodPut, want: http.MethodPut},
		{name: "DELETE", method: http.MethodDelete, want: http.MethodDelete},
		{name: "PATCH", method: http.MethodPatch, want: http.MethodPatch},
		{name: "a lower-case method is sent as written", method: "post", want: "post"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := newFakeAPI(t, apiReply{body: `{}`})
			r, err := runOVH(t, baseStep(api.URL, venom.TestStep{"method": tt.method}), nil)
			require.NoError(t, err)
			assert.Equal(t, 200, r.StatusCode)
			assert.Equal(t, tt.want, api.last(t).method)
			assert.Equal(t, "/me", api.last(t).path)
		})
	}
	t.Run("an invalid method fails before any call", func(t *testing.T) {
		api := newFakeAPI(t, apiReply{body: `{}`})
		_, err := runOVH(t, baseStep(api.URL, venom.TestStep{"method": "BAD METHOD"}), nil)
		require.EqualError(t, err, `net/http: invalid method "BAD METHOD"`)
		assert.Empty(t, api.seen())
	})
}

func TestRun_requestHeaders(t *testing.T) {
	isolateOVHConfig(t)
	for _, tt := range []struct {
		name       string
		extra      venom.TestStep
		vars       map[string]any
		want       map[string]string
		wantAbsent []string
		wantHost   string
	}{
		{
			name:       "defaults of an unsigned call without a body",
			want:       map[string]string{"Accept": "application/json", "User-Agent": "github.com/ovh/go-ovh", "X-Ovh-Application": fakeAppID},
			wantAbsent: []string{"Content-Type", "X-Ovh-Signature", "X-Ovh-Timestamp", "X-Ovh-Consumer", "Authorization"},
		},
		{name: "a body adds the JSON Content-Type", extra: venom.TestStep{"body": `{"a":1}`}, want: map[string]string{"Content-Type": "application/json;charset=utf-8"}},
		{name: "step headers are sent", extra: venom.TestStep{"headers": map[string]string{"X-Trace": "t-1"}}, want: map[string]string{"X-Trace": "t-1"}},
		{name: "context headers are sent", vars: map[string]any{"ovh.headers": map[string]string{"X-Trace": "ctx-1"}}, want: map[string]string{"X-Trace": "ctx-1"}},
		{
			name:  "a step header wins over the same context header",
			extra: venom.TestStep{"headers": map[string]string{"X-Trace": "step"}},
			vars:  map[string]any{"ovh.headers": map[string]string{"X-Trace": "ctx", "X-Other": "ctx-other"}},
			want:  map[string]string{"X-Trace": "step", "X-Other": "ctx-other"},
		},
		{name: "a header replaces a default one", extra: venom.TestStep{"headers": map[string]string{"Accept": "text/plain", "User-Agent": "venom-test"}}, want: map[string]string{"Accept": "text/plain", "User-Agent": "venom-test"}},
		{name: "a Host header sets the request host", extra: venom.TestStep{"headers": map[string]string{"Host": "api.example.invalid"}}, wantHost: "api.example.invalid"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := newFakeAPI(t, apiReply{body: `{}`})
			_, err := runOVH(t, baseStep(api.URL, tt.extra), tt.vars)
			require.NoError(t, err)
			seen := api.last(t)
			for k, v := range tt.want {
				assert.Equal(t, v, seen.header.Get(k), k)
			}
			for _, k := range tt.wantAbsent {
				assert.Empty(t, seen.header.Values(k), k)
			}
			if tt.wantHost != "" {
				assert.Equal(t, tt.wantHost, seen.host)
			}
		})
	}
}

func TestRun_signsTheCallUnlessNoAuth(t *testing.T) {
	isolateOVHConfig(t)
	for _, tt := range []struct {
		name       string
		extra      venom.TestStep
		vars       map[string]any
		wantSigned bool
	}{
		{name: "no noAuth anywhere signs the call", extra: venom.TestStep{"noAuth": nil, "consumerKey": fakeConsumerID}, wantSigned: true},
		{name: "noAuth true in the step does not sign", extra: venom.TestStep{"noAuth": true}},
		{name: "ovh.noAuth true in the context does not sign", extra: venom.TestStep{"noAuth": nil}, vars: map[string]any{"ovh.noAuth": true}},
		{name: "ovh.noAuth as the string true does not sign", extra: venom.TestStep{"noAuth": nil}, vars: map[string]any{"ovh.noAuth": "true"}},
		{name: "noAuth false in the step wins over the context", extra: venom.TestStep{"noAuth": false, "consumerKey": fakeConsumerID}, vars: map[string]any{"ovh.noAuth": true}, wantSigned: true},
		{name: "the consumer key comes from the context", extra: venom.TestStep{"noAuth": false}, vars: map[string]any{"ovh.consumerKey": fakeConsumerID}, wantSigned: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := newFakeAPI(t, apiReply{body: `{}`})
			_, err := runOVH(t, baseStep(api.URL, tt.extra), tt.vars)
			require.NoError(t, err)
			seen := api.last(t)
			if !tt.wantSigned {
				assert.Equal(t, []string{"/me"}, api.paths(), "no clock call")
				assert.Empty(t, seen.header.Get("X-Ovh-Signature"))
				return
			}
			assert.Equal(t, []string{"/auth/time", "/me"}, api.paths(), "the clock is read first, unsigned")
			assert.Empty(t, api.seen()[0].header.Get("X-Ovh-Signature"))
			assert.Equal(t, fakeConsumerID, seen.header.Get("X-Ovh-Consumer"))
			assert.Regexp(t, signaturePattern, seen.header.Get("X-Ovh-Signature"))
			ts, err := strconv.ParseInt(seen.header.Get("X-Ovh-Timestamp"), 10, 64)
			require.NoError(t, err)
			assert.InDelta(t, time.Now().Unix(), ts, 5)
		})
	}
}

// D106: only a consumer key: go-ovh accepts it as the application method, and with no application key
// the call is neither signed nor preceded by a clock read, even without noAuth.
func TestRun_consumerKeyAloneIsNotSigned(t *testing.T) {
	isolateOVHConfig(t)
	api := newFakeAPI(t, apiReply{body: `{}`})
	r, err := runOVH(t, baseStep(api.URL, venom.TestStep{"applicationKey": nil, "applicationSecret": nil, "noAuth": false, "consumerKey": fakeConsumerID}), nil)
	require.NoError(t, err)
	assert.Equal(t, 200, r.StatusCode)
	assert.Equal(t, []string{"/me"}, api.paths())
	seen := api.last(t)
	for _, k := range []string{"X-Ovh-Application", "X-Ovh-Signature", "X-Ovh-Timestamp", "X-Ovh-Consumer", "Authorization"} {
		assert.Empty(t, seen.header.Values(k), k)
	}
}

// D100: a failing clock read is a Run error, while the same API error on the call itself is
// reported in the result (see TestRun_apiErrorsAreResults).
func TestRun_clockReadFailureIsARunError(t *testing.T) {
	isolateOVHConfig(t)
	api := newFakeAPIWithAuthTime(t, apiReply{body: `{}`}, apiReply{status: http.StatusInternalServerError, body: `{"message":"clock down"}`})
	_, err := runOVH(t, baseStep(api.URL, venom.TestStep{"noAuth": false}), nil)
	require.EqualError(t, err, `OVHcloud API error (status code 500): "clock down"`)
	assert.Equal(t, []string{"/auth/time"}, api.paths())
}

func TestRun_clientConstructionErrors(t *testing.T) {
	isolateOVHConfig(t)
	const unreachable = "http://127.0.0.1:1/1.0"
	for _, tt := range []struct {
		name    string
		step    venom.TestStep
		vars    map[string]any
		wantErr string
	}{
		{
			name:    "no credentials anywhere",
			step:    venom.TestStep{"endpoint": unreachable, "path": "/me"},
			wantErr: "missing authentication information, you need to provide one of the following: application_key/application_secret, client_id/client_secret, or access_token",
		},
		{
			name:    "an application key without its secret",
			step:    baseStep(unreachable, venom.TestStep{"applicationSecret": nil}),
			wantErr: "invalid authentication config, both application_key and application_secret must be given",
		},
		{name: "an endpoint with a trailing slash", step: baseStep(unreachable+"/", nil), wantErr: "endpoint name cannot have a trailing slash"},
		{name: "an unknown endpoint name", step: baseStep("ovh-mars", nil), wantErr: "unknown endpoint 'ovh-mars', consider checking 'Endpoints' list or using an URL"},
		{
			name:    "OAuth2 needs a known OVHcloud endpoint",
			step:    venom.TestStep{"endpoint": unreachable, "clientID": fakeClientID, "clientSecret": fakeClientPair, "path": "/me"},
			wantErr: `oauth2 authentication is not compatible with endpoint "http://127.0.0.1:1/1.0"`,
		},
		{
			name:    "a client ID without its secret falls back to application credentials",
			step:    venom.TestStep{"endpoint": unreachable, "clientID": fakeClientID, "path": "/me"},
			wantErr: "missing authentication information, you need to provide one of the following: application_key/application_secret, client_id/client_secret, or access_token",
		},
		{
			name:    "a client ID and secret win over application credentials",
			step:    baseStep(unreachable, venom.TestStep{"clientID": fakeClientID, "clientSecret": fakeClientPair}),
			wantErr: `oauth2 authentication is not compatible with endpoint "http://127.0.0.1:1/1.0"`,
		},
		{
			name:    "the OAuth2 pair comes from the context",
			step:    venom.TestStep{"endpoint": unreachable, "path": "/me"},
			vars:    map[string]any{"ovh.clientID": fakeClientID, "ovh.clientSecret": fakeClientPair},
			wantErr: `oauth2 authentication is not compatible with endpoint "http://127.0.0.1:1/1.0"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runOVH(t, tt.step, tt.vars)
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestRun_credentialsAndEndpointSources(t *testing.T) {
	isolateOVHConfig(t)
	api := newFakeAPI(t, apiReply{body: `{}`})
	for _, tt := range []struct {
		name    string
		step    venom.TestStep
		vars    map[string]any
		env     map[string]string
		wantApp string
	}{
		{name: "the step values", step: baseStep(api.URL, nil), wantApp: fakeAppID},
		{
			name:    "a step value wins over the context",
			step:    baseStep(api.URL, nil),
			vars:    map[string]any{"ovh.endpoint": "http://127.0.0.1:1", "ovh.applicationKey": otherAppID},
			wantApp: fakeAppID,
		},
		{
			name:    "everything from the context",
			step:    venom.TestStep{"path": "/me"},
			vars:    map[string]any{"ovh.endpoint": api.URL, "ovh.applicationKey": fakeAppID, "ovh.applicationSecret": fakeAppSigning, "ovh.noAuth": true},
			wantApp: fakeAppID,
		},
		{
			name:    "credentials from the OVH_ environment of go-ovh",
			step:    venom.TestStep{"endpoint": api.URL, "noAuth": true, "path": "/me"},
			env:     map[string]string{"OVH_APPLICATION_KEY": otherAppID, "OVH_APPLICATION_SECRET": fakeAppSigning},
			wantApp: otherAppID,
		},
		{
			name:    "the endpoint from OVH_ENDPOINT when neither step nor context has one",
			step:    baseStep("", venom.TestStep{"endpoint": nil}),
			env:     map[string]string{"OVH_ENDPOINT": api.URL},
			wantApp: fakeAppID,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			r, err := runOVH(t, tt.step, tt.vars)
			require.NoError(t, err)
			assert.Equal(t, 200, r.StatusCode)
			assert.Equal(t, tt.wantApp, api.last(t).header.Get("X-Ovh-Application"))
		})
	}
}

// refusingProxy answers every CONNECT with 403, so a named endpoint can be observed without
// leaving the machine.
func refusingProxy(t *testing.T) (proxy *httptest.Server, tunnels func() []string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	proxy = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.Host)
		mu.Unlock()
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(proxy.Close)
	return proxy, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

func TestRun_endpointNamesResolveToOVHcloudURLs(t *testing.T) {
	isolateOVHConfig(t)
	for _, tt := range []struct {
		name, endpoint, path, wantErr, wantTunnel string
	}{
		{name: "ovh-eu", endpoint: "ovh-eu", path: "/me", wantErr: `Get "https://eu.api.ovh.com/1.0/me": Forbidden`, wantTunnel: "CONNECT eu.api.ovh.com:443"},
		{name: "ovh-ca", endpoint: "ovh-ca", path: "/me", wantErr: `Get "https://ca.api.ovh.com/1.0/me": Forbidden`, wantTunnel: "CONNECT ca.api.ovh.com:443"},
		{name: "a /v2 path drops the /1.0 suffix", endpoint: "ovh-eu", path: "/v2/iam/policy", wantErr: `Get "https://eu.api.ovh.com/v2/iam/policy": Forbidden`, wantTunnel: "CONNECT eu.api.ovh.com:443"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			proxy, tunnels := refusingProxy(t)
			_, err := runOVH(t, baseStep(tt.endpoint, venom.TestStep{"path": tt.path, "proxy": proxy.URL}), nil)
			require.EqualError(t, err, tt.wantErr)
			assert.Equal(t, []string{tt.wantTunnel}, tunnels())
		})
	}
}

func TestRun_proxy(t *testing.T) {
	isolateOVHConfig(t)
	const target = "http://api.example.invalid/1.0"
	for _, tt := range []struct {
		name    string
		fromCtx bool
	}{
		{name: "from the step"},
		{name: "from the context", fromCtx: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			proxy := newFakeAPI(t, apiReply{body: `{"via":"proxy"}`})
			step := baseStep(target, venom.TestStep{"proxy": proxy.URL})
			var vars map[string]any
			if tt.fromCtx {
				step = baseStep(target, nil)
				vars = map[string]any{"ovh.proxy": proxy.URL}
			}
			r, err := runOVH(t, step, vars)
			require.NoError(t, err)
			assert.JSONEq(t, `{"via":"proxy"}`, r.Body)
			assert.Equal(t, "http://api.example.invalid/1.0/me", proxy.last(t).requestURI, "the proxy gets the absolute URL")
		})
	}
	t.Run("an unparseable proxy URL fails", func(t *testing.T) {
		_, err := runOVH(t, baseStep(target, venom.TestStep{"proxy": ":bad"}), nil)
		require.EqualError(t, err, `parse ":bad": missing protocol scheme`)
	})
}

func TestRun_resolve(t *testing.T) {
	isolateOVHConfig(t)
	api := newFakeAPI(t, apiReply{body: `{}`})
	addr, ok := api.Listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	host := "api.example.invalid:" + strconv.Itoa(addr.Port)
	mapping := host + ":127.0.0.1"
	for _, tt := range []struct {
		name     string
		endpoint string
		resolve  any
		vars     map[string]any
		wantErr  string
		wantHost string
	}{
		{name: "a matching entry redirects the dial", endpoint: "http://" + host, resolve: []string{mapping}, wantHost: host},
		{name: "entries from the context", endpoint: "http://" + host, vars: map[string]any{"ovh.resolve": []string{mapping}}, wantHost: host},
		{name: "an entry that does not match leaves the dial alone", endpoint: api.URL, resolve: []string{"other.example.invalid:443:127.0.0.9"}, wantHost: addr.String()},
		{
			// D107: the entries are only checked when a connection is dialled, and any bad one fails every dial
			name:     "a malformed entry fails the call",
			endpoint: api.URL,
			resolve:  []string{mapping, "api.example.invalid"},
			wantErr:  `Get "` + api.URL + `/me": invalid value for resolve attribute: [` + mapping + ` api.example.invalid]`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			extra := venom.TestStep{}
			if tt.resolve != nil {
				extra["resolve"] = tt.resolve
			}
			r, err := runOVH(t, baseStep(tt.endpoint, extra), tt.vars)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 200, r.StatusCode)
			assert.Equal(t, tt.wantHost, api.last(t).host)
		})
	}
}

func TestRun_tlsRootCA(t *testing.T) {
	isolateOVHConfig(t)
	api := newFakeTLSAPI(t, apiReply{body: `{"tls":true}`})
	caPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: api.Certificate().Raw}))
	workdir := t.TempDir()
	writeFile(t, filepath.Join(workdir, "certs", "ca.pem"), caPEM)
	inWorkdir := func(vars map[string]any) map[string]any {
		vars["venom.testsuite.workdir"] = workdir
		return vars
	}
	refused := `Get "` + api.URL + `/me": tls: failed to verify certificate: x509: certificate signed by unknown authority`
	for _, tt := range []struct {
		name        string
		extra       venom.TestStep
		vars        map[string]any
		wantErr     string
		beforeACall bool
	}{
		{name: "the step CA is trusted", extra: venom.TestStep{"tlsRootCA": caPEM}},
		{name: "the context CA is trusted", vars: map[string]any{"ovh.tlsRootCA": caPEM}},
		{name: "without it the certificate is refused", wantErr: refused},
		{name: "a value that is not PEM fails before any call", extra: venom.TestStep{"tlsRootCA": "not a certificate"}, wantErr: "TLSRootCA: failed to add a certificate to the cert pool"},
		// #2372: a file path relative to the suite, and the ADR-0009 name tls_root_ca.
		{name: "a step CA given as a path relative to the workdir is trusted", extra: venom.TestStep{"tlsRootCA": "certs/ca.pem"}, vars: inWorkdir(map[string]any{})},
		{name: "a context CA given as a path relative to the workdir is trusted", vars: inWorkdir(map[string]any{"ovh.tlsRootCA": "certs/ca.pem"})},
		{name: "tls_root_ca is trusted", extra: venom.TestStep{"tls_root_ca": caPEM}},
		{name: "tls_root_ca given as a path relative to the workdir is trusted", extra: venom.TestStep{"tls_root_ca": "certs/ca.pem"}, vars: inWorkdir(map[string]any{})},
		{name: "tls_root_ca from another CA is refused", extra: venom.TestStep{"tls_root_ca": string(tlstest.New(t).CAPEM)}, wantErr: refused},
		{name: "a step tls_root_ca wins over the context CA", extra: venom.TestStep{"tls_root_ca": caPEM}, vars: map[string]any{"ovh.tlsRootCA": "not a certificate"}},
		{name: "tlsRootCA and tls_root_ca both set fail before any call", extra: venom.TestStep{"tlsRootCA": caPEM, "tls_root_ca": caPEM}, wantErr: "tlsRootCA and tls_root_ca are both set: keep one", beforeACall: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			callsBefore := len(api.seen())
			r, err := runOVH(t, baseStep(api.URL, tt.extra), tt.vars)
			if tt.beforeACall {
				assert.Len(t, api.seen(), callsBefore, "the step fails before any call")
			}
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.JSONEq(t, `{"tls":true}`, r.Body)
		})
	}
}

// captureWarnings replaces warnf for the test and returns the warnings it receives.
func captureWarnings(t *testing.T) *[]string {
	t.Helper()
	warnings := &[]string{}
	warnf = func(_ context.Context, format string, args ...interface{}) {
		*warnings = append(*warnings, fmt.Sprintf(format, args...))
	}
	t.Cleanup(func() { warnf = venom.Warning })
	return warnings
}

// #2385, ADR-0009 Precedence 3: a root CA only serves TLS connections (a redirect to https:// or an https:// proxy included). With an
// http:// endpoint the step warns instead of keeping silent; the call itself does not change.
func TestRun_tlsRootCAWithAPlainHTTPEndpointWarns(t *testing.T) {
	isolateOVHConfig(t)
	warnings := captureWarnings(t)
	const warning = "ovhapi: tlsRootCA applies only to TLS connections: the endpoint is http:// (it still serves a redirect to https:// or an https:// proxy)"
	plain := newFakeAPI(t, apiReply{body: `{}`})
	secure := newFakeTLSAPI(t, apiReply{body: `{}`})
	secureCA := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: secure.Certificate().Raw}))
	ca := string(tlstest.New(t).CAPEM)
	proxy, _ := refusingProxy(t)
	for _, tt := range []struct {
		name        string
		endpoint    string
		extra       venom.TestStep
		vars        map[string]any
		wantErr     string
		wantWarning []string
	}{
		{name: "a step tlsRootCA, http://", endpoint: plain.URL, extra: venom.TestStep{"tlsRootCA": ca}, wantWarning: []string{warning}},
		{name: "a step tls_root_ca, http://", endpoint: plain.URL, extra: venom.TestStep{"tls_root_ca": ca}, wantWarning: []string{warning}},
		{name: "the ovh.tlsRootCA variable, http://", endpoint: plain.URL, vars: map[string]any{"ovh.tlsRootCA": ca}, wantWarning: []string{warning}},
		{name: "a step tlsRootCA, HTTP://", endpoint: "HTTP://" + plain.Listener.Addr().String(), extra: venom.TestStep{"tlsRootCA": ca}, wantWarning: []string{warning}},
		{name: "a step tlsRootCA, https://", endpoint: secure.URL, extra: venom.TestStep{"tlsRootCA": secureCA}},
		{
			name: "a step tlsRootCA, the ovh-eu alias", endpoint: "ovh-eu", extra: venom.TestStep{"tlsRootCA": ca, "proxy": proxy.URL},
			wantErr: `Get "https://eu.api.ovh.com/1.0/me": Forbidden`,
		},
		{name: "no root CA, http://", endpoint: plain.URL},
	} {
		t.Run(tt.name, func(t *testing.T) {
			*warnings = nil
			_, err := runOVH(t, baseStep(tt.endpoint, tt.extra), tt.vars)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantWarning, *warnings)
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestRun_requestBody(t *testing.T) {
	isolateOVHConfig(t)
	workdir := t.TempDir()
	writeFile(t, filepath.Join(workdir, "body.json"), `{"from":"file"}`)
	writeFile(t, filepath.Join(workdir, "empty.json"), "")
	writeFile(t, filepath.Join(workdir, "sub", "nested.json"), `[1,2]`)
	withWorkdir := map[string]any{"venom.testsuite.workdir": workdir}
	for _, tt := range []struct {
		name     string
		extra    venom.TestStep
		vars     map[string]any
		wantBody string
	}{
		{name: "no body sends nothing", wantBody: ""},
		{name: "a JSON body is re-encoded with sorted keys", extra: venom.TestStep{"body": `{ "b": 1, "a": "x" }`}, wantBody: `{"a":"x","b":1}`},
		{name: "large numbers keep their digits", extra: venom.TestStep{"body": `{"n":12345678901234567890}`}, wantBody: `{"n":12345678901234567890}`},
		{name: "a scalar body", extra: venom.TestStep{"body": `42`}, wantBody: `42`},
		{name: "a JSON null body is sent as null", extra: venom.TestStep{"body": `null`}, wantBody: `null`},
		// D101: only the first JSON value is read; what follows it is dropped silently.
		{name: "content after the first JSON value is dropped", extra: venom.TestStep{"body": `{"a":1} trailing`}, wantBody: `{"a":1}`},
		{name: "a body file relative to the workdir", extra: venom.TestStep{"bodyFile": "body.json"}, vars: withWorkdir, wantBody: `{"from":"file"}`},
		{name: "a body file in a sub-folder", extra: venom.TestStep{"bodyFile": "sub/nested.json"}, vars: withWorkdir, wantBody: `[1,2]`},
		{name: "an absolute body file is still read under the workdir", extra: venom.TestStep{"bodyFile": "/body.json"}, vars: withWorkdir, wantBody: `{"from":"file"}`},
		{name: "body wins over bodyFile", extra: venom.TestStep{"body": `{"from":"step"}`, "bodyFile": "body.json"}, vars: withWorkdir, wantBody: `{"from":"step"}`},
		// D102: a missing body file is not an error; the call goes out without a body.
		{name: "a missing body file sends nothing", extra: venom.TestStep{"bodyFile": "missing.json"}, vars: withWorkdir, wantBody: ""},
		{name: "an empty body file sends nothing", extra: venom.TestStep{"bodyFile": "empty.json"}, vars: withWorkdir, wantBody: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := newFakeAPI(t, apiReply{body: `{}`})
			_, err := runOVH(t, baseStep(api.URL, tt.extra), tt.vars)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBody, api.last(t).body)
		})
	}
}

func TestRun_requestBodyErrors(t *testing.T) {
	isolateOVHConfig(t)
	workdir := t.TempDir()
	writeFile(t, filepath.Join(workdir, "bad.json"), `{nope}`)
	require.NoError(t, os.Mkdir(filepath.Join(workdir, "adir"), 0o750))
	withWorkdir := map[string]any{"venom.testsuite.workdir": workdir}
	for _, tt := range []struct {
		name    string
		extra   venom.TestStep
		wantErr string
	}{
		{name: "a body that is not JSON", extra: venom.TestStep{"body": "not json"}, wantErr: "invalid character 'o' in literal null (expecting 'u')"},
		{name: "a body file that is not JSON", extra: venom.TestStep{"bodyFile": "bad.json"}, wantErr: "invalid character 'n' looking for beginning of object key string"},
		{name: "a body file that is a folder", extra: venom.TestStep{"bodyFile": "adir"}, wantErr: "read " + filepath.Join(workdir, "adir") + ": is a directory"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := newFakeAPI(t, apiReply{body: `{}`})
			_, err := runOVH(t, baseStep(api.URL, tt.extra), withWorkdir)
			require.EqualError(t, err, tt.wantErr)
			assert.Empty(t, api.seen(), "nothing is sent")
		})
	}
}

func TestRun_successfulResponses(t *testing.T) {
	isolateOVHConfig(t)
	for _, tt := range []struct {
		name         string
		reply        apiReply
		wantBody     string
		wantBodyJSON any
	}{
		{name: "a JSON object", reply: apiReply{body: `{"b": 2, "a": [1, "x"]}`}, wantBody: `{"a":[1,"x"],"b":2}`, wantBodyJSON: map[string]any{"a": []any{json.Number("1"), "x"}, "b": json.Number("2")}},
		{name: "a JSON array", reply: apiReply{body: `[1,2]`}, wantBody: `[1,2]`, wantBodyJSON: []any{json.Number("1"), json.Number("2")}},
		{name: "a JSON string", reply: apiReply{body: `"ok"`}, wantBody: `"ok"`, wantBodyJSON: "ok"},
		{name: "large numbers keep their digits", reply: apiReply{body: `12345678901234567890`}, wantBody: `12345678901234567890`, wantBodyJSON: json.Number("12345678901234567890")},
		// D103: every 2xx is reported as 200.
		{name: "201 is reported as 200", reply: apiReply{status: http.StatusCreated, body: `{"id":1}`}, wantBody: `{"id":1}`, wantBodyJSON: map[string]any{"id": json.Number("1")}},
		{name: "204 without a body", reply: apiReply{status: http.StatusNoContent}},
		{name: "a JSON null body leaves Body empty", reply: apiReply{body: `null`}},
		{name: "an empty 200 body", reply: apiReply{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := newFakeAPI(t, tt.reply)
			r, err := runOVH(t, baseStep(api.URL, nil), nil)
			require.NoError(t, err)
			assert.Equal(t, 200, r.StatusCode)
			assert.Equal(t, tt.wantBody, r.Body)
			assert.Equal(t, tt.wantBodyJSON, r.BodyJSON)
			assert.Empty(t, r.Err)
			assert.Positive(t, r.TimeSeconds)
		})
	}
}

func TestRun_responseHeaders(t *testing.T) {
	isolateOVHConfig(t)
	api := newFakeAPI(t, apiReply{body: `{}`, headers: map[string][]string{"X-Ovh-Queryid": {"EU.ext-1"}, "x-multi": {"a", "b"}}})
	r, err := runOVH(t, baseStep(api.URL, nil), nil)
	require.NoError(t, err)
	assert.Equal(t, "EU.ext-1", r.Headers["X-Ovh-Queryid"])
	assert.Equal(t, "a,b", r.Headers["X-Multi"], "several values are joined with a comma, names are canonical")
	assert.Equal(t, "2", r.Headers["Content-Length"])
}

// D104: a success body that is not JSON fails the step with the decoder's error.
func TestRun_nonJSONSuccessBodyIsARunError(t *testing.T) {
	isolateOVHConfig(t)
	api := newFakeAPI(t, apiReply{body: "hello", headers: map[string][]string{"Content-Type": {"text/plain"}}})
	_, err := runOVH(t, baseStep(api.URL, nil), nil)
	require.EqualError(t, err, "invalid character 'h' looking for beginning of value")
}

func TestRun_apiErrorsAreResults(t *testing.T) {
	isolateOVHConfig(t)
	for _, tt := range []struct {
		name       string
		reply      apiReply
		wantStatus int
		wantErr    string
	}{
		{name: "a JSON error keeps its message", reply: apiReply{status: http.StatusNotFound, body: `{"class":"Client::NotFound","message":"This service does not exist"}`}, wantStatus: 404, wantErr: "This service does not exist"},
		{name: "a JSON error without a message", reply: apiReply{status: http.StatusBadRequest, body: `{"class":"Client::BadRequest"}`}, wantStatus: 400},
		{name: "a non-JSON error keeps the raw body", reply: apiReply{status: http.StatusBadGateway, body: "Bad Gateway"}, wantStatus: 502, wantErr: "Bad Gateway"},
		{name: "an empty error body", reply: apiReply{status: http.StatusServiceUnavailable}, wantStatus: 503},
		{name: "a redirect without Location is an error status", reply: apiReply{status: http.StatusFound, body: "moved"}, wantStatus: 302, wantErr: "moved"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			api := newFakeAPI(t, tt.reply)
			r, err := runOVH(t, baseStep(api.URL, nil), nil)
			require.NoError(t, err, "an API error is reported in the result, not as a Run error")
			assert.Equal(t, tt.wantStatus, r.StatusCode)
			assert.Equal(t, tt.wantErr, r.Err)
			assert.Empty(t, r.Body)
			assert.Nil(t, r.BodyJSON)
			assert.NotEmpty(t, r.Headers)
		})
	}
}

func TestRun_stepDecodingErrors(t *testing.T) {
	isolateOVHConfig(t)
	for _, tt := range []struct {
		name    string
		extra   venom.TestStep
		wantErr string
	}{
		{name: "noAuth that is not a boolean", extra: venom.TestStep{"noAuth": "yes"}, wantErr: "1 error(s) decoding:\n\n* 'NoAuth' expected type 'bool', got unconvertible type 'string', value: 'yes'"},
		{name: "headers that are not a map", extra: venom.TestStep{"headers": "X-Trace: 1"}, wantErr: "1 error(s) decoding:\n\n* 'Headers' expected a map, got 'string'"},
		{name: "a path that is not a string", extra: venom.TestStep{"path": 42}, wantErr: "1 error(s) decoding:\n\n* 'Path' expected type 'string', got unconvertible type 'int', value: '42'"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runOVH(t, baseStep("http://127.0.0.1:1", tt.extra), nil)
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestExecutor_declaration(t *testing.T) {
	assert.Equal(t, &Executor{}, New())
	assert.Equal(t, Result{}, Executor{}.ZeroValueResult())
	assert.Equal(t, &venom.StepAssertions{Assertions: []venom.Assertion{"result.statuscode ShouldEqual 200"}}, Executor{}.GetDefaultAssertions())
}

// D105: the step context is not given to the HTTP request, so a cancelled run still makes the
// call; only go-ovh's 180 s client timeout bounds it.
func TestRun_ignoresContextCancellation(t *testing.T) {
	isolateOVHConfig(t)
	api := newFakeAPI(t, apiReply{body: `{}`})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err := Executor{}.Run(ctx, baseStep(api.URL, nil))
	require.NoError(t, err)
	r, ok := res.(Result)
	require.True(t, ok)
	assert.Equal(t, 200, r.StatusCode)
	assert.Len(t, api.seen(), 1)
}
