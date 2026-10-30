package venom

import (
	"maps"
	"reflect"
	"strings"
	"sync"

	"github.com/ovh/venom/interpolate"
)

// A suite that declares one large variable reaches every test case with thousands of strings: initTestSuiteVars keeps
// the nested value and adds one string per flattened key next to it (#2405). No step changes them, yet each step of a
// custom executor, at every level of nesting, flattened the whole scope, interpolated every value and escaped the quotes
// again (#2475). suiteScope does that work once per root test case, and a step then handles only the variables that are
// not the suite's own.
//
// Everything here is an economy, never a different answer: whenever the prepared scope cannot be proved to give what the
// full path gives, the caller takes the full path, which is unchanged.

// suiteScope is the prepared form of the variables of a suite, shared by a root test case and the inner test cases of
// the custom executors it runs, as TestSuiteVars is. It is never shared between test files, so it needs no lock of its
// own beyond the Once that builds it on first use (a step that timed out can still be running when the next one starts).
type suiteScope struct {
	src H

	flatOnce sync.Once
	flat     *preparedSuite

	lowerOnce sync.Once
	lower     *scopeLayers
}

// newSuiteScope returns the prepared scope of src, which it holds and never changes.
func newSuiteScope(src H) *suiteScope {
	return &suiteScope{src: src}
}

// preparedSuite is what a step needs of the suite: the variables as a step interpolates them.
type preparedSuite struct {
	// values are the suite's variables as they were set, to tell which variable of a step is the suite's own.
	values H
	// flat is the flattened scope of the suite, without the whole-map aggregates.
	flat map[string]string
	// escaped holds the entries of flat that change when their quotes are escaped for the JSON of a step.
	escaped map[string]string
	// view is flat with those entries escaped, the scope a step interpolates with, and base its index. Both are built
	// once and only read afterwards, by every step of the suite (#2475).
	view map[string]string
	base *interpolate.Base
}

// prepared returns the suite's prepared scope, or nil when it is not worth having or cannot be trusted.
func (s *suiteScope) prepared() *preparedSuite {
	if s == nil {
		return nil
	}
	s.flatOnce.Do(func() { s.flat = prepareSuite(s.src) })
	return s.flat
}

// prepareSuite flattens src once. It returns nil when a step could not use the result: no variable, a variable named like
// a whole-map aggregate, a value a step would interpolate (it can read step variables), two non-string variables whose
// flattened keys may meet (go-dump then lets its map order decide, which a copy cannot reproduce), or keys that meet with
// different values.
func prepareSuite(src H) *preparedSuite {
	if len(src) == 0 {
		return nil
	}
	strs := make(map[string]string, len(src))
	others := H{}
	for k, v := range src {
		if isAggregate(k) {
			// initTestSuiteVars stores the aggregates of its own dump among the variables. The step's dump
			// overwrites them, as dumpScope does, so they are recognised as the suite's and not copied.
			if !aggregateShadowed(k, v) {
				return nil
			}
			continue
		}
		if s, ok := v.(string); ok && plainKey(k) {
			strs[k] = s
			continue
		}
		others[k] = v
	}
	if !mergeOthers(strs, others) {
		return nil
	}
	p := &preparedSuite{values: src, flat: strs}
	return p.splitEscaped()
}

func isAggregate(name string) bool {
	return name == dumpLenKey || name == dumpTypeKey
}

// aggregateShadowed reports whether a variable named like an aggregate leaves the dump as the aggregate has it: the
// length is overwritten whatever it holds, the type is the one go-dump gives a map (classify, dumpfast.go).
func aggregateShadowed(name string, v interface{}) bool {
	s, ok := v.(string)
	return ok && (name == dumpLenKey || s == dumpTypeMap)
}

// mergeOthers adds the flattened non-string variables to strs, and reports false when the result is not what a dump of
// the whole scope gives for certain.
func mergeOthers(strs map[string]string, others H) bool {
	if len(others) == 0 {
		return true
	}
	for k := range others {
		if sharesPrefixWithAnotherName(k, others) {
			return false
		}
	}
	part, err := dumpHPreserveCase(others)
	if err != nil {
		return false
	}
	for k, v := range part {
		if isAggregate(k) {
			continue
		}
		if old, taken := strs[k]; taken && old != v {
			return false
		}
		strs[k] = v
	}
	return true
}

