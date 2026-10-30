package executors_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors"
	"github.com/ovh/venom/executors/internal/tlsconfig/tlstest"
	"github.com/ovh/venom/executors/kafka"
)

const (
	// clientCN is the common name of the tlstest client certificate.
	clientCN = "venom test client"
	// handshakeWithin is how long a case waits for the server to report a handshake.
	handshakeWithin = 5 * time.Second
	// noHandshakeWithin is how long a case watches for a handshake that must not happen. The step has already returned
	// by then, so a connection it opened would already be reported.
	noHandshakeWithin = 200 * time.Millisecond
	// ovhAppKey and ovhAppSigning are a synthetic OVH application key pair: go-ovh refuses a client without one, even
	// for a call that is not signed.
	ovhAppKey     = "app-key-synthetic"
	ovhAppSigning = "app-signing-synthetic"
)

// tlsSettings are the TLS values of one step, as PEM text. An empty value is a setting left out.
type tlsSettings struct {
	rootCA, clientCert, clientKey string
	skipVerify                    bool
}

// capability is what an executor's TLS settings can express beyond a root CA.
type capability int

const (
	// clientCert: the executor sends a client certificate (tls_client_cert and tls_client_key).
	clientCert capability = 1 << iota
	// skipVerify: the executor has a switch that skips the server certificate check.
	skipVerify
)

// tlsExecutor is one executor under test: what it supports and how a step with given settings is written for it.
type tlsExecutor struct {
	name string
	has  capability
	// step returns the step that connects to addr over TLS with s.
	step func(addr string, s tlsSettings) venom.TestStep
	// isolate prepares the test for this executor, when it needs more than a logger.
	isolate func(t *testing.T)
}

// tlsCase is one conformance case. needs lists the capabilities it requires; an executor without them skips it.
type tlsCase struct {
	name     string
	needs    capability
	mutual   bool
	settings func(ca, other tlstest.PKI) tlsSettings
	check    func(t *testing.T, handshakes <-chan tlstest.Handshake, failure string)
}

// TestTLSConformance runs the same TLS cases (ADR-0009) through the real Run of every executor that builds its own TLS
// client configuration: http, grpc, kafka, imap, smtp and ovhapi, each taken from the executors registry.
//
// The server is a tlstest listener: it completes the TLS handshake, records it, then closes the connection. It speaks
// no protocol, so every step fails right after a successful handshake (no HTTP answer, no IMAP greeting, no Kafka
// metadata…). The cases therefore assert the handshake the server saw, never the step result — except "certificate
// without key", where the step must fail before any connection.
func TestTLSConformance(t *testing.T) {
	for _, ex := range tlsExecutors() {
		for _, c := range tlsCases() {
			t.Run(ex.name+"/"+c.name, func(t *testing.T) {
				if ex.has&c.needs != c.needs {
					t.Skipf("%s has no %s", ex.name, missing(c.needs&^ex.has))
				}
				venom.InitTestLogger(t)
				if ex.isolate != nil {
					ex.isolate(t)
				}
				ca, other := tlstest.New(t), tlstest.New(t)
				addr, handshakes := ca.Listen(t, c.mutual)
				ctx := context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), t.TempDir())

				res, err := executors.Registry[ex.name]().Run(ctx, ex.step(addr, c.settings(ca, other)))
				c.check(t, handshakes, stepFailure(res, err))
			})
		}
	}
}

