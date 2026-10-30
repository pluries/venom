package tlsconfig

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom/executors/internal/tlsconfig/tlstest"
)

// writeFile writes content to dir/name and returns the path.
func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

func TestReadPEM(t *testing.T) {
	workdir := t.TempDir()
	abs := writeFile(t, workdir, "abs.pem", []byte("from absolute"))
	writeFile(t, workdir, "rel.pem", []byte("from relative"))
	require.NoError(t, os.Mkdir(filepath.Join(workdir, "sub"), 0o700))
	writeFile(t, filepath.Join(workdir, "sub"), "nested.pem", []byte("from nested"))

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "an absolute path is read as is", value: abs, want: "from absolute"},
		{name: "a relative path is read under the workdir", value: "rel.pem", want: "from relative"},
		{name: "a relative path in a sub-folder", value: "sub/nested.pem", want: "from nested"},
		{name: "a missing file is taken as inline content", value: "missing.pem", want: "missing.pem"},
		{name: "a missing absolute file is taken as inline content", value: "/no/such/file.pem", want: "/no/such/file.pem"},
		{name: "PEM content is inline", value: "-----BEGIN CERTIFICATE-----\nAA==\n-----END CERTIFICATE-----\n", want: "-----BEGIN CERTIFICATE-----\nAA==\n-----END CERTIFICATE-----\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadPEM(workdir, tt.value, "TLSRootCA")
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}

	t.Run("a path that exists but cannot be read", func(t *testing.T) {
		got, err := ReadPEM(workdir, "sub", "TLSClientKey")
		require.EqualError(t, err, "unable to read TLSClientKey from file "+filepath.Join(workdir, "sub"))
		assert.Nil(t, got)
	})
}

func TestRootCAs(t *testing.T) {
	pki := tlstest.New(t)

	t.Run("the CA is trusted", func(t *testing.T) {
		pool, err := RootCAs(context.Background(), pki.CAPEM)
		require.NoError(t, err)
		block, _ := pem.Decode(pki.ServerCertPEM)
		require.NotNil(t, block)
		server, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		_, err = server.Verify(x509.VerifyOptions{Roots: pool, DNSName: "127.0.0.1"})
		require.NoError(t, err)
	})

	t.Run("content that is not PEM is refused", func(t *testing.T) {
		pool, err := RootCAs(context.Background(), []byte("garbage"))
		require.ErrorIs(t, err, ErrRootCA)
		assert.EqualError(t, err, "failed to add a certificate to the cert pool")
		assert.Nil(t, pool)
	})
}

func TestClientCertificate(t *testing.T) {
	pki := tlstest.New(t)
	workdir := t.TempDir()
	certAbs := writeFile(t, workdir, "client.pem", pki.ClientCertPEM)
	keyAbs := writeFile(t, workdir, "client.key", pki.ClientKeyPEM)
	require.NoError(t, os.Mkdir(filepath.Join(workdir, "adir"), 0o700))

	t.Run("neither set", func(t *testing.T) {
		cert, err := ClientCertificate(workdir, "", "")
		require.NoError(t, err)
		assert.Nil(t, cert)
	})

	for name, pair := range map[string][2]string{
		"inline":         {string(pki.ClientCertPEM), string(pki.ClientKeyPEM)},
		"relative files": {"client.pem", "client.key"},
		"absolute files": {certAbs, keyAbs},
	} {
		t.Run("a pair, "+name, func(t *testing.T) {
			cert, err := ClientCertificate(workdir, pair[0], pair[1])
			require.NoError(t, err)
			require.NotNil(t, cert)
			require.Len(t, cert.Certificate, 1)
			block, _ := pem.Decode(pki.ClientCertPEM)
			require.NotNil(t, block)
			assert.Equal(t, block.Bytes, cert.Certificate[0])
		})
	}

	tests := []struct {
		name, cert, key, wantErr string
	}{
		{"a certificate without its key", certAbs, "", "tls_client_cert is set without tls_client_key"},
		{"a key without its certificate", "", keyAbs, "tls_client_key is set without tls_client_cert"},
		{"a certificate path that cannot be read", "adir", keyAbs, "unable to read TLSClientCert from file " + filepath.Join(workdir, "adir")},
		{"a key path that cannot be read", certAbs, "adir", "unable to read TLSClientKey from file " + filepath.Join(workdir, "adir")},
		{"a certificate and key that do not match", string(pki.ClientCertPEM), string(pki.ServerKeyPEM), "failed to parse x509 mTLS certificate or key: tls: private key does not match public key"},
		{"a pair that is not PEM", "garbage", "garbage", "failed to parse x509 mTLS certificate or key: tls: failed to find any PEM data in certificate input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := ClientCertificate(workdir, tt.cert, tt.key)
			require.EqualError(t, err, tt.wantErr)
			assert.Nil(t, cert)
		})
	}
}

