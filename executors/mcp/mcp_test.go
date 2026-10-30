package mcp

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Unit tests of the mcp executor (#3018). Names carry the id they prove (docs/spec/mcp-executor.md); the fixtures are in
// mcp_helpers_test.go.

func echoArguments(message string) map[string]interface{} {
	return map[string]interface{}{"message": message}
}

func TestFR01_DefaultAssertions(t *testing.T) {
	got := Executor{}.GetDefaultAssertions()
	require.NotNil(t, got)
	assert.Equal(t, []venom.Assertion{"result.err ShouldBeEmpty"}, got.Assertions)
}

func TestFR08_ZeroValueResultKeys(t *testing.T) {
	dump := venom.GetExecutorResult(Executor{}.ZeroValueResult())
	for _, key := range []string{"result.iserror", "result.err", "result.timeseconds"} {
		assert.Contains(t, dump, key)
	}
	assert.Equal(t, false, dump["result.iserror"])
	assert.Equal(t, "", dump["result.err"])
}

func TestFR02_StdioEchoAndChildGone(t *testing.T) {
	step, pidFile := stdioStep(t, childEcho, methodToolsCall)
	step["tool"] = "echo"
	step["arguments"] = echoArguments("over stdio")

	res, _ := runStep(stepContext(t, stepDeadline), t, step)

	assert.Empty(t, res.Err)
	assert.Equal(t, "echo: over stdio", res.Text)
	assert.Equal(t, fixtureServerName, res.ServerInfo.Name)
	requireProcessGone(t, pidFile)
}

func TestFR03_HeaderReachesServerOnEveryRequest(t *testing.T) {
	fx, url := startHTTP(t)
	step := toolStep(url, "echo", echoArguments("hi"))
	step["headers"] = map[string]interface{}{"X-Venom-Test": "present", "Authorization": "Bearer " + bearerValue}

	res, _ := runStep(stepContext(t, stepDeadline), t, step)

	assert.Empty(t, res.Err)
	seen := fx.seenHeaders()
	require.GreaterOrEqual(t, len(seen), 3, "initialize, initialized notification, tools/call")
	for i, header := range seen {
		assert.Equal(t, "present", header.Get("X-Venom-Test"), "request %d", i)
		assert.Equal(t, "Bearer "+bearerValue, header.Get("Authorization"), "request %d", i)
	}
}

func TestFR04_Ping(t *testing.T) {
	_, url := startHTTP(t)
	res, _ := runStep(stepContext(t, stepDeadline), t, httpStep(url, methodPing))
	assert.Empty(t, res.Err)
	assert.False(t, res.IsError)
}

func TestFR04_Initialize(t *testing.T) {
	_, url := startHTTP(t)
	res, _ := runStep(stepContext(t, stepDeadline), t, httpStep(url, methodInitialize))
	assert.Empty(t, res.Err)
	assert.Equal(t, fixtureServerName, res.ServerInfo.Name)
}

func TestFR04_ToolsList(t *testing.T) {
	_, url := startHTTP(t)
	res, _ := runStep(stepContext(t, stepDeadline), t, httpStep(url, methodToolsList))
	assert.Empty(t, res.Err)
	assert.NotEmpty(t, res.Tools)
}

func TestFR04_ToolsCall(t *testing.T) {
	_, url := startHTTP(t)
	res, _ := runStep(stepContext(t, stepDeadline), t, toolStep(url, "echo", echoArguments("x")))
	assert.Empty(t, res.Err)
	assert.Equal(t, "echo: x", res.Text)
}

func TestFR04_RunPingOnSession(t *testing.T) {
	res, err := runPing(stepContext(t, stepDeadline), Executor{}, memorySession(t, nil))
	require.NoError(t, err)
	assert.Equal(t, Result{}, res)
}

func TestFR04_ToolsListFollowsPagination(t *testing.T) {
	session := memorySession(t, &sdk.ServerOptions{PageSize: 1})

	res, err := runToolsList(stepContext(t, stepDeadline), Executor{}, session)

	require.NoError(t, err)
	tools, ok := res.ToolsJSON.([]interface{})
	require.True(t, ok)
	assert.Len(t, tools, 4, "echo, fail, hang and capabilities, served one per page")
}

func TestFR04_RunToolsCallOnSession(t *testing.T) {
	e := Executor{Tool: "echo", Arguments: echoArguments("direct")}
	res, err := runToolsCall(stepContext(t, stepDeadline), e, memorySession(t, nil))
	require.NoError(t, err)
	assert.Equal(t, "echo: direct", res.Text)
}

func TestFR05_OneSessionPerStepAndClosed(t *testing.T) {
	fx, url := startHTTP(t)
	ctx := stepContext(t, stepDeadline)

	runStep(ctx, t, toolStep(url, "echo", echoArguments("once")))
	started, ended := fx.sessions()
	assert.Equal(t, 1, started)
	assert.Equal(t, 1, ended)

	runStep(ctx, t, httpStep(url, methodToolsList))
	started, ended = fx.sessions()
	assert.Equal(t, 2, started, "the second step opens its own session")
	assert.Equal(t, 2, ended)
}

func TestFR06_ToolsCallEcho(t *testing.T) {
	_, url := startHTTP(t)

	res, dump := runStep(stepContext(t, stepDeadline), t, toolStep(url, "echo", echoArguments("hello")))

	assert.Equal(t, "echo: hello", res.Text)
	require.Len(t, res.Content, 1)
	block, ok := res.Content[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "text", block["type"])
	assert.JSONEq(t, `{"echo":"hello","length":5}`, res.Structured)
	assert.Equal(t, map[string]interface{}{"echo": "hello", "length": float64(5)}, res.StructuredJSON)
	assert.False(t, res.IsError)
	assert.Contains(t, dump, "result.iserror", "a false iserror is a key a suite can assert on")
	assert.Equal(t, false, dump["result.iserror"])
}

func TestFR07_InitializeFields(t *testing.T) {
	_, url := startHTTP(t)

	res, dump := runStep(stepContext(t, stepDeadline), t, httpStep(url, methodInitialize))

	require.NotNil(t, res.ServerInfo)
	assert.Equal(t, fixtureServerName, res.ServerInfo.Name)
	assert.Equal(t, fixtureServerVersion, res.ServerInfo.Version)
	assert.Equal(t, defaultProtocolVersion, res.ProtocolVersion)
	capabilities, ok := res.Capabilities.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, capabilities, "tools")
	assert.Greater(t, res.TimeSeconds, 0.0)
	assert.Contains(t, dump, "result.timeseconds")
}

