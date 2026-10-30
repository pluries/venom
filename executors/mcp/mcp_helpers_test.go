package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// bearerValue is a synthetic credential, generated for each test run: it is never a real one, and a value no
// earlier run printed cannot show up in a log or a report by accident.
var bearerValue = syntheticSecret()

func syntheticSecret() string {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return "synthetic-" + hex.EncodeToString(raw)
}

// Fixtures for the tests in mcp_test.go. Three layers, because Run builds its own transport:
//   - an SDK server over mcp.NewInMemoryTransports for the method and mapping functions;
//   - an httptest server wrapping mcp.NewStreamableHTTPHandler for the http transport;
//   - this test binary, re-executed by TestMain as an MCP server on stdin/stdout, for the stdio transport.

const (
	// envMode selects what the re-executed test binary does; unset means "run the tests".
	envMode = "VENOM_MCP_TEST_MODE"
	// envPidFile is where the child writes its pid, so a test can check it is gone.
	envPidFile = "VENOM_MCP_TEST_PIDFILE"
	// envStderrText is the line the child writes on stderr in mode childStderr.
	envStderrText = "VENOM_MCP_TEST_STDERR"

	childEcho   = "echo"
	childHang   = "hang"
	childStderr = "stderr"

	fixtureServerName    = "fixture-server"
	fixtureServerVersion = "9.9.9"

	// shortDeadline is the context deadline of the "server never answers" cases.
	shortDeadline = 3 * time.Second
	// stepDeadline bounds every other step, so a regression fails the test instead of hanging it.
	stepDeadline = 30 * time.Second
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(envMode); mode != "" {
		os.Exit(serveChild(mode))
	}
	// Run logs through venom's logger, which must exist; captureLogs swaps it for one tests can read.
	venom.InitTestLogger(nil)
	os.Exit(m.Run())
}

// serveChild is the whole life of the re-executed binary: the fixture server on stdio, or a process that never answers.
func serveChild(mode string) int {
	if path := os.Getenv(envPidFile); path != "" {
		//nolint:gosec // G703: the path is the parent test's own temp file, handed over through the environment
		if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			return 2
		}
	}
	switch mode {
	case childHang:
		// Never answers; ends when its parent closes stdin or kills it.
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			return 1
		}
		return 0
	case childStderr:
		fmt.Fprintln(os.Stderr, os.Getenv(envStderrText))
	}
	fx := newFixtureServer(nil)
	if err := fx.server.Run(context.Background(), &sdk.StdioTransport{}); err != nil {
		return 1
	}
	return 0
}

type echoInput struct {
	Message string `json:"message"`
}

type echoOutput struct {
	Echo   string `json:"echo"`
	Length int    `json:"length"`
}

// fixtureServer is an SDK server with the tools the tests call, and the counters they assert on.
type fixtureServer struct {
	server *sdk.Server

	mu      sync.Mutex
	started int
	ended   int
	headers []http.Header
}

// newFixtureServer builds the server. A tool call to "hang" waits for release (or its own cancellation).
func newFixtureServer(release <-chan struct{}) *fixtureServer {
	return newFixtureServerOptions(release, nil)
}

func newFixtureServerOptions(release <-chan struct{}, opts *sdk.ServerOptions) *fixtureServer {
	fx := &fixtureServer{}
	if opts == nil {
		opts = &sdk.ServerOptions{}
	}
	opts.InitializedHandler = func(context.Context, *sdk.InitializedRequest) { fx.count(&fx.started) }
	fx.server = sdk.NewServer(&sdk.Implementation{Name: fixtureServerName, Version: fixtureServerVersion}, opts)
	sdk.AddTool(fx.server, &sdk.Tool{Name: "echo", Description: "Echoes the message"}, echoTool)
	fx.server.AddTool(&sdk.Tool{Name: "fail", InputSchema: emptyObjectSchema()}, failTool)
	fx.server.AddTool(&sdk.Tool{Name: "hang", InputSchema: emptyObjectSchema()}, hangTool(release))
	fx.server.AddTool(&sdk.Tool{Name: "capabilities", InputSchema: emptyObjectSchema()}, capabilitiesTool)
	addResources(fx.server)
	addPrompts(fx.server)
	return fx
}

const (
	uriText       = "venom://text/hello"
	uriSecondText = "venom://text/second"
	uriBlob       = "venom://blob/bytes"
)

