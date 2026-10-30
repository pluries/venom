package interpolate

import (
	"sort"
	"strings"
)

// Base is a flat variable scope prepared once, to be read many times through DoOver and UnknownVariablesOver (#2475,
// ADR-0011). Do and UnknownVariables look at every name of their scope on each call (to rewrite the "-" and to pick the
// variables a template reads), which costs thousands of passes when a suite declares a large nested variable. A Base
// keeps the names sorted by their internal spelling and the names that contain a "-", so a call finds what a template
// reads by lookup and pays for the overlay only.
//
// A Base is immutable once built and safe for concurrent reads. It keeps the map it was built from, not a copy: the
// caller must not change that map afterwards.
type Base struct {
	vars     map[string]string
	internal []string // internalName of each name, sorted
	raw      []string // the name as written, at the same index as in internal
	dashed   []string // the names that contain a "-", sorted
}

// NewBase prepares vars, which the caller must not mutate afterwards. A nil map is an empty base.
func NewBase(vars map[string]string) *Base {
	b := &Base{vars: vars}
	// The internal spelling is computed once per name, not on each comparison of the sort.
	type pair struct{ internal, raw string }
	pairs := make([]pair, 0, len(vars))
	for k := range vars {
		pairs = append(pairs, pair{internalName(k), k})
		if strings.Contains(k, "-") {
			b.dashed = append(b.dashed, k)
		}
	}
	sort.Strings(b.dashed)
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].internal < pairs[j].internal })
	b.internal = make([]string, len(pairs))
	b.raw = make([]string, len(pairs))
	for i, p := range pairs {
		b.internal[i], b.raw[i] = p.internal, p.raw
	}
	return b
}

// maxSharedExtras is the number of names WithVars accepts beyond those it shares.
const maxSharedExtras = 16

// WithVars returns a Base over vars that shares the sorted names of b, or nil when it cannot (#2496): the names of vars
// must be the names of b and at most maxSharedExtras others. Those few are merged into copies of the lists, so the sort of
// all the names is not done again. The values of vars do not matter, only its names. A nil result means the caller builds
// the index with NewBase. Like NewBase it keeps vars, which must not be changed afterwards; b is not changed.
func (b *Base) WithVars(vars map[string]string) *Base {
	if b == nil {
		return nil
	}
	extras, ok := b.extraNames(vars)
	if !ok {
		return nil
	}
	derived := &Base{vars: vars, internal: b.internal, raw: b.raw, dashed: b.dashed}
	if len(extras) > 0 {
		derived.internal, derived.raw = b.mergeExtras(extras)
		derived.dashed = b.dashedWith(extras)
	}
	return derived
}

// extraNames returns the names of vars that b does not have, and reports whether vars holds every name of b with no more
// than maxSharedExtras extras: the names of vars that b knows are b's names exactly when they are as many as b has.
func (b *Base) extraNames(vars map[string]string) ([]string, bool) {
	var extras []string
	for k := range vars {
		if _, known := b.vars[k]; known {
			continue
		}
		if len(extras) == maxSharedExtras {
			return nil, false
		}
		extras = append(extras, k)
	}
	return extras, len(vars)-len(extras) == len(b.vars)
}

// mergeExtras returns the sorted lists of b with extras inserted, in one pass over the lists.
func (b *Base) mergeExtras(extras []string) (internal, raw []string) {
	type pair struct{ internal, raw string }
	pairs := make([]pair, len(extras))
	for i, k := range extras {
		pairs[i] = pair{internalName(k), k}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].internal < pairs[j].internal })
	internal = make([]string, 0, len(b.internal)+len(pairs))
	raw = make([]string, 0, len(b.raw)+len(pairs))
	from := 0
	for _, p := range pairs {
		at := from + sort.SearchStrings(b.internal[from:], p.internal)
		internal = append(append(internal, b.internal[from:at]...), p.internal)
		raw = append(append(raw, b.raw[from:at]...), p.raw)
		from = at
	}
	return append(internal, b.internal[from:]...), append(raw, b.raw[from:]...)
}

// dashedWith returns the dashed names of b with those of extras, sorted.
func (b *Base) dashedWith(extras []string) []string {
	dashed := b.dashed
	for _, k := range extras {
		if strings.Contains(k, "-") {
			dashed = append(dashed[:len(dashed):len(dashed)], k)
		}
	}
	if len(dashed) != len(b.dashed) {
		sort.Strings(dashed)
	}
	return dashed
}

// DoOver gives the result and the error of Do on the base with overlay laid over it (an overlay name replaces the base
// one), without a pass over the base: the cost follows the variables the template reads and the size of the overlay.
func DoOver(input string, base *Base, overlay map[string]string) (string, error) {
	return DoOverMasked(input, base, overlay, nil)
}

