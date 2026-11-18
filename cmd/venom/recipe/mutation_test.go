package recipe

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The negative control (US3-AS5, FR-016) changes the expected value of one assertion so that it fails whatever the
// page holds. How depends on the operator: mutateAssertion is that table. It has no browser, so its test runs untagged.

const sentinel = "__negative_control__"

// mutation gives the operator and the expected values that replace those of an assertion.
type mutation func(op string, expected []string) (string, []string)

// withValues keeps the operator and replaces the expected values.
func withValues(values ...string) mutation {
	return func(op string, _ []string) (string, []string) { return op, values }
}

// swapped replaces the operator by its opposite and drops the expected values.
func swapped(opposite string) mutation {
	return func(string, []string) (string, []string) { return opposite, nil }
}

// appended keeps the operator and puts the sentinel at the end of the expected text; a quoted value is read without
// its quotes and the result is quoted again when it holds a space.
func appended(op string, expected []string) (string, []string) {
	text := strings.Join(expected, " ")
	if n := len(text); n >= 2 && text[0] == text[n-1] && (text[0] == '"' || text[0] == '\'') {
		text = text[1 : n-1]
	}
	text += sentinel
	if strings.ContainsAny(text, " 	") {
		text = `"` + text + `"`
	}
	return op, []string{text}
}

// mutations maps an operator to its mutation: a number no page reaches, a range that excludes every count, a pattern
// that matches nothing, a boolean or emptiness turned over, a negated equality turned into the plain one against the
// sentinel.
var mutations = map[string]mutation{
	"ShouldEqual":                  withValues(sentinel),
	"ShouldNotEqual":               swappedWith("ShouldEqual", sentinel),
	"ShouldBeGreaterThan":          withValues("99999"),
	"ShouldBeGreaterThanOrEqualTo": withValues("99999"),
	"ShouldBeLessThan":             withValues("-1"),
	"ShouldBeLessThanOrEqualTo":    withValues("-1"),
	"ShouldBeBetween":              withValues("-2", "-1"),
	"ShouldBeBetweenOrEqual":       withValues("-2", "-1"),
	"ShouldContainSubstring":       appended,
	"ShouldStartWith":              appended,
	"ShouldEndWith":                appended,
	"ShouldMatchRegex":             withValues("a^b"),
	"ShouldBeTrue":                 swapped("ShouldBeFalse"),
	"ShouldBeFalse":                swapped("ShouldBeTrue"),
	"ShouldBeEmpty":                swapped("ShouldNotBeEmpty"),
	"ShouldNotBeEmpty":             swapped("ShouldBeEmpty"),
}

// swappedWith replaces the operator and sets the expected values.
func swappedWith(opposite string, values ...string) mutation {
	return func(string, []string) (string, []string) { return opposite, values }
}

// mutateAssertion returns the assertion text changed so that it fails, and false when its operator is not one the
// control can change.
func mutateAssertion(assertion string) (string, bool) {
	fields := strings.Fields(assertion)
	if len(fields) < 2 {
		return "", false
	}
	mutate, known := mutations[fields[1]]
	if !known {
		return "", false
	}
	op, args := mutate(fields[1], fields[2:])
	out := append([]string{fields[0], op}, args...)
	return strings.Join(out, " "), true
}

func TestMutateAssertion(t *testing.T) {
	tests := []struct {
		name, assertion, want string
	}{
		{"equal", "result.title ShouldEqual Home", "result.title ShouldEqual " + sentinel},
		{"not equal", "result.title ShouldNotEqual Error", "result.title ShouldEqual " + sentinel},
		{"greater than", "result.find ShouldBeGreaterThan 0", "result.find ShouldBeGreaterThan 99999"},
		{"greater or equal", "result.count ShouldBeGreaterThanOrEqualTo 1", "result.count ShouldBeGreaterThanOrEqualTo 99999"},
		{"less than", "result.count ShouldBeLessThan 5", "result.count ShouldBeLessThan -1"},
		{"less or equal", "result.count ShouldBeLessThanOrEqualTo 5", "result.count ShouldBeLessThanOrEqualTo -1"},
		{"between", "result.find ShouldBeBetween 1 3", "result.find ShouldBeBetween -2 -1"},
		{"between or equal", "result.find ShouldBeBetweenOrEqual 1 3", "result.find ShouldBeBetweenOrEqual -2 -1"},
		{"contains", "result.text ShouldContainSubstring milk", "result.text ShouldContainSubstring milk" + sentinel},
		{"contains quoted", `result.text ShouldContainSubstring "two words"`, `result.text ShouldContainSubstring "two words` + sentinel + `"`},
		{"starts with", "result.url ShouldStartWith http", "result.url ShouldStartWith http" + sentinel},
		{"ends with", "result.url ShouldEndWith /fr/", "result.url ShouldEndWith /fr/" + sentinel},
		{"regex", "result.text ShouldMatchRegex ^milk$", "result.text ShouldMatchRegex a^b"},
		{"true", "result.found ShouldBeTrue", "result.found ShouldBeFalse"},
		{"false", "result.found ShouldBeFalse", "result.found ShouldBeTrue"},
		{"empty", "result.text ShouldBeEmpty", "result.text ShouldNotBeEmpty"},
		{"not empty", "result.text ShouldNotBeEmpty", "result.text ShouldBeEmpty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := mutateAssertion(tt.assertion)

			assert.True(t, ok)
			assert.Equal(t, tt.want, got)
			assert.NotEqual(t, tt.assertion, got)
		})
	}
}

func TestMutateAssertionRefusesWhatItCannotChange(t *testing.T) {
	for _, assertion := range []string{"result.title ShouldBeNil", "result.title ShouldNotBeNil", "result.title ShouldBogus x", "result.title", ""} {
		t.Run(assertion, func(t *testing.T) {
			_, ok := mutateAssertion(assertion)

			assert.False(t, ok)
		})
	}
}
