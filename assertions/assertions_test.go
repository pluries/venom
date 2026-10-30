package assertions

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldEqual(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   `a`,
				expected: []interface{}{`a`},
			},
		},
		{
			name: "with string and multiple value",
			args: args{
				actual:   `foo bar goo`,
				expected: []interface{}{`foo`, `bar`, `goo`},
			},
		},
		{
			name: "with int",
			args: args{
				actual:   1,
				expected: []interface{}{1},
			},
		},
		{
			name: "with float",
			args: args{
				actual:   1.0,
				expected: []interface{}{1.0},
			},
		},
		{
			name: "different types",
			args: args{
				actual:   42,
				expected: []interface{}{"42"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldEqual(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldEqual() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotEqual(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   `a`,
				expected: []interface{}{`b`},
			},
		},
		{
			name: "with int",
			args: args{
				actual:   1,
				expected: []interface{}{2},
			},
		},
		{
			name: "with float",
			args: args{
				actual:   1.0,
				expected: []interface{}{2.0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotEqual(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotEqual() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldAlmostEqual(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   `a`,
				expected: []interface{}{`b`},
			},
			wantErr: true,
		},
		{
			name: "with int",
			args: args{
				actual:   10,
				expected: []interface{}{9, 2},
			},
		},
		{
			name: "with float",
			args: args{
				actual:   1.1,
				expected: []interface{}{1.2, 0.1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldAlmostEqual(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("TestShouldAlmostEqual() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotAlmostEqual(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   `a`,
				expected: []interface{}{`b`},
			},
			wantErr: true,
		},
		{
			name: "with int",
			args: args{
				actual:   10,
				expected: []interface{}{5, 2},
			},
		},
		{
			name: "with float",
			args: args{
				actual:   1.1,
				expected: []interface{}{1.5, 0.1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotAlmostEqual(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotAlmostEqual() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldBeTrue(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual: `a`,
			},
			wantErr: true,
		},
		{
			name: "with args",
			args: args{
				actual:   1,
				expected: []interface{}{1},
			},
			wantErr: true,
		},
		{
			name: "with bool",
			args: args{
				actual: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldBeTrue(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldBeTrue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldBeFalse(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual: `a`,
			},
			wantErr: true,
		},
		{
			name: "with args",
			args: args{
				actual:   1,
				expected: []interface{}{1},
			},
			wantErr: true,
		},
		{
			name: "with bool",
			args: args{
				actual: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldBeFalse(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldBeFalse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldBeNil(t *testing.T) {
	var m map[string]string
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual: `a`,
			},
			wantErr: true,
		},
		{
			name: "with int",
			args: args{
				actual: 1,
			},
			wantErr: true,
		},
		{
			name: "with nothing",
		},
		{
			name: "with a nil map",
			args: args{
				actual: m,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldBeNil(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldBeNil() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotBeNil(t *testing.T) {
	var m map[string]string
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual: `a`,
			},
		},
		{
			name: "with int",
			args: args{
				actual: 1,
			},
		},
		{
			name:    "with nothing",
			wantErr: true,
		},
		{
			name: "with a nil map",
			args: args{
				actual: m,
			},
			wantErr: true,
		},
		{
			name: "with an empty slice",
			args: args{
				actual: []string{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotBeNil(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotBeNil() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldBeZeroValue(t *testing.T) {
	var m map[string]string
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual: ``,
			},
		},
		{
			name: "with int",
			args: args{
				actual: 0,
			},
		},
		{
			name: "with nothing",
		},
		{
			name: "with a nil map",
			args: args{
				actual: m,
			},
		},
		{
			name: "with an empty slice",
			args: args{
				actual: []string{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldBeZeroValue(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldBeZeroValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldBeGreaterThan(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   `b`,
				expected: []interface{}{"a"},
			},
		},
		{
			name: "with int",
			args: args{
				actual:   2,
				expected: []interface{}{1},
			},
		},
		{
			name: "with float",
			args: args{
				actual:   2.0,
				expected: []interface{}{1.0},
			},
		},
		{
			name: "with wrong types",
			args: args{
				actual:   2.0,
				expected: []interface{}{"a"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldBeGreaterThan(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldBeGreaterThan() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldBeGreaterThanOrEqualTo(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   `a`,
				expected: []interface{}{"a"},
			},
		},
		{
			name: "with int",
			args: args{
				actual:   2,
				expected: []interface{}{2},
			},
		},
		{
			name: "with float",
			args: args{
				actual:   2.0,
				expected: []interface{}{2.0},
			},
		},
		{
			name: "with string",
			args: args{
				actual:   `b`,
				expected: []interface{}{"a"},
			},
		},
		{
			name: "with int",
			args: args{
				actual:   2,
				expected: []interface{}{1},
			},
		},
		{
			name: "with float",
			args: args{
				actual:   2.0,
				expected: []interface{}{1.0},
			},
		},
		{
			name: "with wrong types",
			args: args{
				actual:   2.0,
				expected: []interface{}{"a"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldBeGreaterThanOrEqualTo(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldBeGreaterThanOrEqualTo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldBeBetween(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   `b`,
				expected: []interface{}{"a", "c"},
			},
		},
		{
			name: "with int",
			args: args{
				actual:   2,
				expected: []interface{}{1, 3},
			},
		},
		{
			name: "with float",
			args: args{
				actual:   2.0,
				expected: []interface{}{1.0, 3.0},
			},
		},
		{
			name: "with wrong types",
			args: args{
				actual:   2.0,
				expected: []interface{}{"a", 3},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldBeBetween(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldBeBetween() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotBeBetween(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   `a`,
				expected: []interface{}{"b", "c"},
			},
		},
		{
			name: "with int",
			args: args{
				actual:   1,
				expected: []interface{}{2, 3},
			},
		},
		{
			name: "with float",
			args: args{
				actual:   1.0,
				expected: []interface{}{2.0, 3.0},
			},
		},
		{
			name: "with wrong types",
			args: args{
				actual:   2.0,
				expected: []interface{}{"a", 3},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotBeBetween(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotBeBetween() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldContain(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   []interface{}{"a", "c"},
				expected: []interface{}{`a`},
			},
		},
		{
			name: "with int",
			args: args{
				actual:   []interface{}{1, 2},
				expected: []interface{}{1},
			},
		},
		{
			name: "with float",
			args: args{
				actual:   []interface{}{1.0, 2.0},
				expected: []interface{}{1.0},
			},
		},
		{
			name: "raise error",
			args: args{
				actual:   []interface{}{1.0, 2.0},
				expected: []interface{}{3.0},
			},
			wantErr: true,
		},
		{
			name: "raise error with nothing",
			args: args{
				expected: []interface{}{"something"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldContain(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldContain() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotContain(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   []interface{}{"a", "c"},
				expected: []interface{}{`b`},
			},
		},
		{
			name: "with int",
			args: args{
				actual:   []interface{}{1, 2},
				expected: []interface{}{3},
			},
		},
		{
			name: "with float",
			args: args{
				actual:   []interface{}{1.0, 2.0},
				expected: []interface{}{1.1},
			},
		},
		{
			name: "raise error",
			args: args{
				actual:   []interface{}{1.0, 2.0},
				expected: []interface{}{1.0},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotContain(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotContain() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldJSONContain(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// Objects and arrays
		{
			name: "object",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}}},
				expected: []interface{}{`{"a":1,"b":2,"c":{"x":1,"y":2}}`},
			},
		},
		{
			// Spaces, newlines, tabs and key order (including in nested objects) don't matter
			name: "object",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}}},
				expected: []interface{}{` { "c" : { "y" : 2 , "x" : 1 }, "b" : 2 ,` + "\n\t" + ` "a" : 1 } `},
			},
		},
		{
			name: "array",
			args: args{
				actual:   []interface{}{[]interface{}{1, 2}},
				expected: []interface{}{`[1,2]`},
			},
		},
		{
			// Spaces, newlines and tabs don't matter
			name: "array",
			args: args{
				actual:   []interface{}{[]interface{}{1, 2}},
				expected: []interface{}{` [ 1 ,` + "\n\t" + ` 2 ] `},
			},
		},
		// Object and array errors
		{
			name: "bad value",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{`{"a":2}`},
			},
			wantErr: true,
		},
		{
			name: "bad type",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{`{"a":"1"}`},
			},
			wantErr: true,
		},
		{
			name: "missing key",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2}},
				expected: []interface{}{`{"a":1}`},
			},
			wantErr: true,
		},
		{
			name: "bad array order",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []float64{1, 2}}},
				expected: []interface{}{`{"a":[2,1]}`},
			},
			wantErr: true,
		},
		{
			name: "object instead of array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{`[1]`},
			},
			wantErr: true,
		},
		{
			name: "array instead of object",
			args: args{
				actual:   []interface{}{[]interface{}{1}},
				expected: []interface{}{`{"a":1}}`},
			},
			wantErr: true,
		},
		// Primitive values
		{
			name: "string",
			args: args{
				actual:   []interface{}{"a"},
				expected: []interface{}{"a"},
			},
		},
		{
			name: "empty string",
			args: args{
				actual:   []interface{}{""},
				expected: []interface{}{""},
			},
		},
		{
			name: "number",
			args: args{
				actual:   []interface{}{json.Number("1")},
				expected: []interface{}{`1`},
			},
		},
		{
			name: "number",
			args: args{
				actual:   []interface{}{json.Number("1.2")},
				expected: []interface{}{`1.2`},
			},
		},
		{
			name: "boolean",
			args: args{
				actual:   []interface{}{true},
				expected: []interface{}{`true`},
			},
		},
		{
			// TODO: Shouldn't be valid, but Venom currently passes an empty string to the assertion function when the JSON value is `null`.
			name: "null",
			args: args{
				actual:   []interface{}{""},
				expected: []interface{}{`null`},
			},
		},
		// Primitive value errors
		{
			name: "bad value",
			args: args{
				actual:   []interface{}{"a"},
				expected: []interface{}{"b"},
			},
			wantErr: true,
		},
		{
			name: "bad type",
			args: args{
				actual:   []interface{}{float64(1)},
				expected: []interface{}{"1"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldJSONContain(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldJSONContain() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotJSONContain(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// Objects and arrays
		{
			name: "object",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}}},
				expected: []interface{}{`{"a":1,"b":2,"c":{"x":1,"y":2}}`},
			},
			wantErr: true,
		},
		{
			// Spaces, newlines, tabs and key order (including in nested objects) don't matter
			name: "object",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}}},
				expected: []interface{}{` { "c" : { "y" : 2 , "x" : 1 }, "b" : 2 ,` + "\n\t" + ` "a" : 1 } `},
			},
			wantErr: true,
		},
		{
			name: "array",
			args: args{
				actual:   []interface{}{[]interface{}{1, 2}},
				expected: []interface{}{`[1,2]`},
			},
			wantErr: true,
		},
		{
			// Spaces, newlines and tabs don't matter
			name: "array",
			args: args{
				actual:   []interface{}{[]interface{}{1, 2}},
				expected: []interface{}{` [ 1 ,` + "\n\t" + ` 2 ] `},
			},
			wantErr: true,
		},
		// Object and array errors
		{
			name: "bad value",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{`{"a":2}`},
			},
		},
		{
			name: "bad type",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{`{"a":"1"}`},
			},
		},
		{
			name: "missing key",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2}},
				expected: []interface{}{`{"a":1}`},
			},
		},
		{
			name: "bad array order",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []float64{1, 2}}},
				expected: []interface{}{`{"a":[2,1]}`},
			},
		},
		{
			name: "object instead of array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{`[1]`},
			},
		},
		{
			name: "array instead of object",
			args: args{
				actual:   []interface{}{[]interface{}{1}},
				expected: []interface{}{`{"a":1}}`},
			},
		},
		// Primitive values
		{
			name: "string",
			args: args{
				actual:   []interface{}{"a"},
				expected: []interface{}{"a"},
			},
			wantErr: true,
		},
		{
			name: "empty string",
			args: args{
				actual:   []interface{}{""},
				expected: []interface{}{""},
			},
			wantErr: true,
		},
		{
			name: "number",
			args: args{
				actual:   []interface{}{json.Number("1")},
				expected: []interface{}{`1`},
			},
			wantErr: true,
		},
		{
			name: "number",
			args: args{
				actual:   []interface{}{json.Number("1.2")},
				expected: []interface{}{`1.2`},
			},
			wantErr: true,
		},
		{
			name: "boolean",
			args: args{
				actual:   []interface{}{true},
				expected: []interface{}{`true`},
			},
			wantErr: true,
		},
		{
			// TODO: Shouldn't be valid, but Venom currently passes an empty string to the assertion function when the JSON value is `null`.
			name: "null",
			args: args{
				actual:   []interface{}{""},
				expected: []interface{}{`null`},
			},
			wantErr: true,
		},
		// Primitive value errors
		{
			name: "bad value",
			args: args{
				actual:   []interface{}{"a"},
				expected: []interface{}{"b"},
			},
		},
		{
			name: "bad type",
			args: args{
				actual:   []interface{}{float64(1)},
				expected: []interface{}{"1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotJSONContain(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotJSONContain() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldJSONContainWithKey(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// Objects and arrays
		{
			name: "object",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}}},
				expected: []interface{}{"c", `{"x":1,"y":2}`},
			},
		},
		{
			// Spaces, newlines, tabs and key order (including in nested objects) don't matter
			name: "object",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}}},
				expected: []interface{}{"c", ` { "y" : 2 ,` + "\n\t" + ` "x" : 1 } `},
			},
		},
		{
			name: "array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []interface{}{1, 2}}},
				expected: []interface{}{"a", `[1,2]`},
			},
		},
		{
			// Spaces, newlines and tabs don't matter
			name: "array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []interface{}{1, 2}}},
				expected: []interface{}{"a", ` [ 1 ,` + "\n\t" + ` 2 ] `},
			},
		},
		// Object and array errors
		{
			name: "bad value",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{"a", `2`},
			},
			wantErr: true,
		},
		{
			name: "bad type",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{"a", "1"},
			},
			wantErr: true,
		},
		{
			name: "missing key",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2}},
				expected: []interface{}{"c", `3`},
			},
			wantErr: true,
		},
		{
			name: "bad array order",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []float64{1, 2}}},
				expected: []interface{}{"a", `[2,1]`},
			},
			wantErr: true,
		},
		{
			name: "object instead of array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{`[1]`},
			},
			wantErr: true,
		},
		{
			name: "array instead of object",
			args: args{
				actual:   []interface{}{[]interface{}{1}},
				expected: []interface{}{`{"a":1}}`},
			},
			wantErr: true,
		},
		{
			name: "missing key",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []float64{1}}},
				expected: []interface{}{`[1]`},
			},
			wantErr: true,
		},
		{
			name: "missing value",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []float64{1}}},
				expected: []interface{}{"a"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldJSONContainWithKey(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldJSONContainWithKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldJSONContainAllWithKey(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// Objects and arrays
		{
			name: "object",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}}},
				expected: []interface{}{"c", `{"x":1,"y":2}`},
			},
		},
		{
			// Spaces, newlines, tabs and key order (including in nested objects) don't matter
			name: "object",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}}},
				expected: []interface{}{"c", ` { "y" : 2 ,` + "\n\t" + ` "x" : 1 } `},
			},
		},
		{
			name: "array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []interface{}{1, 2}}, map[string]interface{}{"a": []interface{}{1, 2}}},
				expected: []interface{}{"a", `[1,2]`},
			},
		},
		{
			// Spaces, newlines and tabs don't matter
			name: "array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []interface{}{1, 2}}, map[string]interface{}{"a": []interface{}{1, 2}}},
				expected: []interface{}{"a", ` [ 1 ,` + "\n\t" + ` 2 ] `},
			},
		},
		// Object and array errors
		{
			name: "bad value",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{"a", `2`},
			},
			wantErr: true,
		},
		{
			name: "bad type",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{"a", "1"},
			},
			wantErr: true,
		},
		{
			name: "missing key in second element of the array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2, "c": 3}, map[string]interface{}{"a": 1, "b": 2}},
				expected: []interface{}{"c", `3`},
			},
			wantErr: true,
		},
		{
			name: "bad array order in second element of the array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []float64{2, 1}}, map[string]interface{}{"a": []float64{1, 2}}},
				expected: []interface{}{"a", `[2,1]`},
			},
			wantErr: true,
		},
		{
			name: "object instead of array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{`[1]`},
			},
			wantErr: true,
		},
		{
			name: "array instead of object",
			args: args{
				actual:   []interface{}{[]interface{}{1}},
				expected: []interface{}{`{"a":1}}`},
			},
			wantErr: true,
		},
		{
			name: "missing key",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []float64{1}}},
				expected: []interface{}{`[1]`},
			},
			wantErr: true,
		},
		{
			name: "missing value",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []float64{1}}},
				expected: []interface{}{"a"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldJSONContainAllWithKey(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldJSONContainAllWithKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotJSONContainWithKey(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// Objects and arrays
		{
			name: "object",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}}},
				expected: []interface{}{"c", `{"x":1,"y":2}`},
			},
			wantErr: true,
		},
		{
			// Spaces, newlines, tabs and key order (including in nested objects) don't matter
			name: "object",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}}},
				expected: []interface{}{"c", ` { "y" : 2 ,` + "\n\t" + ` "x" : 1 } `},
			},
			wantErr: true,
		},
		{
			name: "array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []interface{}{1, 2}}},
				expected: []interface{}{"a", `[1,2]`},
			},
			wantErr: true,
		},
		{
			// Spaces, newlines and tabs don't matter
			name: "array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []interface{}{1, 2}}},
				expected: []interface{}{"a", ` [ 1 ,` + "\n\t" + ` 2 ] `},
			},
			wantErr: true,
		},
		// Object and array errors
		{
			name: "bad value",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{"a", `2`},
			},
		},
		{
			name: "bad type",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{"a", "1"},
			},
		},
		{
			name: "missing key",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1, "b": 2}},
				expected: []interface{}{"c", `3`},
			},
		},
		{
			name: "bad array order",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []float64{1, 2}}},
				expected: []interface{}{"a", `[2,1]`},
			},
		},
		{
			name: "object instead of array",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": 1}},
				expected: []interface{}{`[1]`},
			},
			wantErr: true,
		},
		{
			name: "array instead of object",
			args: args{
				actual:   []interface{}{[]interface{}{1}},
				expected: []interface{}{`{"a":1}}`},
			},
			wantErr: true,
		},
		{
			name: "missing key",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []float64{1}}},
				expected: []interface{}{`[1]`},
			},
			wantErr: true,
		},
		{
			name: "missing value",
			args: args{
				actual:   []interface{}{map[string]interface{}{"a": []float64{1}}},
				expected: []interface{}{"a"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotJSONContainWithKey(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotJSONContainWithKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldContainKey(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   map[string]interface{}{"a": "", "c": ""},
				expected: []interface{}{`a`},
			},
		},
		{
			name: "raise error",
			args: args{
				actual:   map[string]interface{}{"a": "", "c": ""},
				expected: []interface{}{`b`},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldContainKey(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldContainKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotContainKey(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   map[string]interface{}{"a": "", "c": ""},
				expected: []interface{}{`b`},
			},
		},
		{
			name: "raise error",
			args: args{
				actual:   map[string]interface{}{"a": "", "c": ""},
				expected: []interface{}{`a`},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotContainKey(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotContainKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldBeEmpty(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual: map[string]interface{}{},
			},
		},
		{
			name: "ko",
			args: args{
				actual: map[string]interface{}{"a": "", "c": ""},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldBeEmpty(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldBeEmpty() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotBeEmpty(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ko",
			args: args{
				actual: map[string]interface{}{},
			},
			wantErr: true,
		},
		{
			name: "ok",
			args: args{
				actual: map[string]interface{}{"a": "", "c": ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotBeEmpty(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotBeEmpty() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldHaveLength(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok with slice",
			args: args{
				actual:   []interface{}{"a"},
				expected: []interface{}{1},
			},
		},
		{
			name: "ok with map",
			args: args{
				actual:   map[string]interface{}{"a": "a"},
				expected: []interface{}{1},
			},
		},
		{
			name: "ok with string",
			args: args{
				actual:   "a",
				expected: []interface{}{1},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   []interface{}{"a"},
				expected: []interface{}{2},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldHaveLength(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldHaveLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldStartWith(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   "aaa",
				expected: []interface{}{"a"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   "aaa",
				expected: []interface{}{"b"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldStartWith(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldStartWith() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotStartWith(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   "aaa",
				expected: []interface{}{"b"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   "aaa",
				expected: []interface{}{"a"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotStartWith(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotStartWith() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldEndWith(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   "aaa-",
				expected: []interface{}{"a-"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   "aaa-",
				expected: []interface{}{"b"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldEndWith(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldEndWith() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotEndWith(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   "aaa-",
				expected: []interface{}{"b"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   "aaa-",
				expected: []interface{}{"a-"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotEndWith(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotEndWith() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldContainSubstring(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   "aaa-x",
				expected: []interface{}{"a-"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   "aaa-x",
				expected: []interface{}{"b-"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldContainSubstring(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldContainSubstring() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotContainSubstring(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   "aaa-x",
				expected: []interface{}{"b-"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   "aaa-x",
				expected: []interface{}{"a-"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldNotContainSubstring(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotContainSubstring() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldEqualTrimSpace(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok with string",
			args: args{
				actual:   ` a`,
				expected: []interface{}{`a`},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   ` ba`,
				expected: []interface{}{`a`},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldEqualTrimSpace(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldEqualTrimSpace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldHappenBefore(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{time.Now().Add(1 * time.Second)},
			},
		},
		{
			name: "ok",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{"1 minute from now"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{time.Now().Add(-1 * time.Second)},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldHappenBefore(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldHappenBefore() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldHappenOnOrBefore(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{time.Now().Add(1 * time.Second)},
			},
		},
		{
			name: "ok",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{"1 minute from now"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{time.Now().Add(-1 * time.Second)},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldHappenOnOrBefore(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldHappenOnOrBefore() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldHappenAfter(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{time.Now().Add(-1 * time.Second)},
			},
		},
		{
			name: "ok",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{"1 second ago"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{time.Now().Add(1 * time.Second)},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldHappenAfter(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldHappenAfter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldHappenOnOrAfter(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{time.Now().Add(-1 * time.Second)},
			},
		},
		{
			name: "ok",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{"1 second ago"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{time.Now().Add(1 * time.Second)},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldHappenOnOrAfter(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldHappenOnOrAfter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldHappenBetween(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{time.Now().Add(-1 * time.Second), time.Now().Add(1 * time.Second)},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{time.Now().Add(1 * time.Second), time.Now().Add(2 * time.Second)},
			},
			wantErr: true,
		},
		{
			name: "ok",
			args: args{
				actual:   "2006-01-02T15:04:05+07:00",
				expected: []interface{}{"2006-01-02T15:04:00+07:00", "2006-01-02T15:04:10+07:00"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   "2006-01-02T15:04:00+07:00",
				expected: []interface{}{"2006-01-02T15:04:05+07:00", "2006-01-02T15:04:10+07:00"},
			},
			wantErr: true,
		},
		{
			name: "ok",
			args: args{
				actual:   time.Now(),
				expected: []interface{}{"1 minute ago", "1 minute from now"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldHappenBetween(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldHappenBetween() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldTimeEqual(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}

	Parisloc, err := time.LoadLocation("Europe/Paris")
	require.NoError(t, err)

	now := time.Now()
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				actual:   now,
				expected: []interface{}{now.In(Parisloc)},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   now,
				expected: []interface{}{now.Add(1 * time.Second)},
			},
			wantErr: true,
		},
		{
			name: "ok",
			args: args{
				actual:   "2006-01-02T15:04:00+02:00",
				expected: []interface{}{"2006-01-02T13:04:00Z"},
			},
		},
		{
			name: "ko",
			args: args{
				actual:   "2006-01-02T15:04:00+07:00",
				expected: []interface{}{"2006-01-02T15:04:05Z"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldTimeEqual(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldTimeEqual() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldMatchRegex(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with string",
			args: args{
				actual:   `a`,
				expected: []interface{}{`a`},
			},
		},
		{
			name: "with string regex",
			args: args{
				actual:   `abc`,
				expected: []interface{}{`a.*c$`},
			},
		},
		{
			name: "with number regex",
			args: args{
				actual:   `abc-123`,
				expected: []interface{}{`.*[0-9]{3}$`},
			},
		},
		{
			name: "with regex throwing error",
			args: args{
				actual:   `abc-123`,
				expected: []interface{}{`.*[0-9]{6}$`},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldMatchRegex(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				msg := fmt.Sprintf("value %v not matching pattern : %v", tt.args.actual, tt.args.expected[0])
				assert.ErrorContainsf(t, err, msg, "Contains message")
				t.Errorf("ShouldMatchRegex() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldJSONEqual(t *testing.T) {
	type args struct {
		actual   interface{}
		expected []interface{}
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// Objects and arrays
		{
			name: "object",
			args: args{
				actual:   map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}},
				expected: []interface{}{`{"a":1,"b":2,"c":{"x":1,"y":2}}`},
			},
		},
		{
			// Spaces, newlines, tabs and key order (including in nested objects) don't matter
			name: "object",
			args: args{
				actual:   map[string]interface{}{"a": 1, "b": 2, "c": map[string]interface{}{"x": 1, "y": 2}},
				expected: []interface{}{` { "c" : { "y" : 2 , "x" : 1 }, "b" : 2 ,` + "\n\t" + ` "a" : 1 } `},
			},
		},
		{
			name: "array",
			args: args{
				actual:   []interface{}{1, 2},
				expected: []interface{}{`[1,2]`},
			},
		},
		{
			// Spaces, newlines and tabs don't matter
			name: "array",
			args: args{
				actual:   []interface{}{1, 2},
				expected: []interface{}{` [ 1 ,` + "\n\t" + ` 2 ] `},
			},
		},
		// Object and array errors
		{
			name: "bad value",
			args: args{
				actual:   map[string]interface{}{"a": 1},
				expected: []interface{}{`{"a":2}`},
			},
			wantErr: true,
		},
		{
			name: "bad type",
			args: args{
				actual:   map[string]interface{}{"a": 1},
				expected: []interface{}{`{"a":"1"}`},
			},
			wantErr: true,
		},
		{
			name: "missing key",
			args: args{
				actual:   map[string]interface{}{"a": 1, "b": 2},
				expected: []interface{}{`{"a":1}`},
			},
			wantErr: true,
		},
		{
			name: "bad array order",
			args: args{
				actual:   map[string]interface{}{"a": []float64{1, 2}},
				expected: []interface{}{`{"a":[2,1]}`},
			},
			wantErr: true,
		},
		{
			name: "object instead of array",
			args: args{
				actual:   map[string]interface{}{"a": 1},
				expected: []interface{}{`[1]`},
			},
			wantErr: true,
		},
		{
			name: "array instead of object",
			args: args{
				actual:   []interface{}{1},
				expected: []interface{}{`{"a":1}}`},
			},
			wantErr: true,
		},
		// Primitive values
		{
			name: "string",
			args: args{
				actual:   "a",
				expected: []interface{}{"a"},
			},
		},
		{
			name: "empty string",
			args: args{
				actual:   "",
				expected: []interface{}{""},
			},
		},
		{
			name: "number",
			args: args{
				actual:   json.Number("1"),
				expected: []interface{}{`1`},
			},
		},
		{
			name: "number",
			args: args{
				actual:   json.Number("1.2"),
				expected: []interface{}{`1.2`},
			},
		},
		{
			name: "boolean",
			args: args{
				actual:   true,
				expected: []interface{}{`true`},
			},
		},
		{
			// TODO: Shouldn't be valid, but Venom currently passes an empty string to the assertion function when the JSON value is `null`.
			name: "null",
			args: args{
				actual:   "",
				expected: []interface{}{`null`},
			},
		},
		// Primitive value errors
		{
			name: "bad value",
			args: args{
				actual:   "a",
				expected: []interface{}{"b"},
			},
			wantErr: true,
		},
		{
			name: "bad type",
			args: args{
				actual:   float64(1),
				expected: []interface{}{"1"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ShouldJSONEqual(tt.args.actual, tt.args.expected...); (err != nil) != tt.wantErr {
				t.Errorf("ShouldJSONEqual() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldNotJSONEqual(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "equal objects should error",
			actual:   map[string]interface{}{"a": 1, "b": 2},
			expected: `{"a": 1, "b": 2}`,
			wantErr:  true,
		},
		{
			name:     "different objects should not error",
			actual:   map[string]interface{}{"a": 1, "b": 2},
			expected: `{"a": 1, "b": 3}`,
			wantErr:  false,
		},
		{
			name:     "equal arrays should error",
			actual:   []interface{}{1, 2, 3},
			expected: `[1, 2, 3]`,
			wantErr:  true,
		},
		{
			name:     "different arrays should not error",
			actual:   []interface{}{1, 2, 3},
			expected: `[1, 2, 4]`,
			wantErr:  false,
		},
		{
			name:     "equal strings should error",
			actual:   "test",
			expected: "test",
			wantErr:  true,
		},
		{
			name:     "different strings should not error",
			actual:   "test1",
			expected: "test2",
			wantErr:  false,
		},
		{
			name:     "equal numbers should error",
			actual:   json.Number("123"),
			expected: json.Number("123"),
			wantErr:  true,
		},
		{
			name:     "different numbers should not error",
			actual:   json.Number("123"),
			expected: json.Number("456"),
			wantErr:  false,
		},
		{
			name:     "equal booleans should error",
			actual:   true,
			expected: true,
			wantErr:  true,
		},
		{
			name:     "different booleans should not error",
			actual:   true,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "null values should error",
			actual:   "",
			expected: "null",
			wantErr:  true,
		},
		{
			name:     "null and non-null should not error",
			actual:   "",
			expected: "not null",
			wantErr:  false,
		},
		{
			name:     "objects with different key order should error",
			actual:   map[string]interface{}{"a": 1, "b": 2},
			expected: `{"b": 2, "a": 1}`,
			wantErr:  true,
		},
		{
			name:     "nested objects should error when equal",
			actual:   map[string]interface{}{"a": map[string]interface{}{"b": 1}},
			expected: `{"a": {"b": 1}}`,
			wantErr:  true,
		},
		{
			name:     "nested objects should not error when different",
			actual:   map[string]interface{}{"a": map[string]interface{}{"b": 1}},
			expected: `{"a": {"b": 2}}`,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ShouldNotJSONEqual(tt.actual, tt.expected)
			if (err != nil) != tt.wantErr {
				t.Errorf("ShouldNotJSONEqual() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegisterUserAssertFunc(t *testing.T) {
	var assertFunc AssertFunc

	tests := []struct {
		name    string
		actual  string
		wantErr bool
	}{
		{
			name:   "register custom assertion",
			actual: "ShouldCustom",
		},
		{
			name:    "redefine custom assertion error",
			actual:  "ShouldCustom",
			wantErr: true,
		},
		{
			name:    "redefine builtin assertion error",
			actual:  "ShouldEqual",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RegisterUserAssertFunc(tt.actual, assertFunc)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterUserAssertFunc() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAssertMapConcurrentAccess pins the fix for #1974: assertMap is read on every assertion check
// (Get) and written by RegisterUserAssertFunc, and #1983 plans to make the code that calls Get run
// concurrently across files. Run with `go test -race` — without it this test only proves the
// goroutines don't deadlock, not that the access is race-free.
func TestAssertMapConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if _, ok := Get("ShouldEqual"); !ok {
					t.Errorf("Get(%q) = false, want true", "ShouldEqual")
				}
			}
		}()
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("ShouldConcurrentTest%d", i)
			if err := RegisterUserAssertFunc(name, func(_ interface{}, _ ...interface{}) error {
				return nil
			}); err != nil {
				t.Errorf("RegisterUserAssertFunc(%q) = %v, want nil", name, err)
			}
		}(i)
	}

	wg.Wait()
}

// The second-based phrases must resolve against the reference time at any hour of the day — #1984: go-naturaldate read
// "1 second ago" as 01:00 of the day, so ShouldHappenAfter "1 second ago" only held after 01:00 (found by the first CI run).
func TestGetTimeFromStringAt_RelativeSeconds(t *testing.T) {
	now := time.Date(2026, time.September, 21, 0, 27, 53, 0, time.UTC)
	tests := []struct {
		in   string
		want time.Time
	}{
		{"1 second ago", now.Add(-1 * time.Second)},
		{"5 seconds ago", now.Add(-5 * time.Second)},
		{"90 secs ago", now.Add(-90 * time.Second)},
		{"in 30 seconds", now.Add(30 * time.Second)},
		{"30 seconds from now", now.Add(30 * time.Second)},
		{"5 minutes ago", now.Add(-5 * time.Minute)}, // still the humanize parser
		{"2026-09-21T00:00:00Z", time.Date(2026, time.September, 21, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := getTimeFromStringAt(tt.in, now)
			if err != nil {
				t.Fatalf("getTimeFromStringAt(%q) error = %v", tt.in, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("getTimeFromStringAt(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
	// note: go-naturaldate answers the reference time for text it cannot read instead of an error — pre-existing, out of scope here
}

// uncastable is a value that spf13/cast can turn neither into a float64 nor into a string, to reach the error returns
// of the comparators' string fallback.
type uncastable struct{ n int }

// orderingFailureCases are the inputs on which the four ordering assertions (ShouldBeGreaterThan, …OrEqualTo,
// ShouldBeLessThan, …OrEqualTo) fail with the same error, whatever the operator. Characterisation for #2031: the
// messages are the ones the code returns today, byte for byte.
var orderingFailureCases = []struct {
	name     string
	actual   interface{}
	expected []interface{}
	wantErr  string
}{
	{
		name:     "no comparison value fails the arity check",
		actual:   1,
		expected: []interface{}{},
		wantErr:  "This assertion requires exactly 1 comparison values (you provided 0).",
	},
	{
		name:     "two comparison values fail the arity check",
		actual:   1,
		expected: []interface{}{1, 2},
		wantErr:  "This assertion requires exactly 1 comparison values (you provided 2).",
	},
	{
		name:     "int against string fails the same-type check",
		actual:   1,
		expected: []interface{}{"1"},
		wantErr:  "This assertion requires 2 values of same types.",
	},
	{
		name:     "int against float64 fails the same-type check, no numeric widening",
		actual:   2,
		expected: []interface{}{1.0},
		wantErr:  "This assertion requires 2 values of same types.",
	},
	{
		name:     "nil against int fails the same-type check",
		actual:   nil,
		expected: []interface{}{1},
		wantErr:  "This assertion requires 2 values of same types.",
	},
	{
		name:     "invalid json.Number against int64 fails the same-type check",
		actual:   json.Number("abc"),
		expected: []interface{}{int64(1)},
		wantErr:  "This assertion requires 2 values of same types.",
	},
	{
		name:     "invalid json.Number against json.Number returns the ParseFloat error",
		actual:   json.Number("abc"),
		expected: []interface{}{json.Number("1")},
		wantErr:  `strconv.ParseFloat: parsing "abc": invalid syntax`,
	},
	{
		name:     "invalid json.Number against string returns the ParseFloat error",
		actual:   json.Number("abc"),
		expected: []interface{}{"1"},
		wantErr:  `strconv.ParseFloat: parsing "abc": invalid syntax`,
	},
	{
		name:     "numeric string against non-numeric string returns the cast error of expected",
		actual:   "5",
		expected: []interface{}{"abc"},
		wantErr:  `unable to cast "abc" of type string to float64: strconv.ParseFloat: parsing "abc": invalid syntax`,
	},
	{
		name:     "json.Number against non-numeric string returns the cast error of expected",
		actual:   json.Number("5"),
		expected: []interface{}{"abc"},
		wantErr:  `unable to cast "abc" of type string to float64: strconv.ParseFloat: parsing "abc": invalid syntax`,
	},
	{
		name:     "values that are neither numbers nor strings return the cast-to-string error of actual",
		actual:   uncastable{n: 1},
		expected: []interface{}{uncastable{n: 2}},
		wantErr:  "unable to cast assertions.uncastable{n:1} of type assertions.uncastable to string",
	},
}

// orderingCase is one comparison, with what each of the four ordering assertions returns on it today ("" = pass).
type orderingCase struct {
	name     string
	actual   interface{}
	expected interface{}
	gt, ge   string
	lt, le   string
}

// orderingComparisonCases pin the pass/fail result and the exact message of each operator. Note the le column: its
// numeric branch says "expected '%v' less than or equals to" and its string branch "expected: %v less than or equals
// to" — a known inconsistency (plan #2031, Compatibility boundaries), pinned as it is.
var orderingComparisonCases = []orderingCase{
	{
		name: "int, actual above expected", actual: 2, expected: 1,
		gt: "",
		ge: "",
		lt: "expected: 2 less than 1 but it wasn't",
		le: "expected '2' less than or equals to 1 but it wasn't",
	},
	{
		name: "int, actual equal to expected", actual: 1, expected: 1,
		gt: "expected: 1 greater than 1 but it wasn't",
		ge: "",
		lt: "expected: 1 less than 1 but it wasn't",
		le: "",
	},
	{
		name: "int, actual below expected", actual: 1, expected: 2,
		gt: "expected: 1 greater than 2 but it wasn't",
		ge: "expected: 1 greater than or equals to 2 but it wasn't",
		lt: "",
		le: "",
	},
	{
		name: "float64, actual above expected", actual: 2.5, expected: 1.5,
		gt: "",
		ge: "",
		lt: "expected: 2.5 less than 1.5 but it wasn't",
		le: "expected '2.5' less than or equals to 1.5 but it wasn't",
	},
	{
		name: "float64, actual equal to expected", actual: 1.5, expected: 1.5,
		gt: "expected: 1.5 greater than 1.5 but it wasn't",
		ge: "",
		lt: "expected: 1.5 less than 1.5 but it wasn't",
		le: "",
	},
	{
		name: "float64, actual below expected", actual: 1.5, expected: 2.5,
		gt: "expected: 1.5 greater than 2.5 but it wasn't",
		ge: "expected: 1.5 greater than or equals to 2.5 but it wasn't",
		lt: "",
		le: "",
	},
	{
		name: "json.Number against json.Number, actual above expected", actual: json.Number("2"), expected: json.Number("1"),
		gt: "",
		ge: "",
		lt: "expected: 2 less than 1 but it wasn't",
		le: "expected '2' less than or equals to 1 but it wasn't",
	},
	{
		name: "json.Number against json.Number, actual equal to expected", actual: json.Number("1"), expected: json.Number("1.0"),
		gt: "expected: 1 greater than 1.0 but it wasn't",
		ge: "",
		lt: "expected: 1 less than 1.0 but it wasn't",
		le: "",
	},
	{
		name: "json.Number against json.Number, actual below expected", actual: json.Number("1"), expected: json.Number("2"),
		gt: "expected: 1 greater than 2 but it wasn't",
		ge: "expected: 1 greater than or equals to 2 but it wasn't",
		lt: "",
		le: "",
	},
	{
		name: "json.Number against numeric string compares as numbers, not lexically", actual: json.Number("10"), expected: "9",
		gt: "",
		ge: "",
		lt: "expected: 10 less than 9 but it wasn't",
		le: "expected '10' less than or equals to 9 but it wasn't",
	},
	{
		name: "json.Number against float64 compares as numbers", actual: json.Number("2.5"), expected: 2.5,
		gt: "expected: 2.5 greater than 2.5 but it wasn't",
		ge: "",
		lt: "expected: 2.5 less than 2.5 but it wasn't",
		le: "",
	},
	{
		name: "json.Number against int64 compares as numbers", actual: json.Number("3"), expected: int64(4),
		gt: "expected: 3 greater than 4 but it wasn't",
		ge: "expected: 3 greater than or equals to 4 but it wasn't",
		lt: "",
		le: "",
	},
	{
		name: "numeric strings compare as numbers, not lexically", actual: "10", expected: "9",
		gt: "",
		ge: "",
		lt: "expected: 10 less than 9 but it wasn't",
		le: "expected '10' less than or equals to 9 but it wasn't",
	},
	{
		name: "non-numeric strings fall back to string order, actual above expected", actual: "b", expected: "a",
		gt: "",
		ge: "",
		lt: "expected: b less than a but it wasn't",
		le: "expected: b less than or equals to a but it wasn't",
	},
	{
		name: "non-numeric strings fall back to string order, actual equal to expected", actual: "a", expected: "a",
		gt: "expected: a greater than a but it wasn't",
		ge: "",
		lt: "expected: a less than a but it wasn't",
		le: "",
	},
	{
		name: "non-numeric strings fall back to string order, actual below expected", actual: "a", expected: "b",
		gt: "expected: a greater than b but it wasn't",
		ge: "expected: a greater than or equals to b but it wasn't",
		lt: "",
		le: "",
	},
	{
		name: "non-numeric actual against numeric expected falls back to string order", actual: "a", expected: "1",
		gt: "",
		ge: "",
		lt: "expected: a less than 1 but it wasn't",
		le: "expected: a less than or equals to 1 but it wasn't",
	},
	{
		name: "bools compare as numbers, true above false", actual: true, expected: false,
		gt: "",
		ge: "",
		lt: "expected: true less than false but it wasn't",
		le: "expected 'true' less than or equals to false but it wasn't",
	},
}

func runOrderingCharacterisation(t *testing.T, fn AssertFunc, want func(orderingCase) string) {
	t.Helper()
	for _, tt := range orderingFailureCases {
		t.Run(tt.name, func(t *testing.T) {
			require.EqualError(t, fn(tt.actual, tt.expected...), tt.wantErr)
		})
	}
	for _, tt := range orderingComparisonCases {
		t.Run(tt.name, func(t *testing.T) {
			err := fn(tt.actual, tt.expected)
			if w := want(tt); w != "" {
				require.EqualError(t, err, w)
				return
			}
			require.NoError(t, err)
		})
	}
	t.Run("nil against nil panics in the same-type check", func(t *testing.T) {
		assert.PanicsWithValue(t, "reflect: Zero(nil)", func() { t.Errorf("no panic, returned %v", fn(nil, nil)) })
	})
}

func TestShouldBeGreaterThanCharacterisation(t *testing.T) {
	runOrderingCharacterisation(t, ShouldBeGreaterThan, func(c orderingCase) string { return c.gt })
}

func TestShouldBeGreaterThanOrEqualToCharacterisation(t *testing.T) {
	runOrderingCharacterisation(t, ShouldBeGreaterThanOrEqualTo, func(c orderingCase) string { return c.ge })
}

func TestShouldBeLessThan(t *testing.T) {
	runOrderingCharacterisation(t, ShouldBeLessThan, func(c orderingCase) string { return c.lt })
}

func TestShouldBeLessThanOrEqualTo(t *testing.T) {
	runOrderingCharacterisation(t, ShouldBeLessThanOrEqualTo, func(c orderingCase) string { return c.le })
}

// TestShouldJSONEqualCharacterisation completes TestShouldJSONEqual with the exact message of every branch of the type
// switch on actual (#2031). Messages end with a space: that is the code's text, pinned as it is.
func TestShouldJSONEqualCharacterisation(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected []interface{}
		wantErr  string // "" = pass
	}{
		// Arity
		{name: "no expected value fails the arity check", actual: "a", expected: []interface{}{},
			wantErr: "This assertion requires exactly 1 comparison values (you provided 0)."},
		{name: "two expected values fail the arity check", actual: "a", expected: []interface{}{"a", "a"},
			wantErr: "This assertion requires exactly 1 comparison values (you provided 2)."},

		// map[string]interface{}
		{name: "object, equal", actual: map[string]interface{}{"a": 1}, expected: []interface{}{`{"a":1}`}},
		{name: "object, mismatch prints both maps", actual: map[string]interface{}{"a": 1}, expected: []interface{}{`{"a":2}`},
			wantErr: "expected 'map[a:1]' to be JSON equals to 'map[a:2]' "},
		{name: "object, truncated JSON in expected", actual: map[string]interface{}{"a": 1}, expected: []interface{}{`{"a":`},
			wantErr: "unexpected end of JSON input"},
		{name: "object, malformed JSON in expected", actual: map[string]interface{}{"a": 1}, expected: []interface{}{`{bad}`},
			wantErr: "invalid character 'b' looking for beginning of object key string"},
		{name: "object, non-string expected is cast to its string form", actual: map[string]interface{}{"a": 1}, expected: []interface{}{42},
			wantErr: "json: cannot unmarshal number into Go value of type map[string]interface {}"},
		{name: "object, expected not castable to string", actual: map[string]interface{}{"a": 1}, expected: []interface{}{uncastable{n: 1}},
			wantErr: "unable to cast assertions.uncastable{n:1} of type assertions.uncastable to string"},
		{name: "object, actual not marshalable", actual: map[string]interface{}{"f": func() {}}, expected: []interface{}{`{}`},
			wantErr: "json: unsupported type: func()"},
		{name: "empty object against JSON null fails with two identical-looking maps", actual: map[string]interface{}{}, expected: []interface{}{`null`},
			wantErr: "expected 'map[]' to be JSON equals to 'map[]' "},

		// []interface{}
		{name: "array, equal", actual: []interface{}{1, 2}, expected: []interface{}{`[1,2]`}},
		{name: "array, mismatch prints both slices", actual: []interface{}{1, 2}, expected: []interface{}{`[2,1]`},
			wantErr: "expected '[1 2]' to be JSON equals to '[2 1]' "},
		{name: "array, truncated JSON in expected", actual: []interface{}{1, 2}, expected: []interface{}{`[1,`},
			wantErr: "unexpected end of JSON input"},
		{name: "array, non-string expected is cast to its string form", actual: []interface{}{1}, expected: []interface{}{42},
			wantErr: "json: cannot unmarshal number into Go value of type []interface {}"},
		{name: "array, expected not castable to string", actual: []interface{}{1}, expected: []interface{}{uncastable{n: 1}},
			wantErr: "unable to cast assertions.uncastable{n:1} of type assertions.uncastable to string"},
		{name: "array, actual not marshalable", actual: []interface{}{func() {}}, expected: []interface{}{`[]`},
			wantErr: "json: unsupported type: func()"},

		// string
		{name: "string, mismatch", actual: "a", expected: []interface{}{"b"},
			wantErr: "expected 'a' to be JSON equals to 'b' "},
		{name: "string, empty actual equals JSON null", actual: "", expected: []interface{}{"null"}},
		{name: "string, non-empty actual does not equal JSON null", actual: "a", expected: []interface{}{"null"},
			wantErr: "expected 'a' to be JSON equals to 'null' "},
		{name: "string, literal null actual does not equal empty expected", actual: "null", expected: []interface{}{""},
			wantErr: "expected 'null' to be JSON equals to '' "},
		{name: "string, non-string expected is cast to its string form", actual: "1", expected: []interface{}{1}},
		{name: "string, expected not castable to string", actual: "1", expected: []interface{}{uncastable{n: 1}},
			wantErr: "unable to cast assertions.uncastable{n:1} of type assertions.uncastable to string"},

		// json.Number
		{name: "number, equal as floats", actual: json.Number("1"), expected: []interface{}{"1.0"}},
		{name: "number, mismatch prints both floats", actual: json.Number("1"), expected: []interface{}{"2"},
			wantErr: "expected '1' to be JSON equals to '2' "},
		{name: "number, invalid actual", actual: json.Number("abc"), expected: []interface{}{"1"},
			wantErr: `unable to cast "abc" of type json.Number to float64: strconv.ParseFloat: parsing "abc": invalid syntax`},
		{name: "number, non-numeric expected", actual: json.Number("1"), expected: []interface{}{"abc"},
			wantErr: `unable to cast "abc" of type string to float64: strconv.ParseFloat: parsing "abc": invalid syntax`},
		{name: "number, non-string numeric expected", actual: json.Number("1"), expected: []interface{}{1}},

		// bool
		{name: "boolean, mismatch", actual: true, expected: []interface{}{"false"},
			wantErr: "expected 'true' to be JSON equals to 'false' "},
		{name: "boolean, unparsable expected", actual: true, expected: []interface{}{"yes"},
			wantErr: `strconv.ParseBool: parsing "yes": invalid syntax`},
		{name: "boolean, non-zero int expected is true", actual: true, expected: []interface{}{1}},
		{name: "boolean, expected not castable to bool", actual: true, expected: []interface{}{uncastable{n: 1}},
			wantErr: "unable to cast assertions.uncastable{n:1} of type assertions.uncastable to bool"},

		// default
		{name: "float64 actual is an unexpected type", actual: float64(1), expected: []interface{}{"1"},
			wantErr: "unexpected type for actual: float64"},
		{name: "nil actual is an unexpected type", actual: nil, expected: []interface{}{"null"},
			wantErr: "unexpected type for actual: <nil>"},
		{name: "map[string]string actual is an unexpected type", actual: map[string]string{"a": "b"}, expected: []interface{}{`{"a":"b"}`},
			wantErr: "unexpected type for actual: map[string]string"},
		{name: "[]string actual is an unexpected type", actual: []string{"a"}, expected: []interface{}{`["a"]`},
			wantErr: "unexpected type for actual: []string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ShouldJSONEqual(tt.actual, tt.expected...)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
