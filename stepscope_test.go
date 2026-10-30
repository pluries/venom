package venom

import (
	"context"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"github.com/spf13/cast"
)

// The layered scope of a step is an economy, never a different answer (#2475): whatever the prepared suite gives, a
// consumer must read what it read from the two maps of the full path. These tests compare the two over the scopes of
// TestSuiteScopeMatchesFullPath (suitescope_test.go).

// the name the fixtures add to a step more than twice (go:S1192)
const extraVar = "extra"

var readableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

// scopeTemplates returns the templates a step can hold over vars: each name it can read as a field, and the reads that
// are special: a missing name, a name under a string, a default, a name with a dash.
func scopeTemplates(vars map[string]string) []string {
	templates := []string{
		"no template", "{{.absent}}", "{{.plain.x}}", "{{.quoted.k}}", `{{.absent | default "d"}}`,
		"{{.plain}} {{.quoted}} {{.other}}", "{{.nested}}", "{{.venom.teststep.number}} {{.venom.testcase}}",
		"{{.my-key}}", "{{.__Len__}} {{.__Type__}}", `{"a": "{{.quoted}}", "b": {{.body.code}}}`,
		"{{.cfg}} {{.cfg.k}}", "{{ len $ }}", "{{ .a | upper", "{{.a-key}}",
	}
	for k := range vars {
		if readableName.MatchString(k) {
			templates = append(templates, "{{."+k+"}}")
		}
	}
	sort.Strings(templates)
	return templates
}

// sameStepScope fails when got differs from want, the scope of the full path, in a name, a value, the escaped or the
// flattened form, an interpolation or the unknown variables of a template.
func sameStepScope(t *testing.T, want, got *stepScope) {
	t.Helper()
	wantVars, gotVars := want.Materialize(), got.Materialize()
	if !reflect.DeepEqual(wantVars, gotVars) {
		t.Errorf("vars differ: %s", mapDiff(wantVars, gotVars))
	}
	wantFlat, gotFlat := want.MaterializeFlat(), got.MaterializeFlat()
	if !reflect.DeepEqual(wantFlat, gotFlat) {
		t.Errorf("flat differs: %s", mapDiff(wantFlat, gotFlat))
	}
	sameReads(t, wantFlat, got)
	sameInterpolation(t, want, got, wantVars)
}

// sameReads compares Get and Names with the flattened map of the full path.
func sameReads(t *testing.T, wantFlat map[string]string, got *stepScope) {
	t.Helper()
	for k, w := range wantFlat {
		if g, ok := got.Get(k); !ok || g != w {
			t.Errorf("Get(%q) = %q, %v; want %q", k, g, ok, w)
		}
	}
	if v, ok := got.Get("absent"); ok {
		t.Errorf("Get(absent) = %q, want none", v)
	}
	names := got.Names()
	sort.Strings(names)
	wantNames := make([]string, 0, len(wantFlat))
	for k := range wantFlat {
		wantNames = append(wantNames, k)
	}
	sort.Strings(wantNames)
	if !reflect.DeepEqual(wantNames, names) {
		t.Errorf("names differ: %d against %d", len(wantNames), len(names))
	}
}

// sameInterpolation compares Interpolate and UnknownVariables on the templates over vars.
func sameInterpolation(t *testing.T, want, got *stepScope, vars map[string]string) {
	t.Helper()
	for _, in := range scopeTemplates(vars) {
		wantOut, wantErr := want.Interpolate(in)
		gotOut, gotErr := got.Interpolate(in)
		if wantOut != gotOut || (wantErr == nil) != (gotErr == nil) {
			t.Errorf("Interpolate(%q): want %q (%v), got %q (%v)", in, wantOut, wantErr, gotOut, gotErr)
		}
		wantUnknown, wantErr := want.UnknownVariables(in)
		gotUnknown, gotErr := got.UnknownVariables(in)
		if wantErr != nil || gotErr != nil {
			t.Errorf("UnknownVariables(%q) failed: %v, %v", in, wantErr, gotErr)
		}
		if !reflect.DeepEqual(wantUnknown, gotUnknown) {
			t.Errorf("UnknownVariables(%q): want %v, got %v", in, wantUnknown, gotUnknown)
		}
	}
}

