package mobile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/ovh/venom"
)

// Synthetic fixtures: no real device, no real app.
const (
	testDevice    = "emulator-5554"
	testModel     = "Pixel_Test"
	testSessionID = "sess-test-1"
)

// recorded is one request the fake server received.
type recorded struct {
	Method string
	Path   string // the session id is replaced by {id}
	Body   string
}

// handler answers one route of the fake server.
type handler func(w http.ResponseWriter, r *http.Request, body []byte)

var sessionPath = regexp.MustCompile(`^/session/[^/]+`)

// fakeServer is a scripted automation server: it records every request under a mutex and answers from a table of
// handlers keyed by "METHOD /path", with the session id written {id}.
type fakeServer struct {
	srv  *httptest.Server
	done chan struct{}

	mu       sync.Mutex
	reqs     []recorded
	handlers map[string]handler
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	f := &fakeServer{done: make(chan struct{}), handlers: map[string]handler{}}
	f.on("POST /session", reply(http.StatusOK, `{"sessionId":"`+testSessionID+`","capabilities":{"platformName":"Android","appium:platformVersion":"14"}}`))
	f.on("DELETE /session/{id}", reply(http.StatusOK, `null`))
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(func() {
		close(f.done) // frees the hanging handlers, so Close does not wait for them
		f.srv.Close()
	})
	return f
}

func (f *fakeServer) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		panic(http.ErrAbortHandler) // the documented way to drop a connection from a handler
	}
	route := r.Method + " " + sessionPath.ReplaceAllString(r.URL.Path, "/session/{id}")
	f.mu.Lock()
	f.reqs = append(f.reqs, recorded{Method: r.Method, Path: route[len(r.Method)+1:], Body: string(body)})
	h, ok := f.handlers[route]
	f.mu.Unlock()
	if !ok {
		fail(http.StatusNotFound, "unknown command", "no route "+route)(w, r, body)
		return
	}
	h(w, r, body)
}

// on sets the handler of a route.
func (f *fakeServer) on(route string, h handler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[route] = h
}

// count is the number of requests received on a route.
func (f *fakeServer) count(method, path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.reqs {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

func (f *fakeServer) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

// first returns the body of the first request received on a route.
func (f *fakeServer) first(t *testing.T, method, path string) map[string]interface{} {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.reqs {
		if r.Method == method && r.Path == path {
			var out map[string]interface{}
			if err := json.Unmarshal([]byte(r.Body), &out); err != nil {
				t.Fatalf("body of %s %s: %v", method, path, err)
			}
			return out
		}
	}
	t.Fatalf("no %s %s received", method, path)
	return nil
}

// reply answers with a status and the JSON text of the W3C value.
func reply(status int, value string) handler {
	body := []byte(`{"value":` + value + `}`)
	return func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if _, err := w.Write(body); err != nil {
			panic(http.ErrAbortHandler)
		}
	}
}

// fail answers with a W3C error envelope, stack trace included: the client must drop it.
func fail(status int, code, message string) handler {
	return reply(status, fmt.Sprintf(`{"error":%q,"message":%q,"stacktrace":"at secret.Frame(Trace.java:1)"}`, code, message))
}

// hang never answers, until the client gives up or the test ends.
func (f *fakeServer) hang() handler {
	return func(_ http.ResponseWriter, r *http.Request, _ []byte) {
		select {
		case <-r.Context().Done():
		case <-f.done:
		}
	}
}

// testCtx is a context with the suite variables mobile.<name>.
func testCtx(vars map[string]interface{}) context.Context {
	ctx := context.Background()
	for k, v := range vars {
		ctx = context.WithValue(ctx, venom.ContextKey("var.mobile."+k), v)
	}
	return ctx
}

// baseVars are the minimum settings of a test case against the fake server.
func (f *fakeServer) baseVars() map[string]interface{} {
	return map[string]interface{}{"url": f.srv.URL, "platform": "android", "device": testDevice}
}

// withVars returns baseVars with the given entries added.
func (f *fakeServer) withVars(extra map[string]interface{}) map[string]interface{} {
	vars := f.baseVars()
	for k, v := range extra {
		vars[k] = v
	}
	return vars
}