func TestFR07_ToolsAsTextAndParsed(t *testing.T) {
	_, url := startHTTP(t)

	res, _ := runStep(stepContext(t, stepDeadline), t, httpStep(url, methodToolsList))

	assert.Contains(t, res.Tools, `"name":"echo"`)
	tools, ok := res.ToolsJSON.([]interface{})
	require.True(t, ok)
	assert.Len(t, tools, 4)
}

func TestFR07_ApplyInitializeWithoutAnswer(t *testing.T) {
	res := Result{}
	applyInitialize(&res, nil)
	assert.Equal(t, Result{}, res)

	applyInitialize(&res, &sdk.InitializeResult{ProtocolVersion: "v"})
	assert.Equal(t, "v", res.ProtocolVersion)
	assert.Nil(t, res.ServerInfo, "no server info announced")
}

func TestFR06_CallToolResultMapping(t *testing.T) {
	cases := []struct {
		name   string
		called *sdk.CallToolResult
		text   string
		errMsg string
	}{
		{"text blocks joined with a newline", &sdk.CallToolResult{Content: []sdk.Content{
			&sdk.TextContent{Text: "a"}, &sdk.ImageContent{Data: []byte("x"), MIMEType: "image/png"}, &sdk.TextContent{Text: "b"},
		}}, "a\nb", ""},
		{"no content", &sdk.CallToolResult{}, "", ""},
		{"tool error", &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: "nope"}}}, "nope",
			`tool "t" returned an error: nope`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := callToolResult("t", tc.called)
			require.NoError(t, err)
			assert.Equal(t, tc.text, res.Text)
			assert.Equal(t, tc.errMsg, res.Err)
			assert.Equal(t, tc.called.IsError, res.IsError)
		})
	}
}

