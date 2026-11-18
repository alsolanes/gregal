#!/usr/bin/env python3
"""PreToolUse hook: keep Claude from weakening or bypassing the quality gates.

Reads the tool call as JSON on stdin. Edits to gate files become a permission
prompt for the human; shell commands that bypass the gates are refused, and
the reason is shown to Claude.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "scripts"))
from diff_guard import GATE_FILE

REFUSED = [
    (
        r"--no-verify|\bgit\s+commit\b[^|;&]*\s-[a-zA-Z]*n[a-zA-Z]*\b",
        "Do not skip the commit hooks: fix what they report.",
    ),
    (
        r"\bDIFF_GUARD_ALLOW\b|\bSKIP=|core\.hooksPath|pre-commit\s+uninstall",
        "Only a human may bypass a quality gate. Fix the cause, or stop and say what blocks you.",
    ),
    (
        r"\bgit\s+push\b[^|;&]*\s(?:--force|-[a-zA-Z]*f)\b",
        "Do not force-push. If it is really needed, ask the user to do it.",
    ),
]
NEEDS_HUMAN = [
    (
        r"\bgit\s+(?:reset\s+--hard|clean\s+-[a-zA-Z]*f|checkout\s+(?:--\s+)?\.(?:\s|$)"
        r"|restore\s+(?:-\S+\s+)*\.(?:\s|$))",
        "This discards uncommitted work.",
    ),
]


def decide(event: dict) -> tuple[str, str] | None:
    """Return ("deny" | "ask", reason) for a tool call, or None to let it through."""
    tool_input = event.get("tool_input") or {}
    if event.get("tool_name") in ("Bash", "PowerShell"):
        command = str(tool_input.get("command", ""))
        for decision, rules in (("deny", REFUSED), ("ask", NEEDS_HUMAN)):
            for pattern, reason in rules:
                if re.search(pattern, command):
                    return decision, reason
        return None
    path = str(tool_input.get("file_path") or tool_input.get("notebook_path") or "")
    path = path.replace("\\", "/")
    if GATE_FILE.search(path):
        return "ask", f"{path} is a quality gate: changing it is a human decision."
    return None


def main() -> int:
    verdict = decide(json.load(sys.stdin))
    if verdict:
        decision, reason = verdict
        output = {
            "hookEventName": "PreToolUse",
            "permissionDecision": decision,
            "permissionDecisionReason": reason,
        }
        print(json.dumps({"hookSpecificOutput": output}))
    return 0


if __name__ == "__main__":
    sys.exit(main())
