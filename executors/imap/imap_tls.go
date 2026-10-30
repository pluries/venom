package imap

import (
	"context"
	"crypto/tls"
	"sync"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig"
)

// warnf is venom.Warn; a test replaces it. insecureWarning limits the TLS warning to one per run (#2298).
var (
	warnf           = venom.Warn
	insecureWarning sync.Once
)

// resetInsecureWarning re-arms the warning, for tests only.
func resetInsecureWarning() { insecureWarning = sync.Once{} }

// warnInsecureTLS says once per run that a TLS connection does not verify the server certificate. Behaviour unchanged.
func (a AuthConfig) warnInsecureTLS(ctx context.Context) {
	if a.WithTLS && !a.VerifyTLS {
		insecureWarning.Do(func() {
			warnf(ctx, "imap: the server's TLS certificate is not verified (set auth.verifytls: true to verify it)")
		})
	}
}

// checkTLSSettings refuses half a client pair, whether WithTLS is set or not, and warns when TLS settings are given
// without WithTLS: they are not used and the connection stays plaintext (ADR-0009 Precedence 2 and 3).
func (a AuthConfig) checkTLSSettings(ctx context.Context) error {
	if err := tlsconfig.CheckClientPair(a.TLSClientCert, a.TLSClientKey); err != nil {
		return err
	}
	if !a.WithTLS && (a.TLSRootCA != "" || a.TLSClientCert != "") {
		warnf(ctx, "imap: auth.tls_root_ca, auth.tls_client_cert and auth.tls_client_key are not used without auth.withtls")
	}
	return nil
}

// tlsConfig is the client configuration of a WithTLS connection, built by the shared tlsconfig package (ADR-0009).
// VerifyTLS decides whether the server certificate is checked; the root CA is trusted on top of the system roots and
// the client certificate is sent either way. It leaves ServerName empty: imap.DialTLS sets it to the host of the
// address, so a verified connection checks the host name as well as the chain.
func (a AuthConfig) tlsConfig(ctx context.Context, workdir string) (*tls.Config, error) {
	return tlsconfig.New(ctx, tlsconfig.Settings{
		Workdir:            workdir,
		RootCA:             a.TLSRootCA,
		ClientCert:         a.TLSClientCert,
		ClientKey:          a.TLSClientKey,
		InsecureSkipVerify: !a.VerifyTLS,
	})
}
