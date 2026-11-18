#!/usr/bin/env python3
"""Run gregal's deterministic quality checks on what a change touches. No AI involved.

    uv run scripts/check.py              everything the change touches
    uv run scripts/check.py --fast       skip the slow steps (tests)
    uv run scripts/check.py --staged     look at staged changes only (pre-commit)
    uv run scripts/check.py --base REF   compare against REF instead of the upstream (CI)

The checks are scoped to the diff on purpose: the existing code base has
known gofmt, golangci-lint and test debt, and a gate that is red before
anyone touches anything gets ignored. Fixing that debt is ordinary work,
done in its own tasks. The full suites still run in CI (quality.yml).

  Go (*.go)                gofmt, go vet, golangci-lint (new issues only), go test
  internal/web, scripts,   the node --test suites from quality.yml
  desktop (*.js, *.cjs, ...)
  python/                  the client's unittest suite
  vscode-gregal/           typecheck and tests
  android/                 not run here (Gradle is too slow); CI runs it
  everything               the diff guard (scripts/diff_guard.py)

Exits 0 only if every step passes.
"""

from __future__ import annotations

import argparse
import io
import os
import re
import shutil
import subprocess
import sys
import time
from collections.abc import Callable
from dataclasses import dataclass, field
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from diff_guard import collect, resolve_base

ROOT = Path(__file__).resolve().parent.parent
GOLANGCI = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0"
NODE_SUITES = ["internal/web/*.test.js", "internal/web/app/*.test.cjs", "scripts/*.test.cjs"]
WEB_FILE = re.compile(r"^(?:internal/web|scripts|desktop)/.*\.(?:[cm]?js|css|html)$")
MAX_OUTPUT_LINES = 120
ENV = {**os.environ, "NO_COLOR": "1", "FORCE_COLOR": "0"}


@dataclass
class Step:
    name: str
    cmd: list[str] = field(default_factory=list)
    cwd: Path = ROOT
    slow: bool = False
    check: Callable[[], tuple[bool, str]] | None = None  # in-process step
    show_output: bool = False  # print the output even when the step passes


def tool(name: str) -> str | None:
    """Path of an executable on PATH or in Go's bin folder."""
    gobin = Path(os.environ.get("GOPATH") or Path.home() / "go") / "bin"
    return shutil.which(name) or shutil.which(name, path=str(gobin))


def gofmt_check(files: list[str]) -> Callable[[], tuple[bool, str]]:
    """gofmt the files as git stores them (LF), so a CRLF checkout is not a finding."""

    def check() -> tuple[bool, str]:
        unformatted = []
        for name in files:
            source = (ROOT / name).read_bytes().replace(b"\r\n", b"\n")
            proc = subprocess.run(["gofmt"], input=source, capture_output=True, check=False)
            if proc.returncode != 0 or proc.stdout != source:
                unformatted.append(name)
        output = "".join(f"not gofmt-formatted: {name}\n" for name in unformatted)
        return not unformatted, output + ("Fix: gofmt -w <file>\n" if unformatted else "")

    return check


def go_packages(changed: list[str]) -> list[str]:
    """Go packages (./dir) that contain a changed .go file; ./... if the module changed."""
    if {"go.mod", "go.sum"} & set(changed):
        return ["./..."]
    proc = subprocess.run(
        ["go", "list", "-f", "{{.Dir}}", "./..."],
        cwd=ROOT,
        capture_output=True,
        encoding="utf-8",
        check=False,
    )
    known = {Path(line).resolve() for line in proc.stdout.splitlines()}
    dirs = {(ROOT / name).parent.resolve() for name in changed if name.endswith(".go")}
    return sorted(f"./{d.relative_to(ROOT).as_posix()}" for d in dirs if d in known)


def go_steps(changed: list[str], base: str) -> list[Step]:
    go_files = [n for n in changed if n.endswith(".go") and (ROOT / n).is_file()]
    packages = go_packages(changed)
    steps = [Step("gofmt", check=gofmt_check(go_files))] if go_files else []
    if packages:
        lint = [tool("golangci-lint") or "go"]
        if lint[0] == "go":
            lint += ["run", GOLANGCI]
        lint += ["run", f"--new-from-rev={base}", *packages]
        steps += [
            Step("go vet", ["go", "vet", *packages]),
            Step("golangci-lint (new issues)", lint),
            Step("go test", ["go", "test", *packages], slow=True),
        ]
    return steps


