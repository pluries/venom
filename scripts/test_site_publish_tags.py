#!/usr/bin/env python3
"""Tag tests of the publish job (decision D-027 of spec 016: the skill is installed at the tag `skill-v<version>` of the public fork).
PublishTags runs jenkins/publish-steps.sh against a throwaway private repository whose `github` remote is a local bare repository holding
one public commit, with private tags set as each test needs. A private tag named `skill-v<major>.<minor>.<patch>` that points at the
commit being published is pushed to the public fork as a tag of the snapshot commit, in the same atomic push as the branch; any other tag
is never pushed; a tag already on the public fork is never moved. The dry run says which tag it would push and pushes nothing.
Needs git, git-filter-repo, node and bash on PATH; otherwise the class is skipped and says which tool is missing.
Offline, standard library only. Run: python scripts/test_site_publish_tags.py
"""
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from test_site_publish_scan import (  # noqa: E402
    APPROVER, BASH, MISSING, SKIP_REASON, ScanRun, fixture_env, git_out, make_private_repo, make_public_remote, write,
)

TAG = "skill-v1.0.0"
OTHER_TAGS = ("v1.5.0-pluries.3", "skill-v1.0", "skill-v1.0.0-rc1", "skill-vx", "my-skill-v1.0.0")


def run_publish(command: str, private_tags: tuple[str, ...] = (), older_tags: tuple[str, ...] = (), public_tags: tuple[str, ...] = (),
                twice: bool = False) -> ScanRun:
    """Runs publish-steps.sh <command> in a temporary private repository. `older_tags` point at a commit before the published one;
    `public_tags` exist on the public fork before the run, on a commit that is not the snapshot."""
    with tempfile.TemporaryDirectory() as tmp_name:
        tmp = Path(tmp_name)
        env = fixture_env(tmp)
        bare = make_public_remote(tmp, env)
        root = tmp / "private"
        make_private_repo(root, env, bare, {})
        for name in older_tags:
            git_out(env, "tag", name, cwd=root)
        write(root / "src" / "later.txt", "a later commit\n")
        git_out(env, "add", ".", cwd=root)
        git_out(env, "commit", "-q", "-m", "a later private commit", cwd=root)
        for name in private_tags:
            git_out(env, "tag", name, cwd=root)
        for name in public_tags:
            git_out(env, "--git-dir", str(bare), "tag", name, "master")
        key = tmp / "deploy-key"
        write(key, "-".join(("fake", "key", "body", "not", "real", "456")) + "\n")
        run_env = dict(env)
        run_env.update({"PUBLISH_SSH_KEY": key.as_posix(), "IM_MIRROR_USER": "mirror-user-example", "PUBLISH_APPROVED_BY": APPROVER,
                        "PUBLISH_GITHUB_SCRIPT": (root / "scripts" / "publish-github.sh").as_posix()})
        refs_before = git_out(env, "--git-dir", str(bare), "for-each-ref", "--format=%(refname) %(objectname)")
        step = [BASH, (root / "jenkins" / "publish-steps.sh").as_posix(), command]
        completed = subprocess.run(step, cwd=root, env=run_env, capture_output=True, text=True, encoding="utf-8", timeout=240)
        if twice:
            completed = subprocess.run(step, cwd=root, env=run_env, capture_output=True, text=True, encoding="utf-8", timeout=240)
        result = ScanRun(completed, root, bare, refs_before, env)
        try:
            result.master_parent = git_out(env, "--git-dir", str(bare), "rev-parse", "master^").strip()
        except subprocess.CalledProcessError:
            result.master_parent = ""
        return result


def refs(result: ScanRun) -> dict[str, str]:
    return dict(line.split(" ", 1) for line in result.refs_after.splitlines())


@unittest.skipIf(MISSING, SKIP_REASON)
class PublishTags(unittest.TestCase):
    def test_a_skill_tag_at_the_published_commit_is_pushed_as_a_tag_of_the_snapshot(self):
        result = run_publish("publish", private_tags=(TAG,))
        self.assertEqual(result.returncode, 0, result.stderr)
        public = refs(result)
        self.assertIn(f"refs/tags/{TAG}", public, "the skill tag did not reach the public fork")
        self.assertEqual(public[f"refs/tags/{TAG}"], public["refs/heads/master"], "the tag is not on the snapshot commit")
        self.assertIn(TAG, result.stdout + result.report)

    def test_the_dry_run_names_the_tag_and_pushes_nothing(self):
        result = run_publish("dry-run", private_tags=(TAG,))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.refs_after, result.refs_before)
        self.assertIn(f"would also push tag {TAG}", result.report)

    def test_other_tags_are_never_pushed(self):
        result = run_publish("publish", private_tags=OTHER_TAGS)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([r for r in refs(result) if r.startswith("refs/tags/")], [])

    def test_a_skill_tag_on_an_older_commit_is_left_out_and_said_so(self):
        result = run_publish("publish", older_tags=(TAG,))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([r for r in refs(result) if r.startswith("refs/tags/")], [])
        self.assertIn(f"{TAG} does not point at the published commit", result.stdout + result.report)

    def test_a_public_tag_of_the_same_name_on_another_commit_stops_the_run_before_any_push(self):
        result = run_publish("publish", private_tags=(TAG,), public_tags=(TAG,))
        self.assertNotEqual(result.returncode, 0, "the run moved or accepted an existing public tag")
        self.assertEqual(result.refs_after, result.refs_before)
        self.assertIn(f"{TAG} already exists on the public fork", result.stdout + result.stderr + result.report)

    def test_a_second_publish_of_the_same_tree_leaves_the_tag_on_the_first_snapshot(self):
        result = run_publish("publish", private_tags=(TAG,), twice=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(refs(result)[f"refs/tags/{TAG}"], result.master_parent, "the tag moved with the second snapshot")
        self.assertIn(f"{TAG} is already published", result.stdout + result.report)


if __name__ == "__main__":
    unittest.main()
