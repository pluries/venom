package venom

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDump(t *testing.T) {
	v := map[string]interface{}{}
	v["result.systemoutjson"] = map[string]string{"FOO": "bar", "foo": "foo"}
	v["result.systemoutjson.FOO"] = "bar"
	v["result.systemoutjson.foo"] = "foo"
	got, err := Dump(v)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "foo", got["result.systemoutjson.foo"])
	require.Equal(t, "bar", got["result.systemoutjson.FOO"])
}

// nestedJSONValue builds a nested slice-of-maps structure shaped like a decoded HTTP JSON response
// body (executors/http.Result.BodyJSON is interface{}, populated by json.Unmarshal): breadth items
// per level, depth levels of nesting under a "child" key. Used by BenchmarkDumpStringPreserveCase
// (#1982) to vary a single variable's size/nesting independently of variable count.
func nestedJSONValue(depth, breadth int) interface{} {
	if depth == 0 {
		return "leaf-value"
	}
	items := make([]interface{}, breadth)
	for i := range items {
		items[i] = map[string]interface{}{
			"id":     i,
			"name":   fmt.Sprintf("item-%d", i),
			"active": i%2 == 0,
			"child":  nestedJSONValue(depth-1, breadth),
		}
	}
	return items
}

// realisticStepVars mimics the H that process_testcase.go:224-226 assembles before every step:
// tc.Vars, the previous step's vars and the testcase's computed vars, merged into one map. 15
// scalar entries stand in for that accumulated scope; bodyJSON (nil for the baseline) stands in for
// a captured HTTP response carried forward from an earlier step — the case #1982 is measuring.
//
// A value assigned with a step's "vars:" block lands in stepVars TWICE, not once: :419-420 add it
// to both tc.computedVars and previousStepVars, and :225-226 merge the former back in prefixed by
// tc.Name and the latter unprefixed. DumpStringPreserveCase then walks the same payload under two
// distinct keys — this fixture carries bodyJSON the same way so the benchmark doesn't understate
// the real per-step cost by half (code review, #1982).
func realisticStepVars(bodyJSON interface{}) H {
	vars := H{
		"venom.testsuite.filepath": "tests/suite.yml",
		"venom.testsuite.name":     "suite",
		"venom.teststep.number":    3,
	}
	for i := 0; i < 15; i++ {
		vars[fmt.Sprintf("previous.var%d", i)] = fmt.Sprintf("value-%d", i)
	}
	if bodyJSON != nil {
		vars["http.result.bodyjson"] = bodyJSON
		vars["testcase.http.result.bodyjson"] = bodyJSON
	}
	return vars
}

// BenchmarkDumpStringPreserveCase measures process_testcase.go:252's per-step, per-ranged-item call
// to DumpStringPreserveCase as ONE variable's value grows in size and nesting, holding variable
// count constant across the four "has a payload" cases. The neighbouring interpolation loop was
// already shown not to scale with variable count (prior spike, 2026-09-21, 10 vs 400 unused
// variables: 54ms vs 68ms) — this benchmark checks the ticket's actual hypothesis instead: that the
// flatten's reflection walk scales with the size/depth of what it walks, not how many top-level
// variables hold it.
//
// Small/Medium/Large co-vary byte size AND nesting depth (more breadth per level compounds into more
// depth to reach a bigger structure), so a cost difference between them cannot alone say which axis
// drives it (code review, #1982). Large_flat holds depth at 1 (go-dump's "Detailed*" encoder keeps a
// deep value's whole subtree at each level, so depth changes how much re-walking each key triggers,
// not just how many bytes exist) and matches Large's byte size with plain breadth instead, to
// separate "large" from "deep".
func BenchmarkDumpStringPreserveCase(b *testing.B) {
	cases := []struct {
		name           string
		depth, breadth int
	}{
		{"NoNestedValue", 0, 0},
		{"Small_1KB_depth1", 1, 20},
		{"Medium_15KB_depth2", 2, 15},
		{"Large_110KB_depth3", 3, 12},
		{"Large_flat_111KB_depth1", 1, 1700},
	}
	for _, c := range cases {
		var value interface{}
		if c.depth > 0 {
			value = nestedJSONValue(c.depth, c.breadth)
		}
		if value != nil {
			encoded, err := json.Marshal(value)
			require.NoError(b, err)
			b.Logf("%s: %d bytes JSON-encoded", c.name, len(encoded))
		}
		vars := realisticStepVars(value)
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := DumpStringPreserveCase(vars); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
