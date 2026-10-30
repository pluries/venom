package venom

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_splitAssertion(t *testing.T) {
	for _, tt := range []struct {
		Assertion string
		Args      []string
	}{
		{Assertion: `cmd arg`, Args: []string{"cmd", "arg"}},
		{Assertion: `cmd arg1 "arg 2"`, Args: []string{"cmd", "arg1", "arg 2"}},
		{Assertion: `cmd 'arg 1' "arg 2"`, Args: []string{"cmd", "arg 1", "arg 2"}},
		{Assertion: `cmd 'arg 1' "'arg' 2"`, Args: []string{"cmd", "arg 1", "'arg' 2"}},
		{Assertion: `cmd '"arg 1"' "'arg' 2"`, Args: []string{"cmd", "\"arg 1\"", "'arg' 2"}},
	} {
		args := splitAssertion(tt.Assertion)
		if !reflect.DeepEqual(args, tt.Args) {
			t.Errorf("expected args to be equal to %#v, got %#v", tt.Args, args)
		}
	}
}

// checkBranchResult is the result every checkBranch case evaluates against: "a" resolves to the int 1.
var checkBranchResult = map[string]interface{}{"a": 1}

const (
	branchPass = "a ShouldEqual 1"
	branchFail = "a ShouldEqual 2"
)

