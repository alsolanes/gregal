#!/usr/bin/env python3
"""Diff guard: deterministic checks on what a change adds or removes.

It never judges existing code, only the diff, so it can afford to be strict:

  suppressions   new lint or type-checker suppressions
  skipped-tests  new skipped or focused tests
  tests-removed  the change removes more tests than it adds
  junk-files     new files such as parser_v2.py, utils_old.ts, IMPLEMENTATION_SUMMARY.md
  size           too many changed lines of non-test code for one change
  gates (warn)   the change touches the quality gates or the dependency manifests

    uv run scripts/diff_guard.py              working tree vs the base branch (see below)
    uv run scripts/diff_guard.py --staged     staged changes only (pre-commit)
    uv run scripts/diff_guard.py --base REF   working tree vs the merge-base with REF (CI)

The base branch is the current branch's upstream (origin/assistent-v2 for
assistent-v2; task branches get the branch they started from as upstream),
falling back to origin/main.

A human who has looked at a finding can let it through for one run with
DIFF_GUARD_ALLOW=size,suppressions (comma-separated rule names).
"""

from __future__ import annotations

import argparse
import io
import os
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
MAX_CODE_LINES = 400
MAX_UNTRACKED_BYTES = 1_000_000
EMPTY_TREE = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
DEFAULT_BRANCHES = ("@{upstream}", "origin/main", "main")
SELF = {"scripts/diff_guard.py", "scripts/tests/test_diff_guard.py"}
WARN_ONLY = {"gates"}

CODE_FILE = re.compile(r"\.(?:pyi?|[cm]?[jt]sx?|go|kts?)$")
TEST_FILE = re.compile(
    r"(?:^|/)(?:tests?|__tests__)/"
    r"|(?:^|/)(?:test_[^/]*|[^/]*_test|conftest)\.py$"
    r"|\.(?:test|spec)\.[cm]?[jt]sx?$"
    r"|_test\.go$"
    r"|(?:^|/)src/(?:test|androidTest)/"
)
NOT_COUNTED = re.compile(
    r"(?:^|/)(?:package-lock\.json|go\.sum|tasks/.*)$"
    r"|(?:^|/)(?:vendor|testdata|fixtures)/"
    r"|\.(?:md|lock|snap|svg|golden|min\.js)$"
)
SUPPRESSION = re.compile(
    r"#\s*(?:noqa|nosec)\b"
    r"|#\s*(?:type|pyright):\s*ignore"
    r"|#\s*pylint:\s*disable"
    r"|pragma:\s*no\s*cover"
    r"|@ts-(?:ignore|expect-error|nocheck)"
    r"|eslint-(?:disable)"
    r"|biome-(?:ignore)"
    r"|(?:istanbul|v8|c8)\s+ignore"
    r"|//\s*nolint"
    r"|@Suppress(?:Warnings)?\b"
)
SKIPPED_TEST = re.compile(
    r"pytest\.(?:mark\.)?(?:skip|xfail)"
    r"|unittest\.skip"
    r"|\b(?:it|test|describe)\.(?:skip|only|todo|fails)\b"
    r"|\b[xf](?:it|describe)\s*\("
    r"|\bt\.Skip(?:f|Now)?\("
    r"|@(?:Ignore|Disabled)\b"
)
TEST_DEF = re.compile(
    r"^\s*(?:async\s+)?def\s+test_"
    r"|^\s*(?:it|test)(?:\.each)?\s*\("
    r"|^func\s+Test\w*\("
    r"|^\s*@Test\b"
)
JUNK_FILE = re.compile(
    r"[_.-](?:v\d+|new|old|backup|bak|copy|final|tmp|temp|orig|fixed|improved|enhanced|refactored"
    r"|nou|antic|vell|copia)"
    r"\.[^./]+$"
    r"|\.(?:bak|orig|tmp)$"
    r"|(?:^|/)[A-Z0-9_-]*(?:SUMMARY|IMPLEMENTATION|REPORT|ANALYSIS)[A-Z0-9_-]*\.md$"
)
# Also used by .claude/hooks/guard.py to decide which edits need a human.
GATE_FILE = re.compile(
    r"(?:^|/)(?:"
    r"\.claude/(?:settings\.json|hooks/.*)"
    r"|scripts/(?:check|diff_guard)\.py|scripts/audit-public\.cjs"
    r"|\.pre-commit-config\.yaml"
    r"|\.github/workflows/.*"
    r"|\.golangci\.ya?ml|tsconfig[^/]*\.json|ruff\.toml|pyrightconfig\.json|biome\.jsonc?"
    r")$"
)
MANIFEST = re.compile(
    r"(?:^|/)(?:go\.mod|pyproject\.toml|package\.json|build\.gradle\.kts|libs\.versions\.toml)$"
)

ADVICE = {
    "suppressions": "fix what the tool reports instead of silencing it.",
    "skipped-tests": "make the test pass, or remove it on purpose; never skip or focus tests.",
    "tests-removed": "if that behaviour really went away, say so and let a human approve it.",
    "junk-files": "change the existing file instead of adding a copy; no summary documents.",
    "size": "too much for one change. Stop and split the work into smaller tasks.",
    "gates": "quality gates or dependencies changed; a human has to review this part.",
}


@dataclass
class Change:
    added: list[str] = field(default_factory=list)
    removed: list[str] = field(default_factory=list)
    new: bool = False


