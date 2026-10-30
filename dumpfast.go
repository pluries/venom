package venom

import (
	"strconv"
	"strings"
)

// dumpFunc is DumpString or DumpStringPreserveCase.
type dumpFunc func(interface{}) (map[string]string, error)

// A scope reaches a step almost entirely as plain strings: initTestSuiteVars turns a suite's variables, nested ones
// included, into one string per dotted key (#2405). Walking each of them by reflection is the cost of a flatten, for a
// result that is the string itself. dumpScope copies the strings whose key go-dump leaves alone, and hands the rest to
// go-dump. The tests in varsflatten_test.go pin what go-dump does with a string, and compare dumpScope to it.

// dumpScope returns what dump returns for h, with the string values under a plain key copied rather than walked.
// lowerFirst says the key is rewritten the way DumpString does it, and aggLen and aggType name the two whole-map
// aggregates of the dump.
func dumpScope(h H, dump dumpFunc, lowerFirst bool, aggLen, aggType string) (map[string]string, error) {
	out := make(map[string]string, len(h)+2)
	var rest H
	count := 0
	for k, v := range h {
		switch classify(k, v, lowerFirst, aggLen, aggType) {
		case entryShadowed:
			count++
		case entryPlain:
			if s, ok := v.(string); ok { // classify returns entryPlain for a string only
				out[renamedKey(k, lowerFirst)] = s
			}
			count++
		case entryClash:
			return dump(h)
		default:
			if rest == nil {
				rest = H{}
			}
			rest[k] = v
		}
	}
	if rest != nil {
		restCount, ok := mergeDumped(out, rest, dump, aggLen, aggType)
		if !ok {
			return dump(h)
		}
		count += restCount
	}
	out[aggLen] = strconv.Itoa(count)
	out[aggType] = dumpTypeMap
	return out, nil
}

// dumpTypeMap is the value go-dump gives the type aggregate of a map.
const dumpTypeMap = "Map"

// entryKind says how dumpScope handles one variable.
type entryKind int

const (
	entryOpaque   entryKind = iota // left to go-dump: not a string, or a key the formatter rewrites
	entryPlain                     // a string under a key go-dump leaves alone: copied
	entryShadowed                  // named like an aggregate, and the aggregate says the same: counted, not copied
	entryClash                     // named like an aggregate with another value: the whole scope is left to go-dump
)

// classify decides how a variable is dumped. A variable named like the length is overwritten by the aggregate, which
// is pinned by TestDumpPreserveCaseRewritesKeysWithSpaceSlashOrColon; one named like the type keeps its own value, which
// is the aggregate's when it is "Map", as initTestSuiteVars stores it, and is anything go-dump decides otherwise.
func classify(key string, v interface{}, lowerFirst bool, aggLen, aggType string) entryKind {
	renamed := renamedKey(key, lowerFirst)
	switch renamed {
	case aggLen:
		if _, ok := v.(string); ok {
			return entryShadowed
		}
		return entryClash
	case aggType:
		if s, ok := v.(string); ok && s == dumpTypeMap {
			return entryShadowed
		}
		return entryClash
	}
	if _, ok := v.(string); ok && plainKey(key) {
		return entryPlain
	}
	return entryOpaque
}

// mergeDumped adds the dump of rest to out. It reports false when rest cannot be dumped on its own, or when a key
// of its dump is already in out with another value: go-dump then lets its map order decide which one wins, so the
// whole scope is left to go-dump. It returns the entry count go-dump gives for rest, which is not len(rest) when
// go-dump skips an entry.
func mergeDumped(out map[string]string, rest H, dump dumpFunc, aggLen, aggType string) (int, bool) {
	part, err := dump(rest)
	if err != nil {
		return 0, false
	}
	count, err := strconv.Atoi(part[aggLen])
	if err != nil {
		return 0, false
	}
	if t, ok := part[aggType]; ok && t != dumpTypeMap {
		return 0, false
	}
	for k, v := range part {
		if k == aggLen || k == aggType {
			continue
		}
		if old, taken := out[k]; taken && old != v {
			return 0, false
		}
		out[k] = v
	}
	return count, true
}

// plainKey reports whether go-dump copies a string under key unchanged, apart from the first-segment rewrite of
// DumpString: its default formatter turns ' ', '/' and ':' into '_', and it drops an empty key.
func plainKey(key string) bool {
	return key != "" && !strings.ContainsAny(key, " /:")
}

// renamedKey is the key DumpString gives a top-level name (WithFormatterLowerFirstKey), or key itself.
func renamedKey(key string, lowerFirst bool) string {
	if !lowerFirst {
		return key
	}
	if pos := strings.Index(key, "."); pos >= 0 {
		return strings.ToLower(key[:pos]) + key[pos:]
	}
	return strings.ToLower(key)
}

