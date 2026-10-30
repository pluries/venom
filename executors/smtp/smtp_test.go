package smtp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig/tlstest"
)

// Tests for the verifytls option (#2015) against an in-process SMTP peer on a TLS listener. The certificates are
// self-signed and generated at test time (nothing is committed); all addresses are synthetic.

// selfSignedCert is a throw-away certificate for host (an IP address or a DNS name), signed by its own key: no system
// root trusts it.
func selfSignedCert(t *testing.T, host string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "venom test server"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// fakeSMTP is an SMTP peer serving cert on 127.0.0.1, and the messages it accepted.
type fakeSMTP struct {
	port string

	mu    sync.Mutex
	mails []string
}

// serverTLS is the server side of a TLS test that presents cert and asks for no client certificate.
func serverTLS(cert tls.Certificate) *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
}

// startSMTP serves SMTP until the test ends: over TLS with cfg, or plaintext when cfg is nil.
func startSMTP(t *testing.T, cfg *tls.Config) *fakeSMTP {
	t.Helper()
	var lis net.Listener
	var err error
	if cfg == nil {
		lis, err = net.Listen("tcp", "127.0.0.1:0")
	} else {
		lis, err = tls.Listen("tcp", "127.0.0.1:0", cfg)
	}
	require.NoError(t, err)
	f := &fakeSMTP{}
	_, f.port, err = net.SplitHostPort(lis.Addr().String())
	require.NoError(t, err)

	var conns []net.Conn
	var sessions sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			conns = append(conns, conn)
			sessions.Add(1)
			go func() {
				defer sessions.Done()
				f.serve(conn)
			}()
		}
	}()
	t.Cleanup(func() {
		lis.Close()
		<-done
		for _, c := range conns {
			c.Close()
		}
		sessions.Wait()
	})
	return f
}

// received returns a copy of the messages accepted so far.
func (f *fakeSMTP) received() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.mails...)
}

// serve plays the server side of one session: the greeting, then one reply per command until QUIT.
func (f *fakeSMTP) serve(conn net.Conn) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	// The first write runs the TLS handshake: it fails here when the client refuses the certificate.
	if err := tp.PrintfLine("220 venom.test ESMTP"); err != nil {
		return
	}
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		reply, quit := f.handle(tp, line)
		if err := tp.PrintfLine("%s", reply); err != nil || quit {
			return
		}
	}
}

// handle answers one command; DATA reads and records the message first.
func (f *fakeSMTP) handle(tp *textproto.Conn, line string) (reply string, quit bool) {
	verb, _, _ := strings.Cut(line, " ")
	switch strings.ToUpper(verb) {
	case "EHLO", "HELO", "MAIL", "RCPT":
		return "250 OK", false
	case "DATA":
		if err := tp.PrintfLine("354 end with <CRLF>.<CRLF>"); err != nil {
			return "451 write failed", true
		}
		msg, err := tp.ReadDotBytes()
		if err != nil {
			return "451 read failed", true
		}
		f.mu.Lock()
		f.mails = append(f.mails, string(msg))
		f.mu.Unlock()
		return "250 queued", false
	case "QUIT":
		return "221 bye", true
	default:
		return "502 unknown command", false
	}
}

// requireError checks err against want: the whole text, or only its start when prefix is set. With two test CAs of the
// same name, Go appends which candidate failed its signature check, and that hint is not a contract (#2367 review).
func requireError(t *testing.T, err error, want string, prefix bool) {
	t.Helper()
	if !prefix {
		require.EqualError(t, err, want)
		return
	}
	require.Error(t, err)
	require.True(t, strings.HasPrefix(err.Error(), want), err.Error())
}

