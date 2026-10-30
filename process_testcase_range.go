package venom

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ovh/venom/internal/yamljson"
	"github.com/ovh/venom/interpolate"
)

// Parse and format range data to allow iterations over user data
func parseRanged(ctx context.Context, rawStep []byte, stepVars func() H) (Range, error) {
	// Load "range" attribute and perform actions depending on its typing
	var ranged Range
	if err := json.Unmarshal(rawStep, &ranged); err != nil {
		return ranged, fmt.Errorf("unable to parse range expression: %v", err)
	}

	switch v := ranged.RawContent.(type) {

	// Nil means this is not a ranged data, append an empty item to force at least one iteration and exit
	case nil:
		ranged.Items = append(ranged.Items, RangeData{})
		return ranged, nil

	// String needs to be parsed and possibly templated
	case string:
		if err := resolveRangeString(ctx, rawStep, stepVars, v, &ranged); err != nil {
			return ranged, err
		}
	}

	return rangeItems(ctx, ranged)
}

// resolveRangeString parses a string range as JSON data, templating the step with its vars when it is not JSON yet.
func resolveRangeString(ctx context.Context, rawStep []byte, stepVars func() H, rawString string, ranged *Range) error {
	Debug(ctx, "attempting to parse range expression")
	if len(rawString) == 0 {
		return fmt.Errorf("range expression has been specified without any data")
	}

	// Try parsing already templated data
	err := json.Unmarshal([]byte("{\"range\":"+rawString+"}"), ranged)
	// ... or fallback
	if err != nil {
		return templateRange(ctx, rawStep, stepVars, ranged)
	}
	return nil
}

// templateRange interpolates the step with its vars and parses its range again; a failure before the last parse only warns.
func templateRange(ctx context.Context, rawStep []byte, stepVars func() H, ranged *Range) error {
	// Try templating and escaping data
	Debug(ctx, "attempting to template range expression and parse it again")
	vars, err := DumpStringPreserveCase(stepVars())
	if err != nil {
		Warn(ctx, "failed to parse range expression when loading step variables: %v", err)
		return nil
	}
	for i := range vars {
		vars[i] = strings.ReplaceAll(vars[i], "\"", "\\\"")
	}
	content, err := interpolate.Do(string(rawStep), vars)
	if err != nil {
		Warn(ctx, "failed to parse range expression when templating variables: %v", err)
		return nil
	}

	// Try parsing data
	err = json.Unmarshal([]byte(content), ranged)
	if err != nil {
		Warn(ctx, "failed to parse range expression when parsing data into raw string: %v", err)
		return nil
	}
	if rawString, ok := ranged.RawContent.(string); ok {
		err := json.Unmarshal([]byte("{\"range\":"+rawString+"}"), ranged)
		if err != nil {
			Warn(ctx, "failed to parse range expression when parsing raw string into data: %v", err)
			return fmt.Errorf("unable to parse range expression: unable to transform string data into a supported range expression type")
		}
	}
	return nil
}

// rangeItems turns the parsed range data into the items to iterate over.
func rangeItems(ctx context.Context, ranged Range) (Range, error) {
	// Format data
	switch t := ranged.RawContent.(type) {

	// Array-like data
	case []interface{}:
		Debug(ctx, "\"range\" data is array-like")
		for index, value := range t {
			key := strconv.Itoa(index)
			ranged.Items = append(ranged.Items, RangeData{key, value})
		}

	// Number data
	case float64:
		Debug(ctx, "\"range\" data is number-like")
		upperBound := int(t)
		for i := 0; i < upperBound; i++ {
			key := strconv.Itoa(i)
			ranged.Items = append(ranged.Items, RangeData{key, i})
		}

	// Map-like data
	case map[string]interface{}:
		Debug(ctx, "\"range\" data is map-like")
		for key, value := range t {
			ranged.Items = append(ranged.Items, RangeData{key, value})
		}

	// Unsupported data format
	default:
		return ranged, fmt.Errorf("\"range\" was provided an unsupported type %T", t)
	}

	ranged.Enabled = true
	ranged.RawContent = nil
	return ranged, nil
}