func TestFR06_CallToolResultUnreadableStructuredContent(t *testing.T) {
	res, err := callToolResult("t", &sdk.CallToolResult{
		Content:           []sdk.Content{&sdk.TextContent{Text: "kept"}},
		StructuredContent: make(chan int),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `structured content of tool "t"`)
	assert.Equal(t, "kept", res.Text, "what could be read stays in the result")
}

func TestFR06_ToolsResultUnreadable(t *testing.T) {
	_, err := toolsResult([]*sdk.Tool{{Name: "t", InputSchema: make(chan int)}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tools")
}

func TestFR06_NormaliseArguments(t *testing.T) {
	got, err := normaliseArguments(map[interface{}]interface{}{
		"outer": map[interface{}]interface{}{1: []interface{}{map[interface{}]interface{}{"k": "v"}}},
		"plain": map[string]interface{}{"n": 2},
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"outer": map[string]interface{}{"1": []interface{}{map[string]interface{}{"k": "v"}}},
		"plain": map[string]interface{}{"n": 2},
	}, got)

	none, err := normaliseArguments(nil)
	require.NoError(t, err)
	assert.Nil(t, none)

	_, err = normaliseArguments("not a map")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "'arguments' must be a map")
}

func TestFR09_SecretNeverInResultOrLogs(t *testing.T) {
	_, url := startHTTP(t)
	logs := captureLogs(t)
	step := toolStep(url, "echo", echoArguments("hello"))
	step["headers"] = map[string]interface{}{"Authorization": "Bearer " + bearerValue}
	ctx := withSecrets(stepContext(t, stepDeadline), bearerValue)

	res, dump := runStep(ctx, t, step)

	assert.Empty(t, res.Err)
	assert.NotContains(t, dumpText(t, dump), bearerValue)
	for _, entry := range logs.all() {
		assert.NotContains(t, entry, bearerValue)
	}
}

func TestFR09_StdioStderrSecretRedactedAtDebug(t *testing.T) {
	logs := captureLogs(t)
	step, _ := stdioStep(t, childStderr, methodPing)
	t.Setenv(envStderrText, "child says "+bearerValue)
	ctx := withSecrets(stepContext(t, stepDeadline), bearerValue)

	res, dump := runStep(ctx, t, step)

	assert.Empty(t, res.Err)
	assert.NotContains(t, dumpText(t, dump), bearerValue)
	for _, entry := range logs.above() {
		assert.NotContains(t, entry, bearerValue)
	}
	var stderrEntries []string
	for _, entry := range logs.all() {
		assert.NotContains(t, entry, bearerValue)
		if strings.Contains(entry, "mcp server stderr") {
			stderrEntries = append(stderrEntries, entry)
		}
	}
	require.Len(t, stderrEntries, 1, "the child's stderr line is logged once, at Debug")
	assert.Contains(t, stderrEntries[0], "child says __hidden__")
}

func TestFR10_FailuresAreResultsNotGoErrors(t *testing.T) {
	_, url := startHTTP(t)
	badCommand := venom.TestStep{
		"type": Name, "transport": transportStdio, "method": methodPing,
		"command": []string{"/nonexistent/venom-mcp-test-binary"},
	}
	cases := []struct {
		name string
		step venom.TestStep
		want string
	}{
		{"unreachable url", httpStep("http://127.0.0.1:0/mcp", methodPing), "cannot connect"},
		{"bad command", badCommand, "cannot connect"},
		{"unknown tool (JSON-RPC error)", toolStep(url, "no-such-tool", nil), `tool "no-such-tool" call failed`},
		{"arguments refused by the server", toolStep(url, "echo", map[string]interface{}{"message": 5}), `tool "echo" returned an error`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, dump := runStep(stepContext(t, stepDeadline), t, tc.step)
			assert.Contains(t, res.Err, tc.want)
			assert.Contains(t, dump, "result.err")
			assert.Greater(t, res.TimeSeconds, 0.0)
		})
	}
}

func TestFR11_HTTPServerNeverAnswers(t *testing.T) {
	_, url := startHTTP(t)
	start := time.Now()

	res, _ := runStep(stepContext(t, shortDeadline), t, toolStep(url, "hang", nil))

	assert.NotEmpty(t, res.Err)
	assert.Contains(t, res.Err, `"hang"`)
	assert.Less(t, time.Since(start), stepDeadline, "the deadline ended the step")
}

func TestFR11_StdioChildHangs(t *testing.T) {
	step, pidFile := stdioStep(t, childHang, methodPing)

	res, _ := runStep(stepContext(t, shortDeadline), t, step)

	assert.Contains(t, res.Err, "cannot connect")
	requireProcessGone(t, pidFile)
}

func TestFR11_WithDefaultTimeout(t *testing.T) {
	t.Run("adds a deadline of at most 30 seconds", func(t *testing.T) {
		ctx, cancel := withDefaultTimeout(context.Background())
		defer cancel()
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		assert.LessOrEqual(t, time.Until(deadline), defaultTimeout)
		assert.Greater(t, time.Until(deadline), 0*time.Second)
		assert.Equal(t, 30*time.Second, defaultTimeout)
	})
	t.Run("keeps an existing deadline", func(t *testing.T) {
		own, ownCancel := context.WithTimeout(context.Background(), time.Hour)
		defer ownCancel()
		want, _ := own.Deadline()

		ctx, cancel := withDefaultTimeout(own)
		defer cancel()
		got, ok := ctx.Deadline()
		require.True(t, ok)
		assert.Equal(t, want, got)
		cancel()
		assert.NoError(t, own.Err(), "the returned cancel does not end the caller's context")
	})
}

func TestNFR03_ProtocolVersionOverrideIsHonoured(t *testing.T) {
	_, url := startHTTP(t)
	step := httpStep(url, methodInitialize)
	step["protocol_version"] = "2025-06-18"

	res, _ := runStep(stepContext(t, stepDeadline), t, step)

	assert.Empty(t, res.Err)
	assert.Equal(t, "2025-06-18", res.ProtocolVersion)
}

func TestNFR03_SamplingAndElicitationAreNotOffered(t *testing.T) {
	_, url := startHTTP(t)
	res, _ := runStep(stepContext(t, stepDeadline), t, toolStep(url, "capabilities", nil))
	assert.Empty(t, res.Err)
	assert.Equal(t, "sampling=false elicitation=false", res.Text)
}

func TestAC1_ToolsListContainsTheTool(t *testing.T) {
	_, url := startHTTP(t)

	res, _ := runStep(stepContext(t, stepDeadline), t, httpStep(url, methodToolsList))

	assert.Empty(t, res.Err)
	names := toolNames(t, res.ToolsJSON)
	assert.Contains(t, names, "echo")
	assert.Contains(t, names, "fail")
}

func toolNames(t *testing.T, toolsJSON interface{}) []string {
	t.Helper()
	tools, ok := toolsJSON.([]interface{})
	require.True(t, ok)
	var names []string
	for _, tool := range tools {
		entry, ok := tool.(map[string]interface{})
		require.True(t, ok)
		names = append(names, fmt.Sprint(entry["name"]))
	}
	return names
}

func TestAC2_ToolsCallFillsTextAndStructured(t *testing.T) {
	_, url := startHTTP(t)

	res, _ := runStep(stepContext(t, stepDeadline), t, toolStep(url, "echo", echoArguments("ac2")))

	assert.Equal(t, "echo: ac2", res.Text)
	assert.JSONEq(t, `{"echo":"ac2","length":3}`, res.Structured)
	assert.NotNil(t, res.StructuredJSON)
	assert.Empty(t, res.Err)
}

func TestAC3_ToolErrorFailsTheStep(t *testing.T) {
	_, url := startHTTP(t)

	res, dump := runStep(stepContext(t, stepDeadline), t, toolStep(url, "fail", nil))

	assert.True(t, res.IsError)
	assert.Equal(t, true, dump["result.iserror"])
	assert.Contains(t, res.Err, `"fail"`)
	assert.Contains(t, res.Err, "backend is down")
	assert.NotEmpty(t, res.Err, "the default assertion result.err ShouldBeEmpty fails")
}

func TestMisuseIsAGoError(t *testing.T) {
	cases := []struct {
		name string
		step venom.TestStep
		want string
	}{
		{"stdio without command", venom.TestStep{"transport": "stdio", "method": "ping"}, "needs a 'command'"},
		{"http without url", venom.TestStep{"transport": "http", "method": "ping"}, "needs a 'url'"},
		{"unknown transport", venom.TestStep{"transport": "pigeon", "method": "ping"}, `unknown transport "pigeon"`},
		{"missing transport", venom.TestStep{"method": "ping"}, "unknown transport"},
		{"unknown method", httpStep("http://h", "tools/frobnicate"), `unknown method "tools/frobnicate"`},
		{"tools/call without tool", httpStep("http://h", methodToolsCall), "needs a 'tool'"},
		{"arguments not a map", func() venom.TestStep {
			step := httpStep("http://h", methodToolsCall)
			step["arguments"] = "x"
			return step
		}(), "'arguments' must be a map"},
		{"command of the wrong type", venom.TestStep{"transport": "stdio", "command": 3, "method": "ping"}, "Command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorContains(t, runMisuse(t, tc.step), tc.want)
		})
	}
}