// TestCheckBranch characterises checkBranch before #2031 splits it: which branches fail, and the exact text of the
// failure (Failure.Error) and of the report line built around it (Failure.Value).
func TestCheckBranch(t *testing.T) {
	tests := []struct {
		name    string
		branch  map[string]interface{}
		wantErr string // "" = the branch passes (nil *Failure)
	}{
		{
			name:    "no operator fails",
			branch:  map[string]interface{}{},
			wantErr: "expected exactly 1 logical operator but 0 were provided",
		},
		{
			name:    "two operators fail",
			branch:  map[string]interface{}{"and": []interface{}{branchPass}, "or": []interface{}{branchPass}},
			wantErr: "expected exactly 1 logical operator but 2 were provided",
		},
		{
			name:    "operands given as a single string fail",
			branch:  map[string]interface{}{"and": branchPass},
			wantErr: "expected and operands to be an []interface{}, got a ShouldEqual 1",
		},
		{
			name:    "operands given as nil fail",
			branch:  map[string]interface{}{"or": nil},
			wantErr: "expected or operands to be an []interface{}, got <nil>",
		},
		{
			name:    "operands given as a typed []string fail",
			branch:  map[string]interface{}{"and": []string{branchPass}},
			wantErr: "expected and operands to be an []interface{}, got [a ShouldEqual 1]",
		},
		{
			name:   "empty operands pass",
			branch: map[string]interface{}{"and": []interface{}{}},
		},
		{
			name:   "empty operands pass even with an unsupported operator",
			branch: map[string]interface{}{"nand": []interface{}{}},
		},
		{
			name:   "and, every operand passes",
			branch: map[string]interface{}{"and": []interface{}{branchPass, branchPass}},
		},
		{
			name:    "and, one operand fails",
			branch:  map[string]interface{}{"and": []interface{}{branchPass, branchFail}},
			wantErr: "1/2 assertions succeeded:\n  - pass: a ShouldEqual 1\n  - fail: a ShouldEqual 2\n",
		},
		{
			name:    "and, every operand fails",
			branch:  map[string]interface{}{"and": []interface{}{branchFail, branchFail}},
			wantErr: "0/2 assertions succeeded:\n  - fail: a ShouldEqual 2\n  - fail: a ShouldEqual 2\n",
		},
		{
			name:    "or, no operand passes",
			branch:  map[string]interface{}{"or": []interface{}{branchFail, branchFail}},
			wantErr: "no assertions succeeded:\n  - fail: a ShouldEqual 2\n  - fail: a ShouldEqual 2\n",
		},
		{
			name:   "or, exactly one operand passes",
			branch: map[string]interface{}{"or": []interface{}{branchFail, branchPass}},
		},
		{
			name:   "or, every operand passes",
			branch: map[string]interface{}{"or": []interface{}{branchPass, branchPass}},
		},
		{
			name:    "xor, no operand passes",
			branch:  map[string]interface{}{"xor": []interface{}{branchFail, branchFail}},
			wantErr: "no assertions succeeded:\n  - fail: a ShouldEqual 2\n  - fail: a ShouldEqual 2\n",
		},
		{
			name:   "xor, exactly one operand passes",
			branch: map[string]interface{}{"xor": []interface{}{branchPass, branchFail}},
		},
		{
			name:    "xor, more than one operand passes",
			branch:  map[string]interface{}{"xor": []interface{}{branchPass, branchFail, branchPass}},
			wantErr: "multiple assertions succeeded but expected only one to succeed:\n  - pass: a ShouldEqual 1\n  - fail: a ShouldEqual 2\n  - pass: a ShouldEqual 1\n",
		},
		{
			name:   "not, no operand passes",
			branch: map[string]interface{}{"not": []interface{}{branchFail, branchFail}},
		},
		{
			name:    "not, one operand passes",
			branch:  map[string]interface{}{"not": []interface{}{branchFail, branchPass}},
			wantErr: "some assertions succeeded but expected none to succeed:\n  - fail: a ShouldEqual 2\n  - pass: a ShouldEqual 1\n",
		},
		{
			name:    "unsupported operator fails after its operands are evaluated",
			branch:  map[string]interface{}{"nand": []interface{}{branchPass}},
			wantErr: "unsupported assertion operator nand",
		},
		{
			name:   "nested branch that passes counts as a passing operand",
			branch: map[string]interface{}{"and": []interface{}{branchPass, map[string]interface{}{"or": []interface{}{branchFail, branchPass}}}},
		},
		{
			name:    "nested branch that fails is reported with its map form",
			branch:  map[string]interface{}{"and": []interface{}{branchPass, map[string]interface{}{"not": []interface{}{branchPass}}}},
			wantErr: "1/2 assertions succeeded:\n  - pass: a ShouldEqual 1\n  - fail: map[not:[a ShouldEqual 1]]\n",
		},
		{
			name:    "non-string operand fails and is reported with a %!s verb",
			branch:  map[string]interface{}{"or": []interface{}{42}},
			wantErr: "no assertions succeeded:\n  - fail: %!s(int=42)\n",
		},
		{
			name:    "operand with a syntax error fails without its own message",
			branch:  map[string]interface{}{"and": []interface{}{"a"}},
			wantErr: "0/1 assertions succeeded:\n  - fail: a\n",
		},
	}

	ctx := context.WithValue(context.Background(), ContextKey("var.venom.testsuite.filepath"), "suite.yml")
	tc := TestCase{originalName: "tc", SourceLine: 7}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkBranch(ctx, tc, 2, 1, 0, tt.branch, checkBranchResult)
			if tt.wantErr == "" {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.EqualError(t, got.Error, tt.wantErr)
			assert.Equal(t, `Testcase "tc", step #2-1: `+tt.wantErr+` (suite.yml:7)`, got.Value)
			assert.Empty(t, got.Assertion)
			assert.False(t, got.AssertionRequired)
			assert.Equal(t, 2, got.StepNumber)
			assert.Equal(t, 7, got.TestcaseLineNumber)
			assert.Equal(t, "suite.yml", got.TestcaseClassname)
		})
	}

	t.Run("a failing Must operand does not make the branch failure required", func(t *testing.T) {
		got := checkBranch(ctx, tc, 2, 1, 0, map[string]interface{}{"and": []interface{}{"a MustEqual 2"}}, checkBranchResult)
		require.NotNil(t, got)
		require.EqualError(t, got.Error, "0/1 assertions succeeded:\n  - fail: a MustEqual 2\n")
		assert.False(t, got.AssertionRequired)
	})
}

// namedInt is a type stringToType does not list, although its underlying type is int.
type namedInt int

