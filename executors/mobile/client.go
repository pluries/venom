package mobile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ovh/venom"
)

// Limits of a call to the automation server. Package variables so that tests can shorten them (FR-16).
var (
	// callTimeout bounds every call but the opening of a session.
	callTimeout = 60 * time.Second
	// sessionOpenTimeout bounds POST /session unless mobile.sessionTimeout says otherwise.
	sessionOpenTimeout = 180 * time.Second
	// iosSessionOpenTimeout is that default on iOS: the first session builds WebDriverAgent.
	iosSessionOpenTimeout = 300 * time.Second
	// closeTimeout bounds DELETE /session, which runs on a context the step's cancellation cannot reach.
	closeTimeout = 15 * time.Second
	// maxResponseBytes caps what is read from an answer (T8): a screenshot is the largest.
	maxResponseBytes int64 = 16 << 20
	// maxMessageRunes truncates a server message that reaches a step failure; a stack trace never does.
	maxMessageRunes = 500
)

// serverError is the error envelope of a W3C WebDriver answer: its code and message, never its stack trace.
type serverError struct {
	Code    string
	Message string
}

func (e *serverError) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

// client speaks W3C WebDriver with the Appium extensions to one server, over the standard library.
type client struct {
	base string
	hc   *http.Client
}

func newClient(base string) *client {
	return &client{
		base: base,
		hc: &http.Client{
			// a redirect is never followed: the server is the one the suite named (T-25)
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// do sends one request and returns the value of the answer. The call gives up after limit.
func (c *client) do(ctx context.Context, method, path string, body interface{}, limit time.Duration) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()

	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("cannot encode the request: %w", err)
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, payload)
	if err != nil {
		return nil, fmt.Errorf("cannot build the request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// method, path and duration only: a body can hold typed text (NFR-09)
	start := time.Now()
	defer func() {
		venom.Info(ctx, "mobile %s %s %dms", method, path, time.Since(start).Milliseconds())
	}()

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, transportError(ctx, limit, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, transportError(ctx, limit, err)
	}
	if int64(len(raw)) > maxResponseBytes {
		return nil, fmt.Errorf("answer larger than %d bytes", maxResponseBytes)
	}
	return decode(resp.StatusCode, raw)
}

// transportError describes a failure to talk to the server without the request URL, which could carry what the suite
// put in mobile.url.
func transportError(ctx context.Context, limit time.Duration, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("no answer from the automation server within %s", limit)
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return errors.New("call cancelled")
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	return fmt.Errorf("automation server unreachable: %w", err)
}

// envelope is the W3C answer: everything is in "value".
type envelope struct {
	Value json.RawMessage `json:"value"`
}

type errorValue struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// decode turns an HTTP answer into its value or a *serverError.
func decode(status int, raw []byte) (json.RawMessage, error) {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil || env.Value == nil {
		if status >= http.StatusBadRequest || status < http.StatusOK || status >= http.StatusMultipleChoices {
			return nil, &serverError{Code: fmt.Sprintf("http %d", status)}
		}
		return nil, errors.New("the automation server answered without a W3C value")
	}
	var failure errorValue
	if json.Unmarshal(env.Value, &failure) == nil && failure.Error != "" {
		return nil, &serverError{Code: failure.Error, Message: truncate(failure.Message)}
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, &serverError{Code: fmt.Sprintf("http %d", status)}
	}
	return env.Value, nil
}

func truncate(s string) string {
	runes := []rune(s)
	if len(runes) <= maxMessageRunes {
		return s
	}
	return string(runes[:maxMessageRunes]) + "..."
}
