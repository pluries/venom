package interpolate

// This functions come from https://github.com/Masterminds/sprig
// Copyright (C) 2013 Masterminds
// Masterminds/sprig is licensed under the MIT License

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	util "github.com/aokoli/goutils"
	"github.com/huandu/xstrings"
	"github.com/spf13/cast"
)

// InterpolateHelperFuncs is a list of funcs that can be used in go templates.
var InterpolateHelperFuncs template.FuncMap

func init() {
	InterpolateHelperFuncs = wrapHelpers(template.FuncMap{
		"abbrev":     abbrev,
		"abbrevboth": abbrevboth,
		"trunc":      trunc,
		"trim":       strings.TrimSpace,
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"title":      title,
		"untitle":    untitle,
		"substr":     substring,
		// Switch order so that "foo" | repeat 5
		"repeat": func(count int, str string) string { return strings.Repeat(str, count) },
		// Deprecated: Use trimAll.
		"trimall": func(a, b string) string { return strings.Trim(b, a) },
		// Switch order so that "$foo" | trimall "$"
		"trimAll":      func(a, b string) string { return strings.Trim(b, a) },
		"trimSuffix":   func(a, b string) string { return strings.TrimSuffix(b, a) },
		"trimPrefix":   func(a, b string) string { return strings.TrimPrefix(b, a) },
		"nospace":      util.DeleteWhiteSpace,
		"initials":     initials,
		"randAlphaNum": randAlphaNumeric,
		"randAlpha":    randAlpha,
		"randASCII":    randASCII,
		"randNumeric":  randNumeric,
		"swapcase":     util.SwapCase,
		"shuffle":      xstrings.Shuffle,
		"snakecase":    xstrings.ToSnakeCase,
		"camelcase":    xstrings.ToCamelCase,
		"quote":        quote,
		"squote":       squote,
		"indent":       indent,
		"nindent":      nindent,
		"replace":      replace,
		"plural":       plural,
		"toString":     strval,
		"default":      dfault,
		"empty":        empty,
		"coalesce":     coalesce,
		"toJSON":       toJSON,
		"toPrettyJSON": toPrettyJSON,
		"b64enc":       base64encode,
		"b64dec":       base64decode,
		"escape":       escape,
		"stringQuote":  stringQuote,
		"add": func(i ...interface{}) int64 {
			var a int64
			for _, b := range i {
				a += toInt64(b)
			}
			return a
		},
		"sub": func(a, b interface{}) int64 { return toInt64(a) - toInt64(b) },
		"mul": func(a interface{}, v ...interface{}) int64 {
			val := toInt64(a)
			for _, b := range v {
				val *= toInt64(b)
			}
			return val
		},
		"div":       func(a, b interface{}) int64 { return toInt64(a) / toInt64(b) },
		"mod":       func(a, b interface{}) int64 { return toInt64(a) % toInt64(b) },
		"ternary":   ternary,
		"urlencode": url.QueryEscape,
		"dirname":   path.Dir,
		"basename":  path.Base,
	})
}

// wrapHelpers to handle usage of val struct in interpolate.Do
func wrapHelpers(fs template.FuncMap) template.FuncMap {
	wrappedHelpers := make(template.FuncMap, len(fs))
	for key, helper := range fs {
		helperV := reflect.ValueOf(helper)

		// ignore if current helper is not a func
		if helperV.Kind() != reflect.Func {
			continue
		}

		helperT := helperV.Type()
		paramsCount := helperT.NumIn()
		paramsTypes := make([]string, paramsCount)
		for i := 0; i < paramsCount; i++ {
			paramsTypes[i] = helperT.In(i).Name()
		}

		// create the wrapper func
		wrappedHelpers[key] = func(ps ...interface{}) (interface{}, error) {
			// if the helper func need more params than ps length, return an error: text/template reports it
			if len(ps) < paramsCount {
				return nil, fmt.Errorf("missing params (expected: %s)", strings.Join(paramsTypes, ", "))
			}

			// for all helper's params, forward values from wrapper
			values := make([]reflect.Value, len(ps))
			for i := range ps {
				values[i] = helperParam(ps[i])
			}

			return helperResult(helperV.Call(values))
		}
	}
	return wrappedHelpers
}

