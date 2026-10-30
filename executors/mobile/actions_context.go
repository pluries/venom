package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// webViewPrefix starts the name of every web view context the server lists.
const webViewPrefix = "WEBVIEW"

// contextTarget reads the step's context field: native, a named web view, or empty for the first web view. Any other
// name is a misuse, refused before a request. The native name comes back as nativeContext; the others as written.
func contextTarget(name string) (string, error) {
	switch {
	case strings.EqualFold(name, "native"), strings.EqualFold(name, nativeContext):
		return nativeContext, nil
	case name == "", strings.HasPrefix(name, webViewPrefix):
		return name, nil
	default:
		return "", usagef("context must be native, empty (the first web view) or a name starting with %s, got %q", webViewPrefix, name)
	}
}

// actContext switches the session between the native app and a web view (FR-10). A native switch fetches no list, the
// cheaper answer: result.contexts is filled by the web view switches only, with the last list the server gave.
func actContext(ctx context.Context, s *session, step Executor, result *Result) error {
	target, err := contextTarget(step.Context)
	if err != nil {
		return err
	}
	if target == nativeContext {
		return s.switchContext(ctx, nativeContext, result)
	}
	wait, err := s.waitOf(step)
	if err != nil {
		return err
	}
	name, seen, err := s.awaitWebView(ctx, target, wait)
	result.Contexts = seen
	if err != nil {
		return err
	}
	if name == "" {
		return noWebViewError(target, wait, seen)
	}
	return s.switchContext(ctx, name, result)
}

func noWebViewError(want string, wait time.Duration, seen []string) error {
	if want == "" {
		return fmt.Errorf("no web view within %gs; contexts seen: %v", wait.Seconds(), seen)
	}
	return fmt.Errorf("web view %q not listed within %gs; contexts seen: %v", want, wait.Seconds(), seen)
}

// switchContext sends the switch and records the active context.
func (s *session) switchContext(ctx context.Context, name string, result *Result) error {
	if _, err := s.call(ctx, http.MethodPost, "/context", map[string]string{"name": name}); err != nil {
		return err
	}
	result.Context = name
	return nil
}

// listContexts asks the server for its contexts.
func (s *session) listContexts(ctx context.Context) ([]string, error) {
	raw, err := s.call(ctx, http.MethodGet, "/contexts", nil)
	if err != nil {
		return nil, err
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return nil, errors.New("the automation server answered /contexts with something else than a list")
	}
	return names, nil
}

// pickWebView returns the web view a step wants from a list: the named one, or the first when want is empty. Empty
// means none yet.
func pickWebView(names []string, want string) string {
	for _, n := range names {
		if want == "" && strings.HasPrefix(n, webViewPrefix) || want != "" && n == want {
			return n
		}
	}
	return ""
}

// awaitWebView polls the contexts until the wanted web view is listed or the wait is over: a web view appears some
// time after the page opens. It returns the last list seen; an empty name means the wait ran out. The first attempt is
// immediate.
func (s *session) awaitWebView(ctx context.Context, want string, wait time.Duration) (string, []string, error) {
	deadline := time.Now().Add(wait)
	for {
		names, err := s.listContexts(ctx)
		if err != nil {
			return "", names, err
		}
		if pick := pickWebView(names, want); pick != "" {
			return pick, names, nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return "", names, nil
		}
		if err := pause(ctx, min(pollInterval, remaining)); err != nil {
			return "", names, err
		}
	}
}
