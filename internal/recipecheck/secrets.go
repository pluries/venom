package recipecheck

import (
	"regexp"
	"strings"

	yamlv3 "go.yaml.in/yaml/v3"
)

// The rules, the placeholder pattern and the semantics of findSecret are a port of
// .claude/skills/cst-g-commit/scripts/secret-patterns.js; keep the two in step.
var placeholder = regexp.MustCompile(`(?i)(<[^>]{2,}>|\$\{[^}]+\}|\{\{[^}]+\}\}|xxxx|changeme|change-me|your[-_ ]|example|dummy|sample|placeholder|redacted|\*{4,}|000000|\bTODO\b)`)

type secretRule struct {
	kind       string
	re         *regexp.Regexp
	structured bool
}

var secretRules = []secretRule{
	{"private key block", regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`), true},
	{"AWS access key id", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), true},
	{"Anthropic API key", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}`), true},
	{"OpenAI-style API key", regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9]{20,}\b`), true},
	{"GitHub token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}\b`), true},
	{"Slack token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}`), true},
	{"Google API key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`), true},
	{"Azure storage account key", regexp.MustCompile(`AccountKey=[A-Za-z0-9+/=]{40,}`), false},
	{"Azure SAS token", regexp.MustCompile(`[?&]sig=[A-Za-z0-9%+/=]{30,}`), false},
	{"JWT", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), true},
	{"connection string with password", regexp.MustCompile(`(?i)(?:Password|Pwd)\s*=\s*[^;\s'"]{8,}`), false},
	{"URL with embedded credentials", regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s:/@'"]+:[^\s@'"]{6,}@[^\s'"]+`), false},
	{"generic secret assignment", regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|secret|token|passw(?:or)?d|client[_-]?secret|access[_-]?key)\s*[:=]\s*['"][A-Za-z0-9_\-+/=]{16,}['"]`), false},
}

const maxSecretLength = 4000

// findSecretLine applies findSecret to each line of s, as secret-patterns.js does to each line of a file, and returns
// the kind of the first hit with the index of its line.
func findSecretLine(s string) (string, int) {
	for i, line := range strings.Split(s, "\n") {
		if kind := findSecret(strings.TrimSuffix(line, "\r")); kind != "" {
			return kind, i
		}
	}
	return "", 0
}

// findSecret returns the kind of the first credential-like text in one line, or "". A placeholder anywhere in the
// line disarms the rules that are not structured tokens.
func findSecret(s string) string {
	if s == "" || len(s) > maxSecretLength {
		return ""
	}
	disarmed := placeholder.MatchString(s)
	for _, r := range secretRules {
		if r.re.MatchString(s) && (r.structured || !disarmed) {
			return r.kind
		}
	}
	return ""
}

// ruleRC08 reports a credential-like text in any string of a step or in a variable default. The finding names the
// kind, never the text.
func ruleRC08(f *Folder) []Finding {
	var out []Finding
	visitScalars(f, func(st *Step) []*yamlv3.Node { return []*yamlv3.Node{st.Node} }, func(s *Suite, tc *Testcase, n *yamlv3.Node) {
		if kind, i := findSecretLine(n.Value); kind != "" {
			out = append(out, s.finding(tc, "RC08", n.Line+i, "credential-like text ("+kind+"): take it from a variable"))
		}
	})
	return out
}
