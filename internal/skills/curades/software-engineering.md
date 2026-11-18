---
name: software-engineering
title: Software engineering guardrails
description: Use to set up or tighten how a repository checks code written by agents or people, with one check command, edit and stop hooks, git hooks, CI on any forge or none, independent review, and a flow from work item to reviewed change.
---
# Software engineering guardrails

An instruction is a request; a check that exits non-zero is a rule. Use instructions so the work is right the first time, and deterministic checks so wrong work cannot land. Nothing here depends on a particular forge: the repository carries the rules and the host only runs them.

## Look before proposing

Read what the project already has: languages and manifests; the formatter, linter, type checker and test runner in use; `Makefile`, `justfile` or package scripts; git hooks (`.githooks/`, `.pre-commit-config.yaml`, `lefthook.yml`, `.husky/`); CI files (`.github/workflows/`, `.gitlab-ci.yml`, `.forgejo/` or `.gitea/workflows/`, `azure-pipelines.yml`, `bitbucket-pipelines.yml`, `Jenkinsfile`, `.woodpecker*`); and `git remote -v`. Reuse what is there. Ask before adding a dependency, a tool or a CI file.

## One check command

Put every gate behind a single entry point kept in the repository (`make check`, `just check` or `scripts/check`) that needs no network and exits non-zero on failure. Every other layer is one line that calls it, so changing forge, or having none, changes nothing.

Run the cheap gates first: format check, lint, types, tests. Then add these one at a time, and only where they catch something real in this codebase:

- new suppressions in the diff (`noqa`, `ts-ignore`, `eslint-disable`, `nolint`, `any`, skipped tests)
- secrets (for example gitleaks)
- unused code and dependencies (for example knip, vulture, deadcode)
- duplication and complexity limits
- a diff larger than a threshold, or dependency manifests changed without a stated reason

Checks that read the diff compare against the merge base with the default branch, not against the forge's API.

## Layers, in the order they pay off

1. The check command, run by hand.
2. Git hooks that call it: a tracked `.githooks/` with `git config core.hooksPath .githooks`, or the hook manager already in use. Quick gates on pre-commit, the full run on pre-push. This works with no remote at all; it is a per-clone setting, so write the one-time command into the README.
3. Agent hooks: format the file after each edit, and run the check command when the agent says it is done, refusing the stop while it fails.
4. A short instructions file (`AGENTS.md`): stack, the check command, conventions and a "never" list. Short enough to be read in full.
5. An independent reviewer that gets the diff and the acceptance criteria, not the conversation that produced the code.
6. CI: the forge's own pipeline file with one job that runs the check command. With no CI, the pre-push hook or a server-side `pre-receive` hook is the gate; say which one it is.
7. Only then, work items handed to agents in parallel. Without the earlier layers that only produces unchecked code faster.

## From a work item to a change

A work item is a title plus acceptance criteria that can be checked. It may be an issue on any tracker, read with that tracker's CLI (`gh`, `glab`, `tea`, `az boards`, `jira`) or pasted in, or a file in the repository such as `docs/tasks/<id>.md`. If the criteria are missing, write them and have them confirmed before coding. The `work-items` skill covers types, fields, states and splitting.

Then: plan, one branch or worktree per item, implement, pass the check command, review against the criteria, and hand over the way the project does it: a pull or merge request, a patch from `git format-patch`, or a local merge. Put the item id in the branch name and the commit.

## What to catch

Under pressure, agents and people edit or delete tests until they pass, add suppressions, swallow errors in broad catch blocks, build abstractions with a single implementation, leave `_v2` or `_new` copies beside the old code, mock so much that the test proves nothing, add dependencies unasked, keep compatibility shims nobody needs, and report "done" without running anything. For each one that has happened in this project: a check where a script can see it, a line in the "never" list where it cannot, and a point the reviewer looks for.

Never weaken a check, skip a hook with `--no-verify` or edit the linter's configuration to get a pass. If a rule is wrong, say so and change it in a change of its own.

## In Gregal

- Instructions: `AGENTS.md` at the project root and notes in `.gregal/memory.md`.
- After each edit: `hooks.post_edit` in the config, for example `gofmt -w {file}`.
- Done means checked: a `hooks.stop` command that runs the check command and exits 2 with the failure on stderr sends the agent back to work. Raise its `timeout_s` (10 by default, 600 at most); `stop_max_continues` caps the retries per turn.
- Blocking: a `hooks.pre_tool` command that exits 2 denies the call, and `permissions.bash_deny` refuses commands outright.
- Review: `verify.mode` and `/verify`, or a project reviewer in `.gregal/agents/reviewer.md` with `mode: inspect`, used through `delegate`.
- Repeatable steps: `.gregal/commands/<name>.md` for slash commands and `.gregal/skills/<name>.md` for "how we do this here".
- Work items: goal mode takes the acceptance criteria and asks for evidence on each one. `gh_issue` and `gh_pr` read GitHub; any other tracker goes through its CLI with `bash`.

Gregal's hooks live in each person's config, not in the repository. Whatever must hold for everyone belongs in the check command, the git hooks and CI.