// handshake connects with client to a fresh server on 127.0.0.1 and returns the common name of the client certificate
// the server received ("" for none). The server writes it once its handshake succeeds; under TLS 1.3 a server that
// refuses the client certificate only says so on the client's first read, which is why the client reads.
func handshake(t *testing.T, server, client *tls.Config) (string, error) {
	t.Helper()
	lis, err := tls.Listen("tcp", "127.0.0.1:0", server)
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := lis.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		tc, ok := conn.(*tls.Conn)
		if !ok || tc.Handshake() != nil {
			return
		}
		cn := ""
		if certs := tc.ConnectionState().PeerCertificates; len(certs) > 0 {
			cn = certs[0].Subject.CommonName
		}
		if _, err := io.WriteString(tc, cn); err != nil {
			return // the client sees it as its read error
		}
	}()
	t.Cleanup(func() {
		lis.Close()
		<-done
	})

	conn, err := tls.Dial("tcp", lis.Addr().String(), client)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	got, err := io.ReadAll(conn)
	return string(got), err
}

func TestNew(t *testing.T) {
	pki := tlstest.New(t)
	workdir := t.TempDir()
	writeFile(t, workdir, "client.pem", pki.ClientCertPEM)
	writeFile(t, workdir, "client.key", pki.ClientKeyPEM)
	ctx := context.Background()

	t.Run("the root CA makes the server certificate trusted", func(t *testing.T) {
		cfg, err := New(ctx, Settings{Workdir: workdir, RootCA: string(pki.CAPEM)})
		require.NoError(t, err)
		assert.False(t, cfg.InsecureSkipVerify)
		assert.Empty(t, cfg.Certificates)
		cn, err := handshake(t, pki.ServerConfig(false), cfg)
		require.NoError(t, err)
		assert.Empty(t, cn)
	})

	t.Run("a root CA that did not sign the server certificate is refused", func(t *testing.T) {
		other := tlstest.New(t)
		cfg, err := New(ctx, Settings{RootCA: string(other.CAPEM)})
		require.NoError(t, err)
		_, err = handshake(t, pki.ServerConfig(false), cfg)
		var unknown x509.UnknownAuthorityError
		require.ErrorAs(t, err, &unknown)
	})

	t.Run("without a root CA the system defaults apply", func(t *testing.T) {
		cfg, err := New(ctx, Settings{})
		require.NoError(t, err)
		assert.Nil(t, cfg.RootCAs)
		_, err = handshake(t, pki.ServerConfig(false), cfg)
		var unknown x509.UnknownAuthorityError
		require.ErrorAs(t, err, &unknown)
	})

	t.Run("the client certificate is sent", func(t *testing.T) {
		for name, s := range map[string]Settings{
			"with the root CA, from files": {Workdir: workdir, RootCA: string(pki.CAPEM), ClientCert: "client.pem", ClientKey: "client.key"},
			// ADR-0009 Precedence 1: skipping the server check does not drop the client certificate.
			"verification skipped, inline": {InsecureSkipVerify: true, ClientCert: string(pki.ClientCertPEM), ClientKey: string(pki.ClientKeyPEM)},
		} {
			t.Run(name, func(t *testing.T) {
				cfg, err := New(ctx, s)
				require.NoError(t, err)
				assert.Equal(t, s.InsecureSkipVerify, cfg.InsecureSkipVerify)
				cn, err := handshake(t, pki.ServerConfig(true), cfg)
				require.NoError(t, err)
				assert.Equal(t, "venom test client", cn)
			})
		}
	})

	t.Run("without a client certificate a mutual TLS server refuses the connection", func(t *testing.T) {
		cfg, err := New(ctx, Settings{InsecureSkipVerify: true})
		require.NoError(t, err)
		_, err = handshake(t, pki.ServerConfig(true), cfg)
		require.Error(t, err)
		// The server's alert, or now and then the reset that follows it.
		assert.Regexp(t, `^(remote error: tls: certificate required|read tcp [0-9.:]+->[0-9.:]+: read: connection reset by peer)$`, err.Error())
	})

	tests := []struct {
		name    string
		s       Settings
		wantErr string
	}{
		{"a certificate without its key", Settings{InsecureSkipVerify: true, ClientCert: string(pki.ClientCertPEM)}, "tls_client_cert is set without tls_client_key"},
		{"a key without its certificate", Settings{RootCA: string(pki.CAPEM), ClientKey: string(pki.ClientKeyPEM)}, "tls_client_key is set without tls_client_cert"},
		{"a root CA that is not PEM", Settings{RootCA: "garbage"}, "TLSRootCA: failed to add a certificate to the cert pool"},
		{"a root CA path that cannot be read", Settings{Workdir: filepath.Dir(workdir), RootCA: filepath.Base(workdir)}, "unable to read TLSRootCA from file " + workdir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := New(ctx, tt.s)
			require.EqualError(t, err, tt.wantErr)
			assert.Nil(t, cfg)
		})
	}

	t.Run("a root CA that is not PEM wraps ErrRootCA", func(t *testing.T) {
		_, err := New(ctx, Settings{RootCA: "garbage"})
		assert.ErrorIs(t, err, ErrRootCA)
	})
}
