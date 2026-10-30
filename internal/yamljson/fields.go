// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copied from github.com/rockbears/yaml v0.4.0 (itself derived from encoding/json); see NOTICE.md.

package yamljson

import (
	"bytes"
	"encoding"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// indirect walks down v allocating pointers as needed,
// until it gets to a non-pointer.
// if it encounters an Unmarshaler, indirect stops and returns that.
// if decodingNull is true, indirect stops at the last pointer so it can be set to nil.
func indirect(v reflect.Value, decodingNull bool) (json.Unmarshaler, encoding.TextUnmarshaler, reflect.Value) {
	// If v is a named type and is addressable,
	// start with its address, so that if the type has pointer methods,
	// we find them.
	if v.Kind() != reflect.Pointer && v.Type().Name() != "" && v.CanAddr() {
		v = v.Addr()
	}
	for {
		if e, ok := interfaceElem(v, decodingNull); ok {
			v = e
			continue
		}
		if v.Kind() != reflect.Pointer {
			break
		}
		if v.Elem().Kind() != reflect.Pointer && decodingNull && v.CanSet() {
			break
		}
		v = allocIfNil(v)
		if ju, tu := unmarshalerOf(v); ju != nil || tu != nil {
			return ju, tu, reflect.Value{}
		}
		v = v.Elem()
	}
	return nil, nil, v
}

// interfaceElem loads the value held by an interface, but only if the result will be usefully addressable.
func interfaceElem(v reflect.Value, decodingNull bool) (reflect.Value, bool) {
	if v.Kind() != reflect.Interface || v.IsNil() {
		return v, false
	}
	e := v.Elem()
	if e.Kind() == reflect.Pointer && !e.IsNil() && (!decodingNull || e.Elem().Kind() == reflect.Pointer) {
		return e, true
	}
	return v, false
}

// allocIfNil gives a nil pointer a value to point to.
func allocIfNil(v reflect.Value) reflect.Value {
	if !v.IsNil() {
		return v
	}
	if v.CanSet() {
		v.Set(reflect.New(v.Type().Elem()))
		return v
	}
	return reflect.New(v.Type().Elem())
}

// unmarshalerOf returns the json or text unmarshaler v implements, if any.
func unmarshalerOf(v reflect.Value) (json.Unmarshaler, encoding.TextUnmarshaler) {
	if v.Type().NumMethod() == 0 {
		return nil, nil
	}
	if u, ok := v.Interface().(json.Unmarshaler); ok {
		return u, nil
	}
	if u, ok := v.Interface().(encoding.TextUnmarshaler); ok {
		return nil, u
	}
	return nil, nil
}

// A field represents a single field found in a struct.
type field struct {
	name      string
	nameBytes []byte                 // []byte(name)
	equalFold func(s, t []byte) bool // bytes.EqualFold or equivalent

	tag       bool
	index     []int
	typ       reflect.Type
	omitEmpty bool
	quoted    bool
}

func fillField(f field) field {
	f.nameBytes = []byte(f.name)
	f.equalFold = foldFunc(f.nameBytes)
	return f
}

// byName sorts field by name, breaking ties with depth,
// then breaking ties with "name came from json tag", then
// breaking ties with index sequence.
type byName []field

func (x byName) Len() int { return len(x) }

func (x byName) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

func (x byName) Less(i, j int) bool {
	if x[i].name != x[j].name {
		return x[i].name < x[j].name
	}
	if len(x[i].index) != len(x[j].index) {
		return len(x[i].index) < len(x[j].index)
	}
	if x[i].tag != x[j].tag {
		return x[i].tag
	}
	return byIndex(x).Less(i, j)
}

// byIndex sorts field by index sequence.
type byIndex []field

func (x byIndex) Len() int { return len(x) }

func (x byIndex) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

func (x byIndex) Less(i, j int) bool {
	for k, xik := range x[i].index {
		if k >= len(x[j].index) {
			return false
		}
		if xik != x[j].index[k] {
			return xik < x[j].index[k]
		}
	}
	return len(x[i].index) < len(x[j].index)
}

// typeFields returns a list of fields that JSON should recognize for the given type.
// The algorithm is breadth-first search over the set of structs to include - the top struct
// and then any reachable anonymous structs.
func typeFields(t reflect.Type) []field {
	// Anonymous fields to explore at the current level and the next.
	current := []field{}
	next := []field{{typ: t}}

	// Count of queued names for current level and the next.
	var count, nextCount map[reflect.Type]int

	// Types already visited at an earlier level.
	visited := map[reflect.Type]bool{}

	// Fields found.
	var fields []field

	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, map[reflect.Type]int{}

		for _, f := range current {
			if visited[f.typ] {
				continue
			}
			visited[f.typ] = true
			fields, next = scanStruct(f, count, nextCount, fields, next)
		}
	}

	sort.Sort(byName(fields))
	fields = dropHidden(fields)
	sort.Sort(byIndex(fields))
	return fields
}

