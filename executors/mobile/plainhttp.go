package mobile

import (
	"context"
	"net"
	"net/url"
	"strings"
	"sync"
)

// httpWarning limits the plain HTTP warning to one per run (FR-17); a test re-arms it with resetHTTPWarning.
var httpWarning sync.Once

// resetHTTPWarning arms the plain HTTP warning again.
func resetHTTPWarning() { httpWarning = sync.Once{} }

// warnPlainHTTP warns once per run when mobile.url is plain http to a host that is not this machine and no SSH
// tunnel carries the traffic: the automation server has no password of its own and the traffic is not encrypted
// (FR-17). It prints the host and port only, never a credential: mobile.url may not carry one.
func (c setupConfig) warnPlainHTTP(ctx context.Context) {
	if c.ssh != nil {
		return // the URL then names the SSH host's own loopback
	}
	u, err := url.Parse(c.url)
	if err != nil || u.Scheme != "http" || isLoopbackHost(u.Hostname()) {
		return
	}
	httpWarning.Do(func() {
		warnf(ctx, "mobile.url %s is plain http: the traffic to the automation server is not encrypted and the server has no password of its own; "+
			"use mobile.ssh.* to reach it through an SSH tunnel", u.Host)
	})
}

// isLoopbackHost tells localhost, 127.0.0.0/8 and ::1 from any other host name or address.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
