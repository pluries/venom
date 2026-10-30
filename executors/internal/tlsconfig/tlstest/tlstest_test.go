package tlstest

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseCert decodes one PEM certificate.
func parseCert(t *testing.T, certPEM []byte) *x509.Certificate {
	t.Helper()
	block, rest := pem.Decode(certPEM)
	require.NotNil(t, block)
	assert.Empty(t, rest)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	return cert
}

func TestNew(t *testing.T) {
	p := New(t)

	ca := parseCert(t, p.CAPEM)
	assert.True(t, ca.IsCA)
	assert.Equal(t, "venom test CA", ca.Subject.CommonName)

	t.Run("the server certificate chains to the CA for 127.0.0.1", func(t *testing.T) {
		cert := parseCert(t, p.ServerCertPEM)
		_, err := cert.Verify(x509.VerifyOptions{
			Roots: p.CAPool, DNSName: "127.0.0.1", KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		})
		require.NoError(t, err)
		require.Len(t, p.Server.Certificate, 1)
		assert.Equal(t, cert.Raw, p.Server.Certificate[0])
	})

	t.Run("the client certificate chains to the CA", func(t *testing.T) {
		cert := parseCert(t, p.ClientCertPEM)
		assert.Equal(t, "venom test client", cert.Subject.CommonName)
		_, err := cert.Verify(x509.VerifyOptions{Roots: p.CAPool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
		require.NoError(t, err)
		_, err = tls.X509KeyPair(p.ClientCertPEM, p.ClientKeyPEM)
		require.NoError(t, err)
	})

	t.Run("two PKIs do not trust each other", func(t *testing.T) {
		other := New(t)
		_, err := parseCert(t, other.ServerCertPEM).Verify(x509.VerifyOptions{Roots: p.CAPool, DNSName: "127.0.0.1"})
		var unknown x509.UnknownAuthorityError
		require.ErrorAs(t, err, &unknown)
	})
}

func TestServerConfig(t *testing.T) {
	p := New(t)

	server := p.ServerConfig(false)
	assert.Equal(t, []tls.Certificate{p.Server}, server.Certificates)
	assert.Equal(t, uint16(tls.VersionTLS12), server.MinVersion)
	assert.Equal(t, tls.NoClientCert, server.ClientAuth)
	assert.Nil(t, server.ClientCAs)

	mutual := p.ServerConfig(true)
	assert.Equal(t, tls.RequireAndVerifyClientCert, mutual.ClientAuth)
	assert.Same(t, p.CAPool, mutual.ClientCAs)
}

// handshakeWithin is how long a test waits for the server to report a handshake.
const handshakeWithin = 5 * time.Second

// clientHandshake connects to addr and runs the client side of a handshake with cfg. The client's own outcome is not
// the point: the server's report is.
func clientHandshake(t *testing.T, addr string, cfg *tls.Config) {
	t.Helper()
	conn, err := tls.Dial("tcp", addr, cfg)
	if err == nil {
		conn.Close()
	}
}

// nextHandshake returns the next handshake the server reports, or fails the test after handshakeWithin.
func nextHandshake(t *testing.T, handshakes <-chan Handshake) Handshake {
	t.Helper()
	select {
	case h := <-handshakes:
		return h
	case <-time.After(handshakeWithin):
		t.Fatal("the server reported no handshake")
		return Handshake{}
	}
}

func TestListen(t *testing.T) {
	p := New(t)
	other := New(t)
	clientOf := func(pki PKI) []tls.Certificate {
		pair, err := tls.X509KeyPair(pki.ClientCertPEM, pki.ClientKeyPEM)
		require.NoError(t, err)
		return []tls.Certificate{pair}
	}

	t.Run("a trusted client is reported with its certificate", func(t *testing.T) {
		addr, handshakes := p.Listen(t, true)
		clientHandshake(t, addr, &tls.Config{RootCAs: p.CAPool, Certificates: clientOf(p), MinVersion: tls.VersionTLS12})
		h := nextHandshake(t, handshakes)
		require.NoError(t, h.Err)
		assert.Equal(t, "venom test client", h.ClientCert)
	})

	t.Run("a client without a certificate is reported without one", func(t *testing.T) {
		addr, handshakes := p.Listen(t, false)
		clientHandshake(t, addr, &tls.Config{RootCAs: p.CAPool, MinVersion: tls.VersionTLS12})
		h := nextHandshake(t, handshakes)
		require.NoError(t, h.Err)
		assert.Empty(t, h.ClientCert)
	})

	t.Run("a client certificate of another CA is refused", func(t *testing.T) {
		addr, handshakes := p.Listen(t, true)
		clientHandshake(t, addr, &tls.Config{RootCAs: p.CAPool, Certificates: clientOf(other), MinVersion: tls.VersionTLS12})
		h := nextHandshake(t, handshakes)
		require.Error(t, h.Err)
		assert.Empty(t, h.ClientCert)
	})

	t.Run("a client that does not trust the server aborts the handshake", func(t *testing.T) {
		addr, handshakes := p.Listen(t, false)
		clientHandshake(t, addr, &tls.Config{RootCAs: other.CAPool, MinVersion: tls.VersionTLS12})
		require.Error(t, nextHandshake(t, handshakes).Err)
	})
}
