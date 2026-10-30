package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig/tlstest"
)

// Tests for the kafka executor's TLS settings (#2370, ADR-0009). The first two are characterisation tests: they pin the
// step keys that reach the two TLS switches and the configuration getKafkaConfig builds when no TLS setting is given.
// The others handshake the built *tls.Config against a tlstest server: no broker is needed, the PKI is generated at
// test time.

func TestRun_tlsStepKeys(t *testing.T) {
	// Run decodes the step with mapstructure. WithTLS and InsecureTLS carry no mapstructure tag, so their Go name, in any
	// case, reaches them. The snake_case keys the README documents were ignored until #2370 (research.md R1): they now
	// turn the same switches on. The rows check the switches as the built configuration applies them.
	tests := []struct {
		name         string
		step         venom.TestStep
		wantTLS      bool
		wantInsecure bool
	}{
		{name: "no key", step: venom.TestStep{}},
		{name: "withTLS", step: venom.TestStep{"withTLS": true}, wantTLS: true},
		{name: "insecureTLS", step: venom.TestStep{"insecureTLS": true}, wantInsecure: true},
		{name: "with_tls", step: venom.TestStep{"with_tls": true}, wantTLS: true},
		{name: "insecure_tls", step: venom.TestStep{"insecure_tls": true}, wantInsecure: true},
		{name: "both spellings", step: venom.TestStep{"withTLS": true, "with_tls": true, "insecureTLS": true, "insecure_tls": true}, wantTLS: true, wantInsecure: true},
		{name: "one spelling off", step: venom.TestStep{"withTLS": false, "with_tls": true, "insecureTLS": true, "insecure_tls": false}, wantTLS: true, wantInsecure: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e Executor
			require.NoError(t, mapstructure.Decode(tt.step, &e))
			config, err := e.getKafkaConfig(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.wantTLS, config.Net.TLS.Enable, "Net.TLS.Enable")
			assert.Equal(t, tt.wantInsecure, config.Net.TLS.Config.InsecureSkipVerify, "InsecureSkipVerify")
		})
	}
}

func TestGetKafkaConfig_defaultTLS(t *testing.T) {
	config, err := Executor{}.getKafkaConfig(context.Background())
	require.NoError(t, err)
	assert.False(t, config.Net.TLS.Enable)
	require.NotNil(t, config.Net.TLS.Config)
	assert.False(t, config.Net.TLS.Config.InsecureSkipVerify)
	assert.Nil(t, config.Net.TLS.Config.RootCAs)
	assert.Empty(t, config.Net.TLS.Config.Certificates)
}

// workdirCtx is the context a step gets inside a suite whose workdir is dir.
func workdirCtx(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), dir)
}

// writeFile writes content to dir/name.
func writeFile(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), content, 0o600))
}

// serverSaw is what the test server saw of one handshake: its own error and the common name of the client certificate.
type serverSaw struct {
	err      error
	clientCN string
}

// handshake runs one TLS handshake between the client configuration, dialled the way sarama dials a broker (ServerName
// taken from the address), and a server with the server configuration. It returns what the server saw and the client's
// error.
func handshake(t *testing.T, client, server *tls.Config) (serverSaw, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	saw := make(chan serverSaw, 1)
	go serveOneHandshake(ctx, ln, server, saw)

	conn, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	cfg := client.Clone()
	cfg.ServerName = "127.0.0.1"
	clientErr := tls.Client(conn, cfg).HandshakeContext(ctx)
	conn.Close()
	return <-saw, clientErr
}

// serveOneHandshake accepts one connection, runs the server side of the handshake and reports it on saw.
func serveOneHandshake(ctx context.Context, ln net.Listener, server *tls.Config, saw chan<- serverSaw) {
	conn, err := ln.Accept()
	if err != nil {
		saw <- serverSaw{err: err}
		return
	}
	defer conn.Close()
	srv := tls.Server(conn, server)
	s := serverSaw{err: srv.HandshakeContext(ctx)}
	if certs := srv.ConnectionState().PeerCertificates; len(certs) > 0 {
		s.clientCN = certs[0].Subject.CommonName
	}
	saw <- s
}