func TestFR03_HeaderTransportDoesNotMutateTheRequest(t *testing.T) {
	var seen http.Header
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r.Header.Clone()
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})
	req, err := http.NewRequestWithContext(stepContext(t, stepDeadline), http.MethodGet, "http://127.0.0.1/", nil)
	require.NoError(t, err)

	resp, err := headerTransport{base: base, headers: map[string]string{"X-A": "1"}, origin: "http://127.0.0.1"}.RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, "1", seen.Get("X-A"))
	assert.Empty(t, req.Header.Get("X-A"), "the original request is left alone")
}

func TestFR09_HeadersStayOnTheEndpointOrigin(t *testing.T) {
	var gotAuth []string
	other := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
	}))
	t.Cleanup(other.Close)
	// the same listener under another host name is another origin
	otherURL := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, otherURL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(origin.Close)

	transport := headerTransport{base: http.DefaultTransport, headers: map[string]string{"Authorization": "Bearer " + bearerValue}, origin: originOf(origin.URL)}
	req, err := http.NewRequestWithContext(stepContext(t, stepDeadline), http.MethodGet, origin.URL, nil)
	require.NoError(t, err)
	resp, err := (&http.Client{Transport: transport}).Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, []string{""}, gotAuth, "the redirect target was reached without the header")
}

