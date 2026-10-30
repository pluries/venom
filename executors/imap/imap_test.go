package imap

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
	"github.com/yesnault/go-imap/imap"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig/tlstest"
)

func TestMail_containsFlag(t *testing.T) {
	type fields struct {
		UID     uint32
		From    string
		To      string
		Subject string
		Body    string
		Flags   []string
	}
	type args struct {
		flag string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Does not contain flag",
			fields: fields{
				Flags: []string{"Flag1"},
			},
			args: args{flag: "Flag2"},
			want: false,
		},
		{
			name: "Same flags",
			fields: fields{
				Flags: []string{"Flag1"},
			},
			args: args{flag: "Flag1"},
			want: true,
		},
		{
			name: "Contains the flag",
			fields: fields{
				Flags: []string{"Flag1", "Flag2"},
			},
			args: args{flag: "Flag1"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mail{
				UID:     tt.fields.UID,
				From:    tt.fields.From,
				To:      tt.fields.To,
				Subject: tt.fields.Subject,
				Body:    tt.fields.Body,
				Flags:   tt.fields.Flags,
			}
			if got := m.containsFlag(tt.args.flag); got != tt.want {
				t.Errorf("containsFlag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMail_containsFlags(t *testing.T) {
	type fields struct {
		UID     uint32
		From    string
		To      string
		Subject string
		Body    string
		Flags   []string
	}
	type args struct {
		flags []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Does not contain flag",
			fields: fields{
				Flags: []string{"Flag1"},
			},
			args: args{flags: []string{"Flag2"}},
			want: false,
		},
		{
			name: "Same flags",
			fields: fields{
				Flags: []string{"Flag1", "Flag2"},
			},
			args: args{flags: []string{"Flag1", "Flag2"}},
			want: true,
		},
		{
			name: "Contains the flag",
			fields: fields{
				Flags: []string{"Flag1", "Flag2"},
			},
			args: args{flags: []string{"Flag1"}},
			want: true,
		},
		{
			name: "Does not contain one of the flags",
			fields: fields{
				Flags: []string{"Flag1"},
			},
			args: args{flags: []string{"Flag1", "Flag2"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mail{
				UID:     tt.fields.UID,
				From:    tt.fields.From,
				To:      tt.fields.To,
				Subject: tt.fields.Subject,
				Body:    tt.fields.Body,
				Flags:   tt.fields.Flags,
			}
			if got := m.containsFlags(tt.args.flags); got != tt.want {
				t.Errorf("containsFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMail_containsAnyButSeenOrRecent(t *testing.T) {
	type fields struct {
		UID     uint32
		From    string
		To      string
		Subject string
		Body    string
		Flags   []string
	}
	type args struct {
		flags []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Does not contain flag",
			fields: fields{
				Flags: []string{"Flag1"},
			},
			args: args{flags: []string{"Flag2"}},
			want: false,
		},
		{
			name: `\Seen should be ignored`,
			fields: fields{
				Flags: []string{"Flag1", `\Seen`},
			},
			args: args{flags: []string{`\Seen`}},
			want: false,
		},
		{
			name: `\Recent should be ignored`,
			fields: fields{
				Flags: []string{"Flag1", `\Recent`},
			},
			args: args{flags: []string{`\Recent`}},
			want: false,
		},
		{
			name: `\Seen and \Recent should be ignored`,
			fields: fields{
				Flags: []string{"Flag1", `\Seen`, `\Recent`},
			},
			args: args{flags: []string{`\Seen`, `\Recent`}},
			want: false,
		},
		{
			name: "Same flags",
			fields: fields{
				Flags: []string{"Flag1", "Flag2"},
			},
			args: args{flags: []string{"Flag1", "Flag2"}},
			want: true,
		},
		{
			name: "Contains the flag",
			fields: fields{
				Flags: []string{"Flag1", "Flag2"},
			},
			args: args{flags: []string{"Flag1"}},
			want: true,
		},
		{
			name: "Contains one of the flags",
			fields: fields{
				Flags: []string{"Flag1"},
			},
			args: args{flags: []string{"Flag1", "Flag2"}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mail{
				UID:     tt.fields.UID,
				From:    tt.fields.From,
				To:      tt.fields.To,
				Subject: tt.fields.Subject,
				Body:    tt.fields.Body,
				Flags:   tt.fields.Flags,
			}
			if got := m.containsAnyButSeenOrRecent(tt.args.flags); got != tt.want {
				t.Errorf("containsAny() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMail_hasSameFlags(t *testing.T) {
	type fields struct {
		UID     uint32
		From    string
		To      string
		Subject string
		Body    string
		Flags   []string
	}
	type args struct {
		flags []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Does not contain flags",
			fields: fields{
				Flags: []string{"Flag1"},
			},
			args: args{flags: []string{"Flag2"}},
			want: false,
		},
		{
			name: "Same flags ordered",
			fields: fields{
				Flags: []string{"Flag1", "Flag2"},
			},
			args: args{flags: []string{"Flag1", "Flag2"}},
			want: true,
		},
		{
			name: "Same flags unordered",
			fields: fields{
				Flags: []string{"Flag2", "Flag1"},
			},
			args: args{flags: []string{"Flag1", "Flag2"}},
			want: true,
		},
		{
			name: "Contains flag",
			fields: fields{
				Flags: []string{"Flag1", "Flag2"},
			},
			args: args{flags: []string{"Flag1"}},
			want: false,
		},
		{
			name: "Does not contain one of the flags",
			fields: fields{
				Flags: []string{"Flag1"},
			},
			args: args{flags: []string{"Flag1", "Flag2"}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mail{
				UID:     tt.fields.UID,
				From:    tt.fields.From,
				To:      tt.fields.To,
				Subject: tt.fields.Subject,
				Body:    tt.fields.Body,
				Flags:   tt.fields.Flags,
			}
			if got := m.hasSameFlags(tt.args.flags); got != tt.want {
				t.Errorf("hasSameFlags() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Tests for auth.verifytls (#2015) against an in-process IMAP server on a TLS listener that knows just enough of the
// protocol for a greeting and LOGOUT. The certificates are self-signed and generated at test time (nothing is
// committed).
//
// A connection that succeeds is dialled with the executor's own tlsConfig, not through Run. (It used to be because
// connect called c.SetLogMask while go-imap's receiver goroutine, started by DialTLS, read the mask unsynchronised: a
// data race the race detector reported on every successful connect. #2326 fixed it; see TestConnect*DoesNotRace*.)

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

// serverTLS is the server side of a TLS test that presents cert and asks for no client certificate.
func serverTLS(cert tls.Certificate) *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
}

// startIMAP serves IMAP on 127.0.0.1 until the test ends and returns its port: over TLS with cfg, or plaintext when
// cfg is nil. caps is the capability list the greeting announces.
func startIMAP(t *testing.T, cfg *tls.Config, caps string) string {
	t.Helper()
	var lis net.Listener
	var err error
	if cfg == nil {
		lis, err = net.Listen("tcp", "127.0.0.1:0")
	} else {
		lis, err = tls.Listen("tcp", "127.0.0.1:0", cfg)
	}
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(lis.Addr().String())
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
				serveIMAP(conn, caps)
			}()
		}
	}()
	t.Cleanup(func() {
		lis.Close()
		<-done
		// Closing the connections also ends a session the client left open (connect does not close it on error).
		for _, c := range conns {
			c.Close()
		}
		sessions.Wait()
	})
	return port
}

// serveIMAP plays the server side of one session: the greeting, then a reply to each command until LOGOUT.
func serveIMAP(conn net.Conn, caps string) {
	defer conn.Close()
	tp := textproto.NewConn(conn)
	// The first write runs the TLS handshake: it fails here when the client refuses the certificate.
	if err := tp.PrintfLine("* OK [CAPABILITY %s] venom test server ready", caps); err != nil {
		return
	}
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		replies, logout := imapReplies(caps, line)
		for _, reply := range replies {
			if err := tp.PrintfLine("%s", reply); err != nil {
				return
			}
		}
		if logout {
			return
		}
	}
}

// imapReplies returns the server's reply lines to one client command line and whether the session ends after them. A
// test that needs a session that logs in lists X-TEST-LOGIN-OK among the capabilities; otherwise only LOGOUT is known.
func imapReplies(caps, line string) (replies []string, logout bool) {
	tag, command, _ := strings.Cut(line, " ")
	cmd := strings.ToUpper(command)
	loginMode := strings.Contains(caps, "X-TEST-LOGIN-OK")
	switch {
	case loginMode && strings.HasPrefix(cmd, "LOGIN"):
		return []string{tag + " OK LOGIN completed"}, false
	case loginMode && strings.HasPrefix(cmd, "CAPABILITY"):
		// the client asks again after a login that announced no capabilities
		return []string{"* CAPABILITY " + caps, tag + " OK CAPABILITY completed"}, false
	case cmd == "LOGOUT":
		return []string{"* BYE logging out", tag + " OK LOGOUT completed"}, true
	}
	return []string{tag + " BAD unknown command"}, false
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

func TestAuthConfig_tlsConfig(t *testing.T) {
	forLoopback := serverTLS(selfSignedCert(t, "127.0.0.1"))
	forOtherHost := serverTLS(selfSignedCert(t, "venom.test"))
	pki := tlstest.New(t)
	signedByCA := pki.ServerConfig(false)
	workdir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "ca.pem"), pki.CAPEM, 0o600))

	tests := []struct {
		name       string
		auth       AuthConfig
		server     *tls.Config
		wantSkip   bool
		wantErr    string
		wantPrefix bool
	}{
		{name: "unset: a self-signed certificate is accepted", server: forLoopback, wantSkip: true},
		{name: "unset: a certificate for another host is accepted", server: forOtherHost, wantSkip: true},
		{
			name:    "true: a certificate no system root trusts is refused",
			auth:    AuthConfig{VerifyTLS: true},
			server:  forLoopback,
			wantErr: "tls: failed to verify certificate: x509: certificate signed by unknown authority",
		},
		{
			// DialTLS sets ServerName from the address: the host name is checked, not only the chain.
			name:    "true: a certificate for another host is refused",
			auth:    AuthConfig{VerifyTLS: true},
			server:  forOtherHost,
			wantErr: "tls: failed to verify certificate: x509: cannot validate certificate for 127.0.0.1 because it doesn't contain any IP SANs",
		},
		// #2371: auth.tls_root_ca is trusted on top of the system roots.
		{name: "true: the CA that signed the certificate, inline, is trusted", auth: AuthConfig{VerifyTLS: true, TLSRootCA: string(pki.CAPEM)}, server: signedByCA},
		{name: "true: the CA that signed the certificate, as a file of the suite, is trusted", auth: AuthConfig{VerifyTLS: true, TLSRootCA: "ca.pem"}, server: signedByCA},
		{
			// Both test CAs are named "venom test CA", so Go names the candidate whose signature did not match: compare the stable prefix.
			name:       "true: a CA that did not sign the certificate is refused",
			auth:       AuthConfig{VerifyTLS: true, TLSRootCA: string(tlstest.New(t).CAPEM)},
			server:     signedByCA,
			wantErr:    `tls: failed to verify certificate: x509: certificate signed by unknown authority`,
			wantPrefix: true,
		},
		{
			name:     "unset: the client certificate is sent to a server that requires one",
			auth:     AuthConfig{TLSClientCert: string(pki.ClientCertPEM), TLSClientKey: string(pki.ClientKeyPEM)},
			server:   pki.ServerConfig(true),
			wantSkip: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := tt.auth.tlsConfig(context.Background(), workdir)
			require.NoError(t, err)
			assert.Equal(t, tt.wantSkip, cfg.InsecureSkipVerify)
			assert.Empty(t, cfg.ServerName, "left to DialTLS")

			c, err := imap.DialTLS("127.0.0.1:"+startIMAP(t, tt.server, "IMAP4rev1"), cfg)
			if tt.wantErr != "" {
				requireError(t, err, tt.wantErr, tt.wantPrefix)
				return
			}
			require.NoError(t, err)
			_, err = c.Logout(imapClientTimeout)
			require.NoError(t, err)
		})
	}
}

func TestRun_verifyTLS(t *testing.T) {
	venom.InitTestLogger(t)
	forLoopback := serverTLS(selfSignedCert(t, "127.0.0.1"))
	pki := tlstest.New(t)
	workdir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "ca.pem"), pki.CAPEM, 0o600))

	tests := []struct {
		name       string
		server     *tls.Config // forLoopback when nil
		caps       string
		auth       map[string]any
		wantErr    string // the step succeeds when empty
		wantPrefix bool
	}{
		{
			// auth.verifytls reaches connect: without it this server is accepted (TestAuthConfig_tlsConfig).
			name:    "true: a certificate no system root trusts is refused",
			caps:    "IMAP4rev1",
			auth:    map[string]any{"verifytls": true},
			wantErr: "error while connecting: unable to dialTLS: tls: failed to verify certificate: x509: certificate signed by unknown authority",
		},
		{
			// #2043: DialTLS already encrypted the connection, so a server that also lists STARTTLS is used as it is.
			name: "a server announcing STARTTLS over TLS is used without a second negotiation",
			caps: "IMAP4rev1 STARTTLS X-TEST-LOGIN-OK",
		},
		// #2371: auth.tls_root_ca is trusted on top of the system roots; the session logs in and out.
		{
			name:   "true: the CA that signed the certificate, inline, is trusted",
			server: pki.ServerConfig(false),
			caps:   "IMAP4rev1 X-TEST-LOGIN-OK",
			auth:   map[string]any{"verifytls": true, "tls_root_ca": string(pki.CAPEM)},
		},
		{
			name:   "true: the CA that signed the certificate, as a file of the suite, is trusted",
			server: pki.ServerConfig(false),
			caps:   "IMAP4rev1 X-TEST-LOGIN-OK",
			auth:   map[string]any{"verifytls": true, "tls_root_ca": "ca.pem"},
		},
		{
			name:       "true: a CA that did not sign the certificate is refused",
			server:     pki.ServerConfig(false),
			caps:       "IMAP4rev1 X-TEST-LOGIN-OK",
			auth:       map[string]any{"verifytls": true, "tls_root_ca": string(tlstest.New(t).CAPEM)},
			wantErr:    `error while connecting: unable to dialTLS: tls: failed to verify certificate: x509: certificate signed by unknown authority`,
			wantPrefix: true,
		},
		{
			name:    "true: a tls_root_ca that is not PEM fails before the dial",
			server:  pki.ServerConfig(false),
			caps:    "IMAP4rev1 X-TEST-LOGIN-OK",
			auth:    map[string]any{"verifytls": true, "tls_root_ca": "not a certificate"},
			wantErr: "error while connecting: TLSRootCA: failed to add a certificate to the cert pool",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.server
			if server == nil {
				server = forLoopback
			}
			auth := map[string]any{"withtls": true, "host": "127.0.0.1", "port": startIMAP(t, server, tt.caps)}
			for k, v := range tt.auth {
				auth[k] = v
			}

			got, err := Executor{}.Run(workdirCtx(workdir), venom.TestStep{"auth": auth})
			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, Result{Commands: []CommandResult{}}, got)
				return
			}
			requireError(t, err, tt.wantErr, tt.wantPrefix)
			assert.Nil(t, got)
		})
	}
}

func TestRun_stepKeys(t *testing.T) {
	// Run decodes the step with mapstructure. The historical fields carry no mapstructure tag, so a key reaches a field
	// when it equals the Go field name in any case (D115): auth.verifytls does; verify_tls, or verifytls outside auth,
	// do not. The TLS settings of #2371 carry one: auth.tls_root_ca does; tlsRootCA, or tls_root_ca outside auth, do not.
	tests := []struct {
		name string
		step venom.TestStep
		want Executor
	}{
		{
			name: "auth.verifytls",
			step: venom.TestStep{"auth": map[string]any{"withtls": true, "verifytls": true}},
			want: Executor{Auth: AuthConfig{WithTLS: true, VerifyTLS: true}},
		},
		{name: "unset", step: venom.TestStep{"auth": map[string]any{"withtls": true}}, want: Executor{Auth: AuthConfig{WithTLS: true}}},
		{name: "a snake_case key is ignored", step: venom.TestStep{"auth": map[string]any{"verify_tls": true}}, want: Executor{}},
		{name: "verifytls outside auth is ignored", step: venom.TestStep{"verifytls": true}, want: Executor{}},
		{
			name: "auth.tls_root_ca, auth.tls_client_cert and auth.tls_client_key",
			step: venom.TestStep{"auth": map[string]any{"withtls": true, "tls_root_ca": "ca.pem", "tls_client_cert": "client.pem", "tls_client_key": "client.key"}},
			want: Executor{Auth: AuthConfig{WithTLS: true, TLSRootCA: "ca.pem", TLSClientCert: "client.pem", TLSClientKey: "client.key"}},
		},
		{name: "a camelCase TLS key is ignored", step: venom.TestStep{"auth": map[string]any{"tlsRootCA": "ca.pem", "tlsClientCert": "client.pem"}}, want: Executor{}},
		{name: "tls_root_ca outside auth is ignored", step: venom.TestStep{"tls_root_ca": "ca.pem"}, want: Executor{}},
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
	step := func(port string, extra map[string]any) venom.TestStep {
		auth := map[string]any{"withtls": true, "host": "127.0.0.1", "port": port}
		for k, v := range extra {
			auth[k] = v
		}
		return venom.TestStep{"auth": auth}
	}

	t.Run("the client certificate is sent when verifytls is unset", func(t *testing.T) {
		for name, pair := range map[string]map[string]any{
			"inline": {"tls_client_cert": string(pki.ClientCertPEM), "tls_client_key": string(pki.ClientKeyPEM)},
			"files":  {"tls_client_cert": "client.pem", "tls_client_key": "client.key"},
		} {
			t.Run(name, func(t *testing.T) {
				server := pki.ServerConfig(true)
				seen := clientRecorder(server)

				got, err := Executor{}.Run(workdirCtx(workdir), step(startIMAP(t, server, "IMAP4rev1 X-TEST-LOGIN-OK"), pair))
				require.NoError(t, err)
				assert.Equal(t, Result{Commands: []CommandResult{}}, got)
				require.Len(t, seen, 1, "the server saw one client certificate")
				assert.Equal(t, "venom test client", <-seen)
			})
		}
	})

	t.Run("without a client certificate the server refuses the session", func(t *testing.T) {
		got, err := Executor{}.Run(workdirCtx(workdir), step(startIMAP(t, pki.ServerConfig(true), "IMAP4rev1 X-TEST-LOGIN-OK"), nil))
		require.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("a certificate without its key, or the reverse, fails before the dial", func(t *testing.T) {
		// ADR-0009 Precedence 2: half a pair fails the step, with or without withtls.
		for name, tc := range map[string]struct {
			extra map[string]any
			want  string
		}{
			"certificate only":              {map[string]any{"tls_client_cert": "client.pem"}, "tls_client_cert is set without tls_client_key"},
			"key only":                      {map[string]any{"tls_client_key": "client.key"}, "tls_client_key is set without tls_client_cert"},
			"certificate only, withtls off": {map[string]any{"withtls": false, "tls_client_cert": "client.pem"}, "tls_client_cert is set without tls_client_key"},
		} {
			t.Run(name, func(t *testing.T) {
				port, accepted := listenAndCount(t)
				got, err := Executor{}.Run(workdirCtx(workdir), step(port, tc.extra))
				require.EqualError(t, err, "error while connecting: "+tc.want)
				assert.Nil(t, got)
				assert.Zero(t, accepted(), "no connection is attempted")
			})
		}
	})
}

// TestRun_tlsSettingsWithoutWithTLS pins ADR-0009 Precedence 3: TLS settings without auth.withtls are not used, the
// session stays plaintext, and a warning says so.
func TestRun_tlsSettingsWithoutWithTLS(t *testing.T) {
	venom.InitTestLogger(t)
	pki := tlstest.New(t)
	// withSettings adds the three TLS settings to auth.
	withSettings := func(auth map[string]any) map[string]any {
		auth["tls_root_ca"] = string(pki.CAPEM)
		auth["tls_client_cert"] = string(pki.ClientCertPEM)
		auth["tls_client_key"] = string(pki.ClientKeyPEM)
		return auth
	}
	run := func(t *testing.T, server *tls.Config, auth map[string]any) {
		t.Helper()
		auth["host"] = "127.0.0.1"
		auth["port"] = startIMAP(t, server, "IMAP4rev1 X-TEST-LOGIN-OK")
		_, err := Executor{}.Run(context.Background(), venom.TestStep{"auth": auth})
		require.NoError(t, err)
	}

	t.Run("plaintext, with a warning", func(t *testing.T) {
		got := captureWarnings(t)
		run(t, nil, withSettings(map[string]any{"withtls": false}))
		assert.Equal(t, []string{"imap: auth.tls_root_ca, auth.tls_client_cert and auth.tls_client_key are not used without auth.withtls"}, *got)
	})

	t.Run("no warning without TLS settings, nor when withtls uses them", func(t *testing.T) {
		got := captureWarnings(t)
		run(t, nil, map[string]any{"withtls": false})
		run(t, pki.ServerConfig(true), withSettings(map[string]any{"withtls": true, "verifytls": true}))
		assert.Empty(t, *got)
	})
}
