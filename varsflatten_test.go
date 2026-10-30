package venom

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// nestedBody builds a slice-of-maps shaped like a decoded HTTP JSON response body
// (executors/http.Result.BodyJSON is interface{}, filled by json.Unmarshal): breadth items per
// level, depth levels of nesting under "child".
func nestedBody(depth, breadth int) interface{} {
	if depth == 0 {
		return "leaf-value"
	}
	items := make([]interface{}, breadth)
	for i := range items {
		items[i] = map[string]interface{}{
			"id":     i,
			"name":   fmt.Sprintf("item-%d", i),
			"active": i%2 == 0,
			"child":  nestedBody(depth-1, breadth),
		}
	}
	return items
}

// flattenFixtures are the shapes the characterisation tests and the cache tests share: the variable
// scope process_testcase.go:224-230 assembles, with and without a captured response carried forward.
func flattenFixtures() map[string]H {
	return map[string]H{
		"empty": {},
		"scalarsOnly": {
			"venom.testsuite.filepath": "tests/suite.yml",
			"venom.teststep.number":    3,
			"aBool":                    true,
			"aFloat":                   1.5,
		},
		"dottedKeys": {
			// venom's own variables carry dots in the key itself, which the flatten must not
			// confuse with the dots it introduces for nested fields.
			"a.b.c":       "dotted",
			"a.b":         "shorter",
			"http.result": "scalar-despite-the-name",
		},
		"allCacheable": {
			// No scalar at all, so the whole-scope dump the flattener still performs is of an empty
			// map — the aggregates have to survive that.
			"aMap":   map[string]interface{}{"x": 1},
			"aSlice": nestedBody(1, 2),
		},
		"nestedMap": {
			"scalar": "s",
			"body":   map[string]interface{}{"x": 1, "y": map[string]interface{}{"z": "deep"}},
		},
		"sliceOfMaps": {
			"scalar": "s",
			"body":   nestedBody(2, 3),
		},
		"emptyNested": {
			"emptyMap":   map[string]interface{}{},
			"emptySlice": []interface{}{},
			"nilValue":   nil,
		},
	}
}

// TestFlattenCharacterisation pins what DumpStringPreserveCase produces today, before #2017's cache
// is introduced underneath it. It is not a specification of the ideal output — it is a record of the
// current one, so the cache can be proven not to change it (brownfield.md §5).
//
// What it establishes: every key either belongs to a top-level entry (the entry's own name, or that
// name followed by a dot) or is one of exactly two whole-map aggregates, __Len__ and __Type__.
func TestFlattenCharacterisation(t *testing.T) {
	for name, vars := range flattenFixtures() {
		t.Run(name, func(t *testing.T) {
			got, err := DumpStringPreserveCase(vars)
			require.NoError(t, err)

			for k := range got {
				if k == dumpLenKey || k == dumpTypeKey {
					continue
				}
				var owned bool
				for entry := range vars {
					if k == entry || len(k) > len(entry) && k[:len(entry)] == entry && k[len(entry)] == '.' {
						owned = true
						break
					}
				}
				require.True(t, owned, "key %q is neither a known aggregate nor owned by a top-level entry", k)
			}

			require.Equal(t, strconv.Itoa(len(vars)), got[dumpLenKey], "__Len__ is the count of top-level entries")
			require.Equal(t, "Map", got[dumpTypeKey], "__Type__ of the variable scope is always Map")
		})
	}
}

// TestFlattenIsDecomposablePerEntry pins the invariant #2017's cache is built on: the flatten of the
// variable scope is the union of the flattens of its single-entry sub-maps, once the whole-map
// aggregates are restored (a single-entry dump reports __Len__ of 1). If a go-dump upgrade ever adds
// another content-dependent whole-map aggregate, this fails loudly instead of the cache silently
// producing a wrong variable set.
func TestFlattenIsDecomposablePerEntry(t *testing.T) {
	for name, vars := range flattenFixtures() {
		t.Run(name, func(t *testing.T) {
			whole, err := DumpStringPreserveCase(vars)
			require.NoError(t, err)

			merged := map[string]string{}
			for k, v := range vars {
				part, err := DumpStringPreserveCase(H{k: v})
				require.NoError(t, err)
				delete(part, dumpLenKey)
				maps.Copy(merged, part)
			}
			merged[dumpLenKey] = strconv.Itoa(len(vars))
			merged[dumpTypeKey] = "Map"

			require.Equal(t, whole, merged,
				"the per-entry dumps plus the restored aggregates must reproduce the whole-map dump")
		})
	}
}