func TestGetKafkaConfig_tlsHandshake(t *testing.T) {
	pki := tlstest.New(t)
	other := tlstest.New(t)
	ca, cert, key := string(pki.CAPEM), string(pki.ClientCertPEM), string(pki.ClientKeyPEM)
	workdir := t.TempDir()
	writeFile(t, workdir, "ca.pem", pki.CAPEM)
	writeFile(t, workdir, "client.pem", pki.ClientCertPEM)
	writeFile(t, workdir, "client-key.pem", pki.ClientKeyPEM)
	ctx := workdirCtx(workdir)

	const unknownCA = "certificate signed by unknown authority"
	tests := []struct {
		name         string
		e            Executor
		mutual       bool
		wantErr      string
		wantClientCN string
	}{
		{name: "the CA that signed the server certificate", e: Executor{WithTLS: true, TLSRootCA: ca}},
		{name: "with_tls and the CA", e: Executor{WithTLSSnake: true, TLSRootCA: ca}},
		{name: "another CA is refused", e: Executor{WithTLS: true, TLSRootCA: string(other.CAPEM)}, wantErr: unknownCA},
		{name: "no CA verifies against the system pool", e: Executor{WithTLS: true}, wantErr: unknownCA},
		{name: "insecureTLS without a CA", e: Executor{WithTLS: true, InsecureTLS: true}},
		{
			name: "mutual TLS", e: Executor{WithTLS: true, TLSRootCA: ca, TLSClientCert: cert, TLSClientKey: key},
			mutual: true, wantClientCN: "venom test client",
		},
		{
			name: "insecureTLS still sends the client certificate", e: Executor{WithTLS: true, InsecureTLS: true, TLSClientCert: cert, TLSClientKey: key},
			mutual: true, wantClientCN: "venom test client",
		},
		{
			name: "insecure_tls still sends the client certificate", e: Executor{WithTLSSnake: true, InsecureTLSSnake: true, TLSClientCert: cert, TLSClientKey: key},
			mutual: true, wantClientCN: "venom test client",
		},
		{
			name: "paths relative to the workdir", e: Executor{WithTLS: true, TLSRootCA: "ca.pem", TLSClientCert: "client.pem", TLSClientKey: "client-key.pem"},
			mutual: true, wantClientCN: "venom test client",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := tt.e.getKafkaConfig(ctx)
			require.NoError(t, err)
			assert.True(t, config.Net.TLS.Enable)

			saw, err := handshake(t, config.Net.TLS.Config, pki.ServerConfig(tt.mutual))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, saw.err)
			assert.Equal(t, tt.wantClientCN, saw.clientCN)
		})
	}
}

func TestGetKafkaConfig_tlsErrors(t *testing.T) {
	pki := tlstest.New(t)
	workdir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(workdir, "adir"), 0o700))
	cert, key := string(pki.ClientCertPEM), string(pki.ClientKeyPEM)

	tests := []struct {
		name    string
		e       Executor
		wantErr string
	}{
		{name: "a certificate without its key", e: Executor{WithTLS: true, TLSClientCert: cert}, wantErr: "tls_client_cert is set without tls_client_key"},
		{name: "a key without its certificate", e: Executor{WithTLS: true, TLSClientKey: key}, wantErr: "tls_client_key is set without tls_client_cert"},
		{name: "a certificate without its key, TLS off", e: Executor{TLSClientCert: cert}, wantErr: "tls_client_cert is set without tls_client_key"},
		{name: "a key without its certificate, TLS off", e: Executor{TLSClientKey: key}, wantErr: "tls_client_key is set without tls_client_cert"},
		{name: "a root CA that is not PEM", e: Executor{WithTLS: true, TLSRootCA: "garbage"}, wantErr: "TLSRootCA: failed to add a certificate to the cert pool"},
		{
			name:    "a root CA path that cannot be read",
			e:       Executor{WithTLS: true, TLSRootCA: "adir"},
			wantErr: "unable to read TLSRootCA from file " + filepath.Join(workdir, "adir"),
		},
		{
			name:    "a client certificate and key that do not match",
			e:       Executor{WithTLS: true, TLSClientCert: cert, TLSClientKey: string(pki.ServerKeyPEM)},
			wantErr: "failed to parse x509 mTLS certificate or key: tls: private key does not match public key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := tt.e.getKafkaConfig(workdirCtx(workdir))
			require.EqualError(t, err, tt.wantErr)
			assert.Nil(t, config)
		})
	}
}

