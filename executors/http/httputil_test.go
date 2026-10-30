package http

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetTransportNegotiatesHTTP2 pins that the transport the http executor builds speaks HTTP/2 to a server that
// offers it, and falls back to HTTP/1.1 when the server does not (#2195: the call that enables it changed with
// golang.org/x/net v0.60.0, which deprecates http2.ConfigureTransport).
func TestGetTransportNegotiatesHTTP2(t *testing.T) {
	hello := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	get := func(t *testing.T, srv *httptest.Server, tr *http.Transport) *http.Response {
		t.Helper()
		resp, err := (&http.Client{Transport: tr}).Get(srv.URL)
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Body.Close() })
		return resp
	}

	h2Server := func(t *testing.T) *httptest.Server {
		t.Helper()
		srv := httptest.NewUnstartedServer(hello)
		srv.EnableHTTP2 = true
		srv.StartTLS()
		t.Cleanup(srv.Close)
		return srv
	}

	t.Run("a TLS server that offers HTTP/2", func(t *testing.T) {
		tr, err := GetTransport(WithTLSInsecureSkipVerify(true))
		require.NoError(t, err)
		assert.Equal(t, 2, get(t, h2Server(t), tr).ProtoMajor)
	})

	t.Run("with a custom dialer set afterwards, as the resolve and unix_sock options do", func(t *testing.T) {
		tr, err := GetTransport(WithTLSInsecureSkipVerify(true))
		require.NoError(t, err)
		tr.DialContext = (&net.Dialer{}).DialContext
		assert.Equal(t, 2, get(t, h2Server(t), tr).ProtoMajor)
	})

	t.Run("a TLS server that only speaks HTTP/1.1", func(t *testing.T) {
		srv := httptest.NewTLSServer(hello)
		t.Cleanup(srv.Close)
		tr, err := GetTransport(WithTLSInsecureSkipVerify(true))
		require.NoError(t, err)
		assert.Equal(t, 1, get(t, srv, tr).ProtoMajor)
	})

	t.Run("a plain HTTP server", func(t *testing.T) {
		srv := httptest.NewServer(hello)
		t.Cleanup(srv.Close)
		tr, err := GetTransport()
		require.NoError(t, err)
		assert.Equal(t, 1, get(t, srv, tr).ProtoMajor)
	})
}
