package venom

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterpolateSuiteVar(t *testing.T) {
	dumped := func(name, text string) map[string]string {
		return map[string]string{name: text, name + ".__Type__": "Map", "ext": `x","admin":"true`, "plain": "p"}
	}
	t.Run("a referenced value stays inside its string", func(t *testing.T) {
		vars := dumped("qp", `{"n":1,"ref":"{{ .ext }}"}`)
		got, err := interpolateSuiteVar("qp", vars["qp"], vars)
		require.NoError(t, err)
		assert.JSONEq(t, `{"n":1,"ref":"x\",\"admin\":\"true"}`, got)
	})
	t.Run("a map without an expression is returned as it is", func(t *testing.T) {
		vars := dumped("qp", `{"b":1,"a":[2]}`)
		got, err := interpolateSuiteVar("qp", vars["qp"], vars)
		require.NoError(t, err)
		assert.Equal(t, `{"b":1,"a":[2]}`, got)
	})
	t.Run("lists and nested maps are followed", func(t *testing.T) {
		vars := dumped("qp", `{"l":["a","{{ .plain }}"],"m":{"k":"{{ .ext }}"}}`)
		got, err := interpolateSuiteVar("qp", vars["qp"], vars)
		require.NoError(t, err)
		assert.JSONEq(t, `{"l":["a","p"],"m":{"k":"x\",\"admin\":\"true"}}`, got)
	})
	t.Run("a number keeps its text", func(t *testing.T) {
		vars := dumped("qp", `{"big":12345678901234567890,"f":1.50}`)
		got, err := interpolateSuiteVar("qp", vars["qp"], vars)
		require.NoError(t, err)
		assert.Equal(t, `{"big":12345678901234567890,"f":1.50}`, got)
	})
	t.Run("a variable that is not a map or list is interpolated as a whole", func(t *testing.T) {
		vars := map[string]string{"s": `{"a":"{{ .plain }}"}`, "plain": "p"}
		got, err := interpolateSuiteVar("s", vars["s"], vars)
		require.NoError(t, err)
		assert.Equal(t, `{"a":"p"}`, got)
	})
	t.Run("a text with two JSON documents falls back to the whole text", func(t *testing.T) {
		vars := map[string]string{"m": `{"a":"{{ .plain }}"} {"b":1}`, "m.__Type__": "Map", "plain": "p"}
		got, err := interpolateSuiteVar("m", vars["m"], vars)
		require.NoError(t, err)
		assert.Equal(t, `{"a":"p"} {"b":1}`, got)
	})
	t.Run("a text that is not JSON falls back to the whole text", func(t *testing.T) {
		vars := map[string]string{"m": `not json {{ .plain }}`, "m.__Type__": "Map", "plain": "p"}
		got, err := interpolateSuiteVar("m", vars["m"], vars)
		require.NoError(t, err)
		assert.Equal(t, `not json p`, got)
	})
	t.Run("an interpolation error is returned", func(t *testing.T) {
		for _, text := range []string{`"{{ .x"`, `{"a":"{{ .x"}`, `["{{ .x"]`, `{"l":["{{ .x"]}`} {
			vars := map[string]string{"m": text, "m.__Type__": "Map"}
			_, err := interpolateSuiteVar("m", text, vars)
			assert.Error(t, err, text)
		}
	})
}

// The whole-map text of a suite variable that references another variable keeps the referenced value inside its
// string (#3376), through initTestSuiteVars as a run calls it.
func TestInitTestSuiteVarsKeepsAReferencedValueInsideTheMap(t *testing.T) {
	ts := &TestSuite{Name: "suite", Vars: H{
		"ext": `x","admin":"true`,
		"qp":  map[string]interface{}{"ref": "{{ .ext }}"},
	}}
	require.NoError(t, New().initTestSuiteVars(ts))
	assert.JSONEq(t, `{"ref":"x\",\"admin\":\"true"}`, fmt.Sprint(ts.Vars["qp"]))
	assert.Equal(t, `x","admin":"true`, ts.Vars["qp.ref"])
}