// tlsCases are the four cases of ADR-0009; the trusted CA is checked with and without a client certificate.
func tlsCases() []tlsCase {
	return []tlsCase{
		{
			name:     "trusted CA",
			settings: func(ca, _ tlstest.PKI) tlsSettings { return tlsSettings{rootCA: string(ca.CAPEM)} },
			check:    handshakeSucceeds(""),
		},
		{
			name:   "trusted CA and a client certificate",
			needs:  clientCert,
			mutual: true,
			settings: func(ca, _ tlstest.PKI) tlsSettings {
				return tlsSettings{rootCA: string(ca.CAPEM), clientCert: string(ca.ClientCertPEM), clientKey: string(ca.ClientKeyPEM)}
			},
			check: handshakeSucceeds(clientCN),
		},
		{
			name:     "untrusted CA",
			settings: func(_, other tlstest.PKI) tlsSettings { return tlsSettings{rootCA: string(other.CAPEM)} },
			check:    handshakeFails,
		},
		{
			// The root CA did not sign the server certificate: skipping the check makes it irrelevant (ADR-0009
			// Precedence 1), and grpc needs one to use TLS at all.
			name:   "verification skipped still sends the client certificate",
			needs:  clientCert | skipVerify,
			mutual: true,
			settings: func(ca, other tlstest.PKI) tlsSettings {
				return tlsSettings{rootCA: string(other.CAPEM), clientCert: string(ca.ClientCertPEM), clientKey: string(ca.ClientKeyPEM), skipVerify: true}
			},
			check: handshakeSucceeds(clientCN),
		},
		{
			name:  "certificate without key",
			needs: clientCert,
			settings: func(ca, _ tlstest.PKI) tlsSettings {
				return tlsSettings{rootCA: string(ca.CAPEM), clientCert: string(ca.ClientCertPEM)}
			},
			check: stepFailsBeforeConnecting("tls_client_key"),
		},
	}
}

// tlsExecutors are the executors in scope, each with a step that turns TLS on and keeps the attempt short.
func tlsExecutors() []tlsExecutor {
	return []tlsExecutor{
		{name: "http", has: clientCert | skipVerify, step: httpStep},
		{name: "grpc", has: clientCert | skipVerify, step: grpcStep},
		{name: "kafka", has: clientCert | skipVerify, step: kafkaStep},
		{name: "imap", has: clientCert | skipVerify, step: imapStep},
		{name: "smtp", has: clientCert | skipVerify, step: smtpStep},
		// ovhapi: a root CA only (tlsRootCA or tls_root_ca). The OVH API takes no client certificate and the executor
		// has no switch that skips the check, so the client-certificate and skip-verify cases do not apply.
		{name: "ovhapi", step: ovhapiStep, isolate: isolateOVHConfig},
	}
}

// httpStep: TLS by the https scheme, ignore_verify_ssl skips the check.
func httpStep(addr string, s tlsSettings) venom.TestStep {
	return venom.TestStep{
		"method": "GET", "url": "https://" + addr + "/",
		"tls_root_ca": s.rootCA, "tls_client_cert": s.clientCert, "tls_client_key": s.clientKey, "ignore_verify_ssl": s.skipVerify,
	}
}

// grpcStep: TLS when tls_root_ca is set, ignore_verify_ssl skips the check. connectTimeout (the Go field name: the
// documented connect_timeout does not decode) bounds the dial to one second, should the client keep redialing.
func grpcStep(addr string, s tlsSettings) venom.TestStep {
	return venom.TestStep{
		"url": addr, "service": "venom.Test", "method": "Call", "connectTimeout": 1,
		"tls_root_ca": s.rootCA, "tls_client_cert": s.clientCert, "tls_client_key": s.clientKey, "ignore_verify_ssl": s.skipVerify,
	}
}

// kafkaStep: a producer with withTLS; insecureTLS skips the check. sarama retries the metadata request three times,
// so the server sees up to four handshakes.
func kafkaStep(addr string, s tlsSettings) venom.TestStep {
	return venom.TestStep{
		"addrs": []string{addr}, "clientType": "producer", "withTLS": true, "insecureTLS": s.skipVerify,
		"messages":    []kafka.Message{{Topic: "venom", Value: "conformance"}},
		"tls_root_ca": s.rootCA, "tls_client_cert": s.clientCert, "tls_client_key": s.clientKey,
	}
}

// imapStep: auth.withtls; the check is skipped unless auth.verifytls is set.
func imapStep(addr string, s tlsSettings) venom.TestStep {
	host, port := splitAddr(addr)
	return venom.TestStep{"auth": map[string]any{
		"host": host, "port": port, "withtls": true, "verifytls": !s.skipVerify,
		"tls_root_ca": s.rootCA, "tls_client_cert": s.clientCert, "tls_client_key": s.clientKey,
	}}
}

