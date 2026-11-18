# Gregal

Open-source coding agent (Go + Bubble Tea) with terminal, web (`internal/web`), desktop (Electron, `desktop/`), Android (`android/`, Kotlin), VS Code (`vscode-gregal/`), Chrome (`extension/`) and Telegram clients over one Go backend. Most code lives in `internal/`. The HTTP contract shared by the clients is `docs/api-contract.md`.

## Commands

- `uv run scripts/check.py` runs the checks on what your change touches: gofmt, go vet, golangci-lint (new issues only), the tests of the touched Go packages and clients, and the diff guard. `--fast` skips the tests.
- `go test ./internal/agent -run TestName` for one Go test. Avoid `go test ./...` while iterating: it takes about 8 minutes (CI runs it).
- `go build -o gregal .` builds; `./gregal --check-config` validates the config.
- `uv run scripts/task.py list` shows the tasks in `tasks/`; `next` the ones that can start; `new "Title"` creates one.

## How work gets done

1. Work comes from a task file in `tasks/`. For anything bigger than a small fix with no task, write the task first (`/task-new`) and agree it with the user.
2. One task, one branch `task/NNN-slug`, one small diff. Anything else you notice goes under `## Notes` in the task file as a follow-up, not into the diff.
3. Read the code you will change and the code next to it before writing. Use what is already there: helpers, patterns, naming.
4. Write a test that fails for the right reason, then the smallest change that makes it pass.
5. Run `uv run scripts/check.py` and fix what it reports.
6. Have the `reviewer` subagent review the diff against the task file, and deal with its must-fix findings.
7. Set `status: done` in the task file and commit on the task branch. Do not push, merge, tag or release unless asked.

## Gregal conventions

- Commit messages are in Catalan, like the history: what changes for the user first, then the how.
- English is the default interface language; Catalan is also supported. When you change a user-facing string, change it in every language that has it.
- A behaviour change in the backend that a client sees must keep `docs/api-contract.md` and every client that uses it consistent, or say in the task notes which client is left behind.
- Never commit `config.yaml` (it holds keys), binaries, session history, screenshots of private workspaces, personal hostnames or user identifiers. Use synthetic examples; `node scripts/audit-public.cjs .` checks this.
- Tools that write ask for confirmation; the hard denylist blocks on its own. Do not weaken either.

## Keep it simple

Write the least code that meets the acceptance criteria. Specifically:

- No abstraction with a single use: no interface, factory, registry or plugin system for one implementation. Three similar lines beat a premature helper.
- No options, parameters, config keys or feature flags that nothing uses yet.
- No handling for situations that cannot happen. Handle an error where you can do something useful with it; never drop it silently (`_ = err`, empty `catch`).
- No compatibility shims or deprecated aliases for code that only this repo calls. Change the callers.
- Change the existing file. Never add `foo_v2.go`, `foo_nou.go` or a copy next to the original, and delete what your change makes unused.
- No new dependency (Go module, npm package, Gradle library) unless the task file lists it.
- Comments say why, when the why is not obvious. No comments that repeat the code, no commented-out code, no summary or report documents.
- Match the style of the file you are in, even if you would have written it differently.

## Tests

- A test has to be able to fail: test behaviour through the public interface, and do not mock the thing under test.
- A bug fix starts with a test that reproduces the bug.
- Never weaken, skip or delete a test to get to green. If a test is wrong because the requirement changed, say so explicitly in your final message.

## The gates are not yours to move

Hooks, the git hooks and CI enforce the rules above without AI. The diff guard rejects new suppressions (`//nolint`, `@Suppress`, `@ts-ignore`, ...), skipped tests (`t.Skip`, `it.skip`, `@Ignore`), removed tests, junk files and oversized diffs. Existing debt (old gofmt and lint findings, known failing tests) is not reported unless you touch it; fixing it is a task of its own.

- Do not edit the gate files to make a check pass: `.claude/settings.json`, `.claude/hooks/`, `scripts/check.py`, `scripts/diff_guard.py`, `.pre-commit-config.yaml`, `.github/workflows/`, `.golangci.yml` and `tsconfig*.json`.
- Do not bypass them: no `--no-verify`, no `SKIP=`, no `DIFF_GUARD_ALLOW`.
- If a gate is wrong for a specific case (a `t.Skip` that is really needed on one OS, say), stop and explain the case to the user. Overriding a gate is their call.

## Say what is true

- "Done" means `uv run scripts/check.py` passed after your last edit and every acceptance criterion has a test. Otherwise say exactly what is not done or not verified.
- If you are blocked (an ambiguous criterion, a check you cannot pass properly, a task that is too big, a missing dependency), stop and say so instead of working around it.