func TestLineLoggerFlushesTheUnterminatedLine(t *testing.T) {
	logs := captureLogs(t)
	logger := &lineLogger{ctx: stepContext(t, stepDeadline)}
	_, err := logger.Write([]byte("first\nlast words"))
	require.NoError(t, err)
	logger.flush()
	logger.flush()

	var lines []string
	for _, entry := range logs.all() {
		if strings.Contains(entry, "mcp server stderr") {
			lines = append(lines, entry)
		}
	}
	require.Len(t, lines, 2, "one complete line, then the flushed rest, once")
	assert.Contains(t, lines[1], "last words")
}

func TestLineLoggerBoundsAnUnterminatedLine(t *testing.T) {
	logger := &lineLogger{ctx: stepContext(t, stepDeadline)}
	_, err := logger.Write(bytes.Repeat([]byte("x"), maxPendingStderr+1))
	require.NoError(t, err)
	assert.Empty(t, logger.pending)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Story #2011 (S12): resources and prompts. AC-1 to AC-4 are the story's criteria (resources/list, resources/read,
// prompts/list, prompts/get).

func entryMaps(t *testing.T, list []interface{}) []map[string]interface{} {
	t.Helper()
	out := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		entry, ok := item.(map[string]interface{})
		require.True(t, ok)
		out = append(out, entry)
	}
	return out
}

func fieldValues(entries []map[string]interface{}, field string) []string {
	var out []string
	for _, entry := range entries {
		out = append(out, fmt.Sprint(entry[field]))
	}
	return out
}

func TestS12AC1_ResourcesList(t *testing.T) {
	_, url := startHTTP(t)

	res, dump := runStep(stepContext(t, stepDeadline), t, httpStep(url, methodResourcesList))

	assert.Empty(t, res.Err)
	entries := entryMaps(t, res.Resources)
	assert.ElementsMatch(t, []string{uriText, uriSecondText, uriBlob}, fieldValues(entries, "uri"))
	for _, entry := range entries {
		if entry["uri"] == uriText {
			assert.Equal(t, "hello", entry["name"])
			assert.Equal(t, "text/plain", entry["mimetype"])
			assert.Equal(t, "A greeting", entry["description"])
		}
	}
	assert.Contains(t, dump, "result.resources")
}

