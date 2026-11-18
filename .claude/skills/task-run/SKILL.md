---
name: task-run
description: Implement ready tasks from tasks/ with implementer subagents in parallel worktrees, then have the reviewer subagent review each one.
argument-hint: "[task ids; default: every ready task]"
disable-model-invocation: true
---

Tasks to run: $ARGUMENTS

If no ids were given, run every task printed by `uv run scripts/task.py next`. You orchestrate; you do not write the code yourself.

1. **Start from green.** `git status --porcelain` must be empty and `uv run scripts/check.py` must pass on the current branch. If not, stop and say why: the subagents branch from this commit, and task files that are not committed do not exist in their worktrees.
2. **Implement.** For each task, start an `implementer` subagent in its own worktree (Agent tool, `subagent_type: "implementer"`, `isolation: "worktree"`). The prompt is the task file path and the base branch (the one you are on, `git branch --show-current`), and nothing about how to solve it. Start independent tasks in one message so they run in parallel, at most 3 at a time. Never run a task in the same wave as one it depends on.
3. **Review.** When an implementer reports DONE, start a `reviewer` subagent with the task file path, the branch, the base branch and the worktree path. Do not pass along the implementer's opinion of its own work.
4. **Fix.** If the verdict is CHANGES NEEDED, send the must-fix findings to the same implementer, then review again. After two rounds without an APPROVE, stop working on that task and leave it for the user.
5. **Report.** Show one line per task: id, branch, verdict, and what is left (BLOCKED reasons, unresolved findings, follow-ups). Copy verdicts and check results from the subagents; do not upgrade them.
6. **Ask, then land.** Ask the user what to do with the approved branches: merge them here, open pull requests, or leave them. Only on their answer:
   - merge one at a time with `git merge --no-ff task/NNN-slug` and run `uv run scripts/check.py` after each one; on a conflict or a failing check, stop and report;
   - once a branch is merged, remove its worktree with `git worktree remove <path>` and delete it with `git branch -d task/NNN-slug`.
