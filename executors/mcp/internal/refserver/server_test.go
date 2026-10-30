package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func connect(t *testing.T) *sdk.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	serverSession, err := newServer().Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := sdk.NewClient(&sdk.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestEchoReturnsTextAndStructured(t *testing.T) {
	session := connect(t)
	res, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: "echo", Arguments: map[string]any{"message": "hi"}})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Len(t, res.Content, 1)
	text, ok := res.Content[0].(*sdk.TextContent)
	require.True(t, ok)
	require.Equal(t, "echo: hi", text.Text)
	require.Equal(t, map[string]any{"message": "hi", "length": float64(2)}, res.StructuredContent)
}

func TestFailIsAToolError(t *testing.T) {
	session := connect(t)
	res, err := session.CallTool(context.Background(), &sdk.CallToolParams{Name: "fail"})
	require.NoError(t, err)
	require.True(t, res.IsError)
	text, ok := res.Content[0].(*sdk.TextContent)
	require.True(t, ok)
	require.Equal(t, failText, text.Text)
}

func TestResources(t *testing.T) {
	session := connect(t)
	ctx := context.Background()
	list, err := session.ListResources(ctx, &sdk.ListResourcesParams{})
	require.NoError(t, err)
	require.Len(t, list.Resources, 3)

	read, err := session.ReadResource(ctx, &sdk.ReadResourceParams{URI: uriText})
	require.NoError(t, err)
	require.Equal(t, "hello", read.Contents[0].Text)

	read, err = session.ReadResource(ctx, &sdk.ReadResourceParams{URI: uriBlob})
	require.NoError(t, err)
	require.Equal(t, []byte{1, 2, 3}, read.Contents[0].Blob)
	require.Empty(t, read.Contents[0].Text)

	_, err = session.ReadResource(ctx, &sdk.ReadResourceParams{URI: "venom://text/missing"})
	require.Error(t, err)
}

func TestPrompts(t *testing.T) {
	session := connect(t)
	ctx := context.Background()
	list, err := session.ListPrompts(ctx, &sdk.ListPromptsParams{})
	require.NoError(t, err)
	require.Len(t, list.Prompts, 2)

	got, err := session.GetPrompt(ctx, &sdk.GetPromptParams{Name: promptGreet, Arguments: map[string]string{argWho: "venom"}})
	require.NoError(t, err)
	text, ok := got.Messages[0].Content.(*sdk.TextContent)
	require.True(t, ok)
	require.Equal(t, "hello venom", text.Text)

	_, err = session.GetPrompt(ctx, &sdk.GetPromptParams{Name: promptGreet})
	require.Error(t, err)

	got, err = session.GetPrompt(ctx, &sdk.GetPromptParams{Name: promptPlain})
	require.NoError(t, err)
	require.Len(t, got.Messages, 1)
}

func TestRequireBearer(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	cases := map[string]struct {
		token, header string
		want          int
	}{
		"no token configured": {"", "", http.StatusNoContent},
		"missing header":      {"tok", "", http.StatusUnauthorized},
		"wrong token":         {"tok", "Bearer other", http.StatusUnauthorized},
		"right token":         {"tok", "Bearer tok", http.StatusNoContent},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			requireBearer(tc.token, next).ServeHTTP(rec, req)
			require.Equal(t, tc.want, rec.Code)
		})
	}
}
