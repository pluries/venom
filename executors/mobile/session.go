package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ovh/venom"
)

// runFacts are what a report shows about the run: which build ran on which device (FR-19, D1). Setup computes them
// once and every result repeats them.
type runFacts struct {
	platform  string
	device    string
	osVersion string
	app       string
	appSHA256 string
	appID     string
}

func (f runFacts) describe() string {
	return fmt.Sprintf("platform=%s device=%s osversion=%s app=%s appsha256=%s appid=%s",
		f.platform, f.device, f.osVersion, f.app, f.appSHA256, f.appID)
}

// session is one device session, owned by one test case. runMu serialises the steps and the close: the engine's step
// timeout abandons a Run that can still be in flight when TearDown comes.
type session struct {
	id          string
	c           *client
	facts       runFacts
	syncTimeout time.Duration

	runMu  sync.Mutex
	closed bool // guarded by runMu

	// closers are what the session opened besides the device session (the SSH tunnel); close runs them last.
	closers closers

	// base is cancelled by close, to stop a call in flight before close waits for runMu.
	base   context.Context
	cancel context.CancelFunc
}

// sessionAnswer is the value of POST /session.
type sessionAnswer struct {
	SessionID    string                 `json:"sessionId"`
	Capabilities map[string]interface{} `json:"capabilities"`
}

// buildCapabilities is what the session request asks for: the executor's own capabilities, then the user's, which the
// conflict check already kept apart.
func buildCapabilities(cfg setupConfig) map[string]interface{} {
	caps := androidCapabilities(cfg)
	if cfg.platform == platformIOS {
		caps = iosCapabilities(cfg)
	}
	for k, v := range appCapabilities(cfg) {
		caps[k] = v
	}
	for k, v := range cfg.capabilities {
		caps[k] = v
	}
	return caps
}

// openSession opens the SSH tunnel when mobile.ssh.host is set, then the device session through it. It leaves nothing
// open when it fails: a failed Setup has no TearDown.
func openSession(ctx context.Context, cfg setupConfig) (*session, error) {
	var opened closers
	base := cfg.url
	if cfg.ssh != nil {
		t, err := openTunnel(ctx, cfg.ssh, cfg.url, cfg.sessionTimeout)
		if err != nil {
			return nil, err
		}
		opened = append(opened, t.close)
		base = t.baseURL
		if err := t.stageApp(ctx, &cfg, &opened); err != nil {
			return nil, errors.Join(err, opened.run())
		}
	}
	s, err := openSessionOn(ctx, cfg, newClient(base))
	if err != nil {
		return nil, errors.Join(err, opened.run())
	}
	s.closers = opened
	return s, nil
}

// openSessionOn opens the device session on the server c talks to. The session it returns owns no closers yet.
func openSessionOn(ctx context.Context, cfg setupConfig, c *client) (*session, error) {
	caps := buildCapabilities(cfg)
	body := map[string]interface{}{"capabilities": map[string]interface{}{"alwaysMatch": caps}}
	raw, err := c.do(ctx, http.MethodPost, "/session", body, cfg.sessionTimeout)
	if err != nil {
		return nil, fmt.Errorf("cannot open a session on device %q: %w", cfg.device, err)
	}
	var answer sessionAnswer
	if err := json.Unmarshal(raw, &answer); err != nil || answer.SessionID == "" {
		return nil, fmt.Errorf("cannot open a session on device %q: the server answered without a session id", cfg.device)
	}

	base, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s := &session{
		id:          answer.SessionID,
		c:           c,
		syncTimeout: cfg.syncTimeout,
		facts:       factsOf(cfg, answer.Capabilities),
		base:        base,
		cancel:      cancel,
	}
	if got := capabilityString(answer.Capabilities, platformNameKey); got != "" && !strings.EqualFold(got, cfg.platform) {
		closeErr := s.close(ctx)
		return nil, errors.Join(fmt.Errorf("device %q: the server opened a %s session, expected %s", cfg.device, got, cfg.platform), closeErr)
	}
	return s, nil
}

// factsOf prefers the OS version the server reports to the one the suite asked for.
func factsOf(cfg setupConfig, got map[string]interface{}) runFacts {
	version := capabilityString(got, "platformVersion")
	if version == "" {
		version = cfg.osVersion
	}
	return runFacts{
		platform:  cfg.platform,
		device:    cfg.device,
		osVersion: version,
		app:       cfg.app,
		appSHA256: cfg.appSHA256,
		appID:     cfg.appID,
	}
}

// capabilityString reads a capability from the server's answer, with or without the appium: prefix.
func capabilityString(caps map[string]interface{}, name string) string {
	for _, key := range []string{capabilityPrefix + name, name} {
		if v, ok := caps[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// callContext returns the context of one step: cancelled with ctx, or when the session closes.
func (s *session) callContext(ctx context.Context) (context.Context, func()) {
	callCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.base, cancel)
	return callCtx, func() {
		stop()
		cancel()
	}
}

// call sends one command of the session, with the per-call limit.
func (s *session) call(ctx context.Context, method, path string, body interface{}) (json.RawMessage, error) {
	return s.c.do(ctx, method, "/session/"+s.id+path, body, callTimeout)
}

// close ends the device session, then what the session opened besides it (the tunnel). It cancels a call in flight
// first, then waits for the step holding runMu. The DELETE runs on a context the cancellation of the run cannot reach
// (Ctrl-C), bounded by closeTimeout.
func (s *session) close(ctx context.Context) error {
	s.cancel()
	s.runMu.Lock()
	defer s.runMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return errors.Join(s.deleteSession(ctx), s.closers.run())
}

// deleteSession sends DELETE /session on a context the cancellation of the run cannot reach.
func (s *session) deleteSession(ctx context.Context) error {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
	defer cancel()
	_, err := s.c.do(closeCtx, http.MethodDelete, "/session/"+s.id, nil, closeTimeout)
	var failure *serverError
	if errors.As(err, &failure) && failure.Code == "invalid session id" {
		// the server already forgot it (idle timeout): nothing left to close
		return nil
	}
	if err != nil {
		venom.Warn(ctx, "mobile session close failed: %v", err)
		return fmt.Errorf("cannot close the session: %w", err)
	}
	return nil
}