// DoOverMasked is DoOver on the base without the names in masked: a step that replaces a string of the suite by a value
// that flattens to other names must not leave the replaced name behind. masked is read, never changed.
func DoOverMasked(input string, base *Base, overlay map[string]string, masked map[string]struct{}) (string, error) {
	if !strings.Contains(input, "{{") {
		return input, nil
	}

	data, flatData, replacements, err := indexForOver(input, base, overlay, masked)
	if err != nil {
		return "", err
	}
	return render(input, data, flatData, replacements)
}

// UnknownVariablesOver gives the result of UnknownVariables on the base with overlay laid over it. UnknownVariables
// can only fail when two names of its scope conflict in the nested index, and the index puts the deeper names first,
// so that error cannot occur; this one has none to report.
func UnknownVariablesOver(input string, base *Base, overlay map[string]string) ([]string, error) {
	return UnknownVariablesOverMasked(input, base, overlay, nil)
}

// UnknownVariablesOverMasked is UnknownVariablesOver on the base without the names in masked.
func UnknownVariablesOverMasked(input string, base *Base, overlay map[string]string, masked map[string]struct{}) ([]string, error) {
	if !strings.Contains(input, "{{") {
		return nil, nil
	}

	inOverlay := make(map[string]void, len(overlay))
	for k := range overlay {
		inOverlay[internalName(k)] = void{}
	}
	return unknownIn(input, base.dashReplacements(overlay, masked), func(name string) bool {
		_, ok := inOverlay[name]
		return ok || base.has(name, masked)
	}), nil
}

// indexForOver is indexFor on a layered scope: it falls back to the merged map, and so to the full index, in the cases
// indexFor does.
func indexForOver(input string, base *Base, overlay map[string]string, masked map[string]struct{}) (val, map[string]string, []string, error) {
	replacements := base.dashReplacements(overlay, masked)
	rewritten := strings.NewReplacer(replacements...).Replace(input)
	names, ok := referencedNames(rewritten)
	if !ok {
		return indexVariables(base.merged(overlay, masked))
	}
	addScannedNames(names, rewritten)
	data, flatData, err := indexSubset(base.selectVariables(names, overlay, masked))
	return data, flatData, replacements, err
}

// merged is the base without the masked names, with overlay laid over it, as one map.
func (b *Base) merged(overlay map[string]string, masked map[string]struct{}) map[string]string {
	m := make(map[string]string, len(b.vars)+len(overlay))
	for k, v := range b.vars {
		if _, gone := masked[k]; !gone {
			m[k] = v
		}
	}
	for k, v := range overlay {
		m[k] = v
	}
	return m
}

// dashReplacements is dashReplacements on the merged scope. The pairs end in a delimiter (" ", "}" or "|") that a name
// does not hold in practice, so none is a prefix of another and the order they are given in does not change what they rewrite; the
// existing function walks a map, so it gives no order either.
func (b *Base) dashReplacements(overlay map[string]string, masked map[string]struct{}) []string {
	names := make(map[string]string, len(b.dashed))
	for _, k := range b.dashed {
		if _, gone := masked[k]; !gone {
			names[k] = ""
		}
	}
	for k := range overlay {
		if strings.Contains(k, "-") {
			names[k] = ""
		}
	}
	return dashReplacements(names)
}

// selectVariables is selectVariables on the merged scope: the base is read by lookup (each name, the names above it, the
// names under it), the overlay, which is small, by the same test as before.
func (b *Base) selectVariables(names map[string]void, overlay map[string]string, masked map[string]struct{}) map[string]string {
	above, under := reachSets(names)
	selected := make(map[string]string, len(names)+len(overlay))
	add := func(raw string) {
		if _, shadowed := overlay[raw]; shadowed {
			return
		}
		if _, gone := masked[raw]; !gone {
			selected[raw] = b.vars[raw]
		}
	}
	for name := range names {
		b.each(name, add)
		for anc := range ancestorsOf(name) {
			b.each(anc, add)
		}
		b.eachUnder(name+".", add)
	}
	for k, v := range overlay {
		if isReachable(internalName(k), names, above, under) {
			selected[k] = v
		}
	}
	return selected
}

// ancestorsOf returns the dotted prefixes of name.
func ancestorsOf(name string) map[string]void {
	anc := map[string]void{}
	addAncestors(anc, name)
	return anc
}

// has reports whether a name whose internal spelling is internal is in the base and not masked.
func (b *Base) has(internal string, masked map[string]struct{}) bool {
	found := false
	b.each(internal, func(raw string) {
		if _, gone := masked[raw]; !gone {
			found = true
		}
	})
	return found
}

// each calls f with every base name whose internal spelling is internal (two names can share one: "a-b" and "aµµµb").
func (b *Base) each(internal string, f func(raw string)) {
	for i := sort.SearchStrings(b.internal, internal); i < len(b.internal) && b.internal[i] == internal; i++ {
		f(b.raw[i])
	}
}

// eachUnder calls f with every base name whose internal spelling starts with prefix.
func (b *Base) eachUnder(prefix string, f func(raw string)) {
	for i := sort.SearchStrings(b.internal, prefix); i < len(b.internal) && strings.HasPrefix(b.internal[i], prefix); i++ {
		f(b.raw[i])
	}
}
