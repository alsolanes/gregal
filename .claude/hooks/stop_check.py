#!/usr/bin/env python3
"""Stop / SubagentStop hook: Claude may not finish while the checks fail.

Runs scripts/check.py in the repo (or worktree) Claude is working in. On
failure it exits 2, which keeps Claude going with the check output as
feedback. After MAX_BLOCKS failed attempts in a row it lets Claude stop and
warns the human instead. Nothing runs if the tree is unchanged since the last
pass, so turns that only answer a question cost nothing.
"""

from __future__ import annotations

import hashlib
import json
import os
import subprocess
import sys
from pathlib import Path

MAX_BLOCKS = 2
MAX_FEEDBACK_CHARS = 8000
CHECKED_AGENTS = {"implementer"}  # subagents that must leave the checks green


def git(root: Path, *args: str) -> str:
    proc = subprocess.run(
        ["git", *args],
        cwd=root,
        capture_output=True,
        encoding="utf-8",
        errors="replace",
        check=False,
    )
    return proc.stdout


def fingerprint(root: Path) -> str:
    """Hash of everything the checks look at: the commit plus all uncommitted work."""
    parts = [
        git(root, "rev-parse", "HEAD"),
        git(root, "status", "--porcelain"),
        git(root, "diff", "HEAD"),
    ]
    for name in git(root, "ls-files", "--others", "--exclude-standard").splitlines():
        file = root / name
        if file.is_file():
            parts.append(f"{name}:{file.stat().st_mtime_ns}")
    return hashlib.sha256("\n".join(parts).encode()).hexdigest()


def main() -> int:
    event = json.load(sys.stdin)
    is_subagent = event.get("hook_event_name") == "SubagentStop"
    if is_subagent and event.get("agent_type") not in CHECKED_AGENTS:
        return 0
    top = git(Path(event.get("cwd") or "."), "rev-parse", "--show-toplevel").strip()
    if not top or not (Path(top) / "scripts" / "check.py").exists():
        return 0
    root = Path(top)
    state_file = root / ".claude" / ".check-state.json"
    state = json.loads(state_file.read_text(encoding="utf-8")) if state_file.exists() else {}
    current = fingerprint(root)
    if state.get("passed") == current:
        return 0

    proc = subprocess.run(
        [sys.executable, str(root / "scripts" / "check.py")],
        cwd=root,
        # A diff that is too big is not something Claude can fix by trying again.
        env={**os.environ, "DIFF_GUARD_ALLOW": "size"},
        capture_output=True,
        encoding="utf-8",
        errors="replace",
        check=False,
    )
    if proc.returncode == 0:
        state_file.write_text(json.dumps({"passed": current}), encoding="utf-8")
        return 0

    blocks = state.get("blocks", 0) if event.get("stop_hook_active") else 0
    if blocks >= MAX_BLOCKS:
        state_file.write_text(json.dumps({"blocks": 0}), encoding="utf-8")
        warning = "Checks still FAIL after repeated attempts. Run: uv run scripts/check.py"
        print(json.dumps({"systemMessage": warning}))
        return 0
    state_file.write_text(json.dumps({"blocks": blocks + 1}), encoding="utf-8")
    print(proc.stdout[-MAX_FEEDBACK_CHARS:], file=sys.stderr)
    print(
        "The checks fail, so the work is not finished. Fix the causes; do not silence them. "
        "If you cannot fix one properly, say exactly which check fails and why, then stop.",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    sys.exit(main())
