package venom

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
)

// decodeStepFields turns, in the step of a custom executor, a string that holds a JSON object or array into the decoded
// value when the field of the target executor that the key maps to is a map, a slice, an array or a struct (#3304).
//
// quoteTemplateExpressions makes `headers: {{ .headers }}` a string whatever the variable is; the executor decodes its
// step with mapstructure and a string does not go into a map field. The field decides here, because a string field
// (`body: {{ .payload }}`) must keep the text. Only the top-level keys are read; a value that is not JSON, a key that
// maps to no field and a string, interface or other scalar field are left as they are. The text is already rendered: it
// is decoded, never written or interpolated again, so redaction and the template pass are not involved.
func decodeStepFields(e Executor, step TestStep) {
	if _, isUser := e.(UserExecutor); isUser {
		return
	}
	fields := executorFields(reflect.TypeOf(e))
	if len(fields) == 0 {
		return
	}
	for key, value := range step {
		text, ok := value.(string)
		if !ok {
			continue
		}
		if t, found := fieldFor(fields, key); found && holdsStructure(t) {
			if decoded, isJSON := jsonContainer(text); isJSON {
				step[key] = decoded
			}
		}
	}
}

// stepField is a field of an executor as mapstructure names it.
type stepField struct {
	name string
	typ  reflect.Type
}

// executorFields lists the exported fields of the struct t points to or is, the embedded structs tagged `squash`
// included. It returns nil for any other type.
func executorFields(t reflect.Type) []stepField {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	var fields []stepField
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		name, squash := mapstructureName(sf)
		switch {
		case name == "-":
		case squash && sf.Type.Kind() == reflect.Struct:
			fields = append(fields, executorFields(sf.Type)...)
		case sf.IsExported():
			fields = append(fields, stepField{name: name, typ: sf.Type})
		}
	}
	return fields
}

// mapstructureName returns the key mapstructure reads a field from, the tag's name or the field's, and whether the tag
// asks to squash an embedded struct.
func mapstructureName(sf reflect.StructField) (string, bool) {
	parts := strings.Split(sf.Tag.Get("mapstructure"), ",")
	name := parts[0]
	if name == "" {
		name = sf.Name
	}
	squash := false
	for _, opt := range parts[1:] {
		squash = squash || opt == "squash"
	}
	return name, squash
}

// fieldFor finds the field a step key maps to: the exact name first, then the first that matches without case, as
// mapstructure does.
func fieldFor(fields []stepField, key string) (reflect.Type, bool) {
	for _, f := range fields {
		if f.name == key {
			return f.typ, true
		}
	}
	for _, f := range fields {
		if strings.EqualFold(f.name, key) {
			return f.typ, true
		}
	}
	return nil, false
}

// holdsStructure reports whether a field takes a decoded JSON object or array: a map, a slice, an array or a struct, or a
// pointer to one. A byte slice is left out, mapstructure fills it from a string.
func holdsStructure(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Map, reflect.Array, reflect.Struct:
		return true
	case reflect.Slice:
		return t.Elem().Kind() != reflect.Uint8
	default:
		return false
	}
}

// jsonContainer decodes the whole of text when it is a JSON object or array. Numbers stay json.Number, so a large
// integer keeps its digits.
func jsonContainer(text string) (interface{}, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(trimmed)))
	dec.UseNumber()
	var out interface{}
	if err := dec.Decode(&out); err != nil {
		return nil, false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, false
	}
	return out, true
}
