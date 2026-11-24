package skillcheck

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	shellArray  = regexp.MustCompile(`(?s)PRIVATE=\((.*?)\)`)
	pythonTuple = regexp.MustCompile(`(?s)PRIVATE_PATHS\s*=\s*\((.*?)\)`)
	quotedItem  = regexp.MustCompile(`"([^"]+)"`)
)

// shellEntries reads the PRIVATE=( ... ) array of the publish script: words separated by blanks and line breaks.
func shellEntries(text string) []string {
	m := shellArray.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	return strings.Fields(m[1])
}

// pythonEntries reads the quoted strings of the PRIVATE_PATHS tuple of build_site.py.
func pythonEntries(text string) []string {
	m := pythonTuple.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	var out []string
	for _, q := range quotedItem.FindAllStringSubmatch(m[1], -1) {
		out = append(out, q[1])
	}
	return out
}

// touchesSkills is true for an entry that is skills, a parent of it or a child of it.
func touchesSkills(entry string) bool {
	e := strings.Trim(entry, "/")
	return e == "skills" || strings.HasPrefix(e, "skills/") || e == "." || e == ""
}

// optionalFile reads a file of the private repository and skips when it is absent (the public mirror).
func optionalFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Skipf("%s is not in this repository (the public mirror)", rel)
	}
	return string(b)
}

// US5-AS3, FR-014: no entry of the PRIVATE array of the publish script, nor of the PRIVATE_PATHS tuple of the
// site build, is skills or a parent or child of it, so the folder is in the snapshot pushed to the public fork.
func TestPublish_US5AS3_SkillsIsNotPrivate(t *testing.T) {
	cases := []struct {
		name, file string
		entries    func(string) []string
	}{
		{"scripts/publish-github.sh", "scripts/publish-github.sh", shellEntries},
		{"scripts/build_site.py", "scripts/build_site.py", pythonEntries},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			entries := c.entries(optionalFile(t, c.file))
			require.NotEmpty(t, entries, "the private list must be found in %s", c.file)
			for _, e := range entries {
				require.False(t, touchesSkills(e), "%s lists %q, which would keep skills/ out of the public fork", c.file, e)
			}
		})
	}
}

const detectorScript = `
const fs = require('node:fs'), path = require('node:path');
const [root, dir] = process.argv.slice(1);
const { findSecret, isFixturePath } = require(path.join(root, '.claude/skills/cst-g-commit/scripts/secret-patterns.js'));
const out = { files: 0, findings: [], fixtures: [] };
(function walk(d) {
  for (const e of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, e.name);
    if (e.isDirectory()) { walk(p); continue; }
    out.files++;
    const rel = path.relative(root, p).split(path.sep).join('/');
    if (isFixturePath(rel)) out.fixtures.push(rel);
    fs.readFileSync(p, 'utf8').split(/\r?\n/).forEach((l, i) => {
      const f = findSecret(l);
      if (f) out.findings.push(rel + ':' + (i + 1) + ': ' + f.kind);
    });
  }
})(dir);
process.stdout.write(JSON.stringify(out));
`

type detectorReport struct {
	Files    int      `json:"files"`
	Findings []string `json:"findings"`
	Fixtures []string `json:"fixtures"`
}

// scanWithDetector runs the real secret detector of the publish scan, through node, over every file of dir.
func scanWithDetector(t *testing.T, dir string) detectorReport {
	t.Helper()
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "cst-g-commit", "scripts", "secret-patterns.js")); err != nil {
		t.Skip("the secret detector is not in this repository (the public mirror)")
	}
	requireTools(t, "node")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "-e", detectorScript, root, dir)
	out, err := cmd.Output()
	require.NoError(t, err, "node detector")
	var rep detectorReport
	require.NoError(t, json.Unmarshal(out, &rep))
	return rep
}

// US5-AS3: the real detector reads every file of skills/, reports nothing and does not treat the path as a fixture.
func TestPublish_US5AS3_DetectorReadsSkillsAndFindsNothing(t *testing.T) {
	rep := scanWithDetector(t, filepath.Join(repoRoot(t), "skills"))
	require.Positive(t, rep.Files, "the detector read no file under skills/")
	require.Empty(t, rep.Findings)
	require.Empty(t, rep.Fixtures, "skills/ must not be exempted as a fixture path")
}

// US5-AS3: the control, the same detector does find a planted secret in a skill-shaped folder.
func TestPublish_US5AS3_DetectorFindsAPlantedSecret(t *testing.T) {
	dir := plantSkill(t, map[string]string{"references/a.md": "token gh" + "p_" + strings.Repeat("a1B2", 9) + "\n"})
	rep := scanWithDetector(t, dir)
	require.Len(t, rep.Findings, 1)
}
