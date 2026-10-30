package venom

import (
	"context"
	"reflect"
	"strconv"
	"testing"

	"github.com/ovh/venom/interpolate"
)

// names the fixtures use more than twice (go:S1192)
const (
	plainVar = "plain"
	otherVar = "other"
)

// The prepared suite scope is an economy: the maps a step gets from it must be the ones the full path gives. These tests
// run both over the same scope, the full path being rangedStepVars on a test case that has no prepared scope.

// suiteScopeFixture returns suite variables as a run gives them: initTestSuiteVars keeps each nested value and adds its
// flattened keys as strings.
func suiteScopeFixture(t *testing.T, extra H) H {
	t.Helper()
	vars := H{
		plainVar:    "value",
		"quoted":    `say "hi" \ back`,
		"multiline": "a\nb\tc é",
		"nested":    nestedJSONValue(2, 3),
		"body.code": "200",
	}
	vars.AddAll(extra)
	ts := &TestSuite{Name: "suite", Vars: vars}
	if err := New().initTestSuiteVars(ts); err != nil {
		t.Fatal(err)
	}
	return ts.Vars
}

// suiteStepVars is the scope a step of a test case over suite starts from: the suite's variables and the step's own.
func suiteStepVars(suite H, own H) H {
	vars := suite.Clone()
	vars.Add("venom.testcase", "case")
	vars.Add("venom.teststep.number", 2)
	vars.AddAll(own)
	return vars
}

// stepVarsBoth runs rangedStepVars on stepVars with and without the prepared scope of suite, and fails when the two
// scopes differ in any way a consumer can see (sameStepScope).
func stepVarsBoth(t *testing.T, suite, stepVars H) {
	t.Helper()
	want, wantErr := rangedStepVars(context.Background(), &TestCase{}, &TestStepResult{}, stepVars.Clone(), &Range{}, 0, RangeData{})
	tc := &TestCase{suite: newSuiteScope(suite)}
	got, gotErr := rangedStepVars(context.Background(), tc, &TestStepResult{}, stepVars.Clone(), &Range{}, 0, RangeData{})
	if (wantErr == nil) != (gotErr == nil) {
		t.Fatalf("error: want %v, got %v", wantErr, gotErr)
	}
	if wantErr == nil {
		sameStepScope(t, want, got)
	}
}

func mapDiff(want, got map[string]string) string {
	if len(want) != len(got) {
		return "length " + strconv.Itoa(len(want)) + " against " + strconv.Itoa(len(got))
	}
	for k, w := range want {
		if g, ok := got[k]; !ok || g != w {
			return "key " + k + ": want " + w + ", got " + g
		}
	}
	return "none"
}

// preparedStepVars reports whether the prepared scope of suite answers for stepVars.
func preparedStepVars(suite, stepVars H) bool {
	_, ok := newSuiteScope(suite).stepVars(&TestCase{}, stepVars)
	return ok
}

func TestSuiteScopeMatchesFullPath(t *testing.T) {
	tests := []struct {
		name     string
		suite    H
		own      H
		prepared bool
	}{
		{"suite alone", nil, nil, true},
		{"step variables of every kind", nil, H{
			"previous.var": "x",
			"captured":     map[string]interface{}{"a": "1", "b": map[string]interface{}{"c": `q"uote`}},
			"list":         []interface{}{"a", "b"},
			"count":        3,
			"flag":         true,
		}, true},
		{"step string with quotes", nil, H{"said": `he said "no"`}, true},
		{"step variable reads a suite variable", nil, H{"echo": "{{.plain}} and {{.quoted}}"}, false},
		{"step variable replaces a suite string", nil, H{plainVar: otherVar}, true},
		{"step number replaces a suite string", nil, H{plainVar: 7}, true},
		{"step variable replaces a suite map", H{"cfg": map[string]interface{}{"k": "v"}}, H{"cfg": otherVar}, false},
		{"step map replaces a suite string", nil, H{"quoted": map[string]interface{}{"k": "v"}}, true},
		{"step map next to a suite key, no shared key", nil, H{"body": map[string]interface{}{otherVar: "x"}}, true},
		{"step map shares a key with the suite, same value", nil, H{"body": map[string]interface{}{"code": "200"}}, true},
		{"templated suite value", H{"tpl": "{{.plain}}-x"}, H{"a": "b"}, false},
		{"secret-like suite value", H{"api.password": `p"w\d`}, nil, true},
		{"suite key with a space", H{"a key": "v"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suite := suiteScopeFixture(t, nil)
			// set after initTestSuiteVars, which would resolve a template: a step receives these as they are
			suite.AddAll(tt.suite)
			stepVars := suiteStepVars(suite, tt.own)
			if got := preparedStepVars(suite, stepVars); got != tt.prepared {
				t.Errorf("prepared path taken = %v, want %v", got, tt.prepared)
			}
			stepVarsBoth(t, suite, stepVars)
		})
	}
}

// A step map that gives a key the suite also has, with another value, is left to go-dump, whose map order decides which
// wins: the prepared scope must not answer for it, and the full path is what runs.
func TestSuiteScopeLeavesConflictingKeysToTheFullPath(t *testing.T) {
	suite := suiteScopeFixture(t, nil)
	stepVars := suiteStepVars(suite, H{"body": map[string]interface{}{"code": "500"}})
	if preparedStepVars(suite, stepVars) {
		t.Error("the prepared scope answered for a step map whose key conflicts with the suite")
	}
}

