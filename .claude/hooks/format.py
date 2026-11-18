#!/usr/bin/env python3
"""PostToolUse hook: gofmt the Go file Claude just wrote. Best effort, never blocks.

Only formats; everything else is reported by scripts/check.py, so a half-written
change is not "fixed" by removing an import that is about to be used.
"""

from __future__ import annotations

import contextlib
import json
import shutil
import subprocess
import sys
from pathlib import Path

TIMEOUT_SECONDS = 30


def main() -> int:
    tool_input = json.load(sys.stdin).get("tool_input") or {}
    path = Path(str(tool_input.get("file_path", "")))
    gofmt = shutil.which("gofmt")
    if path.suffix == ".go" and path.is_file() and gofmt:
        # Formatting is a convenience; scripts/check.py is what enforces it.
        with contextlib.suppress(OSError, subprocess.TimeoutExpired):
            subprocess.run(
                [gofmt, "-w", str(path)], capture_output=True, timeout=TIMEOUT_SECONDS, check=False
            )
    return 0


if __name__ == "__main__":
    sys.exit(main())
