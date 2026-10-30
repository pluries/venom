package venom

import (
	"maps"
	"reflect"
	"strconv"
	"strings"
)

// When go-dump flattens the variable scope it emits two whole-map aggregates alongside the
// per-entry keys: the entry count and the type. Every other key belongs to exactly one top-level
// entry — that decomposition is what varsFlattener caches, and the characterisation tests in
// varsflatten_test.go pin it, so a go-dump upgrade that changed it would fail the build rather than
// quietly produce a wrong variable set.
const (
	dumpLenKey  = "__Len__"
	dumpTypeKey = "__Type__"
)

// varsFlattener produces the flat map[string]string that a step interpolates against, reusing the
// previous result for any variable whose value has not changed since the last step.
//
// Why: DumpStringPreserveCase walks every value by reflection, at a cost proportional to the
// value's size and multiplied by its nesting depth — #1982 measured ~16ms for a single ~110KB
// captured API response. A suite that captures a response once and refers to it over several steps
// paid that walk again on every step, for a byte-identical result.
//
// One flattener belongs to one test case. It is not safe for concurrent use and does not need to
// be: the steps of a test case run in order.
type varsFlattener struct {
	// entries is keyed by variable name, so it holds at most one cached value per name in scope —
	// naturally bounded, and superseded automatically when a name takes a new value.
	entries map[string]flattenedEntry
}

type flattenedEntry struct {
	id valueIdentity
	// keys is the entry's contribution to the flattened scope, with the whole-map aggregates
	// stripped. It is never handed to a caller directly — see flatten.
	keys map[string]string
	// value is retained so the address in id cannot be recycled by a later allocation while this
	// entry is cached, which would turn the pointer comparison into a false hit.
	value interface{}
}

// valueIdentity identifies a value by object rather than by content: comparing content would mean
// walking it, which is the cost being avoided.
type valueIdentity struct {
	typ      reflect.Type
	ptr      uintptr
	length   int
	capacity int
}

// identify returns an identity for v, and whether v is worth caching at all. Only map, slice and
// pointer values get one: they are the shapes a decoded JSON body takes, they are the expensive
// ones, and they are the kinds reflect can address. Scalars, structs and arrays are flattened on
// the normal path — they are inline values with no stable address, and they cost microseconds.
func identify(v interface{}) (valueIdentity, bool) {
	if v == nil {
		return valueIdentity{}, false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map:
		// Nothing to gain: an empty map flattens to a couple of keys. (It is also not a false-hit
		// risk — empty maps do get distinct addresses — so this is an economy, not a guard.)
		if rv.Len() == 0 {
			return valueIdentity{}, false
		}
		// length is not needed to establish identity, but it does catch the subset of in-place
		// mutation that adds or removes an entry: such a value is re-flattened instead of served
		// stale. Replacing a value under an existing key is still invisible — see the risk section
		// of docs/feat/2017/plan.md.
		return valueIdentity{typ: rv.Type(), ptr: rv.Pointer(), length: rv.Len()}, true
	case reflect.Slice:
		if rv.Len() == 0 {
			return valueIdentity{}, false
		}
		// Two slices over one backing array share a pointer, so length is part of the identity.
		// Capacity is not needed to tell two windows apart — same pointer and length means the same
		// elements — but it is cheap and keeps the identity conservative.
		return valueIdentity{typ: rv.Type(), ptr: rv.Pointer(), length: rv.Len(), capacity: rv.Cap()}, true
	case reflect.Pointer:
		if rv.IsNil() {
			return valueIdentity{}, false
		}
		return valueIdentity{typ: rv.Type(), ptr: rv.Pointer()}, true
	default:
		return valueIdentity{}, false
	}
}

// sharesPrefixWithAnotherName reports whether another variable in the same scope flattens into
// keys that can collide with name's.
//
// Suite- and testcase-level vars reach a step already flattened to strings, so a scope routinely
// holds both "body.status" (a string from the suite) and "body" (the map a step just captured).
// Flattening those two separately and merging would give a fixed winner, where dumping the whole
// scope in one call lets go-dump's map iteration order decide — which is to say, it is a coin flip
// today. Keeping a shadowed name out of the cache leaves that resolution exactly as it was: this
// change is a cache, and #2017 puts any change to the flattened output out of scope. The cost is
// only paid when a name is genuinely shadowed, which the common capture-and-reuse case is not.
func sharesPrefixWithAnotherName(name string, vars H) bool {
	for other := range vars {
		if other == name {
			continue
		}
		if strings.HasPrefix(other, name+".") || strings.HasPrefix(name, other+".") {
			return true
		}
	}
	return false
}

// flatten renders vars as the engine's flat variable map. The result is always a fresh map: the
// caller interpolates and quote-escapes it in place (process_testcase.go), so handing back a cached
// map would corrupt the cache.
func (f *varsFlattener) flatten(vars H) (map[string]string, error) {
	// Everything not worth caching is dumped in one call rather than one call per entry, so a scope
	// of plain scalars costs what it always did.
	uncached := make(H, len(vars))
	cacheable := make(map[string]valueIdentity, len(vars))
	for k, v := range vars {
		if id, ok := identify(v); ok && !sharesPrefixWithAnotherName(k, vars) {
			cacheable[k] = id
			continue
		}
		uncached[k] = v
	}

	out, err := dumpHPreserveCase(uncached)
	if err != nil {
		return nil, err
	}

	for k, id := range cacheable {
		entry, hit := f.entries[k]
		if !hit || entry.id != id {
			part, err := DumpStringPreserveCase(H{k: vars[k]})
			if err != nil {
				return nil, err
			}
			delete(part, dumpLenKey)
			delete(part, dumpTypeKey)
			entry = flattenedEntry{id: id, keys: part, value: vars[k]}
			if f.entries == nil {
				f.entries = make(map[string]flattenedEntry, len(cacheable))
			}
			f.entries[k] = entry
		}
		maps.Copy(out, entry.keys)
	}

	// The aggregates describe the whole scope, not the subset that was dumped above.
	out[dumpLenKey] = strconv.Itoa(len(vars))

	return out, nil
}
