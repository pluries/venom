package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/ovh/venom"
)

// Name for test mcp
const Name = "mcp"

const (
	transportStdio = "stdio"
	transportHTTP  = "http"

	// defaultProtocolVersion is pinned so that a test suite does not change behaviour when the SDK
	// moves its default to a newer revision of the protocol.
	defaultProtocolVersion = "2025-11-25"

	// defaultTimeout applies only when the engine set no deadline (no `timeout` on the step).
	defaultTimeout = 30 * time.Second
)

// New returns a new Test mcp
func New() venom.Executor {
	return &Executor{}
}

// Executor represents a Test mcp
type Executor struct {
	Transport       string                 `json:"transport" yaml:"transport"`
	Command         []string               `json:"command" yaml:"command"`
	URL             string                 `json:"url" yaml:"url"`
	Headers         map[string]string      `json:"headers" yaml:"headers"`
	Method          string                 `json:"method" yaml:"method"`
	Tool            string                 `json:"tool" yaml:"tool"`
	URI             string                 `json:"uri" yaml:"uri"`
	Prompt          string                 `json:"prompt" yaml:"prompt"`
	Arguments       map[string]interface{} `json:"arguments" yaml:"arguments" mapstructure:"-"`
	ProtocolVersion string                 `json:"protocol_version" yaml:"protocol_version" mapstructure:"protocol_version"`
}

// ServerInfo is the name and version the server announced in its initialize answer.
type ServerInfo struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
}

// Result represents a step result
type Result struct {
	Content         []interface{} `json:"content,omitempty" yaml:"content,omitempty"`
	Text            string        `json:"text,omitempty" yaml:"text,omitempty"`
	Structured      string        `json:"structured,omitempty" yaml:"structured,omitempty"`
	StructuredJSON  interface{}   `json:"structuredjson,omitempty" yaml:"structuredjson,omitempty"`
	Tools           string        `json:"tools,omitempty" yaml:"tools,omitempty"`
	ToolsJSON       interface{}   `json:"toolsjson,omitempty" yaml:"toolsjson,omitempty"`
	Resources       []interface{} `json:"resources,omitempty" yaml:"resources,omitempty"`
	Prompts         []interface{} `json:"prompts,omitempty" yaml:"prompts,omitempty"`
	Description     string        `json:"description,omitempty" yaml:"description,omitempty"`
	ServerInfo      *ServerInfo   `json:"serverinfo,omitempty" yaml:"serverinfo,omitempty"`
	Capabilities    interface{}   `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	ProtocolVersion string        `json:"protocolversion,omitempty" yaml:"protocolversion,omitempty"`
	IsError         bool          `json:"iserror" yaml:"iserror"`
	Err             string        `json:"err" yaml:"err"`
	TimeSeconds     float64       `json:"timeseconds" yaml:"timeseconds"`
}

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return Result{}
}

// GetDefaultAssertions return default assertions for type mcp
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.err ShouldBeEmpty"}}
}

// Run execute TestStep of type mcp
func (Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	// mapstructure does not decode a map[interface{}]interface{}, the shape YAML gives nested maps
	arguments, err := normaliseArguments(step["arguments"])
	if err != nil {
		return nil, err
	}
	e.Arguments = arguments

	if err := e.validate(); err != nil {
		return nil, err
	}

	start := time.Now()
	result := e.execute(ctx)
	result.TimeSeconds = time.Since(start).Seconds()
	return result, nil
}

// validate reports a misuse of the step; every other failure belongs to the result.
func (e Executor) validate() error {
	switch e.Transport {
	case transportStdio:
		if len(e.Command) == 0 {
			return fmt.Errorf("transport %s needs a 'command'", transportStdio)
		}
	case transportHTTP:
		if e.URL == "" {
			return fmt.Errorf("transport %s needs a 'url'", transportHTTP)
		}
	default:
		return fmt.Errorf("unknown transport %q, use %s or %s", e.Transport, transportStdio, transportHTTP)
	}
	return e.validateMethod()
}

// execute opens one session, runs the method and closes the session. Failures go to Result.Err.
func (e Executor) execute(ctx context.Context) Result {
	ctx, cancel := withDefaultTimeout(ctx)
	defer cancel()

	session, flush, err := e.connect(ctx)
	defer flush()
	if err != nil {
		return Result{Err: fmt.Sprintf("cannot connect: %v", err)}
	}
	defer session.Close()

	result, err := methods[e.Method](ctx, e, session)
	applyInitialize(&result, session.InitializeResult())
	if err != nil {
		result.Err = err.Error()
	}
	return result
}

// withDefaultTimeout adds defaultTimeout when ctx has no deadline; an existing deadline is kept.
func withDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {
			// the caller owns this context and its deadline: nothing for us to cancel
		}
	}
	return context.WithTimeout(ctx, defaultTimeout)
}

// normaliseArguments turns the YAML maps of the step into JSON-encodable ones.
func normaliseArguments(raw interface{}) (map[string]interface{}, error) {
	if raw == nil {
		return nil, nil
	}
	arguments, ok := normaliseValue(raw).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("'arguments' must be a map, got %T", raw)
	}
	return arguments, nil
}

func normaliseValue(v interface{}) interface{} {
	switch typed := v.(type) {
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(typed))
		for k, item := range typed {
			out[fmt.Sprint(k)] = normaliseValue(item)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for k, item := range typed {
			out[k] = normaliseValue(item)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, item := range typed {
			out[i] = normaliseValue(item)
		}
		return out
	default:
		return v
	}
}