// dumpHPreserveCase is DumpStringPreserveCase for a variable scope.
func dumpHPreserveCase(h H) (map[string]string, error) {
	return dumpScope(h, DumpStringPreserveCase, false, dumpLenKey, dumpTypeKey)
}

// dumpHLowerFirst is DumpString for a variable scope.
func dumpHLowerFirst(h H) (map[string]string, error) {
	return dumpScope(h, DumpString, true, "__len__", "__type__")
}

// scopeEdit records the variables set on a scope after it was dumped, so the dump can be brought up to date by dumping
// those variables alone instead of the whole scope again (#2465). It follows only the edits whose effect on the dump is
// local: a new variable, or a string replacing a string under a plain key. Any other edit leaves the whole scope to
// dumpScope.
type scopeEdit struct {
	fresh    H                 // variables that were not in the scope
	replaced map[string]string // string values that replaced a string under a plain key
	complex  bool
}

// set stores v under k in h, the way h.Add does, and records it.
func (e *scopeEdit) set(h H, k string, v interface{}) {
	e.setUnder(h, nil, k, v)
}

// setUnder is set for a scope held as h over under: a name that only under holds counts as one the scope had, since the
// whole scope, h laid over under, has it. under is only read.
func (e *scopeEdit) setUnder(h, under H, k string, v interface{}) {
	old, had := h[k]
	if !had {
		old, had = under[k]
	}
	h[k] = v
	switch ns, isString := v.(string); {
	case !had:
		e.addFresh(k, v)
	case e.inFresh(k):
		e.fresh[k] = v
	case isString && plainKey(k):
		if _, wasString := old.(string); wasString {
			if e.replaced == nil {
				e.replaced = map[string]string{}
			}
			e.replaced[k] = ns
			return
		}
		e.complex = true
	default:
		e.complex = true
	}
}

func (e *scopeEdit) inFresh(k string) bool {
	_, ok := e.fresh[k]
	return ok
}

func (e *scopeEdit) addFresh(k string, v interface{}) {
	if e.fresh == nil {
		e.fresh = H{}
	}
	e.fresh[k] = v
}

// apply updates base, a dump of the scope as it was before the edits, to be the dump of the scope after them. It reports
// false when base must be recomputed: an edit it does not follow, a variable named like an aggregate, or a key of the
// fresh variables' dump that is already in base with another value. base is changed only when it reports true.
func (e *scopeEdit) apply(base map[string]string, dump dumpFunc, lowerFirst bool, aggLen, aggType string) bool {
	if base == nil {
		return false
	}
	return e.applyLayered(scopeLayers{overlay: base}, dump, lowerFirst, aggLen, aggType)
}

// applyLayered is apply on a dump held as layers: the edits are written to the overlay, and the shared base under it is
// only read (#2475).
func (e *scopeEdit) applyLayered(base scopeLayers, dump dumpFunc, lowerFirst bool, aggLen, aggType string) bool {
	if e.complex || base.overlay == nil {
		return false
	}
	aggregate, _ := base.get(aggLen)
	count, err := strconv.Atoi(aggregate)
	if err != nil {
		return false
	}
	for k := range e.replaced {
		if isAggregateName(k, lowerFirst, aggLen, aggType) {
			return false
		}
	}
	for k := range e.fresh {
		if isAggregateName(k, lowerFirst, aggLen, aggType) {
			return false
		}
	}
	if len(e.fresh) > 0 {
		part, ok := dumpFresh(base, e.fresh, dump, aggLen, aggType)
		if !ok {
			return false
		}
		count += part.count
		for k, v := range part.entries {
			base.overlay[k] = v
		}
	}
	for k, v := range e.replaced {
		base.overlay[renamedKey(k, lowerFirst)] = v
	}
	base.overlay[aggLen] = strconv.Itoa(count)
	return true
}

func isAggregateName(k string, lowerFirst bool, aggLen, aggType string) bool {
	r := renamedKey(k, lowerFirst)
	return r == aggLen || r == aggType
}

type freshDump struct {
	entries map[string]string
	count   int
}

// dumpFresh dumps the variables fresh and checks them against base as mergeDumped does.
func dumpFresh(base scopeLayers, fresh H, dump dumpFunc, aggLen, aggType string) (freshDump, bool) {
	into := map[string]string{}
	count, ok := mergeDumped(into, fresh, dump, aggLen, aggType)
	if !ok {
		return freshDump{}, false
	}
	for k, v := range into {
		if old, taken := base.get(k); taken && old != v {
			return freshDump{}, false
		}
	}
	return freshDump{entries: into, count: count}, true
}