// addResources registers two text resources and one blob resource; the uri not served by the handler is the SDK's not found.
func addResources(server *sdk.Server) {
	server.AddResource(&sdk.Resource{URI: uriText, Name: "hello", MIMEType: "text/plain", Description: "A greeting"}, readFixtureResource)
	server.AddResource(&sdk.Resource{URI: uriSecondText, Name: "second", MIMEType: "text/plain"}, readFixtureResource)
	server.AddResource(&sdk.Resource{URI: uriBlob, Name: "bytes", MIMEType: "application/octet-stream"}, readFixtureResource)
}

func readFixtureResource(_ context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	uri := req.Params.URI
	content := &sdk.ResourceContents{URI: uri, MIMEType: "text/plain"}
	switch uri {
	case uriText:
		content.Text = "hello"
	case uriSecondText:
		content.Text = "second"
	case uriBlob:
		content.MIMEType = "application/octet-stream"
		content.Blob = []byte{1, 2, 3}
	default:
		return nil, sdk.ResourceNotFoundError(uri)
	}
	return &sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{content}}, nil
}

// addPrompts registers greet (one required argument, echoed in its message) and plain (no argument).
func addPrompts(server *sdk.Server) {
	greet := &sdk.Prompt{
		Name: "greet", Description: "Greets someone",
		Arguments: []*sdk.PromptArgument{{Name: "who", Description: "Who to greet", Required: true}},
	}
	server.AddPrompt(greet, greetPrompt)
	server.AddPrompt(&sdk.Prompt{Name: "plain", Description: "Takes nothing"}, plainPrompt)
}

func greetPrompt(_ context.Context, req *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
	who, ok := req.Params.Arguments["who"]
	if !ok {
		return nil, fmt.Errorf("missing required argument %q", "who")
	}
	text := "hello " + who
	return &sdk.GetPromptResult{
		Description: "A greeting",
		Messages:    []*sdk.PromptMessage{{Role: "user", Content: &sdk.TextContent{Text: text}}},
	}, nil
}

func plainPrompt(context.Context, *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
	return &sdk.GetPromptResult{Messages: []*sdk.PromptMessage{{Role: "assistant", Content: &sdk.TextContent{Text: "plain"}}}}, nil
}

func emptyObjectSchema() map[string]any {
	return map[string]any{"type": "object"}
}

func echoTool(_ context.Context, _ *sdk.CallToolRequest, in echoInput) (*sdk.CallToolResult, echoOutput, error) {
	out := echoOutput{Echo: in.Message, Length: len(in.Message)}
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "echo: " + in.Message}}}, out, nil
}

func failTool(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
	return &sdk.CallToolResult{
		IsError: true,
		Content: []sdk.Content{&sdk.TextContent{Text: "backend is down"}},
	}, nil
}

func hangTool(release <-chan struct{}) sdk.ToolHandler {
	return func(ctx context.Context, _ *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		select {
		case <-ctx.Done():
		case <-release:
		}
		return nil, ctx.Err()
	}
}

// capabilitiesTool reports which optional client capabilities the server was told about.
func capabilitiesTool(_ context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
	raw, err := json.Marshal(req.Session.InitializeParams().Capabilities)
	if err != nil {
		return nil, err
	}
	var caps map[string]any
	if err := json.Unmarshal(raw, &caps); err != nil {
		return nil, err
	}
	_, sampling := caps["sampling"]
	_, elicitation := caps["elicitation"]
	text := fmt.Sprintf("sampling=%t elicitation=%t", sampling, elicitation)
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: text}}}, nil
}

func (f *fixtureServer) count(counter *int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	*counter++
}

// sessions returns how many sessions were initialised and how many the client ended with a DELETE.
func (f *fixtureServer) sessions() (started, ended int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started, f.ended
}

// seenHeaders returns a copy of the headers of every HTTP request the server received.
func (f *fixtureServer) seenHeaders() []http.Header {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]http.Header(nil), f.headers...)
}

// record notes the headers of a request and counts a session end.
func (f *fixtureServer) record(req *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.headers = append(f.headers, req.Header.Clone())
	if req.Method == http.MethodDelete {
		f.ended++
	}
}

// startHTTP serves the fixture over streamable HTTP on loopback and returns its MCP endpoint. The hang tool is
// released, then the server closed, when the test ends.
func startHTTP(t *testing.T) (*fixtureServer, string) {
	t.Helper()
	release := make(chan struct{})
	fx := newFixtureServer(release)
	handler := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return fx.server }, nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fx.record(r)
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	return fx, srv.URL
}

