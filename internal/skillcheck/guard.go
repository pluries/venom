package skillcheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The rules CheckSkill reports, one per kind of thing the skill folder must not hold (US5-AS2).
const (
	RulePrivatePath      = "private-path"
	RuleSDLCTerm         = "sdlc-term"
	RuleHostName         = "host-name"
	RuleSecret           = "secret"
	RuleCarriageReturn   = "carriage-return"
	RuleDisableApprovals = "disable-approvals"
	RuleUnreadable       = "unreadable"
)

// Finding is one thing CheckSkill refuses: the file (relative to the folder given to CheckSkill, with
// forward slashes), the line (1-based), the rule and a short detail that never repeats a secret value.
type Finding struct {
	File   string
	Line   int
	Rule   string
	Detail string
}

// lineRule is a rule that applies to one line at a time.
type lineRule struct {
	rule   string
	detail string
	re     *regexp.Regexp
}

var sdlcTerms = []*regexp.Regexp{
	regexp.MustCompile(`\bG1\b`),
	regexp.MustCompile(`\bG2\b`),
	regexp.MustCompile(`spec-driven`),
	regexp.MustCompile(`decisions\.md`),
	regexp.MustCompile(`\bcst-`),
	regexp.MustCompile(`#\d{4}\b`),
}

var hostNames = []*regexp.Regexp{
	regexp.MustCompile(`\bcicd\.`),
	regexp.MustCompile(`\b(?:10\.\d{1,3}\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3})\b`),
}

var secretShapes = []*regexp.Regexp{
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36}\b`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`\bBearer [A-Za-z0-9._-]{20,}`),
}

// fixedPrivatePaths are repository-relative path tokens of this project's private files, matched whatever the
// publish script says. The skill may say ".claude/skills" (where a skill is installed), "docs" and "Jenkinsfile".
var fixedPrivatePaths = []string{"docs/specs/", "jenkins/", ".claude/rules", ".claude/hooks", ".claude/agents"}

// publishEntriesSkipped are PRIVATE entries of the publish script that are ordinary words or names a public skill may
// say (the folder a skill is installed to, a docs folder of the user's project, a Jenkinsfile).
var publishEntriesSkipped = map[string]bool{".claude": true, "docs": true, "Jenkinsfile": true, "jenkins": true}

var approvalOff = []*regexp.Regexp{
	regexp.MustCompile(`(?i)dangerously-skip-permissions`), // the first entry is flagged whatever the sentence says
	regexp.MustCompile(`(?i)\b(?:turn|switch)\s+off\s+(?:the\s+)?(?:approvals?|permission)`),
	regexp.MustCompile(`(?i)\b(?:turn|switch)\s+(?:the\s+)?(?:approvals?|permission prompts?)\s+off\b`),
	regexp.MustCompile(`(?i)\b(?:disable|skip|bypass)\s+(?:the\s+)?(?:approvals?|permission)`),
}

var negationWord = regexp.MustCompile(`(?i)\b(?:never|not|no|don't|doesn't|cannot|can't)\b`)

// CheckSkill walks every file under dir, scripts included, and reports what a public skill must not hold.
// A file or folder it cannot read is a finding too, so that nothing passes unread.
func CheckSkill(dir string) []Finding {
	rules := lineRules(privatePathRegexp())
	var out []Finding
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		rel := relSlash(dir, path)
		if err != nil {
			out = append(out, Finding{rel, 1, RuleUnreadable, "an entry that cannot be read"})
			return nil //nolint:nilerr // the finding is recorded and the walk goes on
		}
		if d.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(path) //nolint:gosec // the folder is the caller's own and is only read
		if rerr != nil {
			out = append(out, Finding{rel, 1, RuleUnreadable, "a file that cannot be read"})
			return nil //nolint:nilerr // the finding is recorded and the walk goes on
		}
		out = append(out, checkFile(rel, string(b), rules)...)
		return nil
	})
	if walkErr != nil {
		out = append(out, Finding{".", 1, RuleUnreadable, "the folder cannot be walked"})
	}
	return out
}

