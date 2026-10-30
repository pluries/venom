package venom

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/ovh/venom/internal/yamljson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestProcessVariableAssignments(t *testing.T) {
	InitTestLogger(t)
	assign := AssignStep{}
	assign.Assignments = make(map[string]Assignment)
	assign.Assignments["assignVar"] = Assignment{
		From: "here.some.value",
	}
	assign.Assignments["assignVarWithRegex"] = Assignment{
		From:  "here.some.value",
		Regex: `this is (?s:(.*))`,
	}

	b, err := yaml.Marshal(assign)
	require.NoError(t, err)
	t.Log("\n" + string(b))

	tcVars := H{"here.some.value": "this is the \nvalue"}

	result, is, err := processVariableAssignments(context.TODO(), "", tcVars, b)
	assert.True(t, is)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	t.Log(result)
	assert.Equal(t, "map[assignVar:this is the \nvalue assignVarWithRegex:the \nvalue]", fmt.Sprint(result))

	var wrongStepIn TestStep
	b = []byte(`type: exec
script: echo 'foo'
`)
	assert.NoError(t, yaml.Unmarshal(b, &wrongStepIn))
	result, is, err = processVariableAssignments(context.TODO(), "", tcVars, b)
	assert.False(t, is)
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.Empty(t, result)
}

// TestInterpolateStepContentResolvesKnownVariables pins the happy path: known variables resolve
// in a single pass, exactly as they did through the old ten-iteration loop (#1981).
func TestInterpolateStepContentResolvesKnownVariables(t *testing.T) {
	vars := map[string]string{"name": "value"}
	content, err := interpolateStepContent([]byte(`hello {{.name}}`), newWholeScope(vars, vars))
	assert.NoError(t, err)
	assert.Equal(t, "hello value", content)
}

// TestInterpolateStepContentLeavesUnknownPlaceholderCharacterisation pins the exact behaviour the
// old ten-iteration loop had for the case it exists to handle: an unknown variable name (a
// misspelling, or template syntax from another tool that looks like venom's own). interpolate.Do
// deliberately leaves it untouched, so the result still contains "{{". Calling it again with the
// same raw content and vars reproduces the same result, because neither changed and the unknown
// path doesn't invoke anything non-reproducible (it isn't a claim that interpolate.Do is always
// deterministic — the rand* helper functions it exposes are not). This is the characterisation
// test the ticket asks for: it must keep passing, unchanged, after the fix.
func TestInterpolateStepContentLeavesUnknownPlaceholderCharacterisation(t *testing.T) {
	vars := map[string]string{"known": "value"}
	raw := []byte(`known={{.known}} unknown={{.doesnotexist}}`)

	content, err := interpolateStepContent(raw, newWholeScope(vars, vars))
	assert.NoError(t, err)
	assert.Equal(t, `known=value unknown={{.doesnotexist}}`, content)
	assert.Contains(t, content, "{{", "an unknown variable's placeholder is left as written")

	// A second call with the same raw content and vars must be byte-identical to the first —
	// this is exactly why the old loop's extra nine calls never changed anything.
	again, err := interpolateStepContent(raw, newWholeScope(vars, vars))
	assert.NoError(t, err)
	assert.Equal(t, content, again)
}

// bigStepFixture builds a step roughly matching the ticket's benchmark setup: ~40 KB of text with
// 60 variables. leftover controls whether one placeholder refers to a variable not present in
// vars, reproducing the "one left over per step" case that used to make the loop run all ten
// iterations instead of stopping after the first.
func bigStepFixture(leftover bool) (raw []byte, vars map[string]string) {
	vars = make(map[string]string, 60)
	var b strings.Builder
	b.WriteString("type: http\n")
	for i := 0; i < 60; i++ {
		name := "var" + strconv.Itoa(i)
		vars[name] = strings.Repeat("x", 40) + strconv.Itoa(i)
		fmt.Fprintf(&b, "field%d: {{.%s}}\n", i, name)
		// padding so the step is roughly 40 KB, like the ticket's measurement
		b.WriteString(strings.Repeat("# padding line to reach a realistic step size\n", 10))
	}
	if leftover {
		b.WriteString("extra: {{.doesnotexist}}\n")
	}
	return []byte(b.String()), vars
}