def other_steps(changed: list[str]) -> list[Step]:
    steps: list[Step] = []
    if any(WEB_FILE.match(name) for name in changed):
        suites = list(NODE_SUITES)
        if (ROOT / "desktop" / "node_modules").exists():
            suites.append("desktop/*.test.js")
        cmd = ["node", "--experimental-vm-modules", "--test", *suites]
        steps.append(Step("node tests", cmd, slow=True))
    if any(name.startswith("python/") for name in changed):
        cmd = ["uv", "run", "--no-project", "--quiet", "--with", "./python"]
        cmd += ["python", "-m", "unittest", "discover", "-s", "python/tests"]
        steps.append(Step("python client tests", cmd, slow=True))
    if any(name.startswith("vscode-gregal/") for name in changed):
        vscode = ROOT / "vscode-gregal"
        if not (vscode / "node_modules").exists():
            steps.append(Step("vscode: install", ["npm", "ci", "--no-audit", "--no-fund"], vscode))
        steps.append(Step("vscode: typecheck", ["npm", "run", "--silent", "typecheck"], vscode))
        steps.append(Step("vscode: test", ["npm", "test", "--silent"], vscode, slow=True))
    if any(name.startswith("android/") for name in changed):
        print("NOTE  android changed: run `./gradlew testDebugUnitTest` in android/ (CI does).")
    return steps


def find_steps(staged: bool, ref: str | None) -> list[Step]:
    base = "HEAD" if staged else resolve_base(ref)
    changes = collect(staged, ref)
    if base is None or changes is None:
        sys.exit(f"check: cannot find the base {ref!r} (shallow clone? fetch it first)")
    changed = sorted(changes)
    guard = [sys.executable, str(ROOT / "scripts" / "diff_guard.py")]
    guard += ["--staged"] if staged else ["--base", ref] if ref else []
    return [
        *go_steps(changed, base),
        *other_steps(changed),
        Step("diff guard", guard, show_output=True),
    ]


def execute(step: Step) -> tuple[bool, str]:
    if step.check:
        return step.check()
    executable = shutil.which(step.cmd[0])
    if executable is None:
        return False, f"`{step.cmd[0]}` is not installed\n"
    proc = subprocess.run(
        [executable, *step.cmd[1:]],
        cwd=step.cwd,
        env=ENV,
        stdin=subprocess.DEVNULL,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        encoding="utf-8",
        errors="replace",
        check=False,
    )
    return proc.returncode == 0, proc.stdout


def run(step: Step) -> bool:
    """Run one step, print its result line, and return whether it passed."""
    start = time.monotonic()
    passed, output = execute(step)
    print(f"{'PASS' if passed else 'FAIL'}  {step.name}  ({time.monotonic() - start:.1f}s)")
    if not passed or step.show_output:
        lines = output.strip().splitlines()
        if len(lines) > MAX_OUTPUT_LINES:
            cut = len(lines) - MAX_OUTPUT_LINES
            lines = [f"... {cut} lines cut ...", *lines[-MAX_OUTPUT_LINES:]]
        print("".join(f"    {line}\n" for line in lines), end="")
    return passed


def main() -> int:
    if isinstance(sys.stdout, io.TextIOWrapper):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument("--fast", action="store_true", help="skip the slow steps (tests)")
    parser.add_argument("--staged", action="store_true", help="check staged changes only")
    parser.add_argument("--base", metavar="REF", help="compare against the merge-base with REF")
    args = parser.parse_args()

    steps = [s for s in find_steps(args.staged, args.base) if not (args.fast and s.slow)]
    failed = [step.name for step in steps if not run(step)]
    if failed:
        print(f"\n{len(failed)} of {len(steps)} checks FAILED")
        return 1
    print(f"\nAll {len(steps)} checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