func TestSuiteScopeLeavesAnotherScopeToTheFullPath(t *testing.T) {
	suite := suiteScopeFixture(t, nil)
	stepVars := suiteStepVars(suite, nil)
	delete(stepVars, plainVar)
	if preparedStepVars(suite, stepVars) {
		t.Error("the prepared scope answered for a scope that lacks a suite variable")
	}
	stepVars = suiteStepVars(suite, H{"__Len__": "7"})
	if preparedStepVars(suite, stepVars) {
		t.Error("the prepared scope answered for a variable named like an aggregate")
	}
	var none *suiteScope
	if _, ok := none.stepVars(&TestCase{}, stepVars); ok {
		t.Error("a nil prepared scope answered")
	}
}

func TestSuiteScopeIsUnusableWhenTheSuiteIs(t *testing.T) {
	for name, suite := range map[string]H{
		"empty":                    {},
		"aggregate name":           {"__Type__": "List"},
		"two maps whose keys meet": {"a": map[string]interface{}{"b": "1"}, "a.b": map[string]interface{}{"c": "2"}},
	} {
		if newSuiteScope(suite).prepared() != nil {
			t.Errorf("%s: a scope was prepared", name)
		}
	}
}

// RunUserExecutor starts from a clone of the suite's variables: the prepared dump must stand in for dumping them.
func TestSuiteScopeLowerDumpMatchesProcessUserExecutorInputs(t *testing.T) {
	ux := stepUserExecutor("executor: dump\ninput:\n  foo: default\n  bar: '{{.nested.k0.k0}}'\n  quoted: '{{.plain}}'\nsteps: []\n")
	step := TestStep{"foo": "from step"}
	for name, extra := range map[string]H{plainVar: nil, "templated": {"tpl": "{{.plain}}-x"}} {
		t.Run(name, func(t *testing.T) {
			suite := suiteScopeFixture(t, extra)
			wantVrs, wantDump, err := processUserExecutorInputs(ux, step, suite.Clone(), &UserExecutor{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			gotVrs, gotDump, err := processUserExecutorInputs(ux, step, suite.Clone(), &UserExecutor{}, newSuiteScope(suite))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(wantVrs, gotVrs) {
				t.Error("scopes differ")
			}
			sameLayers(t, wantDump, gotDump)
		})
	}
}

// sameLayers fails when two scopes of the same variables differ in names, values or interpolation.
func sameLayers(t *testing.T, want, got scopeLayers) {
	t.Helper()
	wantMap := want.materialize()
	if gotMap := got.materialize(); !reflect.DeepEqual(wantMap, gotMap) {
		t.Errorf("dumps differ: %s", mapDiff(wantMap, gotMap))
	}
	for _, in := range scopeTemplates(wantMap) {
		wantOut, wantErr := want.do(in)
		gotOut, gotErr := got.do(in)
		if wantOut != gotOut || (wantErr == nil) != (gotErr == nil) {
			t.Errorf("%q: want %q (%v), got %q (%v)", in, wantOut, wantErr, gotOut, gotErr)
		}
	}
}

// The overlay a caller receives is its own: editing it must not reach the next caller's.
func TestSuiteScopeLowerLayersAreACopy(t *testing.T) {
	suite := suiteScopeFixture(t, nil)
	s := newSuiteScope(suite)
	first := s.lowerLayers(suite.Clone())
	first.overlay[plainVar] = "changed"
	second := s.lowerLayers(suite.Clone())
	if got, _ := second.get(plainVar); got != "value" {
		t.Errorf("the dump was shared: %q", got)
	}
	if s.lowerLayers(H{otherVar: "x"}) != nil {
		t.Error("a dump was given for a scope that is not the suite's")
	}
}

// TestLowerOverIndexMatchesNewBase checks that the index of the lower dump, derived from the prepared scope's when the names
// allow it and built with NewBase when they do not, answers like NewBase on the dump (#2496).
func TestLowerOverIndexMatchesNewBase(t *testing.T) {
	lowerCased := H{"url": "http://localhost", "token": "t", "nested": map[string]interface{}{"a": "b"}}
	capital := H{"Token": "t", "URL": "http://localhost", "name": "n"}
	for name, vars := range map[string]H{"large": realisticStepVars(nestedJSONValue(3, 4)), "lower-case names": lowerCased, "capital names": capital} {
		t.Run(name, func(t *testing.T) {
			ts := &TestSuite{Name: "suite", Vars: vars}
			if err := New().initTestSuiteVars(ts); err != nil {
				t.Fatal(err)
			}
			layers := newSuiteScope(ts.Vars).lowerOver()
			if layers == nil {
				t.Fatal("the suite variables cannot be dumped")
			}
			requireSameAnswers(t, layers.baseVars, layers.base, interpolate.NewBase(layers.baseVars))
		})
	}
}

// requireSameAnswers fails the test when got answers differently from want on a template that reads each variable of vars.
func requireSameAnswers(t *testing.T, vars map[string]string, got, want *interpolate.Base) {
	t.Helper()
	for k := range vars {
		in := "{{." + k + "}} and {{.missing}}"
		wantOut, wantErr := interpolate.DoOver(in, want, nil)
		gotOut, gotErr := interpolate.DoOver(in, got, nil)
		if (wantErr == nil) != (gotErr == nil) || wantOut != gotOut {
			t.Fatalf("input %q: got %q, %v; want %q, %v", in, gotOut, gotErr, wantOut, wantErr)
		}
	}
}