// layeredScope returns the scope the prepared suite gives for stepVars, and fails when it does not answer for them.
func layeredScope(t *testing.T, suite, stepVars H) *stepScope {
	t.Helper()
	scope, ok := newSuiteScope(suite).stepVars(&TestCase{}, stepVars)
	if !ok {
		t.Fatal("the prepared scope did not answer")
	}
	return scope
}

// A string of the suite that the step replaces by a value whose flattened keys do not include its name must not stay
// visible, though no overlay key carries that name: it is the case the masked names are for. go-dump gives a map its own
// key as well, so the layer is built directly.
func TestStepScopeMasksAReplacedSuiteString(t *testing.T) {
	suite := suiteScopeFixture(t, nil)
	scope := newSuiteScope(suite).prepared().layer(map[string]string{plainVar + ".x": "1"}, []string{plainVar}, 3)
	if v, ok := scope.Get(plainVar); ok {
		t.Errorf("the replaced string is still there: %q", v)
	}
	if v, _ := scope.Get(plainVar + ".x"); v != "1" {
		t.Errorf("the replacing map is missing: %q", v)
	}
	if _, ok := scope.Materialize()[plainVar]; ok {
		t.Error("the replaced string is still in the whole map")
	}
	got, err := scope.Interpolate("{{.plain}}{{.plain.x}}")
	if err != nil || got != "{{.plain}}1" {
		t.Errorf("interpolated %q, %v", got, err)
	}
}

// The aggregates describe the whole scope of the step, through Get and the whole map alike, and the step's count wins
// over the suite's own aggregates.
func TestStepScopeAggregates(t *testing.T) {
	suite := suiteScopeFixture(t, nil)
	stepVars := suiteStepVars(suite, H{extraVar: "e"})
	scope := layeredScope(t, suite, stepVars)
	for name, want := range map[string]string{dumpLenKey: strconv.Itoa(len(stepVars)), dumpTypeKey: dumpTypeMap} {
		if got, _ := scope.Get(name); got != want {
			t.Errorf("Get(%s) = %q, want %q", name, got, want)
		}
		if got := scope.Materialize()[name]; got != want {
			t.Errorf("Materialize()[%s] = %q, want %q", name, got, want)
		}
	}
}

// Reading, interpolating and materialising a layered scope leave the prepared suite as it was: steps share it.
func TestStepScopeLeavesThePreparedSuiteAlone(t *testing.T) {
	suite := suiteScopeFixture(t, nil)
	s := newSuiteScope(suite)
	p := s.prepared()
	flatBefore := make(map[string]string, len(p.flat))
	for k, v := range p.flat {
		flatBefore[k] = v
	}
	viewBefore := make(map[string]string, len(p.view))
	for k, v := range p.view {
		viewBefore[k] = v
	}
	scope, ok := s.stepVars(&TestCase{}, suiteStepVars(suite, H{plainVar: otherVar, extraVar: `q"q`}))
	if !ok {
		t.Fatal("the prepared scope did not answer")
	}
	scope.Materialize()[extraVar] = "changed"
	scope.MaterializeFlat()[plainVar] = "changed"
	if _, err := scope.Interpolate("{{.plain}} {{.quoted}}"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(flatBefore, p.flat) || !reflect.DeepEqual(viewBefore, p.view) {
		t.Error("the prepared suite changed")
	}
}

// The context of a step reads the scope through Get and Names, and the plugin lookup reads the workdir through Get.
func TestStepVarsContextOverALayeredScope(t *testing.T) {
	suite := suiteScopeFixture(t, nil)
	scope := layeredScope(t, suite, suiteStepVars(suite, H{plainVar: otherVar, "venom.testsuite.workdir": "/w"}))
	ctx := withStepScope(context.Background(), scope)
	if got := StringVarFromCtx(ctx, plainVar); got != otherVar {
		t.Errorf("var.plain = %q", got)
	}
	if got := StringVarFromCtx(ctx, "venom.testsuite.workdir"); got != "/w" {
		t.Errorf("workdir = %q", got)
	}
	names := cast.ToStringSlice(ctx.Value(ContextKey("vars")))
	sort.Strings(names)
	want := scope.Names()
	sort.Strings(want)
	if !reflect.DeepEqual(names, want) {
		t.Errorf("%d names, want %d", len(names), len(want))
	}
}
