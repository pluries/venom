package venom

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/ovh/venom/interpolate"
)

// interpolateSuiteVar interpolates the value of a suite variable against the other suite variables. The value of a
// variable that holds a whole map or list (its dump has a "<name>.__Type__" entry) is JSON text: the variables it
// references are inserted into its string values one by one, so that a referenced value with a quote or a comma cannot
// add a key or an element (#3376). Any other value, and a text that is not valid JSON, is interpolated as a whole.
func interpolateSuiteVar(name, value string, vars map[string]string) (string, error) {
	// Without an expression there is nothing to insert: the text is returned as it is, without decoding it.
	if _, ok := vars[name+".__Type__"]; !ok || !strings.Contains(value, "{{") {
		return interpolate.Do(value, vars)
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(value)))
	dec.UseNumber()
	var doc interface{}
	if err := dec.Decode(&doc); err != nil {
		return interpolate.Do(value, vars)
	}
	if _, err := dec.Token(); err != io.EOF {
		return interpolate.Do(value, vars)
	}
	doc, err := interpolateJSONStrings(doc, vars)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(doc)
	return string(out), err
}

// interpolateJSONStrings interpolates every string value of a decoded JSON document. Keys are left as they are.
func interpolateJSONStrings(doc interface{}, vars map[string]string) (interface{}, error) {
	switch d := doc.(type) {
	case string:
		return interpolate.Do(d, vars)
	case map[string]interface{}:
		for k, item := range d {
			out, err := interpolateJSONStrings(item, vars)
			if err != nil {
				return nil, err
			}
			d[k] = out
		}
	case []interface{}:
		for i, item := range d {
			out, err := interpolateJSONStrings(item, vars)
			if err != nil {
				return nil, err
			}
			d[i] = out
		}
	}
	return doc, nil
}
