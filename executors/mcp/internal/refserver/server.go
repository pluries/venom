package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "venom-mcp-refserver"
	serverVersion = "1.0.0"

	// failText is the fixed text of the `fail` tool.
	failText = "refserver: this tool always fails"

	// envBearerVar, when set, makes the HTTP transport require `Authorization: Bearer <value>`.
	envBearerVar = "MCP_REFSERVER_TOKEN" //nolint:gosec // G101: the name of an environment variable, not a credential

	uriText       = "venom://text/hello"
	uriSecondText = "venom://text/second"
	uriBlob       = "venom://blob/bytes"

	mimeText   = "text/plain"
	mimeBinary = "application/octet-stream"

	promptGreet = "greet"
	promptPlain = "plain"
	argWho      = "who"
)

type echoInput struct {
	Message string `json:"message" jsonschema:"the text to echo back"`
}

type echoOutput struct {
	Message string `json:"message"`
	Length  int    `json:"length"`
}

// newServer builds the reference server: an `echo` tool with text and structured output, a `fail` tool
// that always reports an error, three resources and two prompts.
func newServer() *sdk.Server {
	server := sdk.NewServer(&sdk.Implementation{Name: serverName, Version: serverVersion}, nil)
	sdk.AddTool(server, &sdk.Tool{Name: "echo", Description: "Echoes the message, as text and as structured content"}, echoTool)
	server.AddTool(&sdk.Tool{
		Name:        "fail",
		Description: "Always returns a tool error",
		InputSchema: map[string]any{"type": "object"},
	}, failTool)
	addResources(server)
	addPrompts(server)
	return server
}

// addResources registers two text resources and one blob resource.
func addResources(server *sdk.Server) {
	server.AddResource(&sdk.Resource{URI: uriText, Name: "hello", MIMEType: mimeText, Description: "A greeting"}, readResource)
	server.AddResource(&sdk.Resource{URI: uriSecondText, Name: "second", MIMEType: mimeText}, readResource)
	server.AddResource(&sdk.Resource{URI: uriBlob, Name: "bytes", MIMEType: mimeBinary}, readResource)
}

// readResource serves the three resources; any other uri is the SDK's resource not found error.
func readResource(_ context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	uri := req.Params.URI
	content := &sdk.ResourceContents{URI: uri, MIMEType: mimeText}
	switch uri {
	case uriText:
		content.Text = "hello"
	case uriSecondText:
		content.Text = "second"
	case uriBlob:
		content.MIMEType = mimeBinary
		content.Blob = []byte{1, 2, 3}
	default:
		return nil, sdk.ResourceNotFoundError(uri)
	}
	return &sdk.ReadResourceResult{Contents: []*sdk.ResourceContents{content}}, nil
}

// addPrompts registers greet (one required argument, echoed in its message) and plain (no argument).
func addPrompts(server *sdk.Server) {
	greet := &sdk.Prompt{
		Name: promptGreet, Description: "Greets someone",
		Arguments: []*sdk.PromptArgument{{Name: argWho, Description: "Who to greet", Required: true}},
	}
	server.AddPrompt(greet, greetPrompt)
	server.AddPrompt(&sdk.Prompt{Name: promptPlain, Description: "Takes nothing"}, plainPrompt)
}

// greetPrompt checks the required argument itself: the SDK does not enforce Required.
func greetPrompt(_ context.Context, req *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
	who, ok := req.Params.Arguments[argWho]
	if !ok {
		return nil, fmt.Errorf("missing required argument %q", argWho)
	}
	return &sdk.GetPromptResult{
		Description: "A greeting",
		Messages:    []*sdk.PromptMessage{{Role: "user", Content: &sdk.TextContent{Text: "hello " + who}}},
	}, nil
}

func plainPrompt(context.Context, *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
	return &sdk.GetPromptResult{Messages: []*sdk.PromptMessage{{Role: "assistant", Content: &sdk.TextContent{Text: "plain"}}}}, nil
}

func echoTool(_ context.Context, _ *sdk.CallToolRequest, in echoInput) (*sdk.CallToolResult, echoOutput, error) {
	out := echoOutput{Message: in.Message, Length: len(in.Message)}
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: "echo: " + in.Message}}}, out, nil
}

func failTool(context.Context, *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
	return &sdk.CallToolResult{
		IsError: true,
		Content: []sdk.Content{&sdk.TextContent{Text: failText}},
	}, nil
}

// httpHandler serves the server over streamable HTTP, behind the bearer check when a token is set.
func httpHandler(server *sdk.Server) http.Handler {
	handler := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, nil)
	return requireBearer(os.Getenv(envBearerVar), handler)
}

// requireBearer answers 401 to a request without the expected token; an empty token disables the check.
func requireBearer(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}
	want := "Bearer " + token
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get("Authorization")) != want {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func stdioTransport() sdk.Transport { return &sdk.StdioTransport{} }