// scanStruct scans f.typ for fields to include, and queues the anonymous structs it finds.
func scanStruct(f field, count, nextCount map[reflect.Type]int, fields, next []field) ([]field, []field) {
	for i := 0; i < f.typ.NumField(); i++ {
		sf := f.typ.Field(i)
		if sf.PkgPath != "" { // unexported
			continue
		}
		tag := sf.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts := parseTag(tag)
		if !isValidTag(name) {
			name = ""
		}
		index := make([]int, len(f.index)+1)
		copy(index, f.index)
		index[len(f.index)] = i

		ft := sf.Type
		if ft.Name() == "" && ft.Kind() == reflect.Pointer {
			// Follow pointer.
			ft = ft.Elem()
		}

		// Record found field and index sequence.
		if name != "" || !sf.Anonymous || ft.Kind() != reflect.Struct {
			fields = appendField(fields, name, sf.Name, opts, index, ft, count[f.typ] > 1)
			continue
		}

		// Record new anonymous struct to explore in next round.
		nextCount[ft]++
		if nextCount[ft] == 1 {
			next = append(next, fillField(field{name: ft.Name(), index: index, typ: ft}))
		}
	}
	return fields, next
}

// appendField records a found field; when the struct holding it was queued several times it adds a
// second copy, so that the annihilation code will see a duplicate (it only tells 1 from 2).
func appendField(fields []field, name, goName string, opts tagOptions, index []int, ft reflect.Type, duplicate bool) []field {
	tagged := name != ""
	if name == "" {
		name = goName
	}
	fields = append(fields, fillField(field{
		name:      name,
		tag:       tagged,
		index:     index,
		typ:       ft,
		omitEmpty: opts.Contains("omitempty"),
		quoted:    opts.Contains("string"),
	}))
	if duplicate {
		fields = append(fields, fields[len(fields)-1])
	}
	return fields
}

// runLength returns how many fields at the start of fields share the name of the first one.
func runLength(fields []field) int {
	n := 1
	for n < len(fields) && fields[n].name == fields[0].name {
		n++
	}
	return n
}

// dropHidden deletes all fields that are hidden by the Go rules for embedded fields,
// except that fields with JSON tags are promoted.
//
// The fields are sorted in primary order of name, secondary order
// of field index length. Loop over names; for each name, delete
// hidden fields by choosing the one dominant field that survives.
func dropHidden(fields []field) []field {
	out := fields[:0]
	for advance, i := 0, 0; i < len(fields); i += advance {
		// One iteration per name: find the run of fields with the name of this first field.
		advance = runLength(fields[i:])
		if advance == 1 { // Only one field with this name
			out = append(out, fields[i])
			continue
		}
		if dominant, ok := dominantField(fields[i : i+advance]); ok {
			out = append(out, dominant)
		}
	}
	return out
}

// dominantField looks through the fields, all of which are known to
// have the same name, to find the single field that dominates the
// others using Go's embedding rules, modified by the presence of
// JSON tags. If there are multiple top-level fields, the boolean
// will be false: This condition is an error in Go and we skip all
// the fields.
func dominantField(fields []field) (field, bool) {
	// The fields are sorted in increasing index-length order. The winner
	// must therefore be one with the shortest index length. Drop all
	// longer entries, which is easy: just truncate the slice.
	length := len(fields[0].index)
	tagged := -1 // Index of first tagged field.
	for i, f := range fields {
		if len(f.index) > length {
			fields = fields[:i]
			break
		}
		if f.tag {
			if tagged >= 0 {
				// Multiple tagged fields at the same level: conflict.
				// Return no field.
				return field{}, false
			}
			tagged = i
		}
	}
	if tagged >= 0 {
		return fields[tagged], true
	}
	// All remaining fields have the same length. If there's more than one,
	// we have a conflict (two fields named "X" at the same level) and we
	// return no field.
	if len(fields) > 1 {
		return field{}, false
	}
	return fields[0], true
}

var fieldCache struct {
	sync.RWMutex
	m map[reflect.Type][]field
}

// cachedTypeFields is like typeFields but uses a cache to avoid repeated work.
func cachedTypeFields(t reflect.Type) []field {
	fieldCache.RLock()
	f := fieldCache.m[t]
	fieldCache.RUnlock()
	if f != nil {
		return f
	}

	// Compute fields without lock.
	// Might duplicate effort but won't hold other computations back.
	f = typeFields(t)
	if f == nil {
		f = []field{}
	}

	fieldCache.Lock()
	if fieldCache.m == nil {
		fieldCache.m = map[reflect.Type][]field{}
	}
	fieldCache.m[t] = f
	fieldCache.Unlock()
	return f
}

