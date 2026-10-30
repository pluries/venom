package assertions

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strings"

	"github.com/spf13/cast"
)

// ShouldEqual receives exactly two parameters and does an equality check.
//
// Example of testsuite file:
//
//	name: Assertions testsuite
//	testcases:
//	- name: test assertion
//	  steps:
//	  - script: echo 'foo'
//	    assertions:
//	    - result.code ShouldEqual 0
func ShouldEqual(actual interface{}, expected ...interface{}) error {
	// if expected is an array, we consider that this array is an array of string
	// so, we concat all values before doing the comparison
	if len(expected) > 0 {
		var args string
		for i := range expected {
			actualS, err := cast.ToStringE(expected[i])
			if err != nil {
				return err
			}
			args += actualS + " "
		}
		if deepEqual(actual, strings.TrimRight(args, " ")) {
			return nil
		}
		return fmt.Errorf("expected: %v got: %v", args, actual)
	}

	if err := need(1, expected); err != nil {
		return err
	}
	if deepEqual(actual, expected[0]) {
		return nil
	}
	return fmt.Errorf("expected: %v got: %v", expected[0], actual)
}

// ShouldMatchRegex receives exactly two parameters and does a regex match check.
func ShouldMatchRegex(actual interface{}, expected ...interface{}) error {
	if need(1, expected) != nil {
		return fmt.Errorf("expected one regex pattern")
	}
	expectedAsString, regexNotString := cast.ToStringE(expected[0])
	if regexNotString != nil {
		return fmt.Errorf("expected a string for regex pattern")
	}
	reg, err := regexp.Compile(expectedAsString)
	if err != nil {
		return err
	}
	actualAsString, castingErr := cast.ToStringE(actual)
	if castingErr != nil {
		return castingErr
	}
	if !reg.MatchString(actualAsString) {
		return fmt.Errorf("value %v not matching pattern : %v", actual, expected[0])
	}
	return nil
}

// ShouldNotEqual receives exactly two parameters and does an inequality check.
func ShouldNotEqual(actual interface{}, expected ...interface{}) error {
	if err := ShouldEqual(actual, expected...); err == nil {
		return fmt.Errorf("not expected: %v got: %v", expected[0], actual)
	}
	return nil
}

// ShouldAlmostEqual makes sure that two parameters are close enough to being equal.
// The acceptable delta may be specified with a third argument.
func ShouldAlmostEqual(actual interface{}, expected ...interface{}) error {
	if err := need(2, expected); err != nil {
		return err
	}
	actualF, err := cast.ToFloat64E(actual)
	if err != nil {
		return err
	}

	expectedF, err := cast.ToFloat64E(expected[0])
	if err != nil {
		return err
	}

	deltaF, err := cast.ToFloat64E(expected[1])
	if err != nil {
		return err
	}

	actualDeltaF := math.Abs(actualF - expectedF)

	if actualDeltaF <= deltaF {
		return nil
	}

	return fmt.Errorf("expected: %v(+/- %v) got: %v (%v)", expectedF, deltaF, actualF, actualDeltaF)
}

// ShouldNotAlmostEqual makes sure that two parameters are not close enough to being equal.
// The unacceptable delta may be specified with a third argument.
func ShouldNotAlmostEqual(actual interface{}, expected ...interface{}) error {
	if err := need(2, expected); err != nil {
		return err
	}
	actualF, err := cast.ToFloat64E(actual)
	if err != nil {
		return err
	}

	expectedF, err := cast.ToFloat64E(expected[0])
	if err != nil {
		return err
	}

	deltaF, err := cast.ToFloat64E(expected[1])
	if err != nil {
		return err
	}

	if math.Abs(actualF-expectedF) >= deltaF {
		return nil
	}

	return fmt.Errorf("not expected: %v(+/- %v) got: %v (%v)", expected[0], expected[1], actual, math.Abs(actualF-expectedF))
}

// ShouldBeNil receives a single parameter and ensures that it is nil.
func ShouldBeNil(actual interface{}, expected ...interface{}) error {
	if err := need(0, expected); err != nil {
		return err
	}
	if isNil(actual) {
		return nil
	}
	return fmt.Errorf("expected: Nil but is wasn't")
}

// ShouldNotExist receives a single parameter and ensures that it is nil
func ShouldNotExist(actual interface{}, _ ...interface{}) error {
	if ShouldBeNil(actual) != nil {
		return fmt.Errorf("expected not exist but it was")
	}
	return nil
}

// ShouldNotBeNil receives a single parameter and ensures that it is not nil.
func ShouldNotBeNil(actual interface{}, expected ...interface{}) error {
	if err := need(0, expected); err != nil {
		return err
	}
	if !isNil(actual) {
		return nil
	}
	return fmt.Errorf("expected: Not Nil but is was")
}

// ShouldBeTrue receives a single parameter and ensures that it is true.
func ShouldBeTrue(actual interface{}, expected ...interface{}) error {
	if err := need(0, expected); err != nil {
		return err
	}
	b, err := cast.ToBoolE(actual)
	if err != nil {
		return err
	}
	if b {
		return nil
	}
	return fmt.Errorf("expected: True but is wasn't")
}

// ShouldBeFalse receives a single parameter and ensures that it is false.
func ShouldBeFalse(actual interface{}, expected ...interface{}) error {
	if err := need(0, expected); err != nil {
		return err
	}
	b, err := cast.ToBoolE(actual)
	if err != nil {
		return err
	}
	if !b {
		return nil
	}
	return fmt.Errorf("expected: False but is wasn't")
}

// ShouldBeZeroValue receives a single parameter and ensures that it is
// the Go equivalent of the default value, or "zero" value.
func ShouldBeZeroValue(actual interface{}, expected ...interface{}) error {
	if err := need(0, expected); err != nil {
		return err
	}
	b := actual == nil || reflect.DeepEqual(actual, reflect.Zero(reflect.TypeOf(actual)).Interface())
	if b {
		return nil
	}
	return fmt.Errorf("expected: Zero Value but is wasn't")
}

// ShouldEqualTrimSpace receives exactly 2 string parameters and ensures that the first is equal to the second
// after removing all leading and trailing whitespace using strings.TrimSpace(first).
func ShouldEqualTrimSpace(actual interface{}, expected ...interface{}) error {
	actualS, err := cast.ToStringE(actual)
	if err != nil {
		return err
	}
	return ShouldEqual(strings.TrimSpace(actualS), expected...)
}
