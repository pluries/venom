package assertions

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

type AssertFunc func(actual interface{}, expected ...interface{}) error

// assertMapMu guards assertMap. Every write today happens once, sequentially, before any read
// (Parse's registerUserExecutors completes before Process starts running steps) — but that's an
// artifact of the current call graph, not a guarantee the type system enforces, and #1983 plans
// file-level parallel execution against the very loop that reads this map on every assertion
// check. Guard it now rather than after a future change to *when* registration happens
// (e.g. per-suite lazy loading, the way plugin executors already load lazily per step) silently
// reintroduces the race (#1974).
var assertMapMu sync.RWMutex

// assertMap contains list of assertions func
var assertMap = map[string]AssertFunc{
	"ShouldEqual":                  ShouldEqual,
	"ShouldNotEqual":               ShouldNotEqual,
	"ShouldAlmostEqual":            ShouldAlmostEqual,
	"ShouldNotAlmostEqual":         ShouldNotAlmostEqual,
	"ShouldNotExist":               ShouldNotExist,
	"ShouldBeNil":                  ShouldBeNil,
	"ShouldNotBeNil":               ShouldNotBeNil,
	"ShouldBeTrue":                 ShouldBeTrue,
	"ShouldBeFalse":                ShouldBeFalse,
	"ShouldBeZeroValue":            ShouldBeZeroValue,
	"ShouldBeGreaterThan":          ShouldBeGreaterThan,
	"ShouldBeGreaterThanOrEqualTo": ShouldBeGreaterThanOrEqualTo,
	"ShouldBeLessThan":             ShouldBeLessThan,
	"ShouldBeLessThanOrEqualTo":    ShouldBeLessThanOrEqualTo,
	"ShouldBeBetween":              ShouldBeBetween,
	"ShouldNotBeBetween":           ShouldNotBeBetween,
	"ShouldBeBetweenOrEqual":       ShouldBeBetweenOrEqual,
	"ShouldNotBeBetweenOrEqual":    ShouldNotBeBetweenOrEqual,
	"ShouldContain":                ShouldContain,
	"ShouldNotContain":             ShouldNotContain,
	"ShouldJSONContain":            ShouldJSONContain,
	"ShouldNotJSONContain":         ShouldNotJSONContain,
	"ShouldJSONContainWithKey":     ShouldJSONContainWithKey,
	"ShouldJSONContainAllWithKey":  ShouldJSONContainAllWithKey,
	"ShouldNotJSONContainWithKey":  ShouldNotJSONContainWithKey,
	"ShouldContainKey":             ShouldContainKey,
	"ShouldNotContainKey":          ShouldNotContainKey,
	"ShouldBeIn":                   ShouldBeIn,
	"ShouldNotBeIn":                ShouldNotBeIn,
	"ShouldBeEmpty":                ShouldBeEmpty,
	"ShouldNotBeEmpty":             ShouldNotBeEmpty,
	"ShouldHaveLength":             ShouldHaveLength,
	"ShouldStartWith":              ShouldStartWith,
	"ShouldNotStartWith":           ShouldNotStartWith,
	"ShouldEndWith":                ShouldEndWith,
	"ShouldNotEndWith":             ShouldNotEndWith,
	"ShouldBeBlank":                ShouldBeBlank,
	"ShouldNotBeBlank":             ShouldNotBeBlank,
	"ShouldContainSubstring":       ShouldContainSubstring,
	"ShouldNotContainSubstring":    ShouldNotContainSubstring,
	"ShouldEqualTrimSpace":         ShouldEqualTrimSpace,
	"ShouldHappenBefore":           ShouldHappenBefore,
	"ShouldHappenOnOrBefore":       ShouldHappenOnOrBefore,
	"ShouldHappenAfter":            ShouldHappenAfter,
	"ShouldHappenOnOrAfter":        ShouldHappenOnOrAfter,
	"ShouldHappenBetween":          ShouldHappenBetween,
	"ShouldTimeEqual":              ShouldTimeEqual,
	"ShouldJSONEqual":              ShouldJSONEqual,
	"ShouldNotJSONEqual":           ShouldNotJSONEqual,
	"ShouldBeArray":                ShouldBeArray,
	"ShouldBeMap":                  ShouldBeMap,
	"ShouldMatchRegex":             ShouldMatchRegex,
}

func Get(s string) (AssertFunc, bool) {
	assertMapMu.RLock()
	defer assertMapMu.RUnlock()
	f, ok := assertMap[s]
	return f, ok
}

func deepEqual(x, y interface{}) bool {
	if !reflect.DeepEqual(x, y) {
		return fmt.Sprintf("%v", x) == fmt.Sprintf("%v", y)
	}
	return true
}

func ShouldBeArray(actual interface{}, expected ...interface{}) error {
	if err := need(0, expected); err != nil {
		return err
	}
	_, err := cast.ToSliceE(actual)
	if err != nil {
		return fmt.Errorf("expected: %v to be an array but was not", actual)
	}
	return nil
}

func ShouldBeMap(actual interface{}, expected ...interface{}) error {
	if err := need(0, expected); err != nil {
		return err
	}
	_, err := cast.ToStringMapE(actual)
	if err != nil {
		return fmt.Errorf("expected: %v to be a map but was not", actual)
	}
	return nil
}

// RegisterUserAssertFunc registers a user assertion function
func RegisterUserAssertFunc(name string, ux AssertFunc) error {
	assertMapMu.Lock()
	defer assertMapMu.Unlock()

	if _, ok := assertMap[name]; ok {
		return errors.Errorf("cannot redefine existing assertion %q", name)
	}

	assertMap[name] = ux
	return nil
}
