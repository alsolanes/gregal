---
title: "Codi: el fitxer que el model escriu es veu créixer"
status: todo
depends_on: []
---

## Why

When the model writes a file, the whole content travels as tool-call arguments and the user sees only «preparing a tool call» with a character counter, sometimes for half a minute. It is the longest silent stretch of a turn, and Claude Code and Codex show the file growing instead.

## Acceptance criteria

- [ ] While a model call streams tool-call arguments, `progress` events of `phase=model` carry `draft_name` (the tool being called) and `draft_tail` (the last 600 characters of its arguments, with JSON string escapes decoded); a Go test feeds argument deltas and checks both fields.
- [ ] `draft_tail` is absent while there are no tool arguments, and the reasoning `tail` keeps its current rule.
- [ ] In the web client, while those events arrive, the work group shows a line with the tool being called and the last lines of the draft in monospace; it disappears when the `tool_call` event arrives.
- [ ] `docs/api-contract.md` documents the two fields, and the durable log stores no more than it does today for `progress`.

## Out of scope

- A diff view of the finished edit.
- Android and the other clients: they ignore the new fields. Android is the parity gap of this task and needs its own task.
- Providers that do not stream tool arguments: nothing changes for them.

## Notes

- `internal/llm/progress.go`: `StreamProgress` is `func(kind string, n int)` and carries no text, so the delta text of the tool arguments has to reach `progresTorn` (`internal/web/progres.go`), which already keeps `cua` for the reasoning.
- `internal/web/index.html`: `paintThinkTail` (line 2737) is the existing pattern for a live tail inside the work group.
- Tool arguments can hold secrets; the draft goes only to the live stream of the session that owns the turn, like `tail`.
- At one update every 3 seconds the preview is of little use: do task 010 first if both are planned. It is not a code dependency.