// relSlash returns path relative to dir with forward slashes, or path itself when that cannot be computed.
func relSlash(dir, path string) string {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func lineRules(private *regexp.Regexp) []lineRule {
	rs := []lineRule{{RulePrivatePath, "a private path of this project", private}}
	for _, re := range sdlcTerms {
		rs = append(rs, lineRule{RuleSDLCTerm, "a term of the private delivery process", re})
	}
	for _, re := range hostNames {
		rs = append(rs, lineRule{RuleHostName, "an internal host name or private address", re})
	}
	for _, re := range secretShapes {
		rs = append(rs, lineRule{RuleSecret, "a credential-like value", re})
	}
	return rs
}

func checkFile(rel, text string, rules []lineRule) []Finding {
	var out []Finding
	if strings.HasSuffix(rel, ".sh") {
		if i := strings.Index(text, "\r"); i >= 0 {
			out = append(out, Finding{rel, lineOf(text, i), RuleCarriageReturn, "a carriage return in a shell script"})
		}
	}
	for n, line := range strings.Split(text, "\n") {
		for _, r := range rules {
			if r.re.MatchString(line) {
				out = append(out, Finding{rel, n + 1, r.rule, r.detail})
			}
		}
	}
	return append(out, approvalFindings(rel, text)...)
}

// approvalFindings reports an instruction to turn the approval prompts off. A sentence that holds a negationWord
// ("never ask to turn approvals off") is a refusal, not an instruction.
func approvalFindings(rel, text string) []Finding {
	var out []Finding
	for i, re := range approvalOff {
		for _, loc := range re.FindAllStringIndex(text, -1) {
			if i > 0 && negatedBefore(sentenceBefore(text, loc[0])) {
				continue
			}
			out = append(out, Finding{rel, lineOf(text, loc[0]), RuleDisableApprovals, "an instruction to turn the approval prompts off"})
		}
	}
	return out
}

// negatedBefore tells whether one of the five words just before the verb is a negation: "never ask ... to turn
// approvals off" is a refusal, "if you do not want a prompt, disable the approvals" is not.
func negatedBefore(sentence string) bool {
	words := strings.Fields(sentence)
	if len(words) > 5 {
		words = words[len(words)-5:]
	}
	return negationWord.MatchString(strings.Join(words, " "))
}

// sentenceBefore returns the text from the start of the sentence that holds index i up to i.
func sentenceBefore(text string, i int) string {
	start := 0
	for _, sep := range []string{". ", ".\n", "? ", "! ", "\n\n", "\n- ", "\n* "} {
		if j := strings.LastIndex(text[:i], sep); j >= 0 && j+len(sep) > start {
			start = j + len(sep)
		}
	}
	return text[start:i]
}

func lineOf(text string, i int) int {
	return strings.Count(text[:i], "\n") + 1
}

// privatePathRegexp builds one expression of the path tokens: the fixed ones and the entries of the PRIVATE array of
// scripts/publish-github.sh when it exists (found from the working directory upwards).
func privatePathRegexp() *regexp.Regexp {
	seen := map[string]bool{}
	var tokens []string
	add := func(t string) {
		if t != "" && !seen[t] {
			seen[t] = true
			tokens = append(tokens, regexp.QuoteMeta(t))
		}
	}
	for _, t := range fixedPrivatePaths {
		add(t)
	}
	for _, e := range publishPrivateEntries() {
		if publishEntriesSkipped[e] {
			continue
		}
		if strings.Contains(filepath.Base(e), ".") {
			add(e) // a file name
		} else {
			add(e + "/") // a folder
		}
	}
	sort.Strings(tokens)
	return regexp.MustCompile(`(?:^|[^A-Za-z0-9_.-])(?:` + strings.Join(tokens, "|") + `)`)
}

var privateArray = regexp.MustCompile(`(?s)PRIVATE=\(([^)]*)\)`)

// publishPrivateEntries reads the PRIVATE array of scripts/publish-github.sh, or nothing when the script is absent
// (the public mirror).
func publishPrivateEntries() []string {
	root := findRoot()
	if root == "" {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(root, "scripts", "publish-github.sh"))
	if err != nil {
		return nil
	}
	m := privateArray.FindSubmatch(b)
	if m == nil {
		return nil
	}
	return strings.Fields(string(m[1]))
}

// findRoot returns the folder with go.mod above the working directory, or "".
func findRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
