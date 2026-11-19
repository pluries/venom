package web

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Helpers of the tests of spec 015 (docs/specs/015-web-recipe-actions), used by the unit tests on the fake and by the
// real-browser tests (tag playwright).

// runResult runs one step as webRun does, without its check that the step took more than 0 s: a fast step on the fake
// measures 0 s on a machine whose timer is coarse, and these tests do not depend on timing.
func runResult(ctx context.Context, t *testing.T, step venom.TestStep) (Result, error) {
	t.Helper()
	got, err := Executor{}.Run(ctx, step)
	if err != nil {
		assert.Nil(t, got)
		return Result{}, err
	}
	res, isPtr := got.(*Result)
	require.Truef(t, isPtr, "want a *Result, got %T", got)
	return *res, nil
}

// mustRun is runResult for a step that must pass.
func mustRun(ctx context.Context, t *testing.T, step venom.TestStep) Result {
	t.Helper()
	res, err := runResult(ctx, t, step)
	require.NoError(t, err)
	return res
}

// loc is a find element map of the given locator and selector, with extra key and value pairs.
func loc(kind, selector string, extra ...any) map[string]interface{} {
	m := map[string]interface{}{"locator": kind, "selector": selector}
	for i := 0; i+1 < len(extra); i += 2 {
		m[fmt.Sprint(extra[i])] = extra[i+1]
	}
	return m
}

// findStep is the find action on an element.
func findStep(find interface{}) venom.TestStep {
	return action(map[string]interface{}{"find": find})
}

func ptr(s string) *string { return &s }

// found is the part of a find step's Result that names the element it matched.
type found struct {
	Find        int
	Text, Value string
}

func foundOf(r Result) found {
	return found{Find: r.Find, Text: r.Text, Value: r.Value}
}

// callsWithPrefix keeps the calls that start with prefix.
func callsWithPrefix(calls []string, prefix string) []string {
	var out []string
	for _, c := range calls {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}
