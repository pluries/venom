package imap

import (
	"context"
	"fmt"

	"github.com/yesnault/go-imap/imap"
)

// connect dials host:port, over TLS when WithTLS is set, and logs in. The TLS settings are checked before any dial.
func (e Executor) connect(ctx context.Context, workdir, host, port, imapUsername, imapPassword string) (*imap.Client, error) {
	c, err := e.dial(ctx, workdir, host+":"+port)
	if err != nil {
		return nil, err
	}

	if _, err = imap.Wait(c.Login(imapUsername, imapPassword)); err != nil {
		return nil, fmt.Errorf("unable to login: %w", err)
	}

	return c, nil
}

// dial opens the connection to addr: plaintext, or TLS from the first byte when WithTLS is set.
func (e Executor) dial(ctx context.Context, workdir, addr string) (*imap.Client, error) {
	if err := e.Auth.checkTLSSettings(ctx); err != nil {
		return nil, err
	}
	if !e.Auth.WithTLS {
		c, err := imap.Dial(addr)
		if err != nil {
			return nil, fmt.Errorf("unable to dial: %w", err)
		}
		return c, nil
	}

	cfg, err := e.Auth.tlsConfig(ctx, workdir)
	if err != nil {
		return nil, err
	}
	c, err := imap.DialTLS(addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to dialTLS: %w", err)
	}
	// DialTLS already encrypted the connection: a server that lists STARTTLS anyway is not asked to negotiate again
	// (go-imap refuses with "encryption already enabled"), which used to fail the step and leave the connection open (#2043).
	return c, nil
}