// lowerFirstKeyFixtures are flattenFixtures plus the two key shapes WithFormatterLowerFirstKey (dump.go) rewrites at
// the top level: a first letter in upper case, which it lowers, and a dot, before which it lowers everything and after
// which it keeps the case. flattenFixtures has dotted keys, but all in lower case, so the formatter leaves them as they
// are.
func lowerFirstKeyFixtures() map[string]H {
	fixtures := flattenFixtures()
	fixtures["upperAndDottedKeys"] = H{
		// "Body" becomes "body", while the keys under it keep their case: the formatter only touches level 0.
		"Body": map[string]interface{}{"Status": "ok", "items": nestedBody(1, 2)},
		// "My.Var" becomes "my.Var": only the part before the first dot is lowered.
		"My.Var": "dotted",
		"My.Map": map[string]interface{}{"Inner": "x"},
		"Plain":  "a scalar under an upper-case name",
	}
	return fixtures
}

// TestFlattenLowerFirstKeyIsDecomposablePerEntry extends #2017's invariant (TestFlattenIsDecomposablePerEntry) to
// DumpString, the lower-case-first-key flatten processUserExecutorInputs runs on the variables of every custom executor
// call (#1980): the whole-map dump is the union of the single-entry dumps once the whole-map aggregates are restored. A
// cache built per entry for this key style rests on it; if the formatter ever made one entry's keys depend on another
// entry, this fails instead of the cache.
//
// The aggregates take the formatter too, so in this key style they are __len__ and __type__.
func TestFlattenLowerFirstKeyIsDecomposablePerEntry(t *testing.T) {
	lenKey, typeKey := strings.ToLower(dumpLenKey), strings.ToLower(dumpTypeKey)
	for name, vars := range lowerFirstKeyFixtures() {
		t.Run(name, func(t *testing.T) {
			whole, err := DumpString(vars)
			require.NoError(t, err)

			merged := map[string]string{}
			for k, v := range vars {
				part, err := DumpString(H{k: v})
				require.NoError(t, err)
				delete(part, lenKey)
				delete(part, typeKey)
				maps.Copy(merged, part)
			}
			merged[lenKey] = strconv.Itoa(len(vars))
			merged[typeKey] = "Map"

			require.Equal(t, whole, merged,
				"the per-entry dumps plus the restored aggregates must reproduce the whole-map dump")
		})
	}
}

// TestFlattenerMatchesDumpStringPreserveCase is the equivalence the whole change rests on: the
// cached flattener must produce exactly what the engine produced before it existed, on the first
// call and on every call after it (the second call is the one served from the cache).
func TestFlattenerMatchesDumpStringPreserveCase(t *testing.T) {
	for name, vars := range flattenFixtures() {
		t.Run(name, func(t *testing.T) {
			want, err := DumpStringPreserveCase(vars)
			require.NoError(t, err)

			var f varsFlattener
			for call := 1; call <= 3; call++ {
				got, err := f.flatten(vars)
				require.NoError(t, err)
				require.Equal(t, want, got, "call %d differs from the uncached dump", call)
			}
		})
	}
}

// TestFlattenerReturnsAFreshMap pins the constraint that makes the cache safe to use at the call
// site: process_testcase.go interpolates and quote-escapes the returned map in place, so handing
// back the cached map would corrupt every later step.
func TestFlattenerReturnsAFreshMap(t *testing.T) {
	vars := H{"body": nestedBody(2, 3), "scalar": "s"}

	var f varsFlattener
	first, err := f.flatten(vars)
	require.NoError(t, err)

	for k := range first {
		first[k] = "clobbered"
	}

	second, err := f.flatten(vars)
	require.NoError(t, err)

	want, err := DumpStringPreserveCase(vars)
	require.NoError(t, err)
	require.Equal(t, want, second, "a caller mutating the result must not affect later flattens")
}

