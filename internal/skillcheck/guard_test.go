package skillcheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// cleanSkill is a skill folder that holds nothing forbidden but is full of words a careless guard would flag:
// the folder a skill is installed to, the product names Jenkins and Jenkinsfile, a docs folder of the user's
// project, loopback addresses, short numbers and negated mentions of approvals.
var cleanSkill = map[string]string{
	"SKILL.md": "# Skill\n\nThe skill is installed into .claude/skills/venom-pluries of your project.\n" +
		"It writes a Jenkinsfile for Jenkins, reads the docs folder of your project, and tests http://127.0.0.1:8080 and localhost.\n" +
		"Item #12 and step G10 are fine. The skill never asks to turn approvals off and does not ask to disable a permission prompt.\n",
	"scripts/ok.sh": "#!/bin/sh\necho hello\n",
}

// plantSkill writes the clean skill and then the given files (which replace or add), and returns the folder.
func plantSkill(t *testing.T, planted map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for _, files := range []map[string]string{cleanSkill, planted} {
		for rel, body := range files {
			p := filepath.Join(dir, filepath.FromSlash(rel))
			require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o700))
			require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
		}
	}
	return dir
}

func rulesOf(fs []Finding) []string {
	var rules []string
	for _, f := range fs {
		rules = append(rules, f.File+":"+f.Rule)
	}
	return rules
}

// requireOnly checks that the findings name the planted file with the wanted rule and no other file.
func requireOnly(t *testing.T, fs []Finding, file, rule string) {
	t.Helper()
	require.NotEmpty(t, fs, "the guard must fail on the planted %s", rule)
	require.Contains(t, rulesOf(fs), file+":"+rule)
	for _, f := range fs {
		require.Equal(t, file, f.File, "only the planted file may be flagged: %+v", f)
		require.Positive(t, f.Line)
	}
}

type plant struct {
	name, file, body, rule string
}

// The planted values are assembled at run time so this file holds none of them as text.
func plantedCases() []plant {
	gh := "gh" + "p_" + strings.Repeat("a1B2", 9)
	aws := "AK" + "IA" + "ABCDEFGHIJKLMNOP"
	key := "-----BEGIN " + "RSA PRIVATE KEY-----"
	return []plant{
		{"private path: spec folder", "references/a.md", "See " + "docs/" + "specs/016-x/spec.md for details.\n", RulePrivatePath},
		{"private path: jenkins folder", "references/a.md", "Copy " + "jenkins" + "/release-lib.sh here.\n", RulePrivatePath},
		{"private path: claude rules", "references/a.md", "Read " + ".claude" + "/rules/tdd.md first.\n", RulePrivatePath},
		{"private path in a script", "scripts/plant.sh", "#!/bin/sh\n# " + "jenkins" + "/release-lib.sh\n", RulePrivatePath},
		{"sdlc term: G1", "references/a.md", "After " + "G" + "1 approval.\n", RuleSDLCTerm},
		{"sdlc term: G2", "references/a.md", "At " + "G" + "2 the maker accepts.\n", RuleSDLCTerm},
		{"sdlc term: spec-driven", "references/a.md", "This is " + "spec" + "-driven work.\n", RuleSDLCTerm},
		{"sdlc term: decisions file", "references/a.md", "Log it in " + "decisions" + ".md.\n", RuleSDLCTerm},
		{"sdlc term: cst command", "references/a.md", "Run /" + "cst" + "-feature now.\n", RuleSDLCTerm},
		{"sdlc term: work item id", "references/a.md", "Tracked as #" + "3916" + " on the board.\n", RuleSDLCTerm},
		{"host name: ci host", "references/a.md", "Open https://" + "cicd" + ".corp.test/job/x\n", RuleHostName},
		{"host name: private address", "references/a.md", "Call " + "192.168." + "7.21" + " for it.\n", RuleHostName},
		{"secret: github token", "references/a.md", "token " + gh + "\n", RuleSecret},
		{"secret: cloud key id in a script", "scripts/plant.sh", "#!/bin/sh\nexport K=" + aws + "\n", RuleSecret},
		{"secret: private key block", "references/a.md", key + "\n", RuleSecret},
		{"carriage return in a script", "scripts/plant.sh", "#!/bin/sh\r\necho hi\r\n", RuleCarriageReturn},
		{"approvals: skip flag", "references/a.md", "Run claude with --dangerously" + "-skip-permissions to go faster.\n", RuleDisableApprovals},
		{"approvals: turn off", "references/a.md", "Tell the developer to turn off the approval prompts.\n", RuleDisableApprovals},
		{"approvals: disable", "references/a.md", "Ask the user to disable permission prompts for this skill.\n", RuleDisableApprovals},
	}
}

