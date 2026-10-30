package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

// The value an execute script returns (FR-07). Research ids here are docs/feat/2048/research.md's.

// errSelfReference reports a returned value that contains itself: venom could not flatten it into the step result.
var errSelfReference = errors.New("the script's return value contains itself")

// errUndecoded reports a nil pointer where playwright-go dropped a decoding error: a URL the page accepts and Go's
// url.Parse refuses is a nil *url.URL, whose String method would panic the run.
var errUndecoded = errors.New("the script's return value holds a value playwright-go could not decode")

// scriptValue is what a script returned, as result.value shows it, text, and as result.valuejson shows it, typed,
// after venom's body and bodyjson (R6): a string as is in both, any other value as its JSON text and as jsonValue
// converts it. undefined and null, nil once decoded, leave both empty, so a script that returns nothing keeps the
// result it had.
func scriptValue(v any) (string, any, error) {
	typed, err := jsonValue(v)
	if err != nil || typed == nil {
		return "", nil, err
	}
	if s, ok := typed.(string); ok {
		return s, s, nil
	}
	// The text of JavaScript's JSON.stringify, whose <, > and & are not escaped as json.Marshal escapes them; the
	// keys of an object are sorted.
	var text strings.Builder
	enc := json.NewEncoder(&text)
	enc.SetEscapeHTML(false)
	err = enc.Encode(typed)
	return strings.TrimSuffix(text.String(), "\n"), typed, err
}

// jsonValue converts a value playwright-go decoded from the page into the types encoding/json decodes a document into
// (nil, bool, float64, string, []any, map[string]any), those of http's bodyjson (R6, R7). venom flattens a step result
// by reflection and panics when it cannot, and json.Marshal refuses NaN, the infinities and a cycle: a number is a
// float64, NaN and the infinities JavaScript's own text; a Date its RFC 3339 text in UTC; a returned Error its message;
// a BigInt, a URL, a RegExp and any other value their text; lists, typed arrays and objects element by element. A map
// or a list found again inside itself is errSelfReference; one referenced twice from two places is no cycle. A nil
// pointer, where playwright-go dropped a decoding error, is errUndecoded.
func jsonValue(v any) (any, error) {
	return pathConverter{path: map[reference]bool{}}.convert(v)
}

// reference identifies a map, or a slice by its first element and its length, on the path being converted.
type reference struct {
	ptr uintptr
	len int
}

// pathConverter converts a value, with the maps and slices it is inside of on path.
type pathConverter struct {
	path map[reference]bool
}

// convert is jsonValue at one level. A nil pointer is errUndecoded: its String or Error method would dereference it.
// time.Time and error come before fmt.Stringer, which time.Time implements.
func (c pathConverter) convert(v any) (any, error) {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil, fmt.Errorf("%w: a nil %T", errUndecoded, v)
	}
	switch x := v.(type) {
	case nil, bool, string:
		return x, nil
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano), nil
	case error:
		return x.Error(), nil
	case fmt.Stringer:
		return x.String(), nil
	case map[string]any:
		return c.object(x)
	}
	return c.byKind(v)
}

// byKind converts what convert does not name: every integer and float type (an integral number is an int, a typed
// array's elements its own type), the slices and arrays, and anything else as its text.
func (c pathConverter) byKind(v any) (any, error) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return float64(rv.Uint()), nil
	case reflect.Float32, reflect.Float64:
		return number(rv.Float()), nil
	case reflect.Slice, reflect.Array:
		return c.list(rv)
	}
	return fmt.Sprint(v), nil
}

// number is f, or JavaScript's own text for the values JSON has no number for.
func number(f float64) any {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	return f
}

// object converts a map entry by entry.
func (c pathConverter) object(m map[string]any) (any, error) {
	return c.within(reference{ptr: reflect.ValueOf(m).Pointer()}, func() (any, error) {
		out := make(map[string]any, len(m))
		for k, e := range m {
			v, err := c.convert(e)
			if err != nil {
				return nil, err
			}
			out[k] = v
		}
		return out, nil
	})
}

// list converts a slice or an array element by element into a []any: json.Marshal would write a []uint8 as base64.
// An empty slice cannot contain itself, and an array is a copy: only a slice with elements goes on the path.
func (c pathConverter) list(rv reflect.Value) (any, error) {
	elements := func() (any, error) {
		out := make([]any, rv.Len())
		for i := range out {
			v, err := c.convert(rv.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	}
	if rv.Kind() != reflect.Slice || rv.Len() == 0 {
		return elements()
	}
	return c.within(reference{ptr: rv.Pointer(), len: rv.Len()}, elements)
}

// within runs convert with r on the path, or fails when r is on it already: the value contains itself.
func (c pathConverter) within(r reference, convert func() (any, error)) (any, error) {
	if c.path[r] {
		return nil, errSelfReference
	}
	c.path[r] = true
	defer delete(c.path, r)
	return convert()
}
