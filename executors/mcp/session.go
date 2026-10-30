package mcp

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ovh/venom"
)

// terminateGrace is how long closing a stdio session waits for the server to leave on its own; the
// step context kills it anyway, so the SDK's 5 s default would only slow a step down.
const terminateGrace = time.Second

// maxPendingStderr caps the stderr bytes held while waiting for a newline.
const maxPendingStderr = 64 << 10

// stderrLogFormat is the debug line of a stdio server's stderr.
const stderrLogFormat = "mcp server stderr: %s"

// connect opens the session of the step. Sampling and elicitation handlers stay nil: the client then
// does not advertise those capabilities and a server cannot ask for them. The returned flush is never
// nil; the caller runs it once the session is closed, or when connecting failed, so the last
// unterminated stderr line of a stdio server still reaches the debug log.
func (e Executor) connect(ctx context.Context) (*sdk.ClientSession, func(), error) {
	var transport sdk.Transport
	flush := func() {
		// an HTTP server is no child process: there is no stderr to flush
	}
	if e.Transport == transportStdio {
		var logger *lineLogger
		transport, logger = e.stdioTransport(ctx)
		flush = logger.flush
	} else {
		transport = e.httpTransport()
	}

	client := sdk.NewClient(&sdk.Implementation{Name: "venom", Version: venom.Version}, nil)
	session, err := client.Connect(ctx, transport, &sdk.ClientSessionOptions{ProtocolVersion: e.protocolVersion()})
	return session, flush, err
}

func (e Executor) protocolVersion() string {
	if e.ProtocolVersion != "" {
		return e.ProtocolVersion
	}
	return defaultProtocolVersion
}

// stdioTransport runs the server as a child process in the suite's workdir, with venom's environment.
func (e Executor) stdioTransport(ctx context.Context) (sdk.Transport, *lineLogger) {
	//nolint:gosec // G204: the program and its arguments are the suite author's own command list, started without a shell
	cmd := exec.CommandContext(ctx, e.Command[0], e.Command[1:]...)
	cmd.Dir = venom.StringVarFromCtx(ctx, "venom.testsuite.workdir")
	// A wrapper (npx, uvx, docker run) can leave a grandchild holding stderr open; without a delay Wait
	// would block until that grandchild exits.
	cmd.WaitDelay = terminateGrace
	logger := &lineLogger{ctx: ctx}
	cmd.Stderr = logger
	venom.Debug(ctx, "teststep mcp stdio server '%s'", e.Command[0])
	return &sdk.CommandTransport{Command: cmd, TerminateDuration: terminateGrace}, logger
}

// httpTransport talks to a streamable HTTP server over one connection per step: no standalone event
// stream and no reconnection.
func (e Executor) httpTransport() sdk.Transport {
	return &sdk.StreamableClientTransport{
		Endpoint:             e.URL,
		HTTPClient:           &http.Client{Transport: headerTransport{base: http.DefaultTransport, headers: e.Headers, origin: originOf(e.URL)}},
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}
}

// originOf returns "scheme://host" of rawURL, or "" when it cannot be parsed (the connection then fails
// on its own and no header is ever sent).
func originOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// headerTransport sets the step's headers on a copy of every request to the endpoint's origin, whatever
// the SDK sends. A redirect to another host or scheme gets none: the headers carry credentials, and
// this transport sits below the client's redirect handling, which would not strip them.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
	origin  string
}

func (t headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	if t.origin != "" && strings.EqualFold(req.URL.Scheme+"://"+req.URL.Host, t.origin) {
		for name, value := range t.headers {
			cloned.Header.Set(name, value)
		}
	}
	return t.base.RoundTrip(cloned)
}

// lineLogger sends what the server writes on stderr to the debug log, one line at a time. exec copies
// in one goroutine, but flush runs on another after the copy ended, so the buffer has a lock.
type lineLogger struct {
	ctx     context.Context
	mu      sync.Mutex
	pending []byte
}

func (l *lineLogger) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pending = append(l.pending, p...)
	for {
		end := bytes.IndexByte(l.pending, '\n')
		if end < 0 {
			break
		}
		venom.Debug(l.ctx, stderrLogFormat, string(l.pending[:end]))
		l.pending = l.pending[end+1:]
	}
	if len(l.pending) > maxPendingStderr {
		venom.Debug(l.ctx, stderrLogFormat, string(l.pending))
		l.pending = nil
	}
	return len(p), nil
}

// flush logs a final line the server wrote without a newline.
func (l *lineLogger) flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.pending) > 0 {
		venom.Debug(l.ctx, stderrLogFormat, string(l.pending))
		l.pending = nil
	}
}
