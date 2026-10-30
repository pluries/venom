package mcp

import (
	"context"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	methodInitialize = "initialize"
	methodPing       = "ping"
	methodToolsList  = "tools/list"
	methodToolsCall  = "tools/call"

	methodResourcesList = "resources/list"
	methodResourcesRead = "resources/read"
	methodPromptsList   = "prompts/list"
	methodPromptsGet    = "prompts/get"
)

// methodFunc runs one method on an open session. The error is a failure of the exchange, reported in
// Result.Err; misuse is caught earlier by validate.
type methodFunc func(ctx context.Context, e Executor, session *sdk.ClientSession) (Result, error)

var methods = map[string]methodFunc{
	methodInitialize:    runInitialize,
	methodPing:          runPing,
	methodToolsList:     runToolsList,
	methodToolsCall:     runToolsCall,
	methodResourcesList: runResourcesList,
	methodResourcesRead: runResourcesRead,
	methodPromptsList:   runPromptsList,
	methodPromptsGet:    runPromptsGet,
}

func (e Executor) validateMethod() error {
	if _, ok := methods[e.Method]; !ok {
		return fmt.Errorf("unknown method %q", e.Method)
	}
	if input := e.missingInput(); input != "" {
		return fmt.Errorf("method %s needs a '%s'", e.Method, input)
	}
	if e.Method == methodPromptsGet {
		if _, err := promptArguments(e.Arguments); err != nil {
			return err
		}
	}
	return nil
}

// missingInput names the step field the method cannot run without, or "" when it has all of them.
func (e Executor) missingInput() string {
	switch e.Method {
	case methodToolsCall:
		if e.Tool == "" {
			return "tool"
		}
	case methodResourcesRead:
		if e.URI == "" {
			return "uri"
		}
	case methodPromptsGet:
		if e.Prompt == "" {
			return "prompt"
		}
	}
	return ""
}

// runInitialize has nothing to do: connecting performed the exchange, and execute copies its answer.
func runInitialize(_ context.Context, _ Executor, _ *sdk.ClientSession) (Result, error) {
	return Result{}, nil
}

func runPing(ctx context.Context, _ Executor, session *sdk.ClientSession) (Result, error) {
	if err := session.Ping(ctx, nil); err != nil {
		return Result{}, fmt.Errorf("ping failed: %w", err)
	}
	return Result{}, nil
}

// paginate follows the server's cursors so that a list method returns every item. fetch returns one
// page and the cursor of the next one ("" on the last page). A cursor the server sends twice ends the
// step with an error: the SDK can serve a repeated cursor from its cache without a request, and the
// step deadline would never stop that loop.
func paginate[T any](method string, fetch func(cursor string) ([]T, string, error)) ([]T, error) {
	var all []T
	seen := map[string]bool{}
	cursor := ""
	for {
		items, next, err := fetch(cursor)
		if err != nil {
			return nil, fmt.Errorf("%s failed: %w", method, err)
		}
		all = append(all, items...)
		if next == "" {
			return all, nil
		}
		if seen[next] {
			return nil, fmt.Errorf("%s failed: the server repeated cursor %q", method, next)
		}
		seen[next] = true
		cursor = next
	}
}

// runToolsList follows the server's pagination so that result.tools holds every tool.
func runToolsList(ctx context.Context, _ Executor, session *sdk.ClientSession) (Result, error) {
	tools, err := paginate(methodToolsList, func(cursor string) ([]*sdk.Tool, string, error) {
		page, err := session.ListTools(ctx, &sdk.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, "", err
		}
		return page.Tools, page.NextCursor, nil
	})
	if err != nil {
		return Result{}, err
	}
	return toolsResult(tools)
}

func runToolsCall(ctx context.Context, e Executor, session *sdk.ClientSession) (Result, error) {
	called, err := session.CallTool(ctx, &sdk.CallToolParams{Name: e.Tool, Arguments: e.Arguments})
	if err != nil {
		return Result{}, fmt.Errorf("tool %q call failed: %w", e.Tool, err)
	}
	return callToolResult(e.Tool, called)
}

func runResourcesList(ctx context.Context, _ Executor, session *sdk.ClientSession) (Result, error) {
	resources, err := paginate(methodResourcesList, func(cursor string) ([]*sdk.Resource, string, error) {
		page, err := session.ListResources(ctx, &sdk.ListResourcesParams{Cursor: cursor})
		if err != nil {
			return nil, "", err
		}
		return page.Resources, page.NextCursor, nil
	})
	if err != nil {
		return Result{}, err
	}
	return resourcesListResult(resources)
}

func runResourcesRead(ctx context.Context, e Executor, session *sdk.ClientSession) (Result, error) {
	read, err := session.ReadResource(ctx, &sdk.ReadResourceParams{URI: e.URI})
	if err != nil {
		return Result{}, fmt.Errorf("resource %q read failed: %w", e.URI, err)
	}
	return resourceReadResult(e.URI, read)
}

func runPromptsList(ctx context.Context, _ Executor, session *sdk.ClientSession) (Result, error) {
	prompts, err := paginate(methodPromptsList, func(cursor string) ([]*sdk.Prompt, string, error) {
		page, err := session.ListPrompts(ctx, &sdk.ListPromptsParams{Cursor: cursor})
		if err != nil {
			return nil, "", err
		}
		return page.Prompts, page.NextCursor, nil
	})
	if err != nil {
		return Result{}, err
	}
	return promptsListResult(prompts)
}

func runPromptsGet(ctx context.Context, e Executor, session *sdk.ClientSession) (Result, error) {
	arguments, err := promptArguments(e.Arguments)
	if err != nil {
		return Result{}, err
	}
	got, err := session.GetPrompt(ctx, &sdk.GetPromptParams{Name: e.Prompt, Arguments: arguments})
	if err != nil {
		return Result{}, fmt.Errorf("prompt %q get failed: %w", e.Prompt, err)
	}
	return promptGetResult(e.Prompt, got)
}
