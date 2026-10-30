package venom

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/gosimple/slug"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// guardSecret is a synthetic secret with characters that the slug and the URL forms change. Built by concatenation for
// the secret scan.
var guardSecret = "Gu@rd s3cret" + "/z"

// guardSuiteYAML plants guardSecret in every place the console prints: the suite name, the test case name, the step
// name, the step's info line, the error message of a failing step and the text of a failing assertion. With secrets
// false the variable is not declared secret, which is the control that proves the console does print the value.
func guardSuiteYAML(secrets bool) string {
	declared := ""
	if secrets {
		declared = "secrets: [token]\n"
	}
	return "name: guard-" + guardSecret + "\n" +
		"vars:\n  token: '" + guardSecret + "'\n" +
		declared +
		"testcases:\n" +
		"- name: case-" + guardSecret + "\n" +
		"  steps:\n" +
		"  - type: fake\n    name: pass-{{.token}}\n    code: 0\n    info: 'info {{.token}}'\n" +
		"    assertions:\n    - result.code ShouldEqual 0\n" +
		"  - type: fake\n    name: fail-{{.token}}\n    code: '{{.token}}'\n    info: 'got {{.result.code}}'\n" +
		"    assertions:\n    - result.code ShouldEqual 0\n" +
		"  - type: fake\n    name: error-{{.token}}\n    error: 'cannot reach {{.token}}'\n"
}

// TestConsoleNeverPrintsADeclaredSecret is the guard against a new console print site that skips HideSensitive: it runs
// one suite that carries a declared secret in every field the console prints, with and without -v, and asserts that no
// form of the secret (plain, slug, URL-encoded) reaches the console. A line that prints a new field of a suite, a test
// case or a step must hide it too, or this test names the form that leaked (#2302).
func TestConsoleNeverPrintsADeclaredSecret(t *testing.T) {
	forms := map[string]string{
		"plain":      guardSecret,
		"slug":       slug.Make(guardSecret),
		"query form": url.QueryEscape(guardSecret),
		"path form":  url.PathEscape(guardSecret),
	}
	for _, verbose := range []int{0, 1} {
		t.Run(fmt.Sprintf("verbose %d", verbose), func(t *testing.T) {
			v, c := suiteNewVenom(t, verbose)
			ts, _ := suiteForRunTestCases(t, v, guardSuiteYAML(true))

			v.runTestCases(v.processSecrets(context.Background(), ts, nil), ts)

			out := c.String()
			require.Contains(t, out, "__hidden__", "the run must print the lines the guard watches")
			require.Contains(t, out, "cannot reach", "the failing step's error line must be printed")
			for name, form := range forms {
				assert.NotContains(t, out, form, "the %s form of the secret reached the console", name)
			}
		})
		t.Run(fmt.Sprintf("verbose %d control without secrets", verbose), func(t *testing.T) {
			v, c := suiteNewVenom(t, verbose)
			ts, _ := suiteForRunTestCases(t, v, guardSuiteYAML(false))

			v.runTestCases(v.processSecrets(context.Background(), ts, nil), ts)

			out := c.String()
			assert.True(t, strings.Contains(out, guardSecret) || strings.Contains(out, slug.Make(guardSecret)),
				"without a declared secret the console prints the value: the guard watches a live path")
			assert.NotContains(t, out, "__hidden__")
		})
	}
}
