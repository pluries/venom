package assertions

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cast"
)

// ShouldBeGreaterThan receives exactly two parameters and ensures that the first is greater than the second.
func ShouldBeGreaterThan(actual interface{}, expected ...interface{}) error {
	return compareOrdered(actual, expected,
		func(a, e float64) bool { return a > e },
		func(a, e string) bool { return a > e },
		"expected: %v greater than %v but it wasn't",
		"expected: %v greater than %v but it wasn't")
}

// ShouldBeGreaterThanOrEqualTo receives exactly two parameters and ensures that the first is greater than or equal to the second.
func ShouldBeGreaterThanOrEqualTo(actual interface{}, expected ...interface{}) error {
	return compareOrdered(actual, expected,
		func(a, e float64) bool { return a >= e },
		func(a, e string) bool { return a >= e },
		"expected: %v greater than or equals to %v but it wasn't",
		"expected: %v greater than or equals to %v but it wasn't")
}

// ShouldBeLessThan receives exactly two parameters and ensures that the first is less than the second.
func ShouldBeLessThan(actual interface{}, expected ...interface{}) error {
	return compareOrdered(actual, expected,
		func(a, e float64) bool { return a < e },
		func(a, e string) bool { return a < e },
		"expected: %v less than %v but it wasn't",
		"expected: %v less than %v but it wasn't")
}

// ShouldBeLessThanOrEqualTo receives exactly two parameters and ensures that the first is less than or equal to the second.
func ShouldBeLessThanOrEqualTo(actual interface{}, expected ...interface{}) error {
	return compareOrdered(actual, expected,
		func(a, e float64) bool { return a <= e },
		func(a, e string) bool { return a <= e },
		"expected '%v' less than or equals to %v but it wasn't",
		"expected: %v less than or equals to %v but it wasn't")
}

// parseOrderedFloat is the numeric parse compareOrdered runs on actual. For a json.Number, the
// parse error (if any) is returned as-is and isNumeric stays true, so it short-circuits in the
// caller instead of ever reaching the lexical fallback. For anything else, a cast failure reports
// isNumeric=false with a nil error, which is the caller's signal to fall back to a lexical
// comparison instead of failing outright.
func parseOrderedFloat(actual interface{}) (float64, bool, error) {
	switch x := actual.(type) {
	case json.Number:
		f, err := x.Float64()
		return f, true, err
	default:
		f, err := cast.ToFloat64E(actual)
		if err != nil {
			return 0, false, nil
		}
		return f, true, nil
	}
}

// compareLexically is the lexical fallback compareOrdered runs when actual is not numeric: it
// casts actual and expected to strings, propagating either cast error as-is, then applies strs.
func compareLexically(actual, expected interface{}, strs func(a, e string) bool, stringFailure string) error {
	actualS, err := cast.ToStringE(actual)
	if err != nil {
		return err
	}

	expectedS, err := cast.ToStringE(expected)
	if err != nil {
		return err
	}

	if strs(actualS, expectedS) {
		return nil
	}

	return fmt.Errorf(stringFailure, actual, expected)
}

// compareOrdered is the shared body of the four ordering assertions. The numeric and string
// failures take separate formats because ShouldBeLessThanOrEqualTo words them differently.
func compareOrdered(actual interface{}, expected []interface{},
	numbers func(a, e float64) bool, strs func(a, e string) bool,
	numberFailure, stringFailure string) error {
	if err := need(1, expected); err != nil {
		return err
	}

	if !areSameTypes(actual, expected[0]) {
		return newAssertionError(needSameType)
	}

	actualF, isNumeric, err := parseOrderedFloat(actual)
	if err != nil {
		return err
	}
	if !isNumeric {
		return compareLexically(actual, expected[0], strs, stringFailure)
	}

	expectedF, err := cast.ToFloat64E(expected[0])
	if err != nil {
		return err
	}

	if numbers(actualF, expectedF) {
		return nil
	}

	return fmt.Errorf(numberFailure, actual, expected[0])
}

// ShouldBeBetween receives exactly two parameters and ensures that the first is less than the second.
func ShouldBeBetween(actual interface{}, expected ...interface{}) error {
	if err := need(2, expected); err != nil {
		return err
	}

	if !areSameTypes(expected[0], expected[1]) {
		return newAssertionError(needSameType)
	}

	err1 := ShouldBeLessThan(actual, expected[1])
	err2 := ShouldBeGreaterThan(actual, expected[0])
	if err1 != nil || err2 != nil {
		return fmt.Errorf("expected '%v' between %v and %v but it wasn't", actual, expected[0], expected[1])
	}
	return nil
}

// ShouldNotBeBetween receives exactly three parameters: an actual value, a lower bound, and an upper bound.
// It ensures that the actual value is NOT between both bounds.
func ShouldNotBeBetween(actual interface{}, expected ...interface{}) error {
	if err := ShouldBeBetween(actual, expected...); err != nil {
		if _, ok := err.(*AssertionError); ok {
			return err
		}
		return nil
	}
	return fmt.Errorf("expected '%v' not between %v and %v but it was", actual, expected[0], expected[1])
}

// ShouldBeBetweenOrEqual receives exactly three parameters: an actual value, a lower bound, and an upper bound.
// It ensures that the actual value is between both bounds or equal to one of them.
func ShouldBeBetweenOrEqual(actual interface{}, expected ...interface{}) error {
	if err := need(2, expected); err != nil {
		return err
	}

	if !areSameTypes(expected[0], expected[1]) {
		return newAssertionError(needSameType)
	}

	err1 := ShouldBeLessThanOrEqualTo(actual, expected[1])
	err2 := ShouldBeGreaterThanOrEqualTo(actual, expected[0])
	if err1 != nil || err2 != nil {
		return fmt.Errorf("expected '%v' between %v and %v but it wasn't", actual, expected[0], expected[1])
	}
	return nil
}

// ShouldNotBeBetweenOrEqual receives exactly three parameters: an actual value, a lower bound, and an upper bound.
// It ensures that the actual value is nopt between the bounds nor equal to either of them.
func ShouldNotBeBetweenOrEqual(actual interface{}, expected ...interface{}) error {
	if err := ShouldBeBetweenOrEqual(actual, expected...); err != nil {
		if _, ok := err.(*AssertionError); ok {
			return err
		}
		return nil
	}
	return fmt.Errorf("expected '%v' not between or equal to %v and %v but it was", actual, expected[0], expected[1])
}
