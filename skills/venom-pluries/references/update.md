# Update this skill, and the venom it installs

This skill has its own version and targets one venom version; both are in `release.env` and at the top of `SKILL.md`.

## Update the skill

Install a tagged version of the skill, never the moving default branch. The sure way, which shows exactly what is copied:

```bash
git clone --depth 1 --branch <tag> https://github.com/pluries/venom venom-skill
```

Then copy the folder `skills/venom-pluries/` of that clone into `.claude/skills/venom-pluries/` of the project, after showing the
developer the difference with the copy that is there, and delete the clone.

The one-command install of the skill tool works from the public repository when it names a tag. This is the form that was run against
the public fork and installed a folder identical to the tag:

```bash
DISABLE_TELEMETRY=1 DO_NOT_TRACK=1 npx skills add pluries/venom@<tag> --skill venom-pluries -a claude-code -y --copy
```

Show the developer this command and let Claude Code ask for approval. The tool sends anonymous usage data unless the two switches are set.
It writes the skill into the project and a `skills-lock.json` that records the commit of the tag; check that the `Skill version` row of the
installed `SKILL.md` is the version of the tag. Never run it without a tag or a commit.

A new skill version changes `release.env`. Compare its `SIGNER_SHA256` with the one the project's copy of the scripts uses
(`tools/venom/release.env`). If they differ, the signing certificate was renewed: copy the new scripts and `release.env` into
`tools/venom/` with the difference shown and approval.

## Update the venom of a project

1. Choose the new version with the developer. Never move silently to the newest release; a release can be withdrawn.
2. Write it to `.venom-version` as a shown change.
3. Run the install command of `install.md`; the same checks run again.
