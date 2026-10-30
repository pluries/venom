package assertions

import (
	"fmt"
	"reflect"

	"github.com/spf13/cast"
)

// ShouldContain receives exactly two parameters. The first is a slice or a single value and the
// second is a proposed member. Membership is determined using ShouldEqual.
func ShouldContain(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}
	if actual == nil || reflect.TypeOf(actual).Kind() != reflect.Slice {
		return ShouldEqual(actual, expected[0])
	}
	actualSlice, err := cast.ToSliceE(actual)
	if err != nil {
		return err
	}
	for i := range actualSlice {
		if ShouldEqual(actualSlice[i], expected[0]) == nil {
			return nil
		}
	}
	return fmt.Errorf("expected '%v' contain %v but it wasnt", actual, expected[0])
}

// ShouldNotContain receives exactly two parameters. The first is a slice and the
// second is a proposed member. Membership is determinied using ShouldEqual.
func ShouldNotContain(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}
	actualSlice, err := cast.ToSliceE(actual)
	if err != nil {
		return err
	}
	for i := range actualSlice {
		if ShouldEqual(actualSlice[i], expected[0]) == nil {
			return fmt.Errorf("expected '%v' not contain %v but it was", actual, expected[0])
		}
	}
	return nil
}

// ShouldContainKey receives exactly two parameters. The first is a map and the
// second is a proposed key.
func ShouldContainKey(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}
	actualMap, err := cast.ToStringMapE(actual)
	if err != nil {
		return err
	}
	for k := range actualMap {
		if ShouldEqual(k, expected[0]) == nil {
			return nil
		}
	}
	return fmt.Errorf("expected '%v' contain key %v but it wasnt", actual, expected[0])
}

// ShouldNotContainKey receives exactly two parameters. The first is a map and the
// second is a proposed absent key.
func ShouldNotContainKey(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}
	actualMap, err := cast.ToStringMapE(actual)
	if err != nil {
		return err
	}
	for k := range actualMap {
		if ShouldEqual(k, expected[0]) == nil {
			return fmt.Errorf("expected '%v' not contain key %v but it was", actual, expected[0])
		}
	}
	return nil
}

// ShouldBeIn receives at least 2 parameters. The first is a proposed member of the collection
// that is passed in either as the second parameter, or of the collection that is comprised
// of all the remaining parameters. This assertion ensures that the proposed member is in
// the collection (using ShouldEqual).
//
// Example of testsuite file:
//
//	name: Assertions testsuite
//	testcases:
//	  - name: ShouldBeIn
//	    steps:
//	    - script: echo 1
//	      assertions:
//	      - result.systemoutjson ShouldBeIn 1 2
func ShouldBeIn(actual interface{}, expected ...interface{}) error {
	if err := atLeast(1, expected); err != nil {
		return err
	}

	expectedSlice, err := cast.ToSliceE(expected)
	if err != nil {
		return err
	}
	for i := range expectedSlice {
		if ShouldEqual(expectedSlice[i], actual) == nil {
			return nil
		}
	}
	return fmt.Errorf("expected '%v' in %v but it wasnt", actual, expectedSlice)
}

// ShouldNotBeIn receives at least 2 parameters. The first is a proposed member of the collection
// that is passed in either as the second parameter, or of the collection that is comprised
// of all the remaining parameters. This assertion ensures that the proposed member is NOT in
// the collection (using ShouldEqual).
//
// Example of testsuite file:
//
//	name: Assertions testsuite
//	testcases:
//	  - name: ShouldNotBeIn
//	    steps:
//	    - script: echo 3
//	      assertions:
//	      - result.systemoutjson ShouldNotBeIn 1 2
func ShouldNotBeIn(actual interface{}, expected ...interface{}) error {
	if err := atLeast(1, expected); err != nil {
		return err
	}

	if err := ShouldBeIn(actual, expected...); err != nil {
		return nil
	}

	return fmt.Errorf("expected '%v' not in %v but it was", actual, expected)
}

// ShouldBeEmpty receives a single parameter (actual) and determines whether or not
// calling len(actual) would return `0`.
func ShouldBeEmpty(actual interface{}, expected ...interface{}) error {
	if err := need(0, expected); err != nil {
		return err
	}

	if actual == nil {
		return nil
	}

	value := reflect.ValueOf(actual)
	switch value.Kind() {
	case reflect.Slice, reflect.Chan, reflect.Map, reflect.String:
		if value.Len() == 0 {
			return nil
		}
	case reflect.Pointer:
		elem := value.Elem()
		kind := elem.Kind()
		if (kind == reflect.Slice || kind == reflect.Array) && elem.Len() == 0 {
			return nil
		}
	}

	return fmt.Errorf("expected '%v' to be empty but it wasn't", actual)
}

// ShouldNotBeEmpty receives a single parameter (actual) and determines whether or not
// calling len(actual) would return a value greater than zero.
func ShouldNotBeEmpty(actual interface{}, expected ...interface{}) error {
	if err := need(0, expected); err != nil {
		return err
	}
	if err := ShouldBeEmpty(actual); err != nil {
		return nil
	}
	return fmt.Errorf("expected '%v' not to be empty but it wasn't", actual)
}

// ShouldHaveLength receives 2 parameters. The first is a collection to check
// the length of, the second being the expected length.
func ShouldHaveLength(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}

	length, err := cast.ToInt64E(expected[0])
	if err != nil {
		return err
	}

	var actualLength int

	value := reflect.ValueOf(actual)
	switch value.Kind() {
	case reflect.Slice, reflect.Chan, reflect.Map, reflect.String:
		actualLength = value.Len()
		if value.Len() == int(length) {
			return nil
		}
	case reflect.Pointer:
		elem := value.Elem()
		kind := elem.Kind()
		actualLength = elem.Len()
		if (kind == reflect.Slice || kind == reflect.Array) && actualLength == int(length) {
			return nil
		}
	}

	return fmt.Errorf("expected '%v' have length of %d but it wasn't (%d)", actual, length, actualLength)
}
