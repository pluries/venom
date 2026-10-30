package yamljson

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the behaviour of the bridge ported from github.com/rockbears/yaml v0.4.0
// (YAML -> generic object -> JSON -> encoding/json). They ran green against that library first, with one
// known difference: TestMarshalLeadingNewline passes only against the port, because gopkg.in/yaml.v3,
// under rockbears, dropped a leading newline when encoding a string (go-yaml issue 755).

type rawJSON struct{ Raw string }

func (r *rawJSON) UnmarshalJSON(b []byte) error {
	r.Raw = string(b)
	return nil
}

type target struct {
	Name   string            `json:"name"`
	Count  int               `json:"count"`
	Ratio  float64           `json:"ratio"`
	Tags   map[string]string `json:"tags"`
	Items  []string          `json:"items"`
	Custom rawJSON           `json:"custom"`
	Inner  *struct {
		Label string `json:"label"`
	} `json:"inner"`
	NoTag string
	Skip  string `json:"-"`
}

func decode(t *testing.T, y string) target {
	t.Helper()
	var got target
	require.NoError(t, unmarshal([]byte(y), &got))
	return got
}

func TestUnmarshalScalarIntoString(t *testing.T) {
	cases := map[string]string{
		"name: 12":       "12",
		"name: -7":       "-7",
		"name: 1.5":      "1.5",
		"name: true":     "true",
		"name: false":    "false",
		"name: 0x1F":     "31",
		"name: 1e3":      "1000",
		"name: text":     "text",
		"name: '12'":     "12",
		"name: \"true\"": "true",
	}
	for in, want := range cases {
		assert.Equal(t, want, decode(t, in).Name, in)
	}
}

func TestUnmarshalNestedCoercion(t *testing.T) {
	got := decode(t, "tags:\n  a: 1\n  b: true\n  c: 2.5\nitems: [1, false, x]\ninner:\n  label: 42\n")
	assert.Equal(t, map[string]string{"a": "1", "b": "true", "c": "2.5"}, got.Tags)
	assert.Equal(t, []string{"1", "false", "x"}, got.Items)
	require.NotNil(t, got.Inner)
	assert.Equal(t, "42", got.Inner.Label)
}

func TestUnmarshalNumbersStayNumbers(t *testing.T) {
	got := decode(t, "count: 3\nratio: 0.25\n")
	assert.Equal(t, 3, got.Count)
	assert.InDelta(t, 0.25, got.Ratio, 0)
}

func TestUnmarshalNonStringKeys(t *testing.T) {
	var got map[string]string
	require.NoError(t, unmarshal([]byte("1: a\n2.5: b\ntrue: c\nfalse: d\nx: e\n"), &got))
	assert.Equal(t, map[string]string{"1": "a", "2.5": "b", "true": "c", "false": "d", "x": "e"}, got)
}