func TestRun_verifyTLS(t *testing.T) {
	venom.InitTestLogger(t)
	forLoopback := serverTLS(selfSignedCert(t, "127.0.0.1"))
	forOtherHost := serverTLS(selfSignedCert(t, "venom.test"))
	pki := tlstest.New(t)
	signedByCA := pki.ServerConfig(false)
	workdir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "ca.pem"), pki.CAPEM, 0o600))

	tests := []struct {
		name       string
		server     *tls.Config
		extra      venom.TestStep
		wantErr    string
		wantPrefix bool
	}{
		{name: "unset: a self-signed certificate is accepted", server: forLoopback},
		{name: "false: a self-signed certificate is accepted", server: forLoopback, extra: venom.TestStep{"verifytls": false}},
		{name: "unset: a certificate for another host is accepted", server: forOtherHost},
		{
			name:    "true: a certificate no system root trusts is refused",
			server:  forLoopback,
			extra:   venom.TestStep{"verifytls": true},
			wantErr: "tls dial error: tls: failed to verify certificate: x509: certificate signed by unknown authority",
		},
		{
			name:    "true: a certificate for another host is refused",
			server:  forOtherHost,
			extra:   venom.TestStep{"verifytls": true},
			wantErr: "tls dial error: tls: failed to verify certificate: x509: cannot validate certificate for 127.0.0.1 because it doesn't contain any IP SANs",
		},
		// #2371: tls_root_ca is trusted on top of the system roots.
		{name: "true: the CA that signed the certificate, inline, is trusted", server: signedByCA, extra: venom.TestStep{"verifytls": true, "tls_root_ca": string(pki.CAPEM)}},
		{name: "true: the CA that signed the certificate, as a file of the suite, is trusted", server: signedByCA, extra: venom.TestStep{"verifytls": true, "tls_root_ca": "ca.pem"}},
		{
			// Both test CAs are named "venom test CA", so Go names the candidate whose signature did not match: compare the stable prefix.
			name:       "true: a CA that did not sign the certificate is refused",
			server:     signedByCA,
			extra:      venom.TestStep{"verifytls": true, "tls_root_ca": string(tlstest.New(t).CAPEM)},
			wantErr:    `tls dial error: tls: failed to verify certificate: x509: certificate signed by unknown authority`,
			wantPrefix: true,
		},
		{
			name:    "true: a tls_root_ca that is not PEM fails before the dial",
			server:  signedByCA,
			extra:   venom.TestStep{"verifytls": true, "tls_root_ca": "not a certificate"},
			wantErr: "TLSRootCA: failed to add a certificate to the cert pool",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := startSMTP(t, tt.server)
			step := venom.TestStep{
				"withtls": true, "host": "127.0.0.1", "port": srv.port,
				"from": "venom@example.org", "to": "alice@example.org", "subject": "verifytls", "body": "Hello Alice",
			}
			for k, v := range tt.extra {
				step[k] = v
			}

			got, err := Executor{}.Run(workdirCtx(workdir), step)
			res, ok := got.(Result)
			require.Truef(t, ok, "want a Result, got %T: %#v", got, got)

			if tt.wantErr != "" {
				requireError(t, err, tt.wantErr, tt.wantPrefix)
				assert.Equal(t, Result{Err: err.Error()}, res)
				assert.Empty(t, srv.received(), "no mail reaches a refused server")
				return
			}
			require.NoError(t, err)
			assert.Empty(t, res.Err)
			mails := srv.received()
			require.Len(t, mails, 1)
			assert.Contains(t, mails[0], "Subject: verifytls\n")
			assert.True(t, strings.HasSuffix(mails[0], "\n\nHello Alice\n"), "the body closes the message: %q", mails[0])
		})
	}
}

func TestRun_stepKeys(t *testing.T) {
	// Run decodes the step with mapstructure. The historical fields carry no mapstructure tag, so a key reaches a field
	// when it equals the Go field name in any case (D115): verifytls does, verify_tls does not. The TLS settings of
	// #2371 carry one: tls_root_ca does, tlsRootCA does not.
	tests := []struct {
		name string
		step venom.TestStep
		want Executor
	}{
		{name: "verifytls", step: venom.TestStep{"withtls": true, "verifytls": true}, want: Executor{WithTLS: true, VerifyTLS: true}},
		{name: "unset", step: venom.TestStep{"withtls": true}, want: Executor{WithTLS: true}},
		{name: "a snake_case key is ignored", step: venom.TestStep{"verify_tls": true}, want: Executor{}},
		{
			name: "tls_root_ca, tls_client_cert and tls_client_key",
			step: venom.TestStep{"withtls": true, "tls_root_ca": "ca.pem", "tls_client_cert": "client.pem", "tls_client_key": "client.key"},
			want: Executor{WithTLS: true, TLSRootCA: "ca.pem", TLSClientCert: "client.pem", TLSClientKey: "client.key"},
		},
		{name: "a camelCase TLS key is ignored", step: venom.TestStep{"tlsRootCA": "ca.pem", "tlsClientCert": "client.pem"}, want: Executor{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Executor
			require.NoError(t, mapstructure.Decode(tt.step, &got))
			assert.Equal(t, tt.want, got)
		})
	}
}

// workdirCtx is the context a step gets inside a suite whose workdir is dir.
func workdirCtx(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), dir)
}

// clientRecorder makes cfg report the common name of the client certificate each handshake presents.
func clientRecorder(cfg *tls.Config) <-chan string {
	seen := make(chan string, 10)
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) > 0 {
			seen <- cs.PeerCertificates[0].Subject.CommonName
		}
		return nil
	}
	return seen
}