@dataclass
class Finding:
    rule: str
    detail: str


def parse_diff(diff: str) -> dict[str, Change]:
    """Parse `git diff --no-prefix --unified=0` output into per-file changes."""
    changes: dict[str, Change] = {}
    current = Change()
    old_path = ""
    in_header = False
    for line in diff.splitlines():
        if line.startswith("diff --git "):
            in_header = True
        elif line.startswith("@@"):
            in_header = False
        elif in_header and line.startswith("--- "):
            old_path = line[4:].rstrip("\t")
        elif in_header and line.startswith("+++ "):
            new_path = line[4:].rstrip("\t")
            path = old_path if new_path == "/dev/null" else new_path
            current = changes.setdefault(path, Change(new=old_path == "/dev/null"))
        elif not in_header and line.startswith("+"):
            current.added.append(line[1:])
        elif not in_header and line.startswith("-"):
            current.removed.append(line[1:])
    return changes


def check_file(path: str, change: Change) -> list[Finding]:
    findings: list[Finding] = []
    if change.new and JUNK_FILE.search(path):
        findings.append(Finding("junk-files", path))
    if GATE_FILE.search(path) or MANIFEST.search(path):
        findings.append(Finding("gates", path))
    if not CODE_FILE.search(path):
        return findings
    is_test = bool(TEST_FILE.search(path))
    for line in change.added:
        if SUPPRESSION.search(line):
            findings.append(Finding("suppressions", f"{path}: {line.strip()}"))
        if is_test and SKIPPED_TEST.search(line):
            findings.append(Finding("skipped-tests", f"{path}: {line.strip()}"))
    return findings


def count(pattern: re.Pattern[str], lines: list[str]) -> int:
    return sum(1 for line in lines if pattern.search(line))


def evaluate(changes: dict[str, Change]) -> tuple[list[Finding], int]:
    """Return the findings and the number of changed lines of non-test code."""
    findings: list[Finding] = []
    code_lines = 0
    test_delta = 0
    for path, change in sorted(changes.items()):
        if path in SELF:
            continue
        findings += check_file(path, change)
        if TEST_FILE.search(path):
            test_delta += count(TEST_DEF, change.added) - count(TEST_DEF, change.removed)
        elif not NOT_COUNTED.search(path):
            code_lines += sum(1 for line in change.added + change.removed if line.strip())
    if test_delta < 0:
        findings.append(Finding("tests-removed", f"{-test_delta} more tests removed than added"))
    if code_lines > MAX_CODE_LINES:
        findings.append(Finding("size", f"{code_lines} changed lines (limit {MAX_CODE_LINES})"))
    return findings, code_lines


def git(*args: str) -> str | None:
    """Stdout of a git command run at the repo root, or None if it failed."""
    proc = subprocess.run(
        ["git", "-c", "core.quotepath=off", *args],
        cwd=ROOT,
        capture_output=True,
        encoding="utf-8",
        errors="replace",
        check=False,
    )
    return proc.stdout if proc.returncode == 0 else None


def resolve_base(ref: str | None) -> str | None:
    """Commit to compare the working tree against; None if `ref` cannot be found."""
    if git("rev-parse", "--verify", "--quiet", "HEAD") is None:
        return EMPTY_TREE
    for candidate in [ref] if ref else DEFAULT_BRANCHES:
        base = git("merge-base", candidate, "HEAD")
        if base:
            return base.strip()
    return None if ref else "HEAD"


def collect(staged: bool, ref: str | None) -> dict[str, Change] | None:
    diff = ["diff", "--no-prefix", "--unified=0", "--no-color", "--no-ext-diff"]
    if staged:
        return parse_diff(git(*diff, "--cached") or "")
    base = resolve_base(ref)
    if base is None:
        return None
    changes = parse_diff(git(*diff, base) or "")
    for path in (git("ls-files", "--others", "--exclude-standard") or "").splitlines():
        file = ROOT / path
        if file.is_file() and file.stat().st_size <= MAX_UNTRACKED_BYTES:
            lines = file.read_text(encoding="utf-8", errors="replace").splitlines()
            changes[path] = Change(added=lines, new=True)
    return changes


def main() -> int:
    if isinstance(sys.stdout, io.TextIOWrapper):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument("--staged", action="store_true", help="check staged changes only")
    parser.add_argument("--base", metavar="REF", help="compare against the merge-base with REF")
    args = parser.parse_args()

    if git("rev-parse", "--git-dir") is None:
        print("diff guard: not a git repository, nothing to check")
        return 0
    changes = collect(args.staged, args.base)
    if changes is None:
        print(f"diff guard: cannot find {args.base!r} (shallow clone? fetch it first)")
        return 2

    allowed = set(os.environ.get("DIFF_GUARD_ALLOW", "").split(","))
    findings, code_lines = evaluate(changes)
    failed = False
    for finding in findings:
        soft = finding.rule in WARN_ONLY or finding.rule in allowed
        failed = failed or not soft
        print(f"{'WARN' if soft else 'FAIL'}  {finding.rule}: {finding.detail}")
    for rule in sorted({finding.rule for finding in findings}):
        print(f"  -> {rule}: {ADVICE[rule]}")
    print(f"diff guard: {len(changes)} files, {code_lines} changed lines of non-test code")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