// TestFlattenerInvalidatesWhenAValueChanges covers the case the cache exists to get right: a
// variable that is replaced between steps must be re-flattened, not served from the previous step.
func TestFlattenerInvalidatesWhenAValueChanges(t *testing.T) {
	var f varsFlattener

	before := H{"body": map[string]interface{}{"x": "first"}}
	got, err := f.flatten(before)
	require.NoError(t, err)
	require.Equal(t, "first", got["body.x"])

	// A different object under the same name — what an executor producing a new result looks like.
	after := H{"body": map[string]interface{}{"x": "second"}}
	got, err = f.flatten(after)
	require.NoError(t, err)
	require.Equal(t, "second", got["body.x"])

	want, err := DumpStringPreserveCase(after)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

// TestFlattenerHandlesAVariableLeavingScope pins that a name present in one step and absent in the
// next contributes nothing to the next step's result, cached or not.
func TestFlattenerHandlesAVariableLeavingScope(t *testing.T) {
	var f varsFlattener

	withBody := H{"body": nestedBody(1, 2), "scalar": "s"}
	_, err := f.flatten(withBody)
	require.NoError(t, err)

	withoutBody := H{"scalar": "s"}
	got, err := f.flatten(withoutBody)
	require.NoError(t, err)

	want, err := DumpStringPreserveCase(withoutBody)
	require.NoError(t, err)
	require.Equal(t, want, got, "a variable out of scope must not survive in the cached result")
}

// TestFlattenerSkipsNamesShadowedByAnother pins the decision behind sharesPrefixWithAnotherName: a
// scope can hold both "body" (a map a step captured) and "body.status" (a string the suite declared,
// already flattened by the loader). Those two produce the same flat key. Dumping the whole scope in
// one call lets go-dump's map iteration order pick the winner — a coin flip, but the engine's coin
// flip; flattening them separately and merging would hand the win to a fixed side, which is a
// behaviour change #2017 puts out of scope. So a shadowed name is not cached at all.
//
// Asserted structurally (is it in the cache?) rather than by output, because the output of this
// shape is genuinely nondeterministic in both the old and the new engine.
func TestFlattenerSkipsNamesShadowedByAnother(t *testing.T) {
	var f varsFlattener
	_, err := f.flatten(H{
		"body":        map[string]interface{}{"status": "from-the-captured-map"},
		"body.status": "from-the-pre-flattened-suite-var",
		"other":       map[string]interface{}{"free": "of-collisions"},
	})
	require.NoError(t, err)

	require.NotContains(t, f.entries, "body", "a shadowed name must keep the engine's own resolution")
	require.Contains(t, f.entries, "other", "an unshadowed map is still cached")
}

// TestFlattenerRetainsCachedValues pins flattenedEntry.value, which is written and never read and
// therefore looks like dead weight — deleting it leaves every other test green while making the
// cache wrong. It is what keeps a cached value alive so its address cannot be handed to a different
// object, which would turn the pointer comparison in the identity into a false hit. Measured during
// review: without it, a freed map's address is reused within about nine allocations.
func TestFlattenerRetainsCachedValues(t *testing.T) {
	var f varsFlattener
	_, err := f.flatten(H{"body": map[string]interface{}{"x": "1"}})
	require.NoError(t, err)

	entry, ok := f.entries["body"]
	require.True(t, ok)
	require.NotNil(t, entry.value, "the cache must hold a reference to the value it keyed by address")
}

// BenchmarkFlattenRepeatedValue is #2017's proof: a step scope carrying one large captured response
// that does not change between steps. FirstCall is the cold path (identical work to the old
// engine); RepeatedValue is what every step after the first now costs.
func BenchmarkFlattenRepeatedValue(b *testing.B) {
	vars := H{
		"venom.testsuite.filepath": "tests/suite.yml",
		"venom.teststep.number":    3,
		"http.result.bodyjson":     nestedBody(3, 12),
	}
	for i := 0; i < 15; i++ {
		vars[fmt.Sprintf("previous.var%d", i)] = fmt.Sprintf("value-%d", i)
	}

	b.Run("Uncached_TodaysEngine", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := DumpStringPreserveCase(vars); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Cached_RepeatedValue", func(b *testing.B) {
		var f varsFlattener
		if _, err := f.flatten(vars); err != nil { // warm, as step 1 would
			b.Fatal(err)
		}
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := f.flatten(vars); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// stringScope is a scope of plain string values, the shape a suite's variables have once initTestSuiteVars has run
// (#2405): keys with dots, mixed case, dashes, empty and multi-line values, the aggregate names, and the three
// characters go-dump's default key formatter rewrites.
func stringScope() H {
	return H{
		"A": "Y", "a.b": "x", "Foo.Bar": "q", "x-y": "z", "e": "", "nl": "l1\nl2",
		"cfg": `{"n":3}`, "cfg.n": "3", "Cfg.N": "3", "uni.é": "ü", "trail.": "t", ".lead": "l", "a..b": "dd",
	}
}

// TestDumpPreserveCaseCopiesStringValuesUnchanged pins what a fast path for string values may rely on (#2405): for a
// key the default formatter leaves alone, DumpStringPreserveCase emits the key and the value as they are, plus the two
// whole-map aggregates and nothing else.
func TestDumpPreserveCaseCopiesStringValuesUnchanged(t *testing.T) {
	in := stringScope()
	out, err := DumpStringPreserveCase(in)
	require.NoError(t, err)

	want := map[string]string{dumpLenKey: strconv.Itoa(len(in)), dumpTypeKey: "Map"}
	for k, v := range in {
		str, ok := v.(string)
		require.True(t, ok)
		want[k] = str
	}
	require.Equal(t, want, out)
}

// TestDumpPreserveCaseRewritesKeysWithSpaceSlashOrColon pins the exceptions a fast path must leave to go-dump: the
// default key formatter turns ' ', '/' and ':' into '_', and a variable named like an aggregate loses to the aggregate.
func TestDumpPreserveCaseRewritesKeysWithSpaceSlashOrColon(t *testing.T) {
	out, err := DumpStringPreserveCase(H{"sp ace": "1", "sl/ash": "2", "co:lon": "3", dumpLenKey: "99"})
	require.NoError(t, err)

	require.Equal(t, map[string]string{"sp_ace": "1", "sl_ash": "2", "co_lon": "3", dumpLenKey: "4", dumpTypeKey: "Map"}, out)
}

// TestDumpStringLowersTheFirstSegmentOfStringKeys pins that DumpString, unlike DumpStringPreserveCase, rewrites the
// first segment of a key, and the aggregate names with it: a fast path for it must not copy keys as they are.
func TestDumpStringLowersTheFirstSegmentOfStringKeys(t *testing.T) {
	out, err := DumpString(H{"A": "Y", "Foo.Bar": "q", "x-y": "z"})
	require.NoError(t, err)

	require.Equal(t, map[string]string{"a": "Y", "foo.Bar": "q", "x-y": "z", "__len__": "3", "__type__": "Map"}, out)
}

// TestSuiteScopeIsPlainStringsAfterInit pins the scope a test case of a real run receives from a suite variable
// (#2405 research R3): initTestSuiteVars replaces a nested value by its own JSON string and adds every dotted key next
// to it, resolved, all as strings, so nothing nested is left for the flatten to walk.
func TestSuiteScopeIsPlainStringsAfterInit(t *testing.T) {
	ts := &TestSuite{Vars: H{"host": "example.test", "cfg": map[string]interface{}{"url": "http://{{.host}}/x", "n": 3}}}
	require.NoError(t, New().initTestSuiteVars(ts))

	for k, v := range ts.Vars {
		require.IsType(t, "", v, "variable %q", k)
	}
	require.Equal(t, `{"n":3,"url":"http://example.test/x"}`, ts.Vars["cfg"])
	require.Equal(t, "http://example.test/x", ts.Vars["cfg.url"])
	require.Equal(t, "3", ts.Vars["cfg.n"])
}

// scopeCases are scopes dumpScope must dump as go-dump does: strings only, strings next to maps, slices and structs
// (a value that agrees with its dotted string, which go-dump resolves the same whatever its map order), keys the
// formatter rewrites, names that are, or become, an aggregate, and the empty and nil scope.
func scopeCases() map[string]H {
	return map[string]H{
		"strings":    stringScope(),
		"empty":      {},
		"nil":        nil,
		"aggregates": {"__Len__": "7", "__Type__": "T", "Ok": "1"},
		// what initTestSuiteVars leaves in a real scope: the aggregates of its own dump, as strings
		"suiteScope":     {"__Len__": "2", "__Type__": "Map", "cfg": `{"n":3}`, "cfg.n": "3", "cfg.__Len__": "1", "cfg.__Type__": "Map"},
		"lenIsAMap":      {"__Len__": map[string]interface{}{"x": "1"}, "a": "b"},
		"lenAndTypeMaps": {"__Type__": "Map", "__Len__": "9", "body": map[string]interface{}{"s": "1"}, "body.s": "1"},
		"rewritten":      {"sp ace": "1", "sl/ash": "2", "co:lon": "3", "Sp Ace.Key": "4", "": "5"},
		// a name the formatter rewrites into the type aggregate, with a value other than "Map"
		"typeByRewrite": {"__Type_ ": "T", "a": "b"}, //nolint:gocritic // the whitespace in the key is the case
		"mixed": {
			"Name": "n", "body": map[string]interface{}{"status": "200", "items": []interface{}{"a", "b"}},
			"body.status": "200", "list": []string{"x", "y"}, "n": 3, "flag": true, "nilv": nil,
			"st": struct{ A, B string }{"1", "2"},
		},
	}
}

// TestDumpScopeMatchesGoDump compares the string fast path with go-dump on every case, for both dump modes.
func TestDumpScopeMatchesGoDump(t *testing.T) {
	for name, h := range scopeCases() {
		t.Run(name, func(t *testing.T) {
			want, err := DumpStringPreserveCase(h)
			require.NoError(t, err)
			got, err := dumpHPreserveCase(h)
			require.NoError(t, err)
			require.Equal(t, want, got, "DumpStringPreserveCase")

			want, err = DumpString(h)
			require.NoError(t, err)
			got, err = dumpHLowerFirst(h)
			require.NoError(t, err)
			require.Equal(t, want, got, "DumpString")
		})
	}
}

// TestDumpScopeLeavesDisagreeingNamesToGoDump pins the guard: a string and a map that flatten to the same key with
// different values are handed to go-dump whole, so the result is one go-dump could give.
func TestDumpScopeLeavesDisagreeingNamesToGoDump(t *testing.T) {
	h := H{"body": map[string]interface{}{"status": "200"}, "body.status": "500"}
	got, err := dumpHPreserveCase(h)
	require.NoError(t, err)
	require.Contains(t, []string{"200", "500"}, got["body.status"])
	require.Equal(t, "2", got[dumpLenKey])
}

// TestScopeEditMatchesADumpOfTheEditedScope pins scopeEdit (#2465): bringing a dump up to date after some edits gives
// what a dump of the edited scope gives, and what it cannot follow it reports instead of guessing.
func TestScopeEditMatchesADumpOfTheEditedScope(t *testing.T) {
	type edit struct {
		k string
		v interface{}
	}
	base := func() H {
		return H{"a": "1", "input.x": "old", "input.m": map[string]interface{}{"k": "v"}, "Up.Case": "u", "__len__": "0", "__type__": "Map"}
	}
	cases := map[string]struct {
		edits   []edit
		follows bool
	}{
		"new string":                {[]edit{{"input.y", "2"}}, true},
		"new map":                   {[]edit{{"input.n", map[string]interface{}{"p": []interface{}{"q", 3}}}}, true},
		"new key lowered":           {[]edit{{"Input.Z", "z"}}, true},
		"string replaces string":    {[]edit{{"input.x", "new"}}, true},
		"set twice":                 {[]edit{{"input.y", "1"}, {"input.y", map[string]interface{}{"r": "s"}}}, true},
		"new then replaced":         {[]edit{{"input.y", "1"}, {"input.y", "2"}}, true},
		"map replaces string":       {[]edit{{"input.x", map[string]interface{}{"r": "s"}}}, false},
		"string replaces map":       {[]edit{{"input.m", "flat"}}, false},
		"replaces a key to rewrite": {[]edit{{"a", 5}}, false},
		"new aggregate name":        {[]edit{{"__len__", "9"}}, false},
		"new key clashing in dump":  {[]edit{{"input.m", "x"}, {"input.m.k", "other"}}, false},
		"clash with a base key":     {[]edit{{"input", map[string]interface{}{"x": "other"}}}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			vrs := base()
			delete(vrs, "__len__")
			delete(vrs, "__type__")
			dumped, err := dumpHLowerFirst(vrs)
			require.NoError(t, err)
			var e scopeEdit
			for _, ed := range tc.edits {
				e.set(vrs, ed.k, ed.v)
			}
			ok := e.apply(dumped, DumpString, true, "__len__", "__type__")
			require.Equal(t, tc.follows, ok)
			if !ok {
				return
			}
			want, err := DumpString(vrs)
			require.NoError(t, err)
			require.Equal(t, want, dumped)
		})
	}
}