// listenAndCount accepts connections on 127.0.0.1 and closes them at once, until the test ends; it returns the port
// and the number of connections accepted so far. A step that dials it fails fast, and the count says it dialled.
func listenAndCount(t *testing.T) (string, func() int32) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(lis.Addr().String())
	require.NoError(t, err)
	var accepted atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			conn.Close()
		}
	}()
	t.Cleanup(func() {
		lis.Close()
		<-done
	})
	return port, accepted.Load
}

func TestRun_clientCertificate(t *testing.T) {
	venom.InitTestLogger(t)
	pki := tlstest.New(t)
	workdir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "client.pem"), pki.ClientCertPEM, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "client.key"), pki.ClientKeyPEM, 0o600))

	t.Run("the client certificate is sent when verifytls is unset", func(t *testing.T) {
		for name, pair := range map[string]venom.TestStep{
			"inline": {"tls_client_cert": string(pki.ClientCertPEM), "tls_client_key": string(pki.ClientKeyPEM)},
			"files":  {"tls_client_cert": "client.pem", "tls_client_key": "client.key"},
		} {
			t.Run(name, func(t *testing.T) {
				server := pki.ServerConfig(true)
				seen := clientRecorder(server)
				srv := startSMTP(t, server)

				_, err := Executor{}.Run(workdirCtx(workdir), smtpStep(srv.port, pair))
				require.NoError(t, err)
				assert.Len(t, srv.received(), 1)
				require.Len(t, seen, 1, "the server saw one client certificate")
				assert.Equal(t, "venom test client", <-seen)
			})
		}
	})

	t.Run("without a client certificate the server refuses the session", func(t *testing.T) {
		srv := startSMTP(t, pki.ServerConfig(true))
		_, err := Executor{}.Run(workdirCtx(workdir), smtpStep(srv.port, nil))
		require.Error(t, err)
		assert.Empty(t, srv.received())
	})

	t.Run("a certificate without its key, or the reverse, fails before the dial", func(t *testing.T) {
		// ADR-0009 Precedence 2: half a pair fails the step, with or without withtls.
		for name, tc := range map[string]struct {
			extra venom.TestStep
			want  string
		}{
			"certificate only":              {venom.TestStep{"tls_client_cert": "client.pem"}, "tls_client_cert is set without tls_client_key"},
			"key only":                      {venom.TestStep{"tls_client_key": "client.key"}, "tls_client_key is set without tls_client_cert"},
			"certificate only, withtls off": {venom.TestStep{"withtls": false, "tls_client_cert": "client.pem"}, "tls_client_cert is set without tls_client_key"},
		} {
			t.Run(name, func(t *testing.T) {
				port, accepted := listenAndCount(t)
				got, err := Executor{}.Run(workdirCtx(workdir), smtpStep(port, tc.extra))
				require.EqualError(t, err, tc.want)
				assert.Equal(t, Result{Err: tc.want}, got)
				assert.Zero(t, accepted(), "no connection is attempted")
			})
		}
	})
}

// TestRun_tlsSettingsWithoutWithTLS pins ADR-0009 Precedence 3: TLS settings without withtls are not used, the session
// stays plaintext, and a warning says so.
func TestRun_tlsSettingsWithoutWithTLS(t *testing.T) {
	venom.InitTestLogger(t)
	pki := tlstest.New(t)
	settings := venom.TestStep{
		"withtls": false, "tls_root_ca": string(pki.CAPEM),
		"tls_client_cert": string(pki.ClientCertPEM), "tls_client_key": string(pki.ClientKeyPEM),
	}

	t.Run("plaintext, with a warning", func(t *testing.T) {
		got := captureWarnings(t)
		srv := startSMTP(t, nil)
		_, err := Executor{}.Run(context.Background(), smtpStep(srv.port, settings))
		require.NoError(t, err)
		assert.Len(t, srv.received(), 1)
		assert.Equal(t, []string{"smtp: tls_root_ca, tls_client_cert and tls_client_key are not used without withtls"}, *got)
	})

	t.Run("no warning without TLS settings, nor when withtls uses them", func(t *testing.T) {
		got := captureWarnings(t)
		plain := startSMTP(t, nil)
		_, err := Executor{}.Run(context.Background(), smtpStep(plain.port, venom.TestStep{"withtls": false}))
		require.NoError(t, err)

		withTLS := startSMTP(t, pki.ServerConfig(true))
		_, err = Executor{}.Run(context.Background(), smtpStep(withTLS.port, venom.TestStep{
			"verifytls": true, "tls_root_ca": string(pki.CAPEM),
			"tls_client_cert": string(pki.ClientCertPEM), "tls_client_key": string(pki.ClientKeyPEM),
		}))
		require.NoError(t, err)
		assert.Empty(t, *got)
	})
}
