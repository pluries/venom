package assertions

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/tj/go-naturaldate"
)

// ShouldHappenBefore receives exactly 2 time.Time arguments and asserts that the first happens before the second.
// The arguments have to respect the date format RFC3339 (ie. 2006-01-02T15:04:00+07:00) or humanize format (ie. now, tomorrow, yesterday, 5 minutes ago)
//
// Example of testsuite file:
//
//	name: test ShouldHappenBefore
//	vars:
//	  time: 2006-01-02T15:04:05+07:00
//	  time_with_5s_after: 2006-01-02T15:04:10+07:00
//	testcases:
//	- name: test assertion
//	  steps:
//	  - type: exec
//	    script: "echo {{.time}}"
//	    assertions:
//	      - result.systemout ShouldHappenBefore "{{.time_with_5s_after}}"
//	 - name: test assertion with humanize format
//	   steps:
//	   - type: exec
//	     script: "echo {{.venom.datetime}}"
//	     assertions:
//	       - "result.systemout ShouldHappenBefore tomorrow"
//	       - "result.systemout ShouldHappenBefore '5 minutes from now'"
func ShouldHappenBefore(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}

	actualTime, err := getTimeFromString(actual)
	if err != nil {
		return err
	}
	expectedTime, err := getTimeFromString(expected[0])
	if err != nil {
		return err
	}

	if actualTime.Before(expectedTime) {
		return nil
	}

	return fmt.Errorf("expected '%v' to be before '%v'", actualTime, expectedTime)
}

// ShouldHappenOnOrBefore receives exactly 2 time.Time arguments and asserts that the first happens on or before the second.
// The arguments have to respect the date format RFC3339 (ie. 2006-01-02T15:04:00+07:00) or humanize format (ie. now, tomorrow, yesterday, 5 minutes ago)
//
// Example of testsuite file:
//
//	name: test ShouldHappenOnOrBefore
//	vars:
//	  time: 2006-01-02T15:04:05+07:00
//	  time_with_5s_after: 2006-01-02T15:04:10+07:00
//	testcases:
//	- name: test assertion
//	  steps:
//	  - type: exec
//	    script: "echo {{.time}}"
//	    assertions:
//	      - result.systemout ShouldHappenOnOrBefore "{{.time_with_5s_after}}"
//	 - name: test assertion with humanize format
//	   steps:
//	   - type: exec
//	     script: "echo {{.venom.datetime}}"
//	     assertions:
//	       - "result.systemout ShouldHappenOnOrBefore tomorrow"
//	       - "result.systemout ShouldHappenOnOrBefore '5 minutes from now'"
func ShouldHappenOnOrBefore(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}
	actualTime, err := getTimeFromString(actual)
	if err != nil {
		return err
	}
	expectedTime, err := getTimeFromString(expected[0])
	if err != nil {
		return err
	}

	if actualTime.Before(expectedTime) || actualTime.Equal(expectedTime) {
		return nil
	}

	return fmt.Errorf("expected '%v' to be before on on '%v'", actualTime, expectedTime)
}

// ShouldHappenAfter receives exactly 2 time.Time arguments and asserts that the first happens after the second.
// The arguments have to respect the date format RFC3339 (ie. 2006-01-02T15:04:00+07:00) or humanize format (ie. now, tomorrow, yesterday, 5 minutes ago)
//
// Example of testsuite file:
//
//	name: test ShouldHappenAfter
//	vars:
//	  time_with_5s_before: 2006-01-02T15:04:00+07:00
//	  time: 2006-01-02T15:04:05+07:00
//	testcases:
//	- name: test assertion
//	  steps:
//	  - type: exec
//	    script: "echo {{.time}}"
//	    assertions:
//	      - result.systemout ShouldHappenAfter "{{.time_with_5s_before}}"
//	 - name: test assertion with humanize format
//	   steps:
//	   - type: exec
//	     script: "echo {{.venom.datetime}}"
//	     assertions:
//	       - "result.systemout ShouldHappenAfter yesterday"
//	       - "result.systemout ShouldHappenAfter '5 minutes ago'"
func ShouldHappenAfter(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}
	actualTime, err := getTimeFromString(actual)
	if err != nil {
		return err
	}
	expectedTime, err := getTimeFromString(expected[0])
	if err != nil {
		return err
	}

	if actualTime.After(expectedTime) {
		return nil
	}

	return fmt.Errorf("expected '%v' to be after '%v'", actualTime, expectedTime)
}

// ShouldHappenOnOrAfter receives exactly 2 time.Time arguments and asserts that the first happens on or after the second.
// The arguments have to respect the date format RFC3339 (ie. 2006-01-02T15:04:00+07:00) or humanize format (ie. now, tomorrow, yesterday, 5 minutes ago)
//
// Example of testsuite file:
//
//	name: test ShouldHappenOnOrAfter
//	vars:
//	  time_with_5s_before: 2006-01-02T15:04:00+07:00
//	  time: 2006-01-02T15:04:05+07:00
//	testcases:
//	- name: test assertion
//	  steps:
//	  - type: exec
//	    script: "echo {{.time}}"
//	    assertions:
//	      - result.systemout ShouldHappenOnOrAfter "{{.time_with_5s_before}}"
//	 - name: test assertion with humanize format
//	   steps:
//	   - type: exec
//	     script: "echo {{.venom.datetime}}"
//	     assertions:
//	       - "result.systemout ShouldHappenOnOrAfter yesterday"
//	       - "result.systemout ShouldHappenOnOrAfter '5 minutes ago'"
func ShouldHappenOnOrAfter(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}

	actualTime, err := getTimeFromString(actual)
	if err != nil {
		return err
	}
	expectedTime, err := getTimeFromString(expected[0])
	if err != nil {
		return err
	}

	if actualTime.After(expectedTime) || actualTime.Equal(expectedTime) {
		return nil
	}
	return fmt.Errorf("expected '%v' to be before or on '%v'", actualTime, expectedTime)
}

