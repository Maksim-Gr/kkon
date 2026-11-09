"""Select CI jobs from changed paths; unknown changes run the full validation."""

import json
import os
from pathlib import Path
import re
import subprocess


JOBS = ("quality", "unit", "integration", "packaging", "vulnerabilities")


def select_jobs(paths):
    """None means a release or an unavailable diff, which requires all checks."""
    selected = set()
    if paths is None:
        selected.update(JOBS)
    else:
        for path in paths:
            if path.endswith(".go") or path in {
                "go.mod", "go.sum", ".github/workflows/go.yml",
                "scripts/ci-changes.py", "scripts/test_ci_changes.py",
            }:
                selected.update(JOBS)
            elif path == ".github/workflows/release.yml":
                selected.update(("quality", "packaging"))
            elif path == ".github/workflows/vulnerabilities.yml":
                selected.update(("quality", "vulnerabilities"))
            elif path in {".goreleaser.yaml", "scripts/verify-release.py"}:
                selected.add("packaging")
            elif path in {".golangci.yml", ".golangci.yaml"}:
                selected.add("quality")
            elif path == "docker-compose.yml":
                selected.add("integration")
            elif path.startswith(".github/workflows/"):
                selected.update(JOBS)
            elif (
                path.endswith(".md")
                or path.startswith(("docs/", "assets/", "img/", ".github/ISSUE_TEMPLATE/"))
                or path in {"LICENSE", ".gitignore", ".gitattributes", ".github/CODEOWNERS"}
            ):
                continue
            else:
                # New fixtures, build inputs, and configuration must not silently skip CI.
                selected.update(JOBS)
    return {job: job in selected for job in JOBS}


def changed_paths(event_name, ref, event):
    """Compare PR branches or all commits in a push, including deleted paths."""
    if ref.startswith("refs/tags/"):
        return None
    if event_name == "pull_request":
        base = event.get("pull_request", {}).get("base", {}).get("sha", "")
        head = event.get("pull_request", {}).get("head", {}).get("sha", "")
        separator = "..."
    elif event_name == "push":
        base, head = event.get("before", ""), event.get("after", "")
        separator = ".."
    else:
        return None
    if any(not re.fullmatch(r"[0-9a-f]{40,64}", sha) or set(sha) == {"0"} for sha in (base, head)):
        return None
    try:
        result = subprocess.run(
            ["git", "diff", "--name-only", "--no-renames", "-z", base + separator + head],
            check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        )
    except subprocess.CalledProcessError:
        return None
    return [os.fsdecode(path) for path in result.stdout.split(b"\0") if path]


def main():
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    jobs = select_jobs(changed_paths(os.environ["GITHUB_EVENT_NAME"], os.environ["GITHUB_REF"], event))
    with Path(os.environ["GITHUB_OUTPUT"]).open("a") as output:
        for job, enabled in jobs.items():
            print(f"{job}={str(enabled).lower()}", file=output)
    with Path(os.environ["GITHUB_STEP_SUMMARY"]).open("a") as summary:
        print("| Validation | Relevant changes |\n| --- | --- |", file=summary)
        for job, enabled in jobs.items():
            print(f"| {job} | {'yes' if enabled else 'no'} |", file=summary)


if __name__ == "__main__":
    main()