// helperParam unwraps one argument of a wrapped helper.
func helperParam(p interface{}) reflect.Value {
	if value, ok := p.(*val); ok {
		// if the value is a pointer to val, we should return its internal value
		return reflect.ValueOf((*value)["_"])
	}
	if value, ok := p.(val); ok {
		// if the value is a val, we should return its internal value
		return reflect.ValueOf(value["_"])
	}
	if v := reflect.ValueOf(p); v.IsValid() {
		// for all params that are not val (string, integer...) use it directly
		return v
	}
	// if the value is not valid (means that given value is nil with unknown type), convert to nil void pointer
	var v *void
	return reflect.ValueOf(v)
}

// helperResult is what a wrapped helper hands to text/template: its first result and, for a helper that returns
// (value, error), the error when it is set.
func helperResult(results []reflect.Value) (interface{}, error) {
	if len(results) == 0 {
		return nil, nil
	}
	if len(results) > 1 {
		if err, ok := results[1].Interface().(error); ok && err != nil {
			return nil, err
		}
	}
	return results[0].Interface(), nil
}

// Switch order so that "$foo" | trimall "$"
func dfault(valfault ...interface{}) string {
	var castToString = func(i interface{}) string {
		s, _ := i.(string) //nolint:errcheck // soft cast: a non-string input is meant to fall back to the zero value (""), not an error
		return s
	}

	if len(valfault) == 0 {
		return ""
	}
	if len(valfault) == 1 {
		s := castToString(valfault[0])
		return s
	}
	for i := len(valfault) - 1; i >= 0; i-- {
		if s := castToString(valfault[i]); s != "" {
			return s
		}
	}

	return ""
}

// empty returns true if the given value has the zero value for its type.
func empty(given interface{}) bool {
	g := reflect.ValueOf(given)
	if !g.IsValid() {
		return true
	}

	// Basically adapted from text/template.isTrue
	switch g.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return g.Len() == 0
	case reflect.Bool:
		return !g.Bool()
	case reflect.Complex64, reflect.Complex128:
		return g.Complex() == 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return g.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return g.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return g.Float() == 0
	case reflect.Struct:
		return false
	default:
		return g.IsNil()
	}
}

// coalesce returns the first non-empty value.
func coalesce(v ...interface{}) interface{} {
	for _, val := range v {
		if !empty(val) {
			return val
		}
	}
	return nil
}

// toJSON encodes an item into a JSON string
// toJSON encodes an item into a JSON string. v always comes from already-templated/decoded YAML
// data (string, number, bool, map, slice), which json.Marshal always encodes successfully; on
// failure this degrades to an empty string, same as upstream sprig.
func toJSON(v interface{}) string {
	output, _ := json.Marshal(v) //nolint:errcheck // v is YAML-decoded data, always marshals
	return string(output)
}

// toPrettyJSON encodes an item into a pretty (indented) JSON string. Same guarantee as toJSON.
func toPrettyJSON(v interface{}) string {
	output, _ := json.MarshalIndent(v, "", "  ") //nolint:errcheck // v is YAML-decoded data, always marshals
	return string(output)
}

func base64encode(v string) string {
	return base64.StdEncoding.EncodeToString([]byte(v))
}