// splitEscaped records the escaped form of the values that need one, and reports nil when a value holds a template.
func (p *preparedSuite) splitEscaped() *preparedSuite {
	if !p.recordEscaped() {
		return nil
	}
	p.view = p.flat
	if p.escaped != nil {
		p.view = make(map[string]string, len(p.flat))
		maps.Copy(p.view, p.flat)
		maps.Copy(p.view, p.escaped)
	}
	p.base = interpolate.NewBase(p.view)
	return p
}

// recordEscaped fills p.escaped, and reports false when a value holds a template.
func (p *preparedSuite) recordEscaped() bool {
	for k, v := range p.flat {
		if strings.Contains(v, "{{") {
			return false
		}
		if strings.Contains(v, `"`) {
			if p.escaped == nil {
				p.escaped = map[string]string{}
			}
			p.escaped[k] = escapeForJSON(v)
		}
	}
	return true
}

// sameValue reports whether a step variable holds the very value the suite set: the same string, the same map, slice or
// pointer as varsFlattener tells them apart, or an equal scalar. Anything else is treated as the step's own.
func sameValue(suiteValue, stepValue interface{}) bool {
	if s, ok := suiteValue.(string); ok {
		t, ok := stepValue.(string)
		return ok && s == t
	}
	if id, ok := identify(suiteValue); ok {
		other, ok := identify(stepValue)
		return ok && id == other
	}
	return isScalar(suiteValue) && suiteValue == stepValue
}

