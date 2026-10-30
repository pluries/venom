package assertions

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/spf13/cast"
)

// ShouldJSONContain receives exactly two parameters. The first is a slice, the
// second is a proposed JSON member.
// Equality is determined using ShouldJSONEqual.
func ShouldJSONContain(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}
	actualSlice, err := cast.ToSliceE(actual)
	if err != nil {
		return err
	}
	for i := range actualSlice {
		if ShouldJSONEqual(actualSlice[i], expected[0]) == nil {
			return nil
		}
	}
	return fmt.Errorf("expected '%v' to contain %v but it wasnt", actual, expected[0])
}

// ShouldNotJSONContain receives exactly two parameters. The first is a slice, the
// second is a proposed JSON member.
// Equality is determined using ShouldJSONEqual.
func ShouldNotJSONContain(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}
	actualSlice, err := cast.ToSliceE(actual)
	if err != nil {
		return err
	}
	for i := range actualSlice {
		if ShouldJSONEqual(actualSlice[i], expected[0]) == nil {
			return fmt.Errorf("expected '%v' not contain %v but it was", actual, expected[0])
		}
	}
	return nil
}

// ShouldJSONContainWithKey receives exactly three parameters. The first is a slice, the
// second is a key in the inner slice structure and the third is a proposed value associated to the key.
// Equality is determined using ShouldJSONEqual.
func ShouldJSONContainWithKey(actual interface{}, expected ...interface{}) error {
	if err := need(2, expected); err != nil {
		return err
	}
	actualSlice, err := cast.ToSliceE(actual)
	if err != nil {
		return err
	}
	if reflect.TypeOf(expected[0]).Kind() != reflect.String {
		return fmt.Errorf("expected '%v' to be a string", expected[0])
	}
	expectedKey := cast.ToString(expected[0])
	for i := range actualSlice {
		if reflect.TypeOf(actualSlice[i]).Kind() != reflect.Map && reflect.TypeOf(actualSlice[i]).Kind() != reflect.Struct {
			return fmt.Errorf("expected '%v' to be a map or a struct", actualSlice[i])
		}
		elem := cast.ToStringMap(actualSlice[i])
		if err != nil {
			return err
		}
		if _, ok := elem[expectedKey]; ok {
			if ShouldJSONEqual(elem[expectedKey], expected[1]) == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("expected '%v' contain a key '%s' with value %v but it wasnt", actual, expectedKey, expected[1])
}

// ShouldJSONContainAllWithKey receives exactly three parameters. The first is a slice, the
// second is a key in the inner slice structure and the third is a proposed value associated to the key.
// Equality is determined using ShouldJSONEqual.
func ShouldJSONContainAllWithKey(actual interface{}, expected ...interface{}) error {
	if err := need(2, expected); err != nil {
		return err
	}
	actualSlice, err := cast.ToSliceE(actual)
	if err != nil {
		return err
	}
	if reflect.TypeOf(expected[0]).Kind() != reflect.String {
		return fmt.Errorf("expected '%v' to be a string", expected[0])
	}
	expectedKey := cast.ToString(expected[0])
	for i := range actualSlice {
		if reflect.TypeOf(actualSlice[i]).Kind() != reflect.Map && reflect.TypeOf(actualSlice[i]).Kind() != reflect.Struct {
			return fmt.Errorf("expected '%v' to be a map or a struct", actualSlice[i])
		}
		elem := cast.ToStringMap(actualSlice[i])
		if err != nil {
			return err
		}
		if _, ok := elem[expectedKey]; ok {
			if ShouldJSONEqual(elem[expectedKey], expected[1]) != nil {
				return fmt.Errorf("expected '%v' contain a key '%s' with value %v but it wasnt", actual, expectedKey, expected[1])
			}
		}
	}
	return nil
}

// ShouldNotJSONContainWithKey receives exactly three parameters. The first is a slice, the
// second is a key in the inner slice structure and the third is a proposed value associated to the key.
// Equality is determined using ShouldJSONEqual.
func ShouldNotJSONContainWithKey(actual interface{}, expected ...interface{}) error {
	if err := need(2, expected); err != nil {
		return err
	}
	actualSlice, err := cast.ToSliceE(actual)
	if err != nil {
		return err
	}
	if reflect.TypeOf(expected[0]).Kind() != reflect.String {
		return fmt.Errorf("expected '%v' to be a string", expected[0])
	}
	expectedKey := cast.ToString(expected[0])
	for i := range actualSlice {
		if reflect.TypeOf(actualSlice[i]).Kind() != reflect.Map && reflect.TypeOf(actualSlice[i]).Kind() != reflect.Struct {
			return fmt.Errorf("expected '%v' to be a map or a struct currently %v", actualSlice[i], reflect.TypeOf(actualSlice[i]).Kind())
		}
		elem := cast.ToStringMap(actualSlice[i])
		if err != nil {
			return err
		}
		if _, ok := elem[expectedKey]; ok {
			if ShouldJSONEqual(elem[expectedKey], expected[1]) == nil {
				return fmt.Errorf("expected '%v' not contain a key '%s' with value %v but it was", actual, expectedKey, expected[1])
			}
		}
	}
	return nil
}

// ShouldJSONEqual receives exactly 2 JSON arguments and does a JSON equality check.
// The latest JSON spec doesn't only allow objects and arrays, but primitive values are valid JSON as well.
// For object equality keys can be in different order, and whitespace (except in keys or values) is ignored.
// For arrays the order is important, but whitespace (except in values) is ignored.
// String, number, true/false are compared as-is.
// `null` JSON values are currently passed as empty string, and are compared against the "null" string.
//
// For an example scenario see `tests/assertions/ShouldJSONEqual.yml`.
func ShouldJSONEqual(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}

	switch actual.(type) {
	case map[string]interface{}:
		return jsonEqualMap(actual, expected[0])
	case []interface{}:
		return jsonEqualSlice(actual, expected[0])
	case string:
		return jsonEqualString(actual, expected[0])
	case json.Number:
		return jsonEqualNumber(actual, expected[0])
	case bool:
		return jsonEqualBool(actual, expected[0])
	default:
		return fmt.Errorf("unexpected type for actual: %T", actual)
	}
}

func jsonEqualMap(actual, expected interface{}) error {
	actualMap, err := cast.ToStringMapE(actual)
	if err != nil {
		return err
	}
	expectedString, err := cast.ToStringE(expected)
	if err != nil {
		return err
	}

	// Marshal and unmarshal for later deepequal to work
	actualBytes, err := json.Marshal(actualMap)
	if err != nil {
		return err
	}
	err = json.Unmarshal(actualBytes, &actualMap)
	if err != nil {
		return err
	}

	expectedMap := map[string]interface{}{}
	err = json.Unmarshal([]byte(expectedString), &expectedMap)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(actualMap, expectedMap) {
		return nil
	}
	return fmt.Errorf("expected '%v' to be JSON equals to '%v' ", actualMap, expectedMap)
}

func jsonEqualSlice(actual, expected interface{}) error {
	actualSlice, err := cast.ToSliceE(actual)
	if err != nil {
		return err
	}
	expectedString, err := cast.ToStringE(expected)
	if err != nil {
		return err
	}

	// Marshal and unmarshal for later deepequal to work
	actualBytes, err := json.Marshal(actualSlice)
	if err != nil {
		return err
	}
	err = json.Unmarshal(actualBytes, &actualSlice)
	if err != nil {
		return err
	}

	expectedSlice := []interface{}{}
	err = json.Unmarshal([]byte(expectedString), &expectedSlice)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(actualSlice, expectedSlice) {
		return nil
	}
	return fmt.Errorf("expected '%v' to be JSON equals to '%v' ", actualSlice, expectedSlice)
}

func jsonEqualString(actual, expected interface{}) error {
	actualString, err := cast.ToStringE(actual)
	if err != nil {
		return err
	}
	expectedString, err := cast.ToStringE(expected)
	if err != nil {
		return err
	}

	if actualString == expectedString {
		return nil
	}
	// Special case: Venom passes an empty string when `actual` JSON is JSON's `null`.
	// Above check is already valid when `expected` is an empty string, but
	// the user might have passed `null` explicitly.
	// TODO: This should be changed as soon as Venom passes Go's `nil` for JSON `null` values.
	if actualString == "" && expectedString == "null" {
		return nil
	}
	return fmt.Errorf("expected '%v' to be JSON equals to '%v' ", actualString, expectedString)
}

func jsonEqualNumber(actual, expected interface{}) error {
	actualFloat, err := cast.ToFloat64E(actual)
	if err != nil {
		return err
	}
	expectedFloat, err := cast.ToFloat64E(expected)
	if err != nil {
		return err
	}

	if actualFloat == expectedFloat {
		return nil
	}
	return fmt.Errorf("expected '%v' to be JSON equals to '%v' ", actualFloat, expectedFloat)
}

func jsonEqualBool(actual, expected interface{}) error {
	actualBool, err := cast.ToBoolE(actual)
	if err != nil {
		return err
	}
	expectedBool, err := cast.ToBoolE(expected)
	if err != nil {
		return err
	}

	if actualBool == expectedBool {
		return nil
	}
	return fmt.Errorf("expected '%v' to be JSON equals to '%v' ", actualBool, expectedBool)
}

// ShouldNotJSONEqual asserts that the actual value is not JSON equal to the expected value
func ShouldNotJSONEqual(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}

	if err := ShouldJSONEqual(actual, expected...); err == nil {
		return fmt.Errorf("expected %v to not be JSON equals to %v", actual, expected[0])
	}
	return nil
}