// ShouldHappenBetween receives exactly 3 time.Time arguments and asserts that the first happens between (not on) the second and third.
// The arguments have to respect the date format RFC3339 (ie. 2006-01-02T15:04:00+07:00) or humanize format (ie. yesterday, 5 minutes ago)
//
// Example of testsuite file:
//
//	name: test ShouldHappenBetween
//	vars:
//	  time_with_5s_before: 2006-01-02T15:04:00+07:00
//	  time: 2006-01-02T15:04:05+07:00
//	  time_with_5s_after: 2006-01-02T15:04:10+07:00
//	testcases:
//	- name: test assertion
//	  steps:
//	  - type: exec
//	    script: "echo {{.time}}"
//	    assertions:
//	      - result.systemout ShouldHappenBetween "{{.time_with_5s_before}}" "{{.time_with_5s_after}}"
//	- name: test assertion with humanize format
//	   steps:
//	   - type: exec
//	     script: "echo {{.venom.datetime}}"
//	     assertions:
//	       - "result.systemout ShouldHappenBetween yesterday tomorrow"
//	       - "result.systemout ShouldHappenBetween '5 minutes ago' '5 minutes from now'"
func ShouldHappenBetween(actual interface{}, expected ...interface{}) error {
	if err := need(2, expected); err != nil {
		return err
	}

	actualTime, err := getTimeFromString(actual)
	if err != nil {
		return err
	}
	minTime, err := getTimeFromString(expected[0])
	if err != nil {
		return err
	}
	maxTime, err := getTimeFromString(expected[1])
	if err != nil {
		return err
	}

	if actualTime.After(minTime) && actualTime.Before(maxTime) {
		return nil
	}
	return fmt.Errorf("expected '%v' to be between '%v' and '%v' ", actualTime, minTime, maxTime)
}

// ShouldTimeEqual receives exactly 2 time.Time arguments and does a time equality check.
// The arguments have to respect the date format RFC3339, as 2006-01-02T15:04:00+07:00
//
// Example of testsuite file:
//
//	name: test ShouldTimeEqual
//	vars:
//	  time_expected: 2006-01-02T13:04:00Z
//	  time: 2006-01-02T15:04:00+02:00
//	testcases:
//	- name: test assertion
//	  steps:
//	  - type: exec
//	    script: "echo {{.time}}"
//	    assertions:
//	      - result.systemout ShouldTimeEqual "{{.time_expected}}"
func ShouldTimeEqual(actual interface{}, expected ...interface{}) error {
	if err := need(1, expected); err != nil {
		return err
	}

	actualTime, err := getTimeFromString(actual)
	if err != nil {
		return err
	}
	expectedTime, err := getTimeFromString(expected[0])
	if err != nil {
		return err
	}
	if actualTime.Equal(expectedTime) {
		return nil
	}
	return fmt.Errorf("expected '%v' to be time equals to '%v' ", actualTime, expectedTime)
}

func getTimeFromString(in interface{}) (time.Time, error) {
	return getTimeFromStringAt(in, time.Now())
}

// relativeSecondsPattern matches the second-based relative phrases the humanize parser does not understand:
// "1 second ago", "30 seconds from now", "in 5 seconds", "90 secs ago". go-naturaldate has no notion of seconds and
// reads the number as an hour of the day ("1 second ago" became 01:00 today), so these are resolved here first.
var relativeSecondsPattern = regexp.MustCompile(`(?i)^\s*(in\s+)?(\d+)\s*(?:s|sec|secs|second|seconds)(?:\s+(ago|from now))?\s*$`)

// parseRelativeSeconds resolves a second-based relative phrase against now. The second value reports whether the
// phrase was one; anything else is left to the callers' other parsers.
func parseRelativeSeconds(s string, now time.Time) (time.Time, bool) {
	m := relativeSecondsPattern.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return time.Time{}, false
	}
	d := time.Duration(n) * time.Second
	if strings.EqualFold(m[3], "ago") {
		return now.Add(-d), true
	}
	return now.Add(d), true
}

// getTimeFromStringAt is getTimeFromString with an explicit reference time, so the relative phrases are testable at
// any clock: a time.Time is returned as is, then RFC3339, then second-based relative phrases, then the humanize parser.
func getTimeFromStringAt(in interface{}, now time.Time) (time.Time, error) {
	if t, isTime := in.(time.Time); isTime {
		return t, nil
	}

	s, err := cast.ToStringE(in)
	if err != nil {
		return time.Time{}, errors.Errorf("invalid date provided: %q", in)
	}

	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}

	if t, ok := parseRelativeSeconds(s, now); ok {
		return t, nil
	}

	t, innerErr := naturaldate.Parse(s, now)
	if innerErr != nil {
		return time.Time{}, errors.Errorf("invalid date provided with %q not in RFC3339 format or humanize format", in)
	}

	return t, nil
}