func base64decode(v string) string {
	data, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func abbrev(width int, s string) string {
	if width < 4 {
		return s
	}
	// util.Abbreviate(s, width) is AbbreviateFull(s, 0, width); AbbreviateFull's only error case is
	// maxWidth < 4, already excluded by the guard above.
	r, _ := util.Abbreviate(s, width) //nolint:errcheck // width>=4 here (guarded above), the only error case of the underlying AbbreviateFull
	return r
}

func abbrevboth(left, right int, s string) string {
	if right < 4 || left > 0 && right < 7 {
		return s
	}
	// AbbreviateFull's only error case is maxWidth (right) < 4, already excluded by the guard above.
	r, _ := util.AbbreviateFull(s, left, right) //nolint:errcheck // right>=4 here (guarded above), the only error case of AbbreviateFull
	return r
}
func initials(s string) string {
	// Wrap this just to eliminate the var args, which templates don't do well.
	return util.Initials(s)
}

// randAlphaNumeric, randAlpha, randASCII and randNumeric return a failure from the underlying util.Random* call:
// text/template turns the error of a template function into the Execute error. util.RandomSeed errors when
// count < 0, which a template author can pass (e.g. `{{ randAlphaNum -5 }}`).

func randAlphaNumeric(count int) (string, error) {
	r, err := util.RandomAlphaNumeric(count)
	if err != nil {
		return "", fmt.Errorf("randAlphaNumeric: %w", err)
	}
	return r, nil
}

func randAlpha(count int) (string, error) {
	r, err := util.RandomAlphabetic(count)
	if err != nil {
		return "", fmt.Errorf("randAlpha: %w", err)
	}
	return r, nil
}

func randASCII(count int) (string, error) {
	r, err := util.RandomAscii(count)
	if err != nil {
		return "", fmt.Errorf("randASCII: %w", err)
	}
	return r, nil
}

func randNumeric(count int) (string, error) {
	r, err := util.RandomNumeric(count)
	if err != nil {
		return "", fmt.Errorf("randNumeric: %w", err)
	}
	return r, nil
}

func untitle(str string) string {
	return util.Uncapitalize(str)
}

// title reproduces strings.Title's exact word-boundary rule (deprecated since Go 1.18, SA1019 —
// #2028), so the "title" template helper's behaviour does not change for existing suites: any
// character that is not an ASCII letter/digit/underscore, or a Unicode letter/digit, or is a Unicode
// space, starts a new word, and the first rune of each word is title-cased. golang.org/x/text/cases
// is the maintained, Unicode-correct replacement stdlib recommends, but adopting it changes output
// for non-ASCII input and adds a new dependency — out of scope for a lint fix; both stay open under
// the migration filed alongside the deprecated protobuf libraries (see grpc.go).
func title(s string) string {
	prev := ' '
	return strings.Map(func(r rune) rune {
		if isWordSeparator(prev) {
			prev = r
			return unicode.ToTitle(r)
		}
		prev = r
		return r
	}, s)
}

func isWordSeparator(r rune) bool {
	if r <= 0x7F {
		switch {
		case '0' <= r && r <= '9', 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', r == '_':
			return false
		}
		return true
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	return unicode.IsSpace(r)
}

func quote(str ...interface{}) string {
	out := make([]string, len(str))
	for i, s := range str {
		out[i] = fmt.Sprintf("%q", strval(s))
	}
	return strings.Join(out, " ")
}

func squote(str ...interface{}) string {
	out := make([]string, len(str))
	for i, s := range str {
		out[i] = fmt.Sprintf("'%v'", s)
	}
	return strings.Join(out, " ")
}

func indent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return pad + strings.ReplaceAll(v, "\n", "\n"+pad)
}

func nindent(spaces int, v string) string {
	return "\n" + indent(spaces, v)
}

func replace(old, newStr, src string) string {
	return strings.ReplaceAll(src, old, newStr)
}

func plural(one, many string, count int) string {
	if count == 1 {
		return one
	}
	return many
}

func strval(v interface{}) string {
	switch v := v.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case error:
		return v.Error()
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func trunc(c int, s string) string {
	if len(s) <= c {
		return s
	}
	return s[0:c]
}

// substring creates a substring of the given string.
//
// If start is < 0, this calls string[:length].
//
// If start is >= 0 and length < 0, this calls string[start:]
//
// Otherwise, this calls string[start, length].
func substring(start, length int, s string) string {
	if start < 0 {
		return s[:length]
	}
	if length < 0 {
		return s[start:]
	}
	return s[start:length]
}

func escape(s string) string {
	s1 := strings.ReplaceAll(s, "_", "-")
	s1 = strings.ReplaceAll(s1, "/", "-")
	s1 = strings.ReplaceAll(s1, ".", "-")
	return s1
}

func stringQuote(s string) string {
	x := strconv.Quote(s)
	x = strings.TrimPrefix(x, `"`)
	x = strings.TrimSuffix(x, `"`)
	return x
}

func ternary(v, v2, a interface{}) interface{} {
	if cast.ToBool(a) {
		return v
	}
	return v2
}

// toInt64 converts integer types to 64-bit integers
func toInt64(v interface{}) int64 {
	return cast.ToInt64(v)
}
