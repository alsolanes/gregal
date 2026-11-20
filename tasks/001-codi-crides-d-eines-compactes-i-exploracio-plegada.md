---
title: "Codi: crides d'eines compactes i exploracio plegada"
status: done
depends_on: []
---

## Why

In the Codi transcript every tool call is a boxed card, and most say "Run" followed by a raw shell command (`ls … | head -50`, `sed -n 1,40p …`). Ten calls fill the screen with "Run Run Run" and say little about what the agent is doing. Proposals and mock-ups: `output/ui-review/eines-codi.html` (options A + B chosen).

## Acceptance criteria

- [x] A read-only shell command reads as what it does: `ls` → List, `cat`/`head`/`tail`/`sed -n` → Read (with the lines for `sed -n a,bp`), `grep`/`rg` → Search, with the paths or pattern as target instead of the raw command (en and ca).
- [x] Tests (`go test`, `npm test`, `pytest`, `cargo test`, `node --test`) read as Test, and `git …` as Git; any other command keeps Run with the full command.
- [x] A command that writes (output redirected to a file, `sed -i`, `find -delete`/`-exec`, `rm`, …) is never treated as exploration.
- [x] Two or more consecutive exploration calls (read, grep, glob, and read-only shell commands) fold into one "Explored · N files · N searches · N listings" row that expands to the individual calls; narration or any other call in between starts a new row. Live and when a conversation is reopened.
- [x] Tool rows inside the work group are single compact lines without a box; a failed call stays visible (red mark, opened).

## Out of scope

- A `description` argument for the bash tool (Claude Code style): backend + API contract + every client. Follow-up task.
- Android, VS Code and Telegram tool rendering.

## Notes

- `internal/web/app/tool-summary.js` (pure, node tests), `internal/web/index.html` (`toolCard`/`toolCardDone`), `internal/web/app/refine.css`.
- Android parity gap: the Android client renders its own tool rows; it keeps showing raw bash commands until a follow-up task ports the classification.
- Verified in the gsbx sandbox (rebuilt binary): live turn with 8 exploring calls folds into one row, a failed read inside the fold opens it, `ls | sort -o out` stays as Run. The fold logic has no node test (no DOM harness in the repo); reopening a conversation goes through the same `toolCard`/`toolCardDone`.
- `scripts/check.py` must run with `--base assistent-v2` here: the branch has no upstream and the default base measures everything assistent-v2 carries over main.
- Follow-up: a `description` argument for the bash tool, shown instead of the command (Claude Code style).