// TestStringToType characterises stringToType before #2031 splits it. It pins the value and its dynamic type,
// which do not always match the type asked for (int8 → int64, uint8 → uint64, a negative uint wraps).
func TestStringToType(t *testing.T) {
	tests := []struct {
		name    string
		val     string
		valType interface{}
		want    interface{}
		wantErr string // "" = no error
	}{
		{name: "bool parses", val: "true", valType: false, want: true},
		{name: "bool, invalid input returns false and the ParseBool error", val: "yes", valType: false, want: false,
			wantErr: `strconv.ParseBool: parsing "yes": invalid syntax`},
		{name: "string is returned unchanged", val: "abc", valType: "", want: "abc"},
		{name: "int parses with Atoi to an int", val: "42", valType: 0, want: 42},
		{name: "int, invalid input returns 0 and the Atoi error", val: "x", valType: 0, want: 0,
			wantErr: `strconv.Atoi: parsing "x": invalid syntax`},
		{name: "int8 parses to an int64", val: "42", valType: int8(0), want: int64(42)},
		{name: "int8, out of range returns the clamped int64 and the ParseInt error", val: "200", valType: int8(0), want: int64(127),
			wantErr: `strconv.ParseInt: parsing "200": value out of range`},
		{name: "int16 parses to an int64", val: "42", valType: int16(0), want: int64(42)},
		{name: "int32 parses to an int64", val: "42", valType: int32(0), want: int64(42)},
		{name: "int64 parses to an int64", val: "42", valType: int64(0), want: int64(42)},
		{name: "uint parses with Atoi to a uint", val: "42", valType: uint(0), want: uint(42)},
		{name: "uint, negative input wraps around without error", val: "-1", valType: uint(0), want: ^uint(0)},
		{name: "uint, invalid input returns 0 and the Atoi error", val: "x", valType: uint(0), want: uint(0),
			wantErr: `strconv.Atoi: parsing "x": invalid syntax`},
		{name: "uint8 parses to a uint64", val: "42", valType: uint8(0), want: uint64(42)},
		{name: "uint8, negative input returns 0 and the ParseUint error", val: "-1", valType: uint8(0), want: uint64(0),
			wantErr: `strconv.ParseUint: parsing "-1": invalid syntax`},
		{name: "uint8, out of range returns the clamped uint64 and the ParseUint error", val: "300", valType: uint8(0), want: uint64(255),
			wantErr: `strconv.ParseUint: parsing "300": value out of range`},
		{name: "uint16 parses to a uint64", val: "42", valType: uint16(0), want: uint64(42)},
		{name: "uint32 parses to a uint64", val: "42", valType: uint32(0), want: uint64(42)},
		{name: "uint64 parses to a uint64", val: "42", valType: uint64(0), want: uint64(42)},
		{name: "float32 parses to a float32", val: "1.5", valType: float32(0), want: float32(1.5)},
		{name: "float32, invalid input returns 0 and the ParseFloat error", val: "x", valType: float32(0), want: float32(0),
			wantErr: `strconv.ParseFloat: parsing "x": invalid syntax`},
		{name: "float64 parses to a float64", val: "1.5", valType: float64(0), want: 1.5},
		{name: "float64, invalid input returns 0 and the ParseFloat error", val: "x", valType: float64(0), want: float64(0),
			wantErr: `strconv.ParseFloat: parsing "x": invalid syntax`},
		{name: "time.Time parses RFC3339", val: "2026-01-02T03:04:05Z", valType: time.Time{},
			want: time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)},
		{name: "time.Time, invalid input returns the zero time and the Parse error", val: "x", valType: time.Time{}, want: time.Time{},
			wantErr: `parsing time "x" as "2006-01-02T15:04:05Z07:00": cannot parse "x" as "2006"`},
		{name: "time.Duration parses", val: "1m30s", valType: time.Duration(0), want: 90 * time.Second},
		{name: "time.Duration, invalid input returns 0 and the ParseDuration error", val: "x", valType: time.Duration(0), want: time.Duration(0),
			wantErr: `time: invalid duration "x"`},
		{name: "nil type returns the string unchanged", val: "42", valType: nil, want: "42"},
		{name: "unlisted named int type returns the string unchanged", val: "42", valType: namedInt(0), want: "42"},
		{name: "unlisted slice type returns the string unchanged", val: "42", valType: []interface{}{}, want: "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stringToType(tt.val, tt.valType)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.IsType(t, tt.want, got)
			assert.Equal(t, tt.want, got)
		})
	}
}
