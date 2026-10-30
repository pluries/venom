package interpolate

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	splitBase      = "base"
	splitOverlay   = "overlay"
	splitAlternate = "alternate"
	splitShadow    = "shadow"
	splitMsg       = "input %q, split %s"
)

// splitScope lays scope out over a base and an overlay in one of the ways a step can: everything in the base, everything
// in the overlay, the names alternating, and an overlay that shadows base values which differ. Whatever the split, the
// base laid under the overlay is scope again.
func splitScope(scope map[string]string, how string) (base, overlay map[string]string) {
	keys := make([]string, 0, len(scope))
	for k := range scope {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	base, overlay = map[string]string{}, map[string]string{}
	for i, k := range keys {
		switch {
		case how == splitBase:
			base[k] = scope[k]
		case how == splitOverlay:
			overlay[k] = scope[k]
		case how == splitAlternate && i%2 == 0:
			base[k] = scope[k]
		case how == splitAlternate:
			overlay[k] = scope[k]
		case how == splitShadow && i%2 == 0:
			base[k] = "stale " + scope[k]
			overlay[k] = scope[k]
		default:
			base[k] = scope[k]
		}
	}
	return base, overlay
}

var splitHows = []string{splitBase, splitOverlay, splitAlternate, splitShadow}

// TestDoOverMatchesDo checks that DoOver gives the output and the error of Do on the merged scope (#2475), for every
// input of the differential test and every way to split the scope between the base and the overlay.
func TestDoOverMatchesDo(t *testing.T) {
	for _, scope := range differentialScopes() {
		for _, how := range splitHows {
			base, overlay := splitScope(scope, how)
			b := NewBase(base)
			for _, in := range differentialInputs {
				want, wantErr := Do(in, scope)
				got, gotErr := DoOver(in, b, overlay)
				if wantErr != nil {
					require.EqualError(t, gotErr, wantErr.Error(), splitMsg, in, how)
				} else {
					require.NoError(t, gotErr, splitMsg, in, how)
				}
				require.Equal(t, want, got, "input %q, split %s, %d variables", in, how, len(scope))
			}
		}
	}
}

// TestUnknownVariablesOverMatchesUnknownVariables is the same check for UnknownVariablesOver.
func TestUnknownVariablesOverMatchesUnknownVariables(t *testing.T) {
	inputs := append([]string{}, differentialInputs...)
	for _, c := range unknownVariablesCases {
		inputs = append(inputs, c.input)
	}
	scopes := append(differentialScopes(), unknownVariablesVars)
	for _, scope := range scopes {
		for _, how := range splitHows {
			base, overlay := splitScope(scope, how)
			b := NewBase(base)
			for _, in := range inputs {
				want, wantErr := UnknownVariables(in, scope)
				got, gotErr := UnknownVariablesOver(in, b, overlay)
				require.Equal(t, wantErr == nil, gotErr == nil, splitMsg, in, how)
				require.Equal(t, want, got, splitMsg, in, how)
			}
		}
	}
}

// maskedScope returns the base without the masked names laid under the overlay, which Do is the reference on, and the
// masked names: every third name of the base, the ones the overlay also holds included (there the overlay wins).
func maskedScope(base, overlay map[string]string) (merged map[string]string, masked map[string]struct{}) {
	keys := make([]string, 0, len(base))
	for k := range base {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	masked = map[string]struct{}{}
	for i := 0; i < len(keys); i += 3 {
		masked[keys[i]] = struct{}{}
	}
	merged = map[string]string{}
	for k, v := range base {
		if _, gone := masked[k]; !gone {
			merged[k] = v
		}
	}
	for k, v := range overlay {
		merged[k] = v
	}
	return merged, masked
}

// TestOverMaskedMatchesTheScopeWithoutTheMaskedNames checks DoOverMasked and UnknownVariablesOverMasked against Do and
// UnknownVariables on the merged scope from which the masked names are gone (#2475): output, error text and unknown names.
func TestOverMaskedMatchesTheScopeWithoutTheMaskedNames(t *testing.T) {
	inputs := append([]string{}, differentialInputs...)
	for _, c := range unknownVariablesCases {
		inputs = append(inputs, c.input)
	}
	scopes := append(differentialScopes(), unknownVariablesVars)
	for _, scope := range scopes {
		for _, how := range splitHows {
			base, overlay := splitScope(scope, how)
			merged, masked := maskedScope(base, overlay)
			b := NewBase(base)
			for _, in := range inputs {
				want, wantErr := Do(in, merged)
				got, gotErr := DoOverMasked(in, b, overlay, masked)
				require.Equal(t, wantErr == nil, gotErr == nil, splitMsg, in, how)
				if wantErr != nil {
					require.EqualError(t, gotErr, wantErr.Error(), splitMsg, in, how)
				}
				require.Equal(t, want, got, splitMsg, in, how)

				wantUnknown, err := UnknownVariables(in, merged)
				require.NoError(t, err)
				gotUnknown, err := UnknownVariablesOverMasked(in, b, overlay, masked)
				require.NoError(t, err)
				require.Equal(t, wantUnknown, gotUnknown, splitMsg, in, how)
			}
		}
	}
}

// TestOverMaskedHidesWhatTheStepReplaced pins the case the engine has: a suite string replaced by a value that flattens
// to other names, so the overlay does not hold the masked name itself.
func TestOverMaskedHidesWhatTheStepReplaced(t *testing.T) {
	const plain, dashed = "plain", "my-plain"
	base := NewBase(map[string]string{plain: "suite", dashed: "dashed", "keep": "k"})
	overlay := map[string]string{plain + ".x": "1"}
	masked := map[string]struct{}{plain: {}, dashed: {}}
	got, err := DoOverMasked("{{.plain.x}} {{.keep}} {{.my-plain}} {{.plain}}", base, overlay, masked)
	require.NoError(t, err)
	assert.Equal(t, "1 k {{.my-plain}} {{.plain}}", got)
	unknown, err := UnknownVariablesOverMasked("{{.plain}} {{.my-plain}} {{.keep}}", base, overlay, masked)
	require.NoError(t, err)
	assert.Equal(t, []string{dashed, plain}, unknown)
	// the same reads without the mask see the suite's strings
	got, err = DoOver("{{.plain}} {{.my-plain}}", base, overlay)
	require.NoError(t, err)
	assert.Equal(t, "suite dashed", got)
}

// TestDoOverLayers pins how the overlay and the base combine where a name has relatives in the other layer.
func TestDoOverLayers(t *testing.T) {
	tests := []struct {
		name    string
		base    map[string]string
		overlay map[string]string
		input   string
		want    string
	}{
		{name: "the overlay shadows a base name", base: map[string]string{"a": "base"}, overlay: map[string]string{"a": "over"}, input: "{{.a}}", want: "over"},
		{name: "the overlay adds a child under a base name", base: map[string]string{"a": "p"}, overlay: map[string]string{"a.b": "c"}, input: "{{.a.b}} {{.a}}", want: "c p"},
		{name: "the base holds the child of an overlay name", base: map[string]string{"a.b": "c"}, overlay: map[string]string{"a": "p"}, input: "{{.a.b}} {{.a}}", want: "c p"},
		{name: "a dashed name of the base", base: map[string]string{"my-key": "v"}, overlay: map[string]string{"other": "o"}, input: "{{.my-key}}{{.other}}", want: "vo"},
		{name: "a dashed name of the overlay", base: map[string]string{"other": "o"}, overlay: map[string]string{"my-key": "v"}, input: "{{.my-key}}{{.other}} {{.my-key | upper}}", want: "vo V"},
		{name: "a dashed name under a read name", base: map[string]string{"cfg.my-key": "v"}, overlay: nil, input: "{{.cfg.my-key}}", want: "v"},
		{name: "a name above a read name", base: map[string]string{"x": "root", "x.y": "mid"}, overlay: map[string]string{"x.y.z": "deep"}, input: "{{.x}} {{.x.y}} {{.x.y.z}}", want: "root mid deep"},
		{name: "an unrelated base name is left alone", base: map[string]string{"zz": "1", "aa": "2", "a.b": "3"}, overlay: nil, input: "{{.a.b}}", want: "3"},
		{name: "a name the template does not read is left as written", base: map[string]string{"a": "1"}, overlay: nil, input: "{{.u}}", want: "{{.u}}"},
		{name: "the whole data is read: the base and the overlay both count", base: map[string]string{"a": "1"}, overlay: map[string]string{"b": "2"}, input: "{{ len $ }}", want: "2"},
		{name: "a quoted token is looked up like a variable", base: map[string]string{"b": "1"}, overlay: nil, input: `{{ "x .b " | upper }}`, want: "X .B "},
		{name: "default on a name only the overlay holds", base: nil, overlay: map[string]string{"a": "1"}, input: `{{.a | default "d"}}{{.u | default "d"}}`, want: "1d"},
		{name: "an empty base and a nil overlay", base: map[string]string{}, overlay: nil, input: "{{.a}}", want: "{{.a}}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DoOver(tt.input, NewBase(tt.base), tt.overlay)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			merged := map[string]string{}
			for k, v := range tt.base {
				merged[k] = v
			}
			for k, v := range tt.overlay {
				merged[k] = v
			}
			viaDo, err := Do(tt.input, merged)
			require.NoError(t, err)
			assert.Equal(t, viaDo, got, "Do on the merged scope")
		})
	}
}

// TestDoOverReturnsTheErrorsOfDo covers a template that does not parse and one that fails to run.
func TestDoOverReturnsTheErrorsOfDo(t *testing.T) {
	b := NewBase(map[string]string{"a": "x"})
	_, err := DoOver("a {{ b", b, nil)
	assert.EqualError(t, err, `invalid template format "a {{ b": template: input:1: function "b" not defined`)
	_, err = DoOver("{{ .a | trunc }}", b, nil)
	assert.ErrorContains(t, err, "failed to execute template")
}

// TestNewBaseDoesNotChangeItsInput pins that building a Base and reading through it leave the map as it was, and that the
// overlay is not written to either.
func TestNewBaseDoesNotChangeItsInput(t *testing.T) {
	vars := map[string]string{"a-b": "1", "c": "2"}
	overlay := map[string]string{"c": "3"}
	_, err := DoOver("{{.a-b}}{{.c}}", NewBase(vars), overlay)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a-b": "1", "c": "2"}, vars)
	assert.Equal(t, map[string]string{"c": "3"}, overlay)
}

// largeScope returns the scope of a suite that declares one large nested variable: items * fields names like
// body.items.item7.field3.
func largeScope(items, fields int) map[string]string {
	scope := map[string]string{"small": "s", "venom.testcase": "tc", "my-dash": "d"}
	for i := 0; i < items; i++ {
		for f := 0; f < fields; f++ {
			scope[fmt.Sprintf("body.items.item%d.field%d", i, f)] = "value"
		}
	}
	return scope
}

const largeInput = "{{.small}} {{.body.items.item7.field3}} {{.my-dash | upper}}"

// BenchmarkDoLargeScope and BenchmarkDoOverLargeScope read 3 variables out of 27 000: Do pays for every name on each
// call, DoOver for the 3 reads and the overlay.
func BenchmarkDoLargeScope(b *testing.B) {
	scope := largeScope(2700, 10)
	scope["step.input"] = "i"
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if _, err := Do(largeInput, scope); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDoOverLargeScope(b *testing.B) {
	base := NewBase(largeScope(2700, 10))
	overlay := map[string]string{"step.input": "i"}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		if _, err := DoOver(largeInput, base, overlay); err != nil {
			b.Fatal(err)
		}
	}
}

// sortedPairs lists the names of a base as "internal name, raw name", sorted: the content of its index, whatever order
// names with the same internal spelling were put in.
func sortedPairs(b *Base) []string {
	pairs := make([]string, len(b.internal))
	for i := range b.internal {
		pairs[i] = b.internal[i] + "\x00" + b.raw[i]
	}
	sort.Strings(pairs)
	return pairs
}

// withExtras returns scope with the names a derived scope may add, one of them with a "-".
func withExtras(scope map[string]string) map[string]string {
	vars := map[string]string{"__len__": "3", "__type__": "Map", "extra-dashed": "d", "aaa": "a"}
	for k, v := range scope {
		vars[k] = v
	}
	return vars
}

// TestWithVarsMatchesNewBase checks that a base derived with WithVars has the index NewBase builds and gives the same
// answers, for every scope of the differential test with and without extra names (#2496).
func TestWithVarsMatchesNewBase(t *testing.T) {
	inputs := append([]string{}, differentialInputs...)
	for _, c := range unknownVariablesCases {
		inputs = append(inputs, c.input)
	}
	for _, scope := range differentialScopes() {
		for _, vars := range []map[string]string{scope, withExtras(scope)} {
			derived := NewBase(scope).WithVars(vars)
			require.NotNil(t, derived, "%d variables", len(vars))
			fresh := NewBase(vars)
			require.Equal(t, sortedPairs(fresh), sortedPairs(derived))
			require.Equal(t, fresh.dashed, derived.dashed)
			for _, in := range inputs {
				want, wantErr := DoOver(in, fresh, nil)
				got, gotErr := DoOver(in, derived, nil)
				require.Equal(t, wantErr == nil, gotErr == nil, "input %q", in)
				require.Equal(t, want, got, "input %q", in)
				wantNames, wantNamesErr := UnknownVariablesOver(in, fresh, nil)
				gotNames, gotNamesErr := UnknownVariablesOver(in, derived, nil)
				require.Equal(t, wantNamesErr == nil, gotNamesErr == nil, "input %q", in)
				require.Equal(t, wantNames, gotNames, "input %q", in)
			}
		}
	}
}

// TestWithVarsFallsBack checks the cases WithVars refuses: a name of the base that vars lacks, a renamed name, and more
// extra names than it merges.
func TestWithVarsFallsBack(t *testing.T) {
	base := NewBase(map[string]string{"a": "1", "b": "2"})
	assert.Nil(t, base.WithVars(map[string]string{"a": "1"}), "a name missing")
	assert.Nil(t, base.WithVars(map[string]string{"a": "1", "c": "3"}), "a name renamed")
	assert.Nil(t, base.WithVars(map[string]string{"A": "1", "B": "2"}), "names that differ in case")
	many := map[string]string{"a": "1", "b": "2"}
	for i := 0; i <= maxSharedExtras; i++ {
		many[fmt.Sprintf("extra%d", i)] = "x"
	}
	assert.Nil(t, base.WithVars(many), "more extras than maxSharedExtras")
	var none *Base
	assert.Nil(t, none.WithVars(map[string]string{"a": "1"}), "a nil base")
	assert.NotNil(t, NewBase(nil).WithVars(map[string]string{"__len__": "0"}), "an empty base takes extras")
}

// TestWithVarsDoesNotChangeTheBase checks that deriving leaves the receiver, which other steps read, as it was.
func TestWithVarsDoesNotChangeTheBase(t *testing.T) {
	scope := largeScope(5, 5)
	// with the one largeScope has, five dashed names that sort after the extra one: their list has len 5 and cap 8, so an
	// append that is not forced to copy would write into the receiver's array and the sort would reorder it
	for _, k := range []string{"my-a", "my-b", "my-c", "my-d"} {
		scope[k] = "d"
	}
	base := NewBase(scope)
	before := sortedPairs(base)
	dashedBefore := append([]string{}, base.dashed...)
	vars := withExtras(scope)
	derived := base.WithVars(vars)
	require.NotNil(t, derived)
	assert.Equal(t, before, sortedPairs(base))
	assert.Equal(t, dashedBefore, base.dashed)
	assert.Len(t, base.vars, len(scope))
	assert.Len(t, derived.vars, len(vars))
}
