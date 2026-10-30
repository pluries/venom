package mcp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// applyInitialize copies what the server announced when the session opened.
func applyInitialize(result *Result, init *sdk.InitializeResult) {
	if init == nil {
		return
	}
	result.ProtocolVersion = init.ProtocolVersion
	if init.ServerInfo != nil {
		result.ServerInfo = &ServerInfo{Name: init.ServerInfo.Name, Version: init.ServerInfo.Version}
	}
	// A result the engine cannot dump is worth less than the other fields: keep the rest.
	if capabilities, err := plain(init.Capabilities); err == nil {
		result.Capabilities = capabilities
	}
}

// toolsResult exposes the tools as JSON text and as the parsed value.
func toolsResult(tools []*sdk.Tool) (Result, error) {
	text, err := json.Marshal(tools)
	if err != nil {
		return Result{}, fmt.Errorf("cannot encode the tools: %w", err)
	}
	parsed, err := plain(tools)
	if err != nil {
		return Result{}, fmt.Errorf("cannot read the tools: %w", err)
	}
	return Result{Tools: string(text), ToolsJSON: parsed}, nil
}

// callToolResult maps a tool answer. A tool that reports an error is a failed step: Err names the
// tool and carries the text the server gave.
func callToolResult(tool string, called *sdk.CallToolResult) (Result, error) {
	result := Result{IsError: called.IsError, Text: textOf(called.Content)}

	content, err := plain(called.Content)
	if err != nil {
		return result, fmt.Errorf("cannot read the content of tool %q: %w", tool, err)
	}
	if blocks, ok := content.([]interface{}); ok {
		result.Content = blocks
	}

	if called.StructuredContent != nil {
		if err := setStructured(&result, called.StructuredContent); err != nil {
			return result, fmt.Errorf("cannot read the structured content of tool %q: %w", tool, err)
		}
	}

	if called.IsError {
		result.Err = fmt.Sprintf("tool %q returned an error: %s", tool, result.Text)
	}
	return result, nil
}

func setStructured(result *Result, structured interface{}) error {
	text, err := json.Marshal(structured)
	if err != nil {
		return err
	}
	parsed, err := plain(structured)
	if err != nil {
		return err
	}
	result.Structured = string(text)
	result.StructuredJSON = parsed
	return nil
}

// errReadFormat and errPromptArgFormat are the shared texts of the mapper and argument errors.
const (
	errReadFormat      = "cannot read the %s: %w"
	errPromptArgFormat = "prompt argument %q must be a string, number or boolean, got %T"
)

type resourceEntry struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	MIMEType    string `json:"mimetype"`
	Description string `json:"description"`
}

type resourceContentEntry struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimetype"`
	Text     string `json:"text"`
	Blob     string `json:"blob"`
}

type promptEntry struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Arguments   []*sdk.PromptArgument `json:"arguments"`
}

type promptMessageEntry struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// plainList converts a slice of entries into the []interface{} the engine asserts on.
func plainList(what string, entries interface{}) ([]interface{}, error) {
	parsed, err := plain(entries)
	if err != nil {
		return nil, fmt.Errorf(errReadFormat, what, err)
	}
	list, ok := parsed.([]interface{})
	if !ok {
		return nil, fmt.Errorf(errReadFormat, what, fmt.Errorf("unexpected %T", parsed))
	}
	return list, nil
}

// resourcesListResult exposes the listed resources as result.resources.
func resourcesListResult(resources []*sdk.Resource) (Result, error) {
	entries := make([]resourceEntry, 0, len(resources))
	for _, r := range resources {
		if r == nil {
			continue // a server can answer null; the SDK client does not filter it out
		}
		entries = append(entries, resourceEntry{URI: r.URI, Name: r.Name, MIMEType: r.MIMEType, Description: r.Description})
	}
	list, err := plainList("resources", entries)
	return Result{Resources: list}, err
}

// resourceReadResult exposes each content item as result.resources; result.content holds one
// string per text item and result.text their join; a blob stays base64 text and does not reach result.content.
func resourceReadResult(uri string, read *sdk.ReadResourceResult) (Result, error) {
	entries := make([]resourceContentEntry, 0, len(read.Contents))
	var texts []interface{}
	var joined []string
	for _, c := range read.Contents {
		if c == nil {
			continue // see resourcesListResult
		}
		entry := resourceContentEntry{URI: c.URI, MIMEType: c.MIMEType, Text: c.Text}
		if len(c.Blob) > 0 {
			entry.Blob = base64.StdEncoding.EncodeToString(c.Blob)
		} else {
			texts = append(texts, c.Text)
			joined = append(joined, c.Text)
		}
		entries = append(entries, entry)
	}
	list, err := plainList(fmt.Sprintf("contents of resource %q", uri), entries)
	if err != nil {
		return Result{}, err
	}
	return Result{Resources: list, Content: texts, Text: strings.Join(joined, "\n")}, nil
}

// promptsListResult exposes the listed prompts as result.prompts.
func promptsListResult(prompts []*sdk.Prompt) (Result, error) {
	entries := make([]promptEntry, 0, len(prompts))
	for _, p := range prompts {
		if p == nil {
			continue // see resourcesListResult
		}
		entries = append(entries, promptEntry{Name: p.Name, Description: p.Description, Arguments: p.Arguments})
	}
	list, err := plainList("prompts", entries)
	return Result{Prompts: list}, err
}

// promptGetResult exposes the messages as result.prompts, their text joined as result.text and the
// prompt description as result.description.
func promptGetResult(name string, got *sdk.GetPromptResult) (Result, error) {
	entries := make([]promptMessageEntry, 0, len(got.Messages))
	var blocks []sdk.Content
	for _, m := range got.Messages {
		if m == nil {
			continue // see resourcesListResult
		}
		text := ""
		if m.Content != nil {
			text = textOf([]sdk.Content{m.Content})
			blocks = append(blocks, m.Content)
		}
		entries = append(entries, promptMessageEntry{Role: string(m.Role), Text: text})
	}
	list, err := plainList(fmt.Sprintf("messages of prompt %q", name), entries)
	if err != nil {
		return Result{}, err
	}
	return Result{Prompts: list, Description: got.Description, Text: textOf(blocks)}, nil
}

// promptArguments converts the step arguments into the strings prompts/get carries: strings are kept,
// numbers and booleans are written out, anything else is a misuse.
func promptArguments(arguments map[string]interface{}) (map[string]string, error) {
	if len(arguments) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(arguments))
	for k := range arguments {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]string, len(arguments))
	for _, k := range keys {
		text, ok := scalarText(arguments[k])
		if !ok {
			return nil, fmt.Errorf(errPromptArgFormat, k, arguments[k])
		}
		out[k] = text
	}
	return out, nil
}

// scalarText writes a string, number or boolean as text; ok is false for any other value.
func scalarText(v interface{}) (string, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return rv.String(), true
	case reflect.Bool:
		return strconv.FormatBool(rv.Bool()), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10), true
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'f', -1, 64), true
	default:
		return "", false
	}
}

// textOf joins the text blocks with a newline.
func textOf(blocks []sdk.Content) string {
	var texts []string
	for _, block := range blocks {
		if text, ok := block.(*sdk.TextContent); ok {
			texts = append(texts, text.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// plain converts an SDK value into the maps and slices the engine can dump and assert on.
func plain(v interface{}) (interface{}, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