func isScalar(v interface{}) bool {
	if v == nil {
		return false
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// stepSplit is the scope of a step seen from the suite.
type stepSplit struct {
	// rest holds the variables that are not the suite's own, those that replace one of its strings included.
	rest H
	// replaced names the suite's strings that a step variable replaces: the prepared scope must not keep them.
	replaced []string
}

// outsideSuite splits stepVars. It reports false when the scope is not the suite's plus others: a suite variable is
// missing, or a step variable holds a name of the suite with another value, and the suite's is not a plain string.
func (p *preparedSuite) outsideSuite(stepVars H) (stepSplit, bool) {
	sp := stepSplit{rest: H{}}
	known := 0
	for k, v := range stepVars {
		inSuite, ok := p.place(&sp, k, v)
		if !ok {
			return sp, false
		}
		if inSuite {
			known++
		}
	}
	return sp, known == len(p.values)
}

// outsideOverlay is outsideSuite for a scope that holds only what is added to the suite's variables, which are implied:
// there is no count of them to check.
func (p *preparedSuite) outsideOverlay(overlay H) (stepSplit, bool) {
	sp := stepSplit{rest: H{}}
	for k, v := range overlay {
		if _, ok := p.place(&sp, k, v); !ok {
			return sp, false
		}
	}
	return sp, true
}

// place files one variable of a step: outside the suite, or the suite's own (the same value, which the prepared scope
// holds), or replacing a plain suite string. It reports whether the name is the suite's, and false when the variable
// cannot be laid over the prepared scope.
func (p *preparedSuite) place(sp *stepSplit, k string, v interface{}) (inSuite, ok bool) {
	sv, inSuite := p.values[k]
	switch {
	case !inSuite:
		sp.rest[k] = v
	case sameValue(sv, v):
	case isPlainString(k, sv):
		sp.rest[k] = v
		sp.replaced = append(sp.replaced, k)
	default:
		return true, false
	}
	return inSuite, true
}

// isPlainString reports whether the suite's variable is a string the prepared scope holds under its own name.
func isPlainString(name string, v interface{}) bool {
	_, ok := v.(string)
	return ok && plainKey(name) && !isAggregate(name)
}

// stepVars returns what rangedStepVars computes for stepVars, from the prepared scope, as layers over it. It reports false
// when the full path must be taken instead: the scope is not the suite's, a variable of the step is named like an
// aggregate, is flattened to keys the suite also has with another value, or holds a template.
func (s *suiteScope) stepVars(tc *TestCase, stepVars H) (*stepScope, bool) {
	p := s.prepared()
	if p == nil {
		return nil, false
	}
	sp, ok := p.outsideSuite(stepVars)
	if !ok {
		return nil, false
	}
	restFlat, ok := p.flattenRest(tc, sp)
	if !ok {
		return nil, false
	}
	return p.layer(restFlat, sp.replaced, len(stepVars)), true
}

// stepVarsOver is stepVars for a test case that holds only its own variables: overlay is what a step adds to the suite's
// variables, which are implied (TestCase.varsOverSuite). It gives what stepVars gives for the whole map, and reports
// false in the same cases, so that the caller then takes the full path on the whole map (TestCase.fullVars).
func (s *suiteScope) stepVarsOver(tc *TestCase, overlay H) (*stepScope, bool) {
	p := s.prepared()
	if p == nil {
		return nil, false
	}
	sp, ok := p.outsideOverlay(overlay)
	if !ok {
		return nil, false
	}
	restFlat, ok := p.flattenRest(tc, sp)
	if !ok {
		return nil, false
	}
	// the whole map holds the suite's variables and the overlay's names that are not the suite's
	return p.layer(restFlat, sp.replaced, len(p.values)+len(sp.rest)-len(sp.replaced)), true
}

// under returns overlay laid over the suite's variables, as one new map: the whole scope of a test case that holds only
// its own variables. A nil scope has nothing under the overlay.
func (s *suiteScope) under(overlay H) H {
	if s == nil {
		return overlay
	}
	// Clone copies a map in bulk, several times faster than inserting its entries one by one
	whole := s.src.Clone()
	maps.Copy(whole, overlay)
	return whole
}

// flattenRest flattens the variables that are not the suite's, and reports false when they cannot be laid over the
// prepared scope as they are.
func (p *preparedSuite) flattenRest(tc *TestCase, sp stepSplit) (map[string]string, bool) {
	for k := range sp.rest {
		if isAggregate(k) {
			return nil, false
		}
	}
	restFlat, err := tc.varsFlattener.flatten(sp.rest)
	if err != nil {
		return nil, false
	}
	if p.clashes(restFlat, sp.replaced) {
		return nil, false
	}
	return restFlat, true
}

// lowerLayers returns the dump of vrs that processUserExecutorInputs starts from, as an empty overlay over the dump of the
// suite's variables when vrs is the suite's variables as they were set (RunUserExecutor clones them), nil otherwise. The
// base is shared and never written to, so the caller's overlay is its own.
func (s *suiteScope) lowerLayers(vrs H) *scopeLayers {
	if s == nil || len(vrs) != len(s.src) {
		return nil
	}
	return s.lowerOver()
}

// lowerOver is lowerLayers for the variables of a custom executor that are held over the suite's: the overlay is empty
// until the inputs are added. It is nil when the suite's variables cannot be dumped.
func (s *suiteScope) lowerOver() *scopeLayers {
	if s == nil {
		return nil
	}
	s.lowerOnce.Do(func() {
		if dump, err := dumpHLowerFirst(s.src); err == nil {
			s.lower = &scopeLayers{baseVars: dump, base: s.indexOf(dump)}
		}
	})
	if s.lower == nil {
		return nil
	}
	return &scopeLayers{baseVars: s.lower.baseVars, base: s.lower.base, overlay: map[string]string{}}
}

// indexOf returns the index of dump, the dump of the suite's variables with lower-cased names. It shares the sorted names
// of the prepared scope's index when dump has the same names, plus the aggregates (#2496): sorting 27 000 names a second
// time was half of the cost of this build. Names that differ in case (a top-level variable with a capital) take NewBase.
func (s *suiteScope) indexOf(dump map[string]string) *interpolate.Base {
	if p := s.prepared(); p != nil {
		if base := p.base.WithVars(dump); base != nil {
			return base
		}
	}
	return interpolate.NewBase(dump)
}

// clashes reports whether the flattened variables of a step hold a template, or a key the suite also has with another
// value. The keys of the suite strings the step replaced do not count: they are dropped.
func (p *preparedSuite) clashes(restFlat map[string]string, replaced []string) bool {
	dropped := make(map[string]struct{}, len(replaced))
	for _, k := range replaced {
		dropped[k] = struct{}{}
	}
	for k, v := range restFlat {
		if isAggregate(k) {
			continue
		}
		if strings.Contains(v, "{{") {
			return true
		}
		if _, gone := dropped[k]; gone {
			continue
		}
		if old, taken := p.flat[k]; taken && old != v {
			return true
		}
	}
	return false
}
