"""Verify CI selection, including safe defaults for unknown files and missing diffs."""

import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


spec = importlib.util.spec_from_file_location("ci_changes", Path(__file__).with_name("ci-changes.py"))
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


class CIChangesTest(unittest.TestCase):
    def test_job_selection(self):
        for paths, expected in [
            (["README.md", "docs/commands.md", "assets/demo.gif"], set()),
            (["cmd/schema/get.go"], set(ci.JOBS)),
            (["go.mod"], set(ci.JOBS)),
            (["go.sum"], set(ci.JOBS)),
            ([".github/workflows/go.yml"], set(ci.JOBS)),
            (["scripts/ci-changes.py"], set(ci.JOBS)),
            ([".github/workflows/release.yml"], {"quality", "packaging"}),
            ([".github/workflows/vulnerabilities.yml"], {"quality", "vulnerabilities"}),
            ([".goreleaser.yaml", "scripts/verify-release.py"], {"packaging"}),
            ([".golangci.yml"], {"quality"}),
            (["docker-compose.yml"], {"integration"}),
            (["internal/schema/testdata/example.json"], set(ci.JOBS)),
            ([".github/workflows/new.yml"], set(ci.JOBS)),
            (["docs/example.go"], set(ci.JOBS)),
            (["new-build-input"], set(ci.JOBS)),
            (["README.md", ".golangci.yml", ".goreleaser.yaml"], {"quality", "packaging"}),
            (None, set(ci.JOBS)),
        ]:
            with self.subTest(paths=paths):
                selected = {job for job, enabled in ci.select_jobs(paths).items() if enabled}
                self.assertEqual(selected, expected)

    def test_release_and_missing_diff_run_all_checks(self):
        self.assertIsNone(ci.changed_paths("push", "refs/tags/v2.1.0", {}))
        self.assertIsNone(ci.changed_paths("push", "refs/heads/main", {"before": "0" * 40, "after": "a" * 40}))
        self.assertIsNone(ci.changed_paths("pull_request", "refs/pull/1/merge", {}))
        self.assertIsNone(ci.changed_paths("workflow_dispatch", "refs/heads/main", {}))
        self.assertIsNone(ci.changed_paths("push", "refs/heads/main", {"before": "a" * 40, "after": "b" * 40}))

    def test_diff_includes_deleted_code_and_renames(self):
        with tempfile.TemporaryDirectory() as directory:
            def git(*args):
                return subprocess.check_output(["git", "-C", directory, *args], stderr=subprocess.PIPE).decode().strip()

            git("init")
            root = Path(directory)
            (root / "old.go").write_text("package main\n")
            git("add", ".")
            git("-c", "user.name=CI", "-c", "user.email=ci@example.com", "commit", "-m", "base")
            base = git("rev-parse", "HEAD")
            (root / "old.go").rename(root / "README.md")
            git("add", ".")
            git("-c", "user.name=CI", "-c", "user.email=ci@example.com", "commit", "-m", "rename")
            head = git("rev-parse", "HEAD")
            # The diff uses the checked-out repository, as it does in Actions.
            original = os.getcwd()
            try:
                os.chdir(directory)
                for event_name, event in [
                    ("push", {"before": base, "after": head}),
                    ("pull_request", {"pull_request": {"base": {"sha": base}, "head": {"sha": head}}}),
                ]:
                    paths = ci.changed_paths(event_name, "refs/heads/main", event)
                    self.assertEqual(set(paths), {"old.go", "README.md"})
                    self.assertTrue(all(ci.select_jobs(paths).values()))
            finally:
                os.chdir(original)


if __name__ == "__main__":
    unittest.main()
