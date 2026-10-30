// Package tlstest is a throwaway PKI for tests: a CA, a server certificate for 127.0.0.1 and a client certificate,
// generated at run time so that no key material is committed, and a server that records the TLS handshakes it sees. It
// serves the executors' TLS tests (ADR-0009) the way net/http/httptest serves HTTP tests.
package tlstest

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"
)

// PKI is a CA and two certificates it signed: a server certificate for 127.0.0.1 and a client certificate whose
// common name is "venom test client". Every certificate is valid from an hour ago to an hour from now.
type PKI struct {
	// CAPEM is the CA certificate, PEM-encoded: the value of a tls_root_ca setting.
	CAPEM []byte
	// CAPool holds the CA certificate alone.
	CAPool *x509.CertPool
	// Server is the server certificate and its key, ready for a tls.Config.
	Server tls.Certificate
	// ServerCertPEM and ServerKeyPEM are the server certificate and key, PEM-encoded.
	ServerCertPEM, ServerKeyPEM []byte
	// ClientCertPEM and ClientKeyPEM are the client certificate and key, PEM-encoded: the values of tls_client_cert and
	// tls_client_key.
	ClientCertPEM, ClientKeyPEM []byte
}

// New generates a fresh PKI. It stops the test on any error.
func New(t testing.TB) PKI {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("tlstest: generate the CA key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "venom test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("tlstest: create the CA certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("tlstest: parse the CA certificate: %v", err)
	}

	p := PKI{CAPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), CAPool: x509.NewCertPool()}
	p.CAPool.AddCert(caCert)
	p.ServerCertPEM, p.ServerKeyPEM = leaf(t, caCert, caKey, 2, "venom test server", x509.ExtKeyUsageServerAuth, []net.IP{net.IPv4(127, 0, 0, 1)})
	p.ClientCertPEM, p.ClientKeyPEM = leaf(t, caCert, caKey, 3, "venom test client", x509.ExtKeyUsageClientAuth, nil)
	p.Server, err = tls.X509KeyPair(p.ServerCertPEM, p.ServerKeyPEM)
	if err != nil {
		t.Fatalf("tlstest: load the server pair: %v", err)
	}
	return p
}

// leaf returns a certificate signed by the CA, and its key, both PEM-encoded.
func leaf(t testing.TB, ca *x509.Certificate, caKey *ecdsa.PrivateKey, serial int64, name string, usage x509.ExtKeyUsage, ips []net.IP) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("tlstest: generate the key of %q: %v", name, err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("tlstest: create the certificate of %q: %v", name, err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("tlstest: marshal the key of %q: %v", name, err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

// ServerConfig is the server side of a test: the server certificate, TLS 1.2 at least. mutual additionally requires a
// client certificate signed by the CA.
func (p PKI) ServerConfig(mutual bool) *tls.Config {
	cfg := &tls.Config{Certificates: []tls.Certificate{p.Server}, MinVersion: tls.VersionTLS12}
	if mutual {
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		cfg.ClientCAs = p.CAPool
	}
	return cfg
}

// Handshake is what a Listen server saw of one connection.
type Handshake struct {
	// Err is the server's handshake error: nil when the handshake succeeded, set when the client aborted it (it did
	// not trust the server certificate) or the server refused the client certificate.
	Err error
	// ClientCert is the common name of the certificate the client sent, empty when it sent none.
	ClientCert string
}

const (
	// handshakeTimeout bounds one handshake, so that a client that connects and sends nothing does not hold the server.
	handshakeTimeout = 5 * time.Second
	// handshakeBacklog is the number of handshakes Listen keeps for the test to read; later ones are dropped, so a client
	// that retries (kafka, grpc) never blocks the server.
	handshakeBacklog = 64
)

// Listen starts a TLS server on 127.0.0.1 with ServerConfig(mutual) and returns its address and the handshakes it saw,
// as they complete. The server speaks no protocol: it completes the TLS handshake of each connection, reports it, then closes
// the connection, so the client's own protocol fails right after a successful handshake. It stops at the end of the
// test, and the cleanup waits for every goroutine it started.
func (p PKI) Listen(t testing.TB, mutual bool) (addr string, handshakes <-chan Handshake) {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", p.ServerConfig(mutual))
	if err != nil {
		t.Fatalf("tlstest: listen: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	seen := make(chan Handshake, handshakeBacklog)
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Go(func() { record(ctx, conn, seen) })
		}
	})
	t.Cleanup(func() {
		cancel()
		ln.Close()
		wg.Wait()
	})
	return ln.Addr().String(), seen
}

// record completes the handshake of conn, a connection of a tls.Listen listener, reports it without blocking, then
// closes the connection.
func record(ctx context.Context, conn net.Conn, seen chan<- Handshake) {
	defer conn.Close()
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()

	h := Handshake{Err: tlsConn.HandshakeContext(ctx)}
	if certs := tlsConn.ConnectionState().PeerCertificates; h.Err == nil && len(certs) > 0 {
		h.ClientCert = certs[0].Subject.CommonName
	}
	select {
	case seen <- h:
	default:
	}
}
