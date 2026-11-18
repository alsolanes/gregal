#!/usr/bin/env python3
"""Task files in tasks/: one markdown file per task, named NNN-slug.md.

    uv run scripts/task.py list              every task and its state
    uv run scripts/task.py next              tasks that can be started now
    uv run scripts/task.py new "Add login"   create a task from tasks/_template.md

Nobody edits the state while work is in flight; it follows from git:
  todo         `status: todo` in the file and no branch yet
  in-progress  a branch task/NNN-* exists
  done         `status: done`, set on the task branch so it lands with the merge
  blocked      `status: blocked` (the file says why)
A task is ready when it is todo and everything in `depends_on` is done.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
TASKS = ROOT / "tasks"
FRONTMATTER = re.compile(r"\A---\n(.*?)\n---\n", re.DOTALL)
MAX_SLUG = 50


@dataclass
class Task:
    id: int
    path: Path
    title: str
    status: str
    depends_on: list[int]


def load_tasks() -> list[Task]:
    tasks: list[Task] = []
    for path in sorted(TASKS.glob("[0-9]*-*.md")):
        match = FRONTMATTER.match(path.read_text(encoding="utf-8"))
        lines = match.group(1).splitlines() if match else []
        meta = {key.strip(): value.strip() for key, _, value in (li.partition(":") for li in lines)}
        tasks.append(
            Task(
                id=int(path.name.split("-")[0]),
                path=path,
                title=meta.get("title", path.stem),
                status=meta.get("status", "todo"),
                depends_on=[int(n) for n in re.findall(r"\d+", meta.get("depends_on", ""))],
            )
        )
    return tasks


def branch_ids() -> set[int]:
    """Ids of the tasks that have a task/NNN-* branch."""
    proc = subprocess.run(
        ["git", "branch", "--list", "task/*", "--format=%(refname:short)"],
        cwd=ROOT,
        capture_output=True,
        encoding="utf-8",
        check=False,
    )
    return {int(n) for n in re.findall(r"^task/(\d+)", proc.stdout, re.MULTILINE)}


def state(task: Task, in_flight: set[int]) -> str:
    if task.status in ("done", "blocked"):
        return task.status
    return "in-progress" if task.id in in_flight else "todo"


def describe(task: Task, task_state: str) -> str:
    needs = f"  (needs {', '.join(f'{n:03d}' for n in task.depends_on)})" if task.depends_on else ""
    path = task.path.relative_to(ROOT).as_posix()
    return f"{task.id:03d}  {task_state:<11}  {path}  {task.title}{needs}"


def cmd_list(ready_only: bool) -> int:
    tasks = load_tasks()
    in_flight = branch_ids()
    done = {task.id for task in tasks if task.status == "done"}
    for task in tasks:
        task_state = state(task, in_flight)
        ready = task_state == "todo" and done.issuperset(task.depends_on)
        if ready or not ready_only:
            print(describe(task, task_state))
    return 0


def cmd_new(title: str) -> int:
    next_id = max((task.id for task in load_tasks()), default=0) + 1
    slug = re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")[:MAX_SLUG].rstrip("-")
    path = TASKS / f"{next_id:03d}-{slug}.md"
    template = (TASKS / "_template.md").read_text(encoding="utf-8")
    path.write_text(template.replace("{title}", title), encoding="utf-8", newline="\n")
    print(path.relative_to(ROOT).as_posix())
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("list", help="every task and its state")
    commands.add_parser("next", help="tasks that can be started now")
    commands.add_parser("new", help="create a task").add_argument("title")
    args = parser.parse_args()
    if args.command == "new":
        return cmd_new(args.title)
    return cmd_list(ready_only=args.command == "next")


if __name__ == "__main__":
    sys.exit(main())