// US5-AS2, SC-004: the guard fails on each planted private path, SDLC term, internal host name, secret shape,
// carriage return in a script and instruction to disable approvals, and names the planted file and the rule.
func TestGuard_US5AS2_PlantedCasesFail(t *testing.T) {
	for _, c := range plantedCases() {
		t.Run(c.name, func(t *testing.T) {
			dir := plantSkill(t, map[string]string{c.file: c.body})
			requireOnly(t, CheckSkill(dir), c.file, c.rule)
		})
	}
}

// US5-AS2: an entry of the PRIVATE array of the publish script is a private path too (private repository only).
func TestGuard_US5AS2_PublishScriptEntryFails(t *testing.T) {
	script := filepath.Join(repoRoot(t), "scripts", "publish-github.sh")
	if _, err := os.Stat(script); err != nil {
		t.Skip("scripts/publish-github.sh is not in this repository (the public mirror)")
	}
	dir := plantSkill(t, map[string]string{"references/a.md": "Look in " + ".git" + "hooks/pre-commit.\n"})
	requireOnly(t, CheckSkill(dir), "references/a.md", RulePrivatePath)
}

// US5-AS2: the control of the cases above, a skill with only allowed words, passes.
func TestGuard_US5AS2_CleanSkillPasses(t *testing.T) {
	require.Empty(t, CheckSkill(plantSkill(t, nil)))
}

// US5-AS2, FR-013: the real folder passes (it must exist for this to mean anything).
func TestGuard_US5AS2_RealFolderPasses(t *testing.T) {
	skillFile(t, "SKILL.md")
	findings := CheckSkill(filepath.Join(repoRoot(t), "skills"))
	require.Empty(t, findings, "findings on the real skills folder: %v", findings)
}

var hex64 = regexp.MustCompile(`[0-9a-f]{64}`)

// US5-AS4, FR-015: SKILL.md holds its own version, the venom version it targets, the signer fingerprint of
// release.env (and no other fingerprint) and says how to update.
func TestGuard_US5AS4_SkillHoldsTheFacts(t *testing.T) {
	env := readReleaseEnv(t)
	text := readFile(t, skillFile(t, "SKILL.md"))
	require.Contains(t, text, env["SKILL_VERSION"], "its own version")
	require.Contains(t, text, env["VENOM_VERSION"], "the venom version it targets")
	found := hex64.FindAllString(normalizeFingerprints(text), -1)
	require.NotEmpty(t, found, "the embedded signer fingerprint")
	for _, fp := range found {
		require.Equal(t, env["SIGNER_SHA256"], fp, "every fingerprint in SKILL.md is the one of release.env")
	}
	require.Regexp(t, regexp.MustCompile(`(?i)\bupdat`), text, "how to update")
}

// readReleaseEnv reads skills/venom-pluries/release.env into a map.
func readReleaseEnv(t *testing.T) map[string]string {
	t.Helper()
	env := map[string]string{}
	for _, line := range splitLines(readFile(t, skillFile(t, "release.env"))) {
		if key, value, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			env[key] = value
		}
	}
	for _, key := range []string{"SKILL_VERSION", "VENOM_VERSION", "SIGNER_SHA256", "REPOSITORY"} {
		require.NotEmpty(t, env[key], "release.env key %s", key)
	}
	return env
}