func isValidTag(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:<=>?@[]^_{|}~ ", c):
			// Backslash and quote chars are reserved, but
			// otherwise any punctuation chars are allowed
			// in a tag name.
		default:
			if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
				return false
			}
		}
	}
	return true
}

const (
	caseMask     = ^byte(0x20) // Mask to ignore case in ASCII.
	kelvin       = '\u212a'
	smallLongEss = '\u017f'
)

// foldFunc returns one of four different case folding equivalence
// functions, from most general (and slow) to fastest:
//
// 1) bytes.EqualFold, if the key s contains any non-ASCII UTF-8
// 2) equalFoldRight, if s contains special folding ASCII ('k', 'K', 's', 'S')
// 3) asciiEqualFold, no special, but includes non-letters (including _)
// 4) simpleLetterEqualFold, no specials, no non-letters.
//
// The letters S and K are special because they map to 3 runes, not just 2:
//   - S maps to s and to U+017F 'ſ' Latin small letter long s
//   - k maps to K and to U+212A 'K' Kelvin sign
//
// See http://play.golang.org/p/tTxjOc0OGo
//
// The returned function is specialized for matching against s and
// should only be given s. It's not curried for performance reasons.
func foldFunc(s []byte) func(s, t []byte) bool {
	nonLetter := false
	special := false // special letter
	for _, b := range s {
		if b >= utf8.RuneSelf {
			return bytes.EqualFold
		}
		upper := b & caseMask
		if upper < 'A' || upper > 'Z' {
			nonLetter = true
		} else if upper == 'K' || upper == 'S' {
			// See above for why these letters are special.
			special = true
		}
	}
	if special {
		return equalFoldRight
	}
	if nonLetter {
		return asciiEqualFold
	}
	return simpleLetterEqualFold
}

// equalFoldRight is a specialization of bytes.EqualFold when s is
// known to be all ASCII (including punctuation), but contains an 's',
// 'S', 'k', or 'K', requiring a Unicode fold on the bytes in t.
// See comments on foldFunc.
func equalFoldRight(s, t []byte) bool {
	for _, sb := range s {
		n, ok := foldStep(sb, t)
		if !ok {
			return false
		}
		t = t[n:]
	}
	return len(t) == 0
}

// foldStep matches the byte sb of s against the start of t and returns how many bytes of t it used.
func foldStep(sb byte, t []byte) (int, bool) {
	if len(t) == 0 {
		return 0, false
	}
	if tb := t[0]; tb < utf8.RuneSelf {
		return 1, asciiFoldMatch(sb, tb)
	}
	// sb is ASCII and t is not. t must be either kelvin
	// sign or long s; sb must be s, S, k, or K.
	tr, size := utf8.DecodeRune(t)
	return size, specialFoldMatch(sb, tr)
}

func asciiFoldMatch(sb, tb byte) bool {
	if sb == tb {
		return true
	}
	sbUpper := sb & caseMask
	return 'A' <= sbUpper && sbUpper <= 'Z' && sbUpper == tb&caseMask
}

func specialFoldMatch(sb byte, tr rune) bool {
	switch sb {
	case 's', 'S':
		return tr == smallLongEss
	case 'k', 'K':
		return tr == kelvin
	}
	return false
}

// asciiEqualFold is a specialization of bytes.EqualFold for use when
// s is all ASCII (but may contain non-letters) and contains no
// special-folding letters.
// See comments on foldFunc.
func asciiEqualFold(s, t []byte) bool {
	if len(s) != len(t) {
		return false
	}
	for i, sb := range s {
		tb := t[i]
		if sb == tb {
			continue
		}
		if ('a' <= sb && sb <= 'z') || ('A' <= sb && sb <= 'Z') {
			if sb&caseMask != tb&caseMask {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// simpleLetterEqualFold is a specialization of bytes.EqualFold for
// use when s is all ASCII letters (no underscores, etc) and also
// doesn't contain 'k', 'K', 's', or 'S'.
// See comments on foldFunc.
func simpleLetterEqualFold(s, t []byte) bool {
	if len(s) != len(t) {
		return false
	}
	for i, b := range s {
		if b&caseMask != t[i]&caseMask {
			return false
		}
	}
	return true
}

// tagOptions is the string following a comma in a struct field's "json"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

// parseTag splits a struct field's json tag into its name and
// comma-separated options.
func parseTag(tag string) (string, tagOptions) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tagOptions(tag[idx+1:])
	}
	return tag, tagOptions("")
}

// Contains reports whether a comma-separated list of options
// contains a particular substr flag. substr must be surrounded by a
// string boundary or commas.
func (o tagOptions) Contains(optionName string) bool {
	if len(o) == 0 {
		return false
	}
	s := string(o)
	for s != "" {
		var next string
		i := strings.Index(s, ",")
		if i >= 0 {
			s, next = s[:i], s[i+1:]
		}
		if s == optionName {
			return true
		}
		s = next
	}
	return false
}