func TestS12AC2_ResourcesRead(t *testing.T) {
	_, url := startHTTP(t)

	res, _ := runStep(stepContext(t, stepDeadline), t, resourceStep(url, methodResourcesRead, uriText))

	assert.Empty(t, res.Err)
	entries := entryMaps(t, res.Resources)
	require.Len(t, entries, 1)
	assert.Equal(t, uriText, entries[0]["uri"])
	assert.Equal(t, "text/plain", entries[0]["mimetype"])
	assert.Equal(t, "hello", entries[0]["text"])
	assert.Equal(t, []interface{}{"hello"}, res.Content)
	assert.Equal(t, "hello", res.Text)
}

func TestS12AC2_ResourcesReadBlob(t *testing.T) {
	_, url := startHTTP(t)

	res, _ := runStep(stepContext(t, stepDeadline), t, resourceStep(url, methodResourcesRead, uriBlob))

	assert.Empty(t, res.Err)
	entries := entryMaps(t, res.Resources)
	require.Len(t, entries, 1)
	assert.Equal(t, "AQID", entries[0]["blob"])
	assert.Equal(t, "", entries[0]["text"])
	assert.Empty(t, res.Content, "a blob does not reach result.content")
	assert.Empty(t, res.Text, "a blob does not reach result.text")
}

func TestS12AC3_PromptsList(t *testing.T) {
	_, url := startHTTP(t)

	res, dump := runStep(stepContext(t, stepDeadline), t, httpStep(url, methodPromptsList))

	assert.Empty(t, res.Err)
	entries := entryMaps(t, res.Prompts)
	assert.ElementsMatch(t, []string{"greet", "plain"}, fieldValues(entries, "name"))
	for _, entry := range entries {
		if entry["name"] != "greet" {
			continue
		}
		assert.Equal(t, "Greets someone", entry["description"])
		arguments, ok := entry["arguments"].([]interface{})
		require.True(t, ok)
		require.Len(t, arguments, 1)
		argument, ok := arguments[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "who", argument["name"])
		assert.Equal(t, true, argument["required"])
	}
	assert.Contains(t, dump, "result.prompts")
}

func TestS12AC4_PromptsGet(t *testing.T) {
	_, url := startHTTP(t)

	res, dump := runStep(stepContext(t, stepDeadline), t, promptStep(url, "greet", map[string]interface{}{"who": "world"}))

	assert.Empty(t, res.Err)
	entries := entryMaps(t, res.Prompts)
	require.Len(t, entries, 1)
	assert.Equal(t, "user", entries[0]["role"])
	assert.Equal(t, "hello world", entries[0]["text"])
	assert.Equal(t, "hello world", res.Text)
	assert.Equal(t, "A greeting", res.Description)
	assert.Equal(t, "A greeting", dump["result.description"])
}

func TestS12_PromptsGetWithoutArguments(t *testing.T) {
	_, url := startHTTP(t)

	res, _ := runStep(stepContext(t, stepDeadline), t, promptStep(url, "plain", nil))

	assert.Empty(t, res.Err)
	assert.Equal(t, "plain", res.Text)
	assert.Empty(t, res.Description)
}

func TestS12_ListsFollowPagination(t *testing.T) {
	session := memorySession(t, &sdk.ServerOptions{PageSize: 1})
	ctx := stepContext(t, stepDeadline)

	resources, err := runResourcesList(ctx, Executor{}, session)
	require.NoError(t, err)
	assert.Len(t, resources.Resources, 3, "three resources, served one per page")

	prompts, err := runPromptsList(ctx, Executor{}, session)
	require.NoError(t, err)
	assert.Len(t, prompts.Prompts, 2, "two prompts, served one per page")
}

func TestS12_RunFunctionsOnSession(t *testing.T) {
	session := memorySession(t, nil)
	ctx := stepContext(t, stepDeadline)

	read, err := runResourcesRead(ctx, Executor{URI: uriSecondText}, session)
	require.NoError(t, err)
	assert.Equal(t, "second", read.Text)

	got, err := runPromptsGet(ctx, Executor{Prompt: "greet", Arguments: map[string]interface{}{"who": "me"}}, session)
	require.NoError(t, err)
	assert.Equal(t, "hello me", got.Text)

	_, err = runPromptsGet(ctx, Executor{Prompt: "greet", Arguments: map[string]interface{}{"who": []interface{}{"x"}}}, session)
	assert.ErrorContains(t, err, `prompt argument "who"`)
}