func TestRun_tlsErrorIsTheStepError(t *testing.T) {
	// A TLS settings error surfaces as result.err, as the other client configuration errors do, before any connection.
	tests := map[string]venom.TestStep{
		"producer": {"clientType": "producer", "messages": []map[string]interface{}{{"topic": "t", "value": "v"}}},
		"consumer": {"clientType": "consumer", "topics": []string{"t"}},
	}
	for name, step := range tests {
		t.Run(name, func(t *testing.T) {
			step["addrs"] = []string{"127.0.0.1:1"}
			step["with_tls"] = true
			step["tls_client_key"] = "a key"
			got, err := Executor{}.Run(context.Background(), step)
			require.NoError(t, err)
			res, ok := got.(Result)
			require.True(t, ok, "Run returns a Result")
			assert.Equal(t, "tls_client_key is set without tls_client_cert", res.Err)
		})
	}
}

func TestGetKafkaConfig_tlsSettingsWithoutTLS(t *testing.T) {
	var warnings []string
	warnf = func(_ context.Context, format string, args ...interface{}) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}
	t.Cleanup(func() { warnf = venom.Warning })

	pki := tlstest.New(t)
	const warning = "kafka: tls_root_ca, tls_client_cert and tls_client_key are not used without withTLS: the connection is plaintext"
	tests := []struct {
		name        string
		e           Executor
		wantWarning []string
	}{
		{name: "no TLS setting", e: Executor{}},
		{name: "insecureTLS alone", e: Executor{InsecureTLS: true}},
		{name: "a root CA", e: Executor{TLSRootCA: string(pki.CAPEM)}, wantWarning: []string{warning}},
		{name: "a root CA that is not PEM is not read", e: Executor{TLSRootCA: "garbage"}, wantWarning: []string{warning}},
		{
			name: "a client pair", e: Executor{InsecureTLS: true, TLSClientCert: string(pki.ClientCertPEM), TLSClientKey: string(pki.ClientKeyPEM)},
			wantWarning: []string{warning},
		},
		{name: "settings with TLS on", e: Executor{WithTLS: true, TLSRootCA: string(pki.CAPEM)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings = nil
			config, err := tt.e.getKafkaConfig(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tt.wantWarning, warnings)
			assert.Equal(t, tt.e.tlsEnabled(), config.Net.TLS.Enable)
			assert.Equal(t, tt.e.InsecureTLS, config.Net.TLS.Config.InsecureSkipVerify)
			if !tt.e.tlsEnabled() {
				assert.Nil(t, config.Net.TLS.Config.RootCAs, "settings are not used without TLS")
				assert.Empty(t, config.Net.TLS.Config.Certificates, "settings are not used without TLS")
			}
		})
	}
}

func TestProducerMessages(t *testing.T) {
	// produceMessages took the context for the TLS settings (#2370); its message building moved to producerMessages
	// and readMessagesFile unchanged, to keep the function under the complexity threshold.
	workdir := t.TempDir()
	writeFile(t, workdir, "messages.json", []byte(`[{"topic":"from-file","key":"k","value":"v"}]`))
	writeFile(t, workdir, "not-json.json", []byte(`{`))
	writeFile(t, workdir, "value.txt", []byte("file value"))
	inline := []Message{{Topic: "inline", Key: "k", Value: "v"}, {Topic: "inline", ValueFile: "value.txt"}}

	t.Run("messages", func(t *testing.T) {
		got, err := Executor{Messages: inline}.producerMessages(workdir)
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "inline", got[0].Topic)
		assert.Equal(t, sarama.ByteEncoder("v"), got[0].Value)
		assert.Equal(t, sarama.ByteEncoder("file value"), got[1].Value)
	})
	t.Run("messages_file wins over messages", func(t *testing.T) {
		got, err := Executor{Messages: inline, MessagesFile: "messages.json"}.producerMessages(workdir)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "from-file", got[0].Topic)
		assert.Equal(t, sarama.ByteEncoder("k"), got[0].Key)
	})
	for name, file := range map[string]string{"a missing messages_file": "missing.json", "a messages_file that is not JSON": "not-json.json"} {
		t.Run(name, func(t *testing.T) {
			got, err := Executor{MessagesFile: file}.producerMessages(workdir)
			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
	t.Run("a value file that cannot be read", func(t *testing.T) {
		_, err := Executor{Messages: []Message{{ValueFile: "missing.txt"}}}.producerMessages(workdir)
		require.ErrorContains(t, err, "can't get value: can't read from")
	})
}
