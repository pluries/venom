package venom

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type decodeEmbedded struct {
	Labels map[string]string
}

type decodeNested struct {
	Name string
}

type decodeShapes struct {
	decodeEmbedded `mapstructure:",squash"`
	Tagged         map[string]string `mapstructure:"query_parameters"`
	Items          []string
	List           [2]string
	Nested         decodeNested
	NestedPtr      *decodeNested
	Any            interface{}
	Text           string
	Raw            []byte
	Count          int
	Skipped        map[string]string `mapstructure:"-"`
	hidden         map[string]string
}

func (d decodeShapes) Run(_ context.Context, _ TestStep) (interface{}, error) {
	return len(d.hidden), nil
}

func decodedStep(step TestStep) TestStep {
	decodeStepFields(decodeShapes{}, step)
	return step
}

func TestDecodeStepFieldsConverts(t *testing.T) {
	step := decodedStep(TestStep{
		"labels":           `{"a":"1"}`,
		"query_parameters": ` {"q":"x"} `,
		"ITEMS":            `["a","b"]`,
		"list":             `["a","b"]`,
		"nested":           `{"name":"n"}`,
		"nestedptr":        `{"name":"n"}`,
	})
	assert.Equal(t, map[string]interface{}{"a": "1"}, step["labels"])
	assert.Equal(t, map[string]interface{}{"q": "x"}, step["query_parameters"])
	assert.Equal(t, []interface{}{"a", "b"}, step["ITEMS"])
	assert.Equal(t, []interface{}{"a", "b"}, step["list"])
	assert.Equal(t, map[string]interface{}{"name": "n"}, step["nested"])
	assert.Equal(t, map[string]interface{}{"name": "n"}, step["nestedptr"])
}

func TestDecodeStepFieldsLeavesAsIs(t *testing.T) {
	doc := `{"a":"b"}`
	for name, step := range map[string]TestStep{
		"interface field":    {"any": doc},
		"string field":       {"text": doc},
		"byte slice":         {"raw": `["a"]`},
		"int field":          {"count": `[1]`},
		"tag minus":          {"skipped": doc, "Skipped": doc},
		"tag renames field":  {"tagged": doc},
		"unexported":         {"hidden": doc},
		"unknown key":        {"nope": doc},
		"invalid json":       {"labels": `{"a":`},
		"trailing data":      {"labels": doc + " x"},
		"two documents":      {"labels": doc + doc},
		"not a container":    {"labels": `"a"`},
		"null":               {"labels": `null`},
		"number":             {"labels": `12`},
		"empty":              {"labels": ""},
		"templates in value": {"labels": `{"a":"{{ .x }}"}`, "items": `{{ .x }}`},
		"non string value":   {"labels": 12, "items": []string{"a"}},
	} {
		want := TestStep{}
		for k, v := range step {
			want[k] = v
		}
		if name == "templates in value" {
			want["labels"] = map[string]interface{}{"a": "{{ .x }}"}
		}
		assert.Equal(t, want, decodedStep(step), name)
	}
}

func TestDecodeStepFieldsEdgeValues(t *testing.T) {
	step := decodedStep(TestStep{
		"labels": `{}`,
		"items":  `[]`,
		"nested": `{"name":"é世😀"}`,
	})
	assert.Equal(t, map[string]interface{}{}, step["labels"])
	assert.Equal(t, []interface{}{}, step["items"])
	assert.Equal(t, map[string]interface{}{"name": "é世😀"}, step["nested"])

	big := TestStep{"items": "[" + strings.Repeat(`"x",`, 100000) + `"x"]`}
	require.NotPanics(t, func() { decodeStepFields(decodeShapes{}, big) })
	assert.Len(t, big["items"], 100001)

	huge := TestStep{"labels": `{"n":12345678901234567890}`}
	decodeStepFields(&decodeShapes{}, huge)
	assert.Equal(t, map[string]interface{}{"n": json.Number("12345678901234567890")}, huge["labels"])
}

func TestDecodeStepFieldsNoPanic(t *testing.T) {
	step := TestStep{"labels": `{"a":"1"}`}
	require.NotPanics(t, func() {
		decodeStepFields(nil, step)
		decodeStepFields(stepDefaultsExecutor{}, step)
		decodeStepFields(UserExecutor{}, step)
		decodeStepFields(sleeperExecutor{}, step)
		decodeStepFields(&decodeShapes{}, nil)
	})
	assert.Equal(t, `{"a":"1"}`, step["labels"], "unchanged")
}
