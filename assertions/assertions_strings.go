package assertions

import (
	"fmt"
	"strings"

	"github.com/spf13/cast"
)

// ShouldStartWith receives exactly 2 string parameters and ensures that the first starts with the second.
func ShouldStartWith(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}

	s, err := cast.ToStringE(actual)
	if err != nil {
		return err
	}

	prefix, err := cast.ToStringE(expected[0])
	if err != nil {
		return err
	}

	if strings.HasPrefix(s, prefix) {
		return nil
	}

	return fmt.Errorf("expected '%v' have prefix %q but it wasn't", s, prefix)
}

// ShouldNotStartWith receives exactly 2 string parameters and ensures that the first does not start with the second.
func ShouldNotStartWith(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}

	s, err := cast.ToStringE(actual)
	if err != nil {
		return err
	}

	prefix, err := cast.ToStringE(expected[0])
	if err != nil {
		return err
	}

	if strings.HasPrefix(s, prefix) {
		return fmt.Errorf("expected '%v' not have prefix %q but it was", s, prefix)
	}

	return nil
}

// ShouldEndWith receives exactly 2 string parameters and ensures that the first ends with the second.
func ShouldEndWith(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}

	s, err := cast.ToStringE(actual)
	if err != nil {
		return err
	}

	suffix, err := cast.ToStringE(expected[0])
	if err != nil {
		return err
	}

	if strings.HasSuffix(s, suffix) {
		return nil
	}

	return fmt.Errorf("expected '%v' have suffix %q but it wasn't", s, suffix)
}

// ShouldNotEndWith receives exactly 2 string parameters and ensures that the first does not end with the second.
func ShouldNotEndWith(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}

	s, err := cast.ToStringE(actual)
	if err != nil {
		return err
	}

	suffix, err := cast.ToStringE(expected[0])
	if err != nil {
		return err
	}

	if strings.HasSuffix(s, suffix) {
		return fmt.Errorf("expected '%v' not have suffix %q but it was", s, suffix)
	}

	return nil
}

// ShouldBeBlank receives exactly 1 string parameter and ensures that it is equal to "".
func ShouldBeBlank(actual interface{}, expected ...interface{}) error {
	if err := need(0, expected); err != nil {
		return err
	}

	s, err := cast.ToStringE(actual)
	if err != nil {
		return err
	}

	if s == "" {
		return nil
	}

	return fmt.Errorf("expected '%v' to be blank but it wasn't", s)
}

// ShouldNotBeBlank receives exactly 1 string parameter and ensures that it is equal to "".
func ShouldNotBeBlank(actual interface{}, expected ...interface{}) error {
	if err := need(0, expected); err != nil {
		return err
	}

	s, err := cast.ToStringE(actual)
	if err != nil {
		return err
	}

	if s == "" {
		return fmt.Errorf("expected value to not be blank but it was")
	}

	return nil
}

// ShouldContainSubstring receives exactly 2 string parameters and ensures that the first contains the second as a substring.
func ShouldContainSubstring(actual interface{}, expected ...interface{}) error {
	var arg string
	for _, e := range expected {
		arg += fmt.Sprintf("%v ", e)
	}

	s, err := cast.ToStringE(actual)
	if err != nil {
		return err
	}

	ss := strings.TrimSpace(arg)

	if strings.Contains(s, ss) {
		return nil
	}

	return fmt.Errorf("expected '%v' to contain '%v' but it wasn't", s, ss)
}

// ShouldNotContainSubstring receives exactly 2 string parameters and ensures that the first does NOT contain the second as a substring.
func ShouldNotContainSubstring(actual interface{}, expected ...interface{}) error {
	var arg string
	for _, e := range expected {
		arg += fmt.Sprintf("%v ", e)
	}

	s, err := cast.ToStringE(actual)
	if err != nil {
		return err
	}

	ss := strings.TrimSpace(arg)

	if strings.Contains(s, ss) {
		return fmt.Errorf("expected '%v' to not contain '%v' but it was", s, ss)
	}

	return nil
}
