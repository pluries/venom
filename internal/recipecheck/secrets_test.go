package recipecheck

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every credential-shaped value below is assembled from fragments when the test runs, so that no source line holds a
// whole credential for the repository's secret scanner. The values are synthetic.

type credential struct {
	kind  string
	value string
}

// credentials are the 13 kinds of .claude/skills/cst-g-commit/scripts/secret-patterns.js, in its order.
func credentials() []credential {
	alnum36 := strings.Repeat("aB3dE5", 6)
	return []credential{
		{"private key block", "-----BEGIN " + "RSA " + "PRIVATE KEY-----"},
		{"AWS access key id", "AK" + "IA" + "ABCDEFGH12345678"},
		{"Anthropic API key", "sk-" + "ant-" + "aB3dE5gH7jK9mN2pQ4rS6t"},
		{"OpenAI-style API key", "sk-" + "proj-" + "aB3dE5gH7jK9mN2pQ4rS6tU8"},
		{"GitHub token", "gh" + "p_" + alnum36},
		{"Slack token", "xo" + "xb-" + "1234567890-aB3dE5gH7j"},
		{"Google API key", "AI" + "za" + strings.Repeat("aB3dE5g", 5)},
		{"Azure storage account key", "Account" + "Key=" + "aB3dE5gH7jK9mN2pQ4rS6tU8vW0yZ1aB3dE5gH7jK9mN=="},
		{"Azure SAS token", "https://h.test/c?sv=1&" + "s" + "ig=" + "aB3dE5gH7jK9mN2pQ4rS6tU8vW0yZ1aB3dE5"},
		{"JWT", "ey" + "J" + "hbGciOiJIUzI1NiIs" + ".ey" + "J" + "zdWIiOiJhYmMxMjM0NTY" + "." + "aB3dE5gH7jK9mN2p"},
		{"connection string with password", "Pass" + "word=" + "aB3dE5gH7jK9"},
		{"URL with embedded credentials", "postgres://" + "svc" + ":" + "aB3dE5gH7j" + "@db.test/app"},
		{"generic secret assignment", "api_" + "key" + `="` + "aB3dE5gH7jK9mN2pQ4" + `"`},
	}
}

// fillStep is a web step YAML (after "- type: web") that types value into an input; the text is on line 11 of
// suiteWithStep.
func fillStep(value string) string {
	return "    action:\n      fill:\n      - find: input\n        text: " + yamlQuote(value) + "\n"
}

// US3-AS3: each of the 13 credential kinds is reported once, on the line of the string that holds it.
func TestRC08ReportsEachCredentialKind(t *testing.T) {
	kinds := credentials()
	require.Len(t, kinds, 13)
	for _, c := range kinds {
		t.Run(c.kind, func(t *testing.T) {
			folder := oneFile(t, suiteWithStep(fillStep("prefix "+c.value+" suffix")), specOf)

			findings := onlyRule(t, ruleRC08(folder), "RC08")

			require.Len(t, findings, 1)
			assert.Equal(t, 11, findings[0].Line)
			assert.Equal(t, "US1-AS1 one", findings[0].Testcase)
			assert.Equal(t, "a.yml", findings[0].File)
			assert.NotContains(t, findings[0].Message, c.value, "the finding must not repeat the credential")
		})
	}
}

// placeholderCases are strings that must not be reported: a template or a placeholder word disarms the generic,
// connection and URL rules, and plain text matches nothing.
func placeholderCases() map[string]string {
	return map[string]string{
		"template only":                       "{{.password}}",
		"bearer with template":                "Bearer {{.token}}",
		"connection string with template":     "Pass" + "word={{.dbPassword}}",
		"URL with template":                   "postgres://svc:{{.dbPw}}@db.test/app",
		"generic with changeme":               "api_" + "key" + `="` + "changeme-aB3dE5gH7jK9mN2p" + `"`,
		"generic with example":                "api_" + "key" + `="` + "example-aB3dE5gH7jK9mN2p" + `"`,
		"connection string with placeholder":  "Pass" + "word=" + "changeme123",
		"URL with a placeholder word":         "postgres://svc:your-password@db.test/app",
		"short generic value":                 "pass" + "word" + `="` + "short" + `"`,
		"the word password alone":             "password",
		"plain text":                          "Recipe fixture",
		"a typed item":                        "milk",
		"an empty string":                     "",
		"a selector":                          `[data-test="item-input"]`,
		"a url with no credentials":           "https://h.test/path?x=1",
		"connection string with a short word": "Pass" + "word=" + "abc",
	}
}

// US3-AS3: placeholders and plain text are not reported.
func TestRC08QuietForPlaceholdersAndPlainText(t *testing.T) {
	for name, value := range placeholderCases() {
		t.Run(name, func(t *testing.T) {
			folder := oneFile(t, suiteWithStep(fillStep(value)), specOf)

			assert.Empty(t, ruleRC08(folder))
		})
	}
}

// US3-AS3: a placeholder word does not disarm a structured token rule, as in the repository's scanner.
func TestRC08StructuredTokensAreNotDisarmedByAPlaceholderWord(t *testing.T) {
	for _, c := range credentials() {
		switch c.kind {
		case "private key block", "AWS access key id", "Anthropic API key", "OpenAI-style API key", "GitHub token", "Slack token", "Google API key", "JWT":
			t.Run(c.kind, func(t *testing.T) {
				folder := oneFile(t, suiteWithStep(fillStep("example "+c.value)), specOf)

				assert.Len(t, onlyRule(t, ruleRC08(folder), "RC08"), 1)
			})
		}
	}
}

