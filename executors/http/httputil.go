package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/ovh/venom/executors/internal/tlsconfig"
)

func GetTransport(opts ...func(*http.Transport) error) (*http.Transport, error) {
	// http.DefaultTransport is *http.Transport in every stdlib version this module builds against;
	// the assertion is done in the safe two-value form so a future stdlib change degrades to a fresh
	// transport instead of panicking.
	tr, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		tr = &http.Transport{}
	} else {
		tr = tr.Clone()
	}
	for _, o := range opts {
		if err := o(tr); err != nil {
			return tr, err
		}
	}

	// HTTP/2 is switched on through the transport's own protocol set: http2.ConfigureTransport is deprecated since
	// golang.org/x/net v0.60.0 (#2195). HTTP/1.1 stays on, so a server that does not offer HTTP/2 is still served.
	tr.Protocols = new(http.Protocols)
	tr.Protocols.SetHTTP1(true)
	tr.Protocols.SetHTTP2(true)
	return tr, nil
}

func WithTLSInsecureSkipVerify(v bool) func(*http.Transport) error {
	return func(t *http.Transport) error {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{}
		}

		t.TLSClientConfig.InsecureSkipVerify = v
		return nil
	}
}

func WithTLSClientAuth(cert tls.Certificate) func(*http.Transport) error {
	return func(t *http.Transport) error {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{}
		}

		t.TLSClientConfig.Certificates = append(t.TLSClientConfig.Certificates, cert)
		return nil
	}
}

// WithTLSRootCA should be called only once, with multiple PEM encoded certificates as input if needed.
func WithTLSRootCA(ctx context.Context, caCert []byte) func(*http.Transport) error {
	return func(t *http.Transport) error {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{}
		}
		caCertPool, err := tlsconfig.RootCAs(ctx, caCert)
		if err != nil {
			return fmt.Errorf("WithTLSRootCA: %w", err)
		}

		t.TLSClientConfig.RootCAs = caCertPool
		return nil
	}
}

func WithProxyFromEnv() func(*http.Transport) error {
	return func(t *http.Transport) error {
		t.Proxy = http.ProxyFromEnvironment
		return nil
	}
}
