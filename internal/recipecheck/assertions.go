package recipecheck

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ovh/venom/assertions"
)

// appFields are the result fields of a web or mobile step that come from the application under test.
var appFields = map[string]bool{
	"find": true, "title": true, "url": true, "text": true, "html": true, "value": true, "valuejson": true,
	"count": true, "found": true, "source": true, "context": true, "device": true, "platform": true,
}

// neverFails are the operators that hold on any value the engine reads, so they never count as reading the app.
var neverFails = map[string]bool{"ShouldNotBeNil": true, "ShouldNotExist": true}

// ownResult matches a template that reads the result of the step itself.
var ownResult = regexp.MustCompile(`\{\{[^}]*\.result\b`)

// splitAssertion splits an assertion string on spaces, keeping a quoted argument whole and without its quotes, as the
// engine's own splitAssertion does.
func splitAssertion(a string) []string {
	lastQuote := rune(0)
	f := func(c rune) bool {
		switch {
		case c == lastQuote:
			lastQuote = rune(0)
			return false
		case lastQuote != rune(0):
			return false
		case unicode.In(c, unicode.Quotation_Mark):
			lastQuote = c
			return false
		default:
			return unicode.IsSpace(c)
		}
	}
	m := strings.FieldsFunc(a, f)
	for i, e := range m {
		first, _ := utf8.DecodeRuneInString(e)
		last, _ := utf8.DecodeLastRuneInString(e)
		if utf8.RuneCountInString(e) >= 2 && unicode.In(first, unicode.Quotation_Mark) && first == last {
			m[i] = string([]rune(e)[1 : utf8.RuneCountInString(e)-1])
		}
	}
	return m
}

// isAppReading reports whether an assertion string reads the application: a listed result field, a registered
// operator that is not a success-only one on a non-app field, and an expected value not derived from the same step.
// The success-only operators only matter on fields outside the list, and those never count.
func isAppReading(assertion string) bool {
	parts := splitAssertion(assertion)
	if len(parts) < 2 {
		return false
	}
	field, ok := strings.CutPrefix(strings.ToLower(parts[0]), "result.")
	if !ok {
		return false
	}
	name, _, _ := strings.Cut(field, ".")
	if !appFields[name] {
		return false
	}
	if _, registered := assertions.Get(parts[1]); !registered || neverFails[parts[1]] {
		return false
	}
	for _, expected := range parts[2:] {
		if ownResult.MatchString(expected) {
			return false
		}
	}
	return true
}

// ruleRC05 reports a testcase with no app-reading assertion, and a suite with no testcase.
func ruleRC05(f *Folder) []Finding {
	var out []Finding
	for _, s := range f.Suites {
		if len(s.Testcases) == 0 {
			out = append(out, s.finding(nil, "RC05", s.line(), "the suite has no testcase, so it proves nothing"))
			continue
		}
		for _, tc := range s.Testcases {
			if !tc.Skipped && !readsApp(tc) {
				out = append(out, s.finding(tc, "RC05", tc.Line, "the testcase has no assertion that reads the application"))
			}
		}
	}
	return out
}

func readsApp(tc *Testcase) bool {
	for _, step := range tc.Steps {
		for _, a := range step.Assertions {
			if isAppReading(a.Text) {
				return true
			}
		}
	}
	return false
}

// ruleRC07 reports a tautological assertion.
func ruleRC07(f *Folder) []Finding {
	var out []Finding
	for _, s := range f.Suites {
		for _, tc := range s.Testcases {
			out = append(out, tautologies(s, tc)...)
		}
	}
	return out
}

func tautologies(s *Suite, tc *Testcase) []Finding {
	var out []Finding
	for _, step := range tc.Steps {
		for _, a := range step.Assertions {
			if why := tautology(a.Text); why != "" {
				out = append(out, s.finding(tc, "RC07", a.Line, "the assertion cannot fail: "+why))
			}
		}
	}
	return out
}

var (
	sameSideOperators = map[string]bool{
		"ShouldEqual": true, "ShouldResemble": true, "ShouldEqualTrimSpace": true, "ShouldContainSubstring": true,
		"ShouldStartWith": true, "ShouldEndWith": true, "ShouldBeGreaterThanOrEqualTo": true, "ShouldBeLessThanOrEqualTo": true,
	}
	emptyStringOperators = map[string]bool{"ShouldContainSubstring": true, "ShouldStartWith": true, "ShouldEndWith": true}
	matchAll             = map[string]bool{".*": true, "^.*$": true, "(?s).*": true}
)

const (
	minInt64 = -9223372036854775808.0
	maxInt64 = 9223372036854775807.0
)

// tautology returns why an assertion string cannot fail, or "" when it can.
func tautology(assertion string) string {
	p := splitAssertion(assertion)
	if len(p) < 3 {
		return ""
	}
	field, op, args := p[0], p[1], p[2:]
	switch {
	case sameSideOperators[op] && field == args[0]:
		return "the same expression is on both sides"
	case emptyStringOperators[op] && args[0] == "":
		return "every string has the empty string"
	case op == "ShouldMatchRegex" && matchAll[args[0]]:
		return "the pattern matches everything"
	case countAtLeastNegative(field, op, args[0]):
		return "a count is never below zero"
	case op == "ShouldBeBetween" && len(args) == 2 && coversWholeType(args[0], args[1]):
		return "the range covers the whole type"
	}
	return ""
}

func countAtLeastNegative(field, op, arg string) bool {
	if f := strings.ToLower(field); f != "result.count" && f != "result.find" {
		return false
	}
	n, err := strconv.ParseFloat(arg, 64)
	if err != nil {
		return false
	}
	return (op == "ShouldBeGreaterThan" && n < 0) || (op == "ShouldBeGreaterThanOrEqualTo" && n <= 0)
}

func coversWholeType(lo, hi string) bool {
	low, errLow := strconv.ParseFloat(lo, 64)
	high, errHigh := strconv.ParseFloat(hi, 64)
	return errLow == nil && errHigh == nil && low <= minInt64 && high >= maxInt64
}
