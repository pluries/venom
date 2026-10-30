// Package tlsconfig is the one place where executors turn their TLS settings (tls_root_ca, tls_client_cert,
// tls_client_key and a switch that skips the server check) into a *tls.Config (ADR-0009). New builds a whole
// configuration; ReadPEM, RootCAs and ClientCertificate are its parts, for an executor that applies them one at a
// time (http).
package tlsconfig

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ovh/venom"
)

// Settings are an executor's TLS settings, as the step wrote them. Each PEM value is a file path (relative to Workdir)
// or the PEM text itself.
type Settings struct {
	// Workdir is the suite's directory: relative paths are read under it.
	Workdir string
	// RootCA is trusted on top of the system pool. Empty leaves the system defaults.
	RootCA string
	// ClientCert and ClientKey are the client certificate and its key: both or neither.
	ClientCert string
	ClientKey  string
	// InsecureSkipVerify skips the server certificate check. The client certificate is still sent.
	InsecureSkipVerify bool
}

// ErrRootCA is returned when a root CA holds no certificate that can be added to the pool.
var ErrRootCA = errors.New("failed to add a certificate to the cert pool")

// New builds the client configuration the settings describe. A lone client certificate or key is an error that
// names the missing setting.
func New(ctx context.Context, s Settings) (*tls.Config, error) {
	cfg := &tls.Config{
		//nolint:gosec // InsecureSkipVerify is a per-step, test-writer-controlled option — the deliberate point of a test tool
		InsecureSkipVerify: s.InsecureSkipVerify,
	}

	if s.RootCA != "" {
		caPEM, err := ReadPEM(s.Workdir, s.RootCA, "TLSRootCA")
		if err != nil {
			return nil, err
		}
		cfg.RootCAs, err = RootCAs(ctx, caPEM)
		if err != nil {
			return nil, fmt.Errorf("TLSRootCA: %w", err)
		}
	}

	cert, err := ClientCertificate(s.Workdir, s.ClientCert, s.ClientKey)
	if err != nil {
		return nil, err
	}
	if cert != nil {
		cfg.Certificates = []tls.Certificate{*cert}
	}
	return cfg, nil
}

// ReadPEM returns the content of the file named by value (relative to workdir) when it exists, else value itself
// taken as PEM content. field names the setting in the read error.
func ReadPEM(workdir, value, field string) ([]byte, error) {
	path := value
	if !filepath.IsAbs(value) {
		path = filepath.Join(workdir, value)
	}
	if _, err := os.Stat(path); err != nil {
		return []byte(value), nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read %s from file %s", field, path)
	}
	return content, nil
}

// RootCAs returns the system pool plus the certificates of caPEM. When the system pool cannot be read, it logs a
// warning and starts from an empty pool. It returns ErrRootCA when caPEM adds no certificate.
func RootCAs(ctx context.Context, caPEM []byte) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		venom.Warning(ctx, "tls: failed to load default system cert pool, fallback to an empty cert pool")
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, ErrRootCA
	}
	return pool, nil
}

// CheckClientPair returns an error that names the missing setting when only one of cert and key is set. It reads
// nothing, so an executor that does not use the pair (grpc without a root CA) still refuses half of one.
func CheckClientPair(cert, key string) error {
	switch {
	case cert != "" && key == "":
		return errors.New("tls_client_cert is set without tls_client_key")
	case cert == "" && key != "":
		return errors.New("tls_client_key is set without tls_client_cert")
	}
	return nil
}

// ClientCertificate returns the client certificate made of cert and key, or nil when neither is set. One without the
// other is an error that names the missing setting.
func ClientCertificate(workdir, cert, key string) (*tls.Certificate, error) {
	if err := CheckClientPair(cert, key); err != nil {
		return nil, err
	}
	if cert == "" {
		return nil, nil
	}

	certPEM, err := ReadPEM(workdir, cert, "TLSClientCert")
	if err != nil {
		return nil, err
	}
	keyPEM, err := ReadPEM(workdir, key, "TLSClientKey")
	if err != nil {
		return nil, err
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse x509 mTLS certificate or key: %w", err)
	}
	return &pair, nil
}
