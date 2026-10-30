package venom

import (
	"maps"
	"strconv"
	"strings"

	"github.com/ovh/venom/interpolate"
)

// A step used to hold its scope as one map of every variable of the suite plus its own: for a suite that declares a
// large nested variable, 27 000 entries cloned, scanned and flattened again by each step (#2475, ADR-0011). stepScope
// holds the prepared suite, shared and never copied, and a small overlay of what the step adds or replaces. The whole map
// is built only where a caller needs one.

// scopeLayers is a variable scope seen as a shared base under an overlay, minus the base names the step replaced. The
// zero value is an empty scope; a scope without a base is the overlay alone.
type scopeLayers struct {
	// baseVars is shared between steps and test files of one suite: it is never written to once built.
	baseVars map[string]string
	// base indexes baseVars for interpolation. It is nil for a scope that is never interpolated, or has no base.
	base *interpolate.Base
	// overlay is the step's own, and wins over the base.
	overlay map[string]string
	// masked names the base entries that no longer exist: a suite string the step replaced by a value that flattens to
	// other names (plain replaced by a map is plain.x), which the overlay therefore does not shadow.
	masked map[string]struct{}
}

// get returns the value of a variable.
func (l scopeLayers) get(name string) (string, bool) {
	if v, ok := l.overlay[name]; ok {
		return v, true
	}
	if _, gone := l.masked[name]; gone {
		return "", false
	}
	v, ok := l.baseVars[name]
	return v, ok
}

// names returns the names of the scope, each once, in no particular order.
func (l scopeLayers) names() []string {
	names := make([]string, 0, len(l.baseVars)+len(l.overlay))
	for k := range l.baseVars {
		if _, shadowed := l.overlay[k]; shadowed {
			continue
		}
		if _, gone := l.masked[k]; !gone {
			names = append(names, k)
		}
	}
	for k := range l.overlay {
		names = append(names, k)
	}
	return names
}

// materialize returns the scope as one map. A scope without a base returns its overlay itself, which the scope owns.
func (l scopeLayers) materialize() map[string]string {
	if l.baseVars == nil {
		return l.overlay
	}
	m := maps.Clone(l.baseVars)
	for k := range l.masked {
		delete(m, k)
	}
	maps.Copy(m, l.overlay)
	return m
}

// do is interpolate.Do on the scope.
func (l scopeLayers) do(input string) (string, error) {
	if l.base == nil {
		return interpolate.Do(input, l.materialize())
	}
	return interpolate.DoOverMasked(input, l.base, l.overlay, l.masked)
}

// unknown is interpolate.UnknownVariables on the scope.
func (l scopeLayers) unknown(input string) ([]string, error) {
	if l.base == nil {
		return interpolate.UnknownVariables(input, l.materialize())
	}
	return interpolate.UnknownVariablesOverMasked(input, l.base, l.overlay, l.masked)
}

// stepScope is the scope of one step: its variables as a step interpolates them (the double quotes escaped for the JSON
// of the step), and as they were flattened, before that. When the prepared suite cannot answer for the step, both views
// are plain maps and the old computation stands behind them, so a consumer has one way to read either.
type stepScope struct {
	escaped scopeLayers
	flat    scopeLayers
}

// newWholeScope returns the scope that is exactly the two maps, which it holds: escaped is what a step interpolates
// with, flat what it was before the quotes were escaped (the same map when the caller does not tell them apart).
func newWholeScope(escaped, flat map[string]string) *stepScope {
	return &stepScope{escaped: scopeLayers{overlay: escaped}, flat: scopeLayers{overlay: flat}}
}

// Get returns a variable as it was flattened, the form a step's context and its executor's lookups read.
func (s *stepScope) Get(name string) (string, bool) {
	return s.flat.get(name)
}

// Names returns the names of the variables, each once, in no particular order.
func (s *stepScope) Names() []string {
	return s.flat.names()
}

// Materialize returns the variables the way a step interpolates them, as one map: the whole scope, which is what the
// step result reports. A caller may change it only if the scope is not used afterwards.
func (s *stepScope) Materialize() map[string]string {
	return s.escaped.materialize()
}

// inputLayers returns the variables the way a step interpolates them, as layers, for a step result to build its InputVars
// from when it is reported. The copy holds the escaped layers alone, so that the result does not keep the flattened ones.
func (s *stepScope) inputLayers() *scopeLayers {
	l := s.escaped
	return &l
}

// MaterializeFlat returns the flattened variables, before their quotes were escaped, as one map.
func (s *stepScope) MaterializeFlat() map[string]string {
	return s.flat.materialize()
}

// Interpolate substitutes the variables of the step in input.
func (s *stepScope) Interpolate(input string) (string, error) {
	return s.escaped.do(input)
}

// UnknownVariables returns the variables input reads that the step does not hold.
func (s *stepScope) UnknownVariables(input string) ([]string, error) {
	return s.escaped.unknown(input)
}

// layer lays the flattened variables of the step over the prepared suite, without the strings the step replaced. count is
// the number of variables of the step, which the aggregates report.
func (p *preparedSuite) layer(restFlat map[string]string, replaced []string, count int) *stepScope {
	overlay := make(map[string]string, len(restFlat)+2)
	flat := make(map[string]string, len(restFlat)+2)
	for k, v := range restFlat {
		flat[k] = v
		if strings.Contains(v, `"`) {
			v = escapeForJSON(v)
		}
		overlay[k] = v
	}
	n := strconv.Itoa(count)
	flat[dumpLenKey], overlay[dumpLenKey] = n, n
	flat[dumpTypeKey], overlay[dumpTypeKey] = dumpTypeMap, dumpTypeMap

	var masked map[string]struct{}
	if len(replaced) > 0 {
		masked = make(map[string]struct{}, len(replaced))
		for _, k := range replaced {
			masked[k] = struct{}{}
		}
	}
	return &stepScope{
		escaped: scopeLayers{baseVars: p.view, base: p.base, overlay: overlay, masked: masked},
		flat:    scopeLayers{baseVars: p.flat, overlay: flat, masked: masked},
	}
}