// BenchmarkInterpolateStepContent is the proof the ticket asks to keep: the "one left over"
// case used to cost roughly twice the "nothing left over" case (127ms vs 61ms on a 40 KB / 60
// variable step, measured 2026-09-21) because the old loop ran all ten passes instead of
// stopping after the first. With a single pass, the two cases cost the same.
func BenchmarkInterpolateStepContent(b *testing.B) {
	b.Run("NoPlaceholderLeftover", func(b *testing.B) {
		raw, vars := bigStepFixture(false)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := interpolateStepContent(raw, newWholeScope(vars, vars)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("OnePlaceholderLeftoverPerStep", func(b *testing.B) {
		raw, vars := bigStepFixture(true)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := interpolateStepContent(raw, newWholeScope(vars, vars)); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// TestParseRangedCharacterisation pins parseRanged before #2031 splits it: the items built for each range shape, the
// Enabled flag, what is left in RawContent, and the exact error texts. JSON numbers arrive as float64, the number form
// yields int values. Map ranges come from a map iteration, so their items are compared sorted by key.
func TestParseRangedCharacterisation(t *testing.T) {
	const cannotTransform = "unable to parse range expression: unable to transform string data into a supported range expression type"
	tests := []struct {
		name        string
		rawStep     string
		stepVars    H
		wantItems   []RangeData
		unordered   bool
		wantEnabled bool
		wantRaw     interface{}
		wantErr     string
	}{
		// Not ranged: one empty item so the step still runs once.
		{name: "no range key: one empty item, not enabled", rawStep: `{"type":"exec"}`, wantItems: []RangeData{{}}},
		{name: "a null range: one empty item, not enabled", rawStep: `{"range":null}`, wantItems: []RangeData{{}}},

		// Arrays.
		{
			name:        "an array: one item per element, keyed by index",
			rawStep:     `{"range":[1,"a",{"k":1}]}`,
			wantItems:   []RangeData{{"0", float64(1)}, {"1", "a"}, {"2", map[string]interface{}{"k": float64(1)}}},
			wantEnabled: true,
		},
		{name: "an empty array: no item, but enabled", rawStep: `{"range":[]}`, wantEnabled: true},

		// Numbers.
		{name: "a number n: n items 0..n-1 holding ints", rawStep: `{"range":3}`, wantItems: []RangeData{{"0", 0}, {"1", 1}, {"2", 2}}, wantEnabled: true},
		{name: "zero: no item, but enabled", rawStep: `{"range":0}`, wantEnabled: true},
		{name: "a non-integer is truncated", rawStep: `{"range":2.5}`, wantItems: []RangeData{{"0", 0}, {"1", 1}}, wantEnabled: true},
		{name: "a negative number: no item, but enabled", rawStep: `{"range":-2}`, wantEnabled: true},

		// Objects.
		{
			name:        "an object: one item per key",
			rawStep:     `{"range":{"a":1,"b":"x"}}`,
			wantItems:   []RangeData{{"a", float64(1)}, {"b", "x"}},
			unordered:   true,
			wantEnabled: true,
		},
		{name: "an empty object: no item, but enabled", rawStep: `{"range":{}}`, wantEnabled: true},

		// Strings holding JSON, parsed without templating.
		{name: "a string holding a JSON array", rawStep: `{"range":"[1,2]"}`, wantItems: []RangeData{{"0", float64(1)}, {"1", float64(2)}}, wantEnabled: true},
		{name: "a string holding a number", rawStep: `{"range":"2"}`, wantItems: []RangeData{{"0", 0}, {"1", 1}}, wantEnabled: true},
		{name: "a string holding a JSON object", rawStep: `{"range":"{\"a\":1}"}`, wantItems: []RangeData{{"a", float64(1)}}, wantEnabled: true},
		{name: "a string holding a JSON string", rawStep: `{"range":"\"x\""}`, wantRaw: "x", wantErr: `"range" was provided an unsupported type string`},
		{name: "a string holding a boolean", rawStep: `{"range":"true"}`, wantRaw: true, wantErr: `"range" was provided an unsupported type bool`},
		// quirk: the literal null means "not ranged", the string "null" is an error
		{name: "a string holding null", rawStep: `{"range":"null"}`, wantErr: `"range" was provided an unsupported type <nil>`},

		// Strings resolved from the step variables.
		{
			name:        "a template resolving to a JSON array string",
			rawStep:     `{"range":"{{.items}}"}`,
			stepVars:    H{"items": `["a","b"]`},
			wantItems:   []RangeData{{"0", "a"}, {"1", "b"}},
			wantEnabled: true,
		},
		{
			name:        "a template resolving to an array variable",
			rawStep:     `{"range":"{{.items}}"}`,
			stepVars:    H{"items": []interface{}{"a", "b"}},
			wantItems:   []RangeData{{"0", "a"}, {"1", "b"}},
			wantEnabled: true,
		},
		{
			name:        "a template resolving to a map variable",
			rawStep:     `{"range":"{{.m}}"}`,
			stepVars:    H{"m": map[string]interface{}{"k": "v", "l": "w"}},
			wantItems:   []RangeData{{"k", "v"}, {"l", "w"}},
			unordered:   true,
			wantEnabled: true,
		},
		{
			name:        "a template resolving to a JSON object string",
			rawStep:     `{"range":"{{.m}}"}`,
			stepVars:    H{"m": `{"k":"v"}`},
			wantItems:   []RangeData{{"k", "v"}},
			wantEnabled: true,
		},
		{
			name:        "a template resolving to a number",
			rawStep:     `{"range":"{{.n}}"}`,
			stepVars:    H{"n": 2},
			wantItems:   []RangeData{{"0", 0}, {"1", 1}},
			wantEnabled: true,
		},
		{
			name:        "a template building an array from several variables",
			rawStep:     `{"range":"[{{.a}},{{.b}}]"}`,
			stepVars:    H{"a": 1, "b": 2},
			wantItems:   []RangeData{{"0", float64(1)}, {"1", float64(2)}},
			wantEnabled: true,
		},
		{
			name:        "a quote in another variable of the step does not break the templating",
			rawStep:     `{"range":"{{.items}}","other":"{{.s}}"}`,
			stepVars:    H{"items": `[1]`, "s": `a"b`},
			wantItems:   []RangeData{{"0", float64(1)}},
			wantEnabled: true,
		},
		{name: "a template resolving to a plain string", rawStep: `{"range":"{{.s}}"}`, stepVars: H{"s": "abc"}, wantRaw: "abc", wantErr: cannotTransform},
		{
			name:     "a template resolving to a string with a quote",
			rawStep:  `{"range":"{{.s}}"}`,
			stepVars: H{"s": `with "quote"`},
			wantRaw:  `with "quote"`,
			wantErr:  cannotTransform,
		},
		{name: "a template resolving to a boolean", rawStep: `{"range":"{{.s}}"}`, stepVars: H{"s": "true"}, wantRaw: true, wantErr: `"range" was provided an unsupported type bool`},
		{name: "a template of an unknown variable stays as written", rawStep: `{"range":"{{.missing}}"}`, stepVars: H{}, wantRaw: "{{.missing}}", wantErr: cannotTransform},
		// quirk: the three "not a range" strings below fail with a different error than a plain string
		{name: "a template that does not parse", rawStep: `{"range":"{{ .x"}`, stepVars: H{}, wantRaw: "{{ .x", wantErr: `"range" was provided an unsupported type string`},
		{
			name:     "a variable with a newline breaks the templated JSON",
			rawStep:  `{"range":"{{.s}}"}`,
			stepVars: H{"s": "a\nb"},
			wantRaw:  "{{.s}}",
			wantErr:  `"range" was provided an unsupported type string`,
		},
		// quirk: quotes are escaped but backslashes are not, so an escaped quote in a JSON array variable breaks it
		{
			name:     "a JSON array variable holding an escaped quote breaks the templated JSON",
			rawStep:  `{"range":"{{.s}}"}`,
			stepVars: H{"s": `["a\"b"]`},
			wantRaw:  "{{.s}}",
			wantErr:  `"range" was provided an unsupported type string`,
		},
		{name: "a plain string without template", rawStep: `{"range":"abc"}`, wantRaw: "abc", wantErr: cannotTransform},
		{name: "an empty string", rawStep: `{"range":""}`, wantRaw: "", wantErr: "range expression has been specified without any data"},

		// Other types and invalid input.
		{name: "a boolean", rawStep: `{"range":true}`, wantRaw: true, wantErr: `"range" was provided an unsupported type bool`},
		{name: "truncated JSON", rawStep: `{"range":`, wantErr: "unable to parse range expression: unexpected end of JSON input"},
		{name: "not JSON", rawStep: `not json`, wantErr: "unable to parse range expression: invalid character 'o' in literal null (expecting 'u')"},
		{name: "a JSON array step", rawStep: `[1]`, wantErr: "unable to parse range expression: json: cannot unmarshal array into Go value of type venom.Range"},

		// quirk: Range's Enabled and Items fields have no JSON tag, so step keys named enabled / items (any case) fill them
		{name: "a step key 'enabled' enables a step without range", rawStep: `{"enabled":true}`, wantItems: []RangeData{{}}, wantEnabled: true},
		{
			name:        "a step key 'Enabled' stays set on the error path",
			rawStep:     `{"Enabled":true,"range":"abc"}`,
			wantEnabled: true,
			wantRaw:     "abc",
			wantErr:     cannotTransform,
		},
		{
			name:        "a step key 'items' is kept before the range items",
			rawStep:     `{"items":[{"Key":"x","Value":1}],"range":[5]}`,
			wantItems:   []RangeData{{"x", float64(1)}, {"0", float64(5)}},
			wantEnabled: true,
		},
		{
			name:    "a step key 'items' of another shape is a parse error",
			rawStep: `{"items":"bad"}`,
			wantErr: "unable to parse range expression: json: cannot unmarshal string into Go struct field Range.items of type []venom.RangeData",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitTestLogger(t)
			got, err := parseRanged(context.Background(), []byte(tt.rawStep), func() H { return tt.stepVars })
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErr)
			}
			if tt.unordered {
				sort.Slice(got.Items, func(i, j int) bool { return got.Items[i].Key < got.Items[j].Key })
			}
			assert.Equal(t, tt.wantItems, got.Items)
			assert.Equal(t, tt.wantEnabled, got.Enabled)
			assert.Equal(t, tt.wantRaw, got.RawContent)
		})
	}
}

// zeroValueExecutor stands for exec in the parseTestCase tests: the real executors live in package executors, which
// imports venom. Its zero-value result holds a map, the shape the __Type__ workaround in parseTestCase exists for.
type zeroValueExecutor struct{}

func (zeroValueExecutor) Run(context.Context, TestStep) (interface{}, error) { return nil, nil }

func (zeroValueExecutor) ZeroValueResult() interface{} {
	type Result struct {
		Code    int               `json:"code"`
		Headers map[string]string `json:"headers"`
	}
	return Result{}
}

// zeroValueVars is what one zeroValueExecutor step of test case "tc" adds to the extracted variables: every key of its
// dumped zero value, bare and prefixed with the test case name, plus the trimmed map name (trailing dot included).
var zeroValueVars = []string{
	"__type__", "tc.__type__",
	"result.__Len__", "tc.result.__Len__",
	"result.code", "tc.result.code",
	"result.headers", "tc.result.headers",
	"result.headers.__Len__", "tc.result.headers.__Len__",
	"result.headers.__Type__", "tc.result.headers.__Type__",
	"tc.result.headers.",
}

// sortedCopy returns a sorted copy of the given slices joined together.
func sortedCopy(parts ...[]string) []string {
	out := []string{}
	for _, p := range parts {
		out = append(out, p...)
	}
	sort.Strings(out)
	return out
}

// testCaseFromYAML builds a TestCase the way readFiles does: the YAML goes through internal/yamljson, so every step
// becomes a json.RawMessage.
func testCaseFromYAML(t *testing.T, content string, vars H) *TestCase {
	t.Helper()
	var in TestCaseInput
	require.NoError(t, yamljson.Unmarshal([]byte(content), &in))
	in.Vars = vars
	return &TestCase{TestCaseInput: in}
}

// TestParseTestCaseCharacterisation pins parseTestCase before #2031 splits it: the "used but not defined" variables
// (first result) and the extracted variables (second result). Both are built from map iterations, so they are
// compared sorted; duplicates are kept, since the function keeps them.
//
// quirk: the first result is always empty. varRegEx has no capture group, so FindStringSubmatch returns only the whole
// match, and the loop skips index 0: no template ever reaches vars. Every case below asserts it.
func TestParseTestCaseCharacterisation(t *testing.T) {
	tests := []struct {
		name          string
		yaml          string
		vars          H
		wantExtracted []string
		wantErr       string
	}{
		{name: "no steps", yaml: "name: tc\nsteps: []\n", wantExtracted: []string{}},
		{name: "a step without zero value, without template", yaml: "name: tc\nsteps:\n- type: plain\n  script: echo\n", wantExtracted: []string{}},
		{
			name:          "a step with a zero-value result extracts its keys, bare and prefixed",
			yaml:          "name: tc\nsteps:\n- type: exec\n  script: echo\n",
			wantExtracted: sortedCopy(zeroValueVars),
		},
		{
			name:          "an untyped step with a script is exec",
			yaml:          "name: tc\nsteps:\n- script: echo {{.u}}\n",
			wantExtracted: sortedCopy(zeroValueVars),
		},
		{
			name:          "two exec steps extract everything twice",
			yaml:          "name: tc\nsteps:\n- type: exec\n  script: echo\n- type: exec\n  script: echo\n",
			wantExtracted: sortedCopy(zeroValueVars, zeroValueVars),
		},
		{
			name:          "a template on an undefined variable is not reported",
			yaml:          "name: tc\nsteps:\n- type: plain\n  script: echo {{.undefined}} {{ .spaced }}\n  list: [\"{{.x}}\", \"{{.y}}\"]\n",
			wantExtracted: []string{},
		},
		{
			name:          "a template on a defined variable",
			yaml:          "name: tc\nsteps:\n- type: plain\n  script: echo {{.known}}\n",
			vars:          H{"known": "k"},
			wantExtracted: []string{},
		},
		{
			name:          "a template on an extracted variable",
			yaml:          "name: tc\nsteps:\n- type: exec\n  script: echo\n- type: plain\n  script: echo {{.tc.result.code}} {{.tc.result.headers.x}}\n",
			wantExtracted: sortedCopy(zeroValueVars),
		},
		{
			name: "vars and extracts blocks extract their first-level names, once per dumped key",
			yaml: "name: tc\nsteps:\n- type: plain\n  vars:\n    out:\n      from: result.systemout\n  extracts:\n    ex: x\n" +
				"- type: plain\n  script: echo {{.tc.out}} {{.tc.ex}}\n",
			// vars.out, vars.out.from, vars.out.__Len__, vars.out.__Type__ → tc.out four times; vars.__Len__ and
			// extracts.__Len__ → tc.__Len__ twice, same for __Type__.
			wantExtracted: []string{"tc.__Len__", "tc.__Len__", "tc.__Type__", "tc.__Type__", "tc.ex", "tc.out", "tc.out", "tc.out", "tc.out"},
		},
		{
			name:          "range and info keys add nothing",
			yaml:          "name: tc\nsteps:\n- type: plain\n  range: [1, \"{{.r}}\"]\n  info: \"{{.i}}\"\n  infos: [\"{{.j}}\"]\n",
			wantExtracted: []string{},
		},
		{
			name:          "the test case name prefixes the extracted variables as it is",
			yaml:          "name: My Case\nsteps:\n- type: plain\n  extracts:\n    ex: x\n",
			wantExtracted: []string{"My Case.__Len__", "My Case.__Type__", "My Case.ex"},
		},
		{
			name:          "a variable with quotes is escaped before the step is parsed",
			yaml:          "name: tc\nsteps:\n- type: plain\n  script: \"{{.q}}\"\n",
			vars:          H{"q": `say "hi"`},
			wantExtracted: []string{},
		},
		// quirk: only values holding a quote are escaped, so a trailing backslash escapes the closing quote
		{
			name:    "a variable ending with a backslash breaks the step",
			yaml:    "name: tc\nsteps:\n- type: plain\n  script: \"{{.q}}\"\n",
			vars:    H{"q": `a\`},
			wantErr: "unable to unmarshal teststep: error converting YAML to JSON: yaml: did not find expected ',' or '}'",
		},
		{
			name:    "an interpolation error",
			yaml:    "name: tc\nsteps:\n- type: plain\n  script: \"{{ .x\"\n",
			wantErr: `invalid template format "{"script":"{{ .x","type":"plain"}": template: input:1: bad character U+0022 '"'`,
		},
		{
			name:    "an unknown executor type",
			yaml:    "name: tc\nsteps:\n- type: unknown\n",
			wantErr: `user executor "unknown" not found - loaded executors are: [user]`,
		},
		{
			name:    "an invalid runner setting",
			yaml:    "name: tc\nsteps:\n- type: plain\n  retry: abc\n",
			wantErr: `attribute "retry" is not an integer`,
		},
		{
			name:    "the first failing step stops the parse, later steps are not read",
			yaml:    "name: tc\nsteps:\n- type: exec\n  script: echo\n- type: unknown\n- type: exec\n",
			wantErr: `user executor "unknown" not found - loaded executors are: [user]`,
		},
		// quirk: the root keys are lower-cased (__len__, __type__), so the __Type__ workaround only fires one level down
		{
			name: "a user executor extracts the keys of its output",
			yaml: "name: tc\nsteps:\n- type: user\n",
			wantExtracted: sortedCopy([]string{
				"__len__", "tc.__len__", "__type__", "tc.__type__",
				"result", "tc.result",
				"result.__Len__", "tc.result.__Len__", "result.__Type__", "tc.result.__Type__", "tc.result.",
				"result.foo", "tc.result.foo",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitTestLogger(t)
			v := New()
			v.RegisterExecutorBuiltin("exec", zeroValueExecutor{})
			v.RegisterExecutorBuiltin("plain", sleeperExecutor{})
			require.NoError(t, v.RegisterExecutorUser("user", UserExecutor{Executor: "user", Output: json.RawMessage(`{"foo":"bar"}`)}))
			tc := testCaseFromYAML(t, tt.yaml, tt.vars)

			vars, extracted, err := v.parseTestCase(nil, tc)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Nil(t, vars)
				assert.Nil(t, extracted)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{}, vars)
			assert.Equal(t, tt.wantExtracted, sortedCopy(extracted))
		})
	}
}

// TestEscapeForJSONMatchesStrconvQuote pins escapeForJSON (#2465) to the strconv.Quote it replaces on the hot path.
func TestEscapeForJSONMatchesStrconvQuote(t *testing.T) {
	for _, s := range []string{
		``, `"`, `say "hi"`, `a\b"c`, `\"`, `tab	"x"`, "line\n\"x\"", "nul\x00\"", "del\x7f\"", "é\"ü", "日本\"語",
		"\xff\"", `{"a":{"b":"c\\d"}}`, strings.Repeat(`"x\`, 50),
	} {
		want := strings.TrimSuffix(strings.TrimPrefix(strconv.Quote(s), `"`), `"`)
		require.Equal(t, want, escapeForJSON(s), "%q", s)
	}
}