func TestS12_FailuresAreResultsNotGoErrors(t *testing.T) {
	_, url := startHTTP(t)
	cases := []struct {
		name string
		step venom.TestStep
		want string
	}{
		{"unknown uri", resourceStep(url, methodResourcesRead, "venom://text/missing"), `resource "venom://text/missing" read failed`},
		{"unknown prompt", promptStep(url, "no-such-prompt", nil), `prompt "no-such-prompt" get failed`},
		{"missing required argument", promptStep(url, "greet", nil), `prompt "greet" get failed`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, dump := runStep(stepContext(t, stepDeadline), t, tc.step)
			assert.Contains(t, res.Err, tc.want)
			assert.Contains(t, dump, "result.err")
		})
	}
}

func TestS12_RunFunctionsWrapTheSDKError(t *testing.T) {
	session := memorySession(t, nil)
	ctx := stepContext(t, stepDeadline)
	require.NoError(t, session.Close())

	cases := []struct {
		name string
		run  methodFunc
		e    Executor
		want string
	}{
		{"resources/list", runResourcesList, Executor{}, "resources/list failed"},
		{"prompts/list", runPromptsList, Executor{}, "prompts/list failed"},
		{"tools/list", runToolsList, Executor{}, "tools/list failed"},
		{"resources/read", runResourcesRead, Executor{URI: uriText}, `resource "venom://text/hello" read failed`},
		{"prompts/get", runPromptsGet, Executor{Prompt: "plain"}, `prompt "plain" get failed`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.run(ctx, tc.e, session)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestS12_PromptsGetMisuse(t *testing.T) {
	deep := func(value interface{}) venom.TestStep {
		return promptStep("http://h", "greet", map[string]interface{}{"who": "x", "extra": value})
	}
	cases := []struct {
		name string
		step venom.TestStep
		want string
	}{
		{"resources/read without uri", httpStep("http://h", methodResourcesRead), "needs a 'uri'"},
		{"prompts/get without prompt", httpStep("http://h", methodPromptsGet), "needs a 'prompt'"},
		{"nested map argument", deep(map[string]interface{}{"a": 1}), `prompt argument "extra" must be a string, number or boolean`},
		{"list argument", deep([]interface{}{"a"}), `prompt argument "extra" must be a string, number or boolean`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorContains(t, runMisuse(t, tc.step), tc.want)
		})
	}
}

func TestS12_PromptArgumentsAreStrings(t *testing.T) {
	got, err := promptArguments(map[string]interface{}{
		"s": "text", "i": 42, "u": uint8(7), "f": 1.5, "whole": float64(3), "t": true, "n": int64(-2),
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"s": "text", "i": "42", "u": "7", "f": "1.5", "whole": "3", "t": "true", "n": "-2"}, got)

	none, err := promptArguments(nil)
	require.NoError(t, err)
	assert.Nil(t, none)

	_, err = promptArguments(map[string]interface{}{"x": nil})
	assert.ErrorContains(t, err, `prompt argument "x"`)
}

func TestS12_NumberAndBooleanArgumentsReachThePrompt(t *testing.T) {
	_, url := startHTTP(t)
	cases := []struct {
		name  string
		value interface{}
		want  string
	}{
		{"number", 42, "hello 42"},
		{"boolean", true, "hello true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _ := runStep(stepContext(t, stepDeadline), t, promptStep(url, "greet", map[string]interface{}{"who": tc.value}))
			assert.Equal(t, tc.want, res.Text)
		})
	}
}

func TestS12_ZeroValueResultKeys(t *testing.T) {
	// go-dump ignores omitempty, so a zero Result already carries the keys a suite can assert on.
	dump := venom.GetExecutorResult(Executor{}.ZeroValueResult())
	for _, key := range []string{"result.resources", "result.prompts", "result.description"} {
		assert.Contains(t, dump, key)
	}
	assert.Equal(t, "", dump["result.description"])
}

func TestS12_MappersUnreadableValues(t *testing.T) {
	_, err := plainList("things", make(chan int))
	require.ErrorContains(t, err, "cannot read the things")

	_, err = plainList("things", map[string]string{"a": "b"})
	require.ErrorContains(t, err, "unexpected map[string]interface {}")
}

func TestS12_MappersOnEmptyAnswers(t *testing.T) {
	list, err := resourcesListResult(nil)
	require.NoError(t, err)
	assert.Empty(t, list.Resources)

	prompts, err := promptsListResult(nil)
	require.NoError(t, err)
	assert.Empty(t, prompts.Prompts)

	read, err := resourceReadResult(uriText, &sdk.ReadResourceResult{})
	require.NoError(t, err)
	assert.Empty(t, read.Text)

	got, err := promptGetResult("p", &sdk.GetPromptResult{})
	require.NoError(t, err)
	assert.Empty(t, got.Text)
}

func TestS12_MappersSkipNullEntries(t *testing.T) {
	// the SDK client does not filter a null inside a server's answer; a nil pointer here must not panic the run (FR-10)
	list, err := resourcesListResult([]*sdk.Resource{nil, {URI: "a", Name: "n"}, nil})
	require.NoError(t, err)
	assert.Len(t, list.Resources, 1)

	prompts, err := promptsListResult([]*sdk.Prompt{nil, {Name: "p"}})
	require.NoError(t, err)
	assert.Len(t, prompts.Prompts, 1)

	read, err := resourceReadResult(uriText, &sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{nil, {URI: "a", Text: "one"}}})
	require.NoError(t, err)
	assert.Equal(t, "one", read.Text)
	assert.Len(t, read.Resources, 1)

	got, err := promptGetResult("p", &sdk.GetPromptResult{Messages: []*sdk.PromptMessage{
		nil, {Role: "user"}, {Role: "user", Content: &sdk.TextContent{Text: "hi"}},
	}})
	require.NoError(t, err)
	assert.Equal(t, "hi", got.Text)
	assert.Len(t, got.Prompts, 2, "a message without content stays as an entry with empty text")
}