// smtpStep: withtls; the check is skipped unless verifytls is set.
func smtpStep(addr string, s tlsSettings) venom.TestStep {
	host, port := splitAddr(addr)
	return venom.TestStep{
		"host": host, "port": port, "from": "venom@example.invalid", "to": "conformance@example.invalid",
		"withtls": true, "verifytls": !s.skipVerify,
		"tls_root_ca": s.rootCA, "tls_client_cert": s.clientCert, "tls_client_key": s.clientKey,
	}
}

// ovhapiStep: an https endpoint and tlsRootCA; no signed call, so the first request is the step's own.
func ovhapiStep(addr string, s tlsSettings) venom.TestStep {
	return venom.TestStep{
		"endpoint": "https://" + addr + "/1.0", "applicationKey": ovhAppKey, "applicationSecret": ovhAppSigning, "noAuth": true, "path": "/me",
		"tlsRootCA": s.rootCA,
	}
}

// isolateOVHConfig keeps go-ovh from reading the OVH_* variables or an ./ovh.conf of the machine running the test.
func isolateOVHConfig(t *testing.T) {
	t.Helper()
	for _, name := range []string{"OVH_ENDPOINT", "OVH_APPLICATION_KEY", "OVH_APPLICATION_SECRET", "OVH_CONSUMER_KEY", "OVH_CLIENT_ID", "OVH_CLIENT_SECRET", "OVH_ACCESS_TOKEN"} {
		t.Setenv(name, "")
	}
	t.Chdir(t.TempDir())
}

// splitAddr returns the host and the port of a tlstest address, 127.0.0.1:<port>.
func splitAddr(addr string) (host, port string) {
	host, port, _ = strings.Cut(addr, ":")
	return host, port
}

// missing names the capabilities in c, for a skip message.
func missing(c capability) string {
	var names []string
	if c&clientCert != 0 {
		names = append(names, "client certificate")
	}
	if c&skipVerify != 0 {
		names = append(names, "setting that skips verification")
	}
	return strings.Join(names, " and no ")
}

// stepFailure is the error a step reports: the error Run returns, else the result's err field (kafka reports there).
func stepFailure(res any, err error) string {
	if err != nil {
		return err.Error()
	}
	if r, ok := res.(kafka.Result); ok {
		return r.Err
	}
	return ""
}

// handshakeSucceeds: the server completed a handshake, with a client certificate named cn (none when cn is empty).
func handshakeSucceeds(cn string) func(*testing.T, <-chan tlstest.Handshake, string) {
	return func(t *testing.T, handshakes <-chan tlstest.Handshake, _ string) {
		h := nextHandshake(t, handshakes)
		require.NoError(t, h.Err, "the server saw a failed handshake")
		assert.Equal(t, cn, h.ClientCert, "client certificate the server saw")
	}
}

// handshakeFails: the client aborted the handshake, so the server saw it fail.
func handshakeFails(t *testing.T, handshakes <-chan tlstest.Handshake, _ string) {
	require.Error(t, nextHandshake(t, handshakes).Err, "the server saw a successful handshake")
}

// stepFailsBeforeConnecting: the step failed naming setting, and no handshake reached the server.
func stepFailsBeforeConnecting(setting string) func(*testing.T, <-chan tlstest.Handshake, string) {
	return func(t *testing.T, handshakes <-chan tlstest.Handshake, failure string) {
		assert.Contains(t, failure, setting)
		select {
		case h := <-handshakes:
			t.Errorf("a handshake reached the server: %+v", h)
		case <-time.After(noHandshakeWithin):
		}
	}
}

// nextHandshake returns the first handshake the server reports, or fails the test after handshakeWithin.
func nextHandshake(t *testing.T, handshakes <-chan tlstest.Handshake) tlstest.Handshake {
	t.Helper()
	select {
	case h := <-handshakes:
		return h
	case <-time.After(handshakeWithin):
		t.Fatal("the server reported no handshake")
		return tlstest.Handshake{}
	}
}
