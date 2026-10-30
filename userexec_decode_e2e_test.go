package venom

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeTarget is an executor with a map field, a list field and string fields, the way http has Headers and Body. Run
// keeps the step it received and decodes it with mapstructure as the real executors do.
type decodeTarget struct {
	Headers map[string]string
	Tags    []string
	Body    string
	Script  string
	Stdin   string
	Info    string

	got TestStep
}

func (d *decodeTarget) Run(_ context.Context, step TestStep) (interface{}, error) {
	d.got = TestStep{}
	for k, v := range step {
		d.got[k] = v
	}
	var into decodeTarget
	if err := mapstructure.Decode(step, &into); err != nil {
		return nil, err
	}
	return stepResult(0, "", ""), nil
}

const decodeBody = `executor: Call
steps:
- type: target
  headers: {{ .headers }}
  tags: {{ .tags }}
  body: {{ .payload }}
  script: {{ .payload }}
  stdin: {{ .payload }}
  info: {{ .payload }}
`

// decodeRun runs the user executor decodeBody from a suite holding vars and returns what the target received and what the
// user executor step reported.
func decodeRun(ctx context.Context, t *testing.T, body string, vars H) (*decodeTarget, stepUserRun) {
	t.Helper()
	v, _ := stepNewVenom(t, 1)
	target := &decodeTarget{}
	v.RegisterExecutorBuiltin("target", target)
	ux := stepUserExecutor(body)
	tsIn := &TestStepResult{}
	result, err := v.RunUserExecutor(ctx, newExecutorRunner(ux, ux.Executor, "user", 0, nil, 0, 0, nil), stepUserCaller(vars), tsIn, TestStep{"type": "Call"})
	return target, stepUserRun{result: result, err: err, tsIn: tsIn}
}

func decodeFailures(r stepUserRun) string {
	var all []string
	for _, f := range r.tsIn.Errors {
		all = append(all, f.Value)
	}
	return strings.Join(all, "\n")
}

// #3304: a map variable reaches a map field as a map, a list variable a list field as a list; string fields keep the text.
func TestUserExecutorMapVariableReachesMapField(t *testing.T) {
	vars := H{
		"headers": map[string]interface{}{"X-A": "1", "X-B": "two"},
		"tags":    []interface{}{"a", "b"},
		"payload": map[string]interface{}{"k": "v"},
	}
	target, r := decodeRun(context.Background(), t, decodeBody, vars)
	require.NoError(t, r.err, decodeFailures(r))
	assert.Equal(t, map[string]interface{}{"X-A": "1", "X-B": "two"}, target.got["headers"])
	assert.Equal(t, []interface{}{"a", "b"}, target.got["tags"])
	for _, key := range []string{"body", "script", "stdin", "info"} {
		assert.JSONEq(t, `{"k":"v"}`, target.got[key].(string), key) //nolint:errcheck,forcetypeassert // an assertion failure is the report
	}
}

// F2: a backslash in a map value reaches the executor unchanged.
func TestUserExecutorMapBackslashSurvives(t *testing.T) {
	vars := H{
		"headers": map[string]interface{}{"X-A": "C:\\temp\\new"},
		"tags":    []interface{}{"a"},
		"payload": "plain",
	}
	target, r := decodeRun(context.Background(), t, decodeBody, vars)
	require.NoError(t, r.err, decodeFailures(r))
	assert.Equal(t, map[string]interface{}{"X-A": `C:\temp\new`}, target.got["headers"], "no corruption of a backslash")
}

// F3: a declared secret inside a map is not in the log in its raw form at the debug level. The JSON-escaped form
// (<) is written by the engine before this code runs, on the base binary too: not asserted here.
func TestUserExecutorMapSecretStaysOutOfTheLog(t *testing.T) {
	secret := "Pa" + "<s>&x"
	ctx := context.WithValue(context.Background(), ContextKey("secrets"), []string{secret})
	vars := H{
		"headers": map[string]interface{}{"X-A": secret},
		"tags":    []interface{}{"a"},
		"payload": "plain",
	}
	var buf bytes.Buffer
	v, _ := stepNewVenom(t, 1)
	logger.Logger.SetOutput(&buf)
	previous := logger.Logger.GetLevel()
	logger.Logger.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() { logger.Logger.SetLevel(previous) })
	target := &decodeTarget{}
	v.RegisterExecutorBuiltin("target", target)
	ux := stepUserExecutor(decodeBody)
	_, _ = v.RunUserExecutor(ctx, newExecutorRunner(ux, ux.Executor, "user", 0, nil, 0, 0, nil), stepUserCaller(vars), &TestStepResult{}, TestStep{"type": "Call"}) //nolint:errcheck // only the log matters here
	log := buf.String()
	assert.NotContains(t, log, secret)
	assert.NotContains(t, log, `Pa\u003cs\u003e\u0026x`)
}