func TestS12_PaginateStopsOnARepeatedCursor(t *testing.T) {
	calls := 0
	_, err := paginate("things/list", func(string) ([]int, string, error) {
		calls++
		return []int{calls}, "same", nil
	})
	require.ErrorContains(t, err, `things/list failed: the server repeated cursor "same"`)
	assert.Equal(t, 2, calls, "the first page sets the cursor, the second one repeats it")

	all, err := paginate("things/list", func(cursor string) ([]int, string, error) {
		if cursor == "" {
			return []int{1}, "two", nil
		}
		return []int{2}, "", nil
	})
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, all)
}

func TestS12_ResourceReadMixedContents(t *testing.T) {
	read, err := resourceReadResult(uriText, &sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{
		{URI: "a", Text: "one"}, {URI: "b", Blob: []byte{1}}, {URI: "c", Text: "two"},
	}})
	require.NoError(t, err)
	assert.Equal(t, "one\ntwo", read.Text)
	assert.Equal(t, []interface{}{"one", "two"}, read.Content)
	assert.Len(t, read.Resources, 3)
}

// requireHiddenByTheEngine: a value the server echoes back is in the executor's result (the executor does not know what
// the suite considers secret); the engine hides it with the context's secrets before anything is printed or filed.
func requireHiddenByTheEngine(ctx context.Context, t *testing.T, dump map[string]interface{}) {
	t.Helper()
	text := dumpText(t, dump)
	require.Contains(t, text, bearerValue, "the echo is part of the raw result")
	hidden := venom.HideSensitive(ctx, text)
	assert.NotContains(t, hidden, bearerValue)
}

func TestS12_FR09_SecretPromptArgumentNeverInResultOrLogs(t *testing.T) {
	_, url := startHTTP(t)
	logs := captureLogs(t)
	ctx := withSecrets(stepContext(t, stepDeadline), bearerValue)

	res, dump := runStep(ctx, t, promptStep(url, "greet", map[string]interface{}{"who": bearerValue}))

	assert.Empty(t, res.Err)
	requireHiddenByTheEngine(ctx, t, dump)
	for _, entry := range logs.all() {
		assert.NotContains(t, entry, bearerValue)
	}
}

func TestS12_FR09_SecretInFailingURINeverInResultOrLogs(t *testing.T) {
	_, url := startHTTP(t)
	logs := captureLogs(t)
	ctx := withSecrets(stepContext(t, stepDeadline), bearerValue)

	res, dump := runStep(ctx, t, resourceStep(url, methodResourcesRead, "venom://secret/"+bearerValue))

	assert.NotEmpty(t, res.Err)
	requireHiddenByTheEngine(ctx, t, dump)
	for _, entry := range logs.all() {
		assert.NotContains(t, entry, bearerValue)
	}
}