// memorySession connects an SDK client to a fixture server over an in-memory pipe.
func memorySession(t *testing.T, opts *sdk.ServerOptions) *sdk.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), stepDeadline)
	t.Cleanup(cancel)

	fx := newFixtureServerOptions(nil, opts)
	clientSide, serverSide := sdk.NewInMemoryTransports()
	serverSession, err := fx.server.Connect(ctx, serverSide, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := sdk.NewClient(&sdk.Implementation{Name: "venom-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientSide, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// stepContext returns a context that ends after d, cancelled with the test.
func stepContext(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

// withSecrets declares values as the engine's `secrets:` do: through the context.
func withSecrets(ctx context.Context, secrets ...string) context.Context {
	return context.WithValue(ctx, venom.ContextKey("secrets"), secrets)
}

// runStep runs a step through Executor.Run, requires no Go error, and returns the typed Result and its dump, which is
// what the assertions of a suite read.
func runStep(ctx context.Context, t *testing.T, step venom.TestStep) (Result, map[string]interface{}) {
	t.Helper()
	raw, err := Executor{}.Run(ctx, step)
	require.NoError(t, err)
	require.IsType(t, Result{}, raw)
	result, ok := raw.(Result)
	require.True(t, ok)
	return result, venom.GetExecutorResult(raw)
}

// runMisuse runs a step that must be refused with a Go error.
func runMisuse(t *testing.T, step venom.TestStep) error {
	t.Helper()
	raw, err := Executor{}.Run(stepContext(t, stepDeadline), step)
	require.Nil(t, raw)
	require.Error(t, err)
	return err
}

func httpStep(url, method string) venom.TestStep {
	return venom.TestStep{"type": Name, "transport": transportHTTP, "url": url, "method": method}
}

func toolStep(url, tool string, arguments map[string]interface{}) venom.TestStep {
	step := httpStep(url, methodToolsCall)
	step["tool"] = tool
	step["arguments"] = arguments
	return step
}

func resourceStep(url, method, uri string) venom.TestStep {
	step := httpStep(url, method)
	step["uri"] = uri
	return step
}

func promptStep(url, prompt string, arguments map[string]interface{}) venom.TestStep {
	step := httpStep(url, methodPromptsGet)
	step["prompt"] = prompt
	step["arguments"] = arguments
	return step
}

// stdioStep runs the test binary itself as the server, in the given mode, and returns the pid file the child writes.
func stdioStep(t *testing.T, mode, method string) (venom.TestStep, string) {
	t.Helper()
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	t.Setenv(envMode, mode)
	t.Setenv(envPidFile, pidFile)
	return venom.TestStep{"type": Name, "transport": transportStdio, "command": []string{os.Args[0]}, "method": method}, pidFile
}

// requireProcessGone reads the pid the child wrote and requires that no such process is left.
func requireProcessGone(t *testing.T, pidFile string) {
	t.Helper()
	raw, err := os.ReadFile(pidFile)
	require.NoError(t, err, "the child never wrote its pid")
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	require.NoError(t, err)
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	require.Error(t, process.Signal(syscall.Signal(0)), "child process %d is still running", pid)
}

// capturedLogs collects every entry venom logs, at every level. It points venom's logger at logrus' standard logger
// at Debug level with a hook and undoes that when the test ends.
type capturedLogs struct {
	hook *logtest.Hook
}

func captureLogs(t *testing.T) *capturedLogs {
	t.Helper()
	v := venom.New()
	v.Verbose = 2
	v.OutputDir = t.TempDir()
	require.NoError(t, v.InitLogger())
	hook := logtest.NewGlobal()
	t.Cleanup(func() {
		if closer, ok := v.LogOutput.(io.Closer); ok {
			_ = closer.Close()
		}
		std := logrus.StandardLogger()
		std.ReplaceHooks(make(logrus.LevelHooks))
		std.SetOutput(os.Stderr)
		std.SetLevel(logrus.InfoLevel)
		std.SetFormatter(&logrus.TextFormatter{})
		venom.InitTestLogger(t)
	})
	return &capturedLogs{hook: hook}
}

// entryText renders an entry the way a reader of the log file would see it: message and fields.
func entryText(e *logrus.Entry) string {
	return e.Message + " " + fmt.Sprint(e.Data)
}

// above returns the text of the entries more severe than Debug.
func (c *capturedLogs) above() []string {
	var out []string
	for _, e := range c.hook.AllEntries() {
		if e.Level < logrus.DebugLevel {
			out = append(out, entryText(e))
		}
	}
	return out
}

// all returns the text of every entry.
func (c *capturedLogs) all() []string {
	var out []string
	for _, e := range c.hook.AllEntries() {
		out = append(out, entryText(e))
	}
	return out
}

// dumpText is the JSON text of a Result dump, for "this value appears nowhere" checks.
func dumpText(t *testing.T, dump map[string]interface{}) string {
	t.Helper()
	raw, err := json.Marshal(dump)
	require.NoError(t, err)
	return string(raw)
}