// US3-AS3: the generic and connection rules are disarmed by a placeholder anywhere in the string.
func TestRC08GenericAndConnectionRulesAreDisarmedByAPlaceholderWord(t *testing.T) {
	for _, c := range credentials() {
		switch c.kind {
		case "connection string with password", "URL with embedded credentials", "generic secret assignment":
			t.Run(c.kind, func(t *testing.T) {
				folder := oneFile(t, suiteWithStep(fillStep("{{.note}} "+c.value)), specOf)

				assert.Empty(t, ruleRC08(folder))
			})
		}
	}
}

// US3-AS3: a string with two credentials is one finding on its line.
func TestRC08ReportsOnceAStringWithTwoCredentials(t *testing.T) {
	kinds := credentials()
	both := kinds[1].value + " " + kinds[4].value
	folder := oneFile(t, suiteWithStep(fillStep(both)), specOf)

	assert.Len(t, onlyRule(t, ruleRC08(folder), "RC08"), 1)
}

// secretAt places value in the given part of a one-testcase suite and returns the YAML and the line of the value.
func secretAt(where, value string) (string, int) {
	q := yamlQuote(value)
	tc := liveTestcase("US1-AS1 one")
	switch where {
	case "suite vars, nested map":
		return "name: Suite\nvars:\n  spec: 014-web-mobile-recipes\n  web:\n    headers:\n      token: " + q + "\ntestcases:\n" + tc, 6
	case "testcase vars":
		return header("014-web-mobile-recipes") + "testcases:\n- name: US1-AS1 one\n  vars:\n    token: " + q +
			"\n  steps:\n  - type: web\n    action:\n      find: h1\n    assertions:\n    - result.find ShouldEqual 1\n", 7
	case "step vars":
		return suiteWithStep("    vars:\n      token: " + q + "\n    action:\n      find: h1\n"), 9
	case "deep map and list in a step":
		return suiteWithStep("    action:\n      find: h1\n    custom:\n      a:\n        b:\n        - " + q + "\n"), 13
	default: // a field of an action
		return suiteWithStep(fillStep(value)), 11
	}
}

// US3-AS3: the walk recurses into maps and lists, and covers the variables of a suite, a testcase and a step.
func TestRC08WalksEveryStringOfAStepAndEveryVariableDefault(t *testing.T) {
	secret := credentials()[4].value
	for _, where := range []string{"an action field", "suite vars, nested map", "testcase vars", "step vars", "deep map and list in a step"} {
		t.Run(where, func(t *testing.T) {
			text, line := secretAt(where, secret)

			findings := onlyRule(t, ruleRC08(oneFile(t, text, specOf)), "RC08")

			require.Len(t, findings, 1)
			assert.Equal(t, line, findings[0].Line)
		})
	}
}

// FR-015: a step of another executor is not examined.
func TestRC08IgnoresStepsOfOtherExecutors(t *testing.T) {
	text := header("014-web-mobile-recipes") + "testcases:\n- name: US1-AS1 one\n  steps:\n  - type: exec\n    script: " +
		yamlQuote("echo "+credentials()[4].value) + "\n"

	assert.Empty(t, ruleRC08(oneFile(t, text, specOf)))
}

// quotedFillStep is fillStep with the value as a double-quoted YAML scalar, so that its newlines are kept.
func quotedFillStep(value string) string {
	return "    action:\n      fill:\n      - find: input\n        text: " + strconv.Quote(value) + "\n"
}

// US3-AS3: the rules apply per line, as secret-patterns.js applies them: a scalar over the length limit is still
// searched line by line, and a placeholder word disarms only the rules on its own line.
func TestRC08AppliesTheRulesPerLine(t *testing.T) {
	token := "gh" + "p_" + strings.Repeat("aB3dE5", 6)
	password := "Pass" + "word=" + "aB3dE5gH7jK9"
	tests := []struct {
		name  string
		value string
		kind  string
	}{
		{"a structured token on a short line of a scalar over 4000 characters",
			strings.Repeat("a line of plain text\n", 250) + token + "\nmore text\n", "GitHub token"},
		{"a placeholder word on another line does not disarm a connection password",
			password + "\nfill the field with the example below\n", "connection string with password"},
		{"a placeholder word on a line before does not disarm a connection password",
			"a changeme note\n" + password + "\n", "connection string with password"},
		{"a placeholder word on the credential's own line still disarms it",
			password + " changeme\nanother line\n", ""},
	}
	require.Greater(t, len(tests[0].value), maxSecretLength)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			folder := oneFile(t, suiteWithStep(quotedFillStep(tt.value)), specOf)

			findings := onlyRule(t, ruleRC08(folder), "RC08")

			if tt.kind == "" {
				assert.Empty(t, findings)
				return
			}
			require.Len(t, findings, 1)
			assert.Contains(t, findings[0].Message, tt.kind)
			assert.NotContains(t, findings[0].Message, token)
			assert.NotContains(t, findings[0].Message, "aB3dE5gH7jK9")
		})
	}
}