func TestUnmarshalUnsupportedKey(t *testing.T) {
	var got map[string]string
	err := unmarshal([]byte("null: a\nx: b\n"), &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported map key of type: %!s(<nil>), key: <nil>")
}

func TestUnmarshalJSONTypeKeepsRawValue(t *testing.T) {
	assert.Equal(t, "12", decode(t, "custom: 12").Custom.Raw)
	assert.Equal(t, "true", decode(t, "custom: true").Custom.Raw)
	assert.Equal(t, `"x"`, decode(t, "custom: x").Custom.Raw)
}

func TestUnmarshalFieldMatching(t *testing.T) {
	assert.Equal(t, "a", decode(t, "NAME: a").Name)
	assert.Equal(t, "b", decode(t, "Name: b").Name)
	assert.Equal(t, "7", decode(t, "NAME: 7").Name)
	assert.Equal(t, "c", decode(t, "nOtAg: c").NoTag)
	assert.Equal(t, "", decode(t, "skip: d").Skip)
}

func TestUnmarshalIntoInterface(t *testing.T) {
	var got interface{}
	require.NoError(t, unmarshal([]byte("a: 1\nb: [x, 2]\n"), &got))
	assert.Equal(t, map[string]interface{}{"a": float64(1), "b": []interface{}{"x", float64(2)}}, got)
}

func TestUnmarshalErrors(t *testing.T) {
	var got target
	err := unmarshal([]byte("name: [1, 2\n"), &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error converting YAML to JSON: yaml: line ")

	err = unmarshal([]byte("count: abc\n"), &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal string into Go struct field")
	assert.NotContains(t, err.Error(), "error converting YAML to JSON")
}

func TestUnmarshalEmptyInput(t *testing.T) {
	// The bridge returns a stack-wrapped EOF, not io.EOF itself: callers must test it with errors.Is (cmd/venom/run/cmd.go does, since #2975).
	var got target
	for _, in := range []string{"", "   \n", "# comment only\n"} {
		err := unmarshal([]byte(in), &got)
		require.Error(t, err, in)
		assert.True(t, errors.Is(err, io.EOF), in)
		assert.Equal(t, "EOF", err.Error(), in)
		assert.NotSame(t, io.EOF, err, in)
	}
}

func TestMarshal(t *testing.T) {
	in := target{Name: "x", Count: 3, Ratio: 1.5, Tags: map[string]string{"b": "2", "a": "1"}, Items: []string{"p", "q"}}
	out, err := marshal(in)
	require.NoError(t, err)
	want := "NoTag: \"\"\ncount: 3\ncustom:\n  Raw: \"\"\ninner: null\nitems:\n  - p\n  - q\nname: x\nratio: 1.5\ntags:\n  a: \"1\"\n  b: \"2\"\n"
	assert.Equal(t, want, string(out))
}

func TestMarshalUnsupported(t *testing.T) {
	_, err := marshal(make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported type")
}

func TestJSONToYAML(t *testing.T) {
	out, err := jsonToYAML([]byte(`{"b":{"z":1,"y":[1,"two",true,null]},"a":1.5,"n":10}`))
	require.NoError(t, err)
	assert.Equal(t, "a: 1.5\nb:\n  \"y\":\n    - 1\n    - two\n    - true\n    - null\n  z: 1\n\"n\": 10\n", string(out))
}

func TestJSONToYAMLInvalid(t *testing.T) {
	_, err := jsonToYAML([]byte("{a: [1,"))
	require.Error(t, err)
}

func TestYAMLToJSON(t *testing.T) {
	cases := map[string]string{
		`{"b":1,"a":[true,"x"]}`:          `{"a":[true,"x"],"b":1}`,
		"a: 1\nb: [x, 2.5]\nc: {d: null}": `{"a":1,"b":["x",2.5],"c":{"d":null}}`,
		"1: a\n2.5: b\ntrue: c":           `{"1":"a","2.5":"b","true":"c"}`,
		"- 1\n- {2: x}\n":                 `[1,{"2":"x"}]`,
		"plain":                           `"plain"`,
		"12":                              `12`,
	}
	for in, want := range cases {
		out, err := yamlToJSON([]byte(in))
		require.NoError(t, err, in)
		assert.Equal(t, want, string(out), in)
	}
}

func TestYAMLToJSONErrors(t *testing.T) {
	_, err := yamlToJSON([]byte("a: [1,\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml: ")

	_, err = yamlToJSON(nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, io.EOF))
}

type EmbU struct {
	Name string
}

type EmbT struct {
	Name string `json:"Name"`
}

type taggedWins struct {
	EmbU
	EmbT
}

type EmbX struct{ Name string }

type EmbY struct{ Name string }

type untaggedClash struct {
	EmbX
	EmbY
}

type Base struct{ ID string }

type Left struct{ Base }

type Right struct{ Base }

type twice struct {
	Left
	Right
}

func TestUnmarshalEmbeddedTaggedWins(t *testing.T) {
	var got taggedWins
	require.NoError(t, unmarshal([]byte("Name: x\n"), &got))
	assert.Equal(t, "x", got.EmbT.Name)
	assert.Equal(t, "", got.EmbU.Name)

	// Quirk, pinned: the bridge looks up the embedded struct, not the promoted field, so a number
	// is not coerced to a string there.
	err := unmarshal([]byte("Name: 5\n"), &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal number")
}

func TestUnmarshalEmbeddedClash(t *testing.T) {
	var got untaggedClash
	require.NoError(t, unmarshal([]byte("name: 5\n"), &got))
	assert.Equal(t, untaggedClash{}, got)
}

func TestUnmarshalSameTypeEmbeddedTwice(t *testing.T) {
	var got twice
	require.NoError(t, unmarshal([]byte("id: 7\n"), &got))
	assert.Equal(t, twice{}, got)
}

type folded struct {
	Skin   string `json:"skin"`
	Key    string `json:"key"`
	Under  string `json:"a_b"`
	Plain  string `json:"plain"`
	Accent string `json:"café"`
}

func TestUnmarshalCaseFolding(t *testing.T) {
	cases := []struct {
		in   string
		pick func(folded) string
		want string
	}{
		{"\u017fkin: 5", func(f folded) string { return f.Skin }, "5"},
		{"SKIN: 6", func(f folded) string { return f.Skin }, "6"},
		{"\u212aey: 7", func(f folded) string { return f.Key }, "7"},
		{"KEY: 8", func(f folded) string { return f.Key }, "8"},
		{"A_B: 9", func(f folded) string { return f.Under }, "9"},
		{"a_B: 10", func(f folded) string { return f.Under }, "10"},
		{"PLAIN: 11", func(f folded) string { return f.Plain }, "11"},
		{"CAFÉ: 12", func(f folded) string { return f.Accent }, "12"},
	}
	for _, c := range cases {
		var got folded
		require.NoError(t, unmarshal([]byte(c.in), &got), c.in)
		assert.Equal(t, c.want, c.pick(got), c.in)
	}
}

func TestUnmarshalCaseFoldingMisses(t *testing.T) {
	for _, in := range []string{"xkin: 5", "a-b: 5", "kez: 5", "\u017fkinx: 5", "plainx: 5"} {
		var got folded
		require.NoError(t, unmarshal([]byte(in), &got), in)
		assert.Equal(t, folded{}, got, in)
	}
}

func TestUnmarshalInterfaceHoldingPointer(t *testing.T) {
	var got interface{} = &target{}
	require.NoError(t, unmarshal([]byte("name: 5\n"), &got))
	p, ok := got.(*target)
	require.True(t, ok)
	assert.Equal(t, "5", p.Name)
}

type upperText string

func (u *upperText) UnmarshalText(b []byte) error {
	*u = upperText("T:" + string(b))
	return nil
}

type withText struct {
	Text upperText `json:"text"`
}

func TestUnmarshalTextUnmarshalerField(t *testing.T) {
	var got withText
	require.NoError(t, unmarshal([]byte("text: abc\n"), &got))
	assert.Equal(t, upperText("T:abc"), got.Text)

	// A number is not coerced to a string for a TextUnmarshaler, so encoding/json refuses it.
	err := unmarshal([]byte("text: 12\n"), &got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot unmarshal number")
}

func TestMarshalLeadingNewline(t *testing.T) {
	for _, s := range []string{"\n", "\nline\n", "\n\nx", "\n  indented\nlast"} {
		out, err := marshal(map[string]string{"a": s})
		require.NoError(t, err)

		var back map[string]string
		require.NoError(t, unmarshal(out, &back))
		assert.Equal(t, s, back["a"], string(out))
	}
	out, err := jsonToYAML([]byte(`{"a":"\n"}`))
	require.NoError(t, err)
	var back map[string]string
	require.NoError(t, unmarshal(out, &back))
	assert.Equal(t, "\n", back["a"])
}
