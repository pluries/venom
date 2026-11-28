The `venom-pluries` skill lets Claude Code set Venom Pluries up in your project. It installs a checked release, writes a first test suite, adds the step to your build and checks recipes for web and mobile pages. You approve each command it wants to run.

## Install it

Name a release tag. Never install from the moving default branch.

```bash
$ npx skills add pluries/venom@skill-v1.0.1 --skill venom-pluries
```

The tool asks which agent to install for; choose Claude Code. It puts the skill in the `venom-pluries` (under `skills`, inside `.claude`) folder of your project and writes a `skills-lock.json` next to it. The tool sends anonymous usage data unless you set `DISABLE_TELEMETRY=1` (or `DO_NOT_TRACK=1`) first.

Skills run with the permissions of your agent. Read the `SKILL.md` in the installed folder before you use it. The skill shows each command and waits for your approval, and it never asks you to turn approvals off.

Prefer to see what is copied? Clone the tag and copy the folder yourself:

```bash
$ git clone --depth 1 --branch skill-v1.0.1 https://github.com/pluries/venom venom-skill
```

Copy the `skills/venom-pluries/` folder of that clone into the `venom-pluries` (under `skills`, inside `.claude`) folder of your project, then delete the clone.

## Check that it is there

Start Claude Code in the project and ask what skills it has: `venom-pluries` is listed. The `Skill version` row at the top of its `SKILL.md` names its version, and it matches the tag you chose.

## Use it

Ask in your own words, for example "set up venom in this project and write a first test", "run it in GitHub Actions" or "check my recipe". The skill installs the venom version it targets (written in its `SKILL.md` and in your `.venom-version`), checks the checksum, the signature and the signer before it runs anything, and tells you what it did and what it did not do.

## What the maintainers did not run

The GitLab CI file was never run on a real GitLab server. The skill says so, and the first run is yours. For mobile recipes the skill does not install or start a device host: you provide an Appium server and an emulator or a device.

## Update it

Install the new tag the same way. If the `SIGNER_SHA256` in `release.env` differs from the one your project uses, the signing certificate was renewed. The skill then tells you to copy the new scripts, with the difference shown.