func processVariableAssignments(ctx context.Context, tcName string, tcVars H, rawStep json.RawMessage) (H, bool, error) {
	return assignVariables(ctx, tcName, func(name string) (interface{}, bool) {
		v, ok := tcVars[name]
		return v, ok
	}, rawStep)
}

// assignVariables is processVariableAssignments over a lookup of the variables, so that a test case need not build the
// map of them all for a step that assigns nothing.
func assignVariables(ctx context.Context, tcName string, lookup func(string) (interface{}, bool), rawStep json.RawMessage) (H, bool, error) {
	var stepAssignment AssignStep
	result := make(H)
	if err := yamljson.Unmarshal(rawStep, &stepAssignment); err != nil {
		Error(ctx, "unable to parse assignments (%s): %v", string(rawStep), err)
		return nil, false, err
	}

	if len(stepAssignment.Assignments) == 0 {
		return nil, false, nil
	}

	for varname, assignment := range stepAssignment.Assignments {
		Debug(ctx, "Processing %s assignment", varname)
		varValue, err := assignmentSource(ctx, tcName, lookup, assignment)
		if err != nil {
			return nil, true, err
		}
		value, err := assignedValue(ctx, varname, assignment, varValue)
		if err != nil {
			return nil, true, err
		}
		result.Add(varname, value)
	}
	return result, true, nil
}

// assignmentSource returns the value an assignment reads: the variable it names, as the test case holds it or prefixed
// with its name, else the default of the assignment.
func assignmentSource(ctx context.Context, tcName string, lookup func(string) (interface{}, bool), assignment Assignment) (interface{}, error) {
	if v, has := lookup(assignment.From); has {
		return v, nil
	}
	if v, has := lookup(tcName + "." + assignment.From); has {
		return v, nil
	}
	if assignment.Default == nil {
		err := fmt.Errorf("%s reference not found", assignment.From)
		Error(ctx, "%v", err)
		return nil, err
	}
	return assignment.Default, nil
}

// assignedValue returns the value assigned to varname: the one read, or the last submatch of the assignment's regex in
// it, empty when the value is not a string or the regex finds nothing.
func assignedValue(ctx context.Context, varname string, assignment Assignment, varValue interface{}) (interface{}, error) {
	if assignment.Regex == "" {
		Info(ctx, "Assign '%s' value '%s'", varname, varValue)
		return varValue, nil
	}
	regex, err := regexp.Compile(assignment.Regex)
	if err != nil {
		Warn(ctx, "unable to compile regexp %q", assignment.Regex)
		return nil, err
	}
	varValueS, ok := varValue.(string)
	if !ok {
		Warn(ctx, "%q is not a string value", varname)
		return "", nil
	}
	submatches := regex.FindStringSubmatch(varValueS)
	if len(submatches) == 0 {
		Warn(ctx, "%s: %q doesn't match anything in %q", varname, regex, varValue)
		return "", nil
	}
	Info(ctx, "Assign %q from regexp %q, values %q", varname, regex, submatches)
	return submatches[len(submatches)-1], nil
}

func escapeQuotes(str string) string {
	if strings.Contains(str, `"`) {
		// Use strconv.Quote which handles escaping correctly
		x := strconv.Quote(str)
		x = strings.TrimPrefix(x, `"`)
		x = strings.TrimSuffix(x, `"`)
		return x
	}
	return str
}

// escapeForJSON is strconv.Quote of s without its surrounding double quotes. A string of printable ASCII, which is what
// nearly every variable is, takes a single pass; anything else goes to strconv.Quote (#2465).
func escapeForJSON(s string) string {
	extra := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			extra++
		case c < 0x20 || c >= 0x7f:
			return strings.TrimSuffix(strings.TrimPrefix(strconv.Quote(s), `"`), `"`)
		}
	}
	var b strings.Builder
	b.Grow(len(s) + extra)
	for i := 0; i < len(s); i++ {
		if c := s[i]; c == '"' || c == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
