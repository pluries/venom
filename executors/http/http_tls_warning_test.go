package http

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig/tlstest"
)

// #2385, ADR-0009 Precedence 3: tls_root_ca, tls_client_cert and tls_client_key only serve TLS connections (a redirect
// to https:// or an https:// proxy included). With an http:// URL the step warns instead of keeping silent; nothing else changes.

const unusedTLSWarning = "http: tls_root_ca, tls_client_cert and tls_client_key apply only to TLS connections: the url is http:// (they still serve a redirect to https:// or an https:// proxy)"

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

func TestTLSOptionsWarnsWhenTheURLIsPlainHTTP(t *testing.T) {
	warnings := captureWarnings(t)
	pki := tlstest.New(t)
	ca, cert, key := string(pki.CAPEM), string(pki.ClientCertPEM), string(pki.ClientKeyPEM)
	const plain, upper, secure = "http://api.example.invalid/x", "HTTP://api.example.invalid/x", "https://api.example.invalid/x"

	cases := []struct {
		name        string
		e           Executor
		wantKinds   []string
		wantWarning []string
	}{
		{"a root CA, http://", Executor{URL: plain, TLSRootCA: ca}, []string{"root-ca"}, []string{unusedTLSWarning}},
		{"a root CA, HTTP://", Executor{URL: upper, TLSRootCA: ca}, []string{"root-ca"}, []string{unusedTLSWarning}},
		{"a client pair, http://", Executor{URL: plain, TLSClientCert: cert, TLSClientKey: key}, []string{"client-cert"}, []string{unusedTLSWarning}},
		{"a client pair, HTTP://", Executor{URL: upper, TLSClientCert: cert, TLSClientKey: key}, []string{"client-cert"}, []string{unusedTLSWarning}},
		{
			"every setting, http://", Executor{URL: plain, IgnoreVerifySSL: true, TLSRootCA: ca, TLSClientCert: cert, TLSClientKey: key},
			[]string{"insecure", "root-ca", "client-cert"}, []string{unusedTLSWarning},
		},
		{"a root CA, https://", Executor{URL: secure, TLSRootCA: ca}, []string{"root-ca"}, nil},
		{"a client pair, https://", Executor{URL: secure, TLSClientCert: cert, TLSClientKey: key}, []string{"client-cert"}, nil},
		{"no TLS setting, http://", Executor{URL: plain}, []string{}, nil},
		{"ignore_verify_ssl alone, http://", Executor{URL: plain, IgnoreVerifySSL: true}, []string{"insecure"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			*warnings = nil
			opts, err := tc.e.TLSOptions(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tc.wantKinds, tlsKinds(opts), "the options do not depend on the URL")
			assert.Equal(t, tc.wantWarning, *warnings)
		})
	}
}

// Half a client pair still fails with the error it gave before #2385; the warning comes first.
func TestTLSOptionsWarnsAndStillRefusesHalfAPair(t *testing.T) {
	warnings := captureWarnings(t)
	pki := tlstest.New(t)

	opts, err := Executor{URL: "http://api.example.invalid/x", TLSClientCert: string(pki.ClientCertPEM)}.TLSOptions(context.Background())
	require.EqualError(t, err, "tls_client_cert is set without tls_client_key")
	assert.Nil(t, opts)
	assert.Equal(t, []string{unusedTLSWarning}, *warnings)
}

// The warning is given once per step, not once per run, and the request is sent as before.
func TestRunWarnsOncePerStepWithAPlainHTTPURL(t *testing.T) {
	warnings := captureWarnings(t)
	srv := newTestServer(t)
	step := venom.TestStep{"method": "GET", "url": srv.URL + "/echo", "tls_root_ca": string(tlstest.New(t).CAPEM)}

	for range 2 {
		res, ok, err := httpRun(context.Background(), t, step)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	}
	assert.Equal(t, []string{unusedTLSWarning, unusedTLSWarning}, *warnings)
}
