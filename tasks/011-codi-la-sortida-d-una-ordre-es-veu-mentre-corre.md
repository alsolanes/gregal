---
title: "Codi: la sortida d'una ordre es veu mentre corre"
status: todo
depends_on: []
---

## Why

A long command (`go test`, `npm install`) shows a still chip until it ends, because `BashCtx` captures the output in a buffer and returns it at the end. The user cannot tell a slow test run from a hung one, and other agents show the last lines as they come.

## Acceptance criteria

- [ ] While a `bash` call runs, `/api/agent` emits `tool_output` events `{call_id, name, tail}` with the last lines written so far (at most 2000 characters, at most one event per second, none for a command that ends in under a second); a Go test runs a command that prints a line, waits and prints another, and sees the first line before the result.
- [ ] The `tool_result` of the call, the text the model receives and the file saved under `.gregal/out` are the same as today.
- [ ] In the web client, the running call's panel opens under its sentence and shows the tail in the terminal style of task 008; when the result arrives the panel closes unless the call failed.
- [ ] `tool_output` is not written to the durable log and is documented in `docs/api-contract.md` under the `/api/agent` SSE events.

## Out of scope

- `bash_background` / `bash_output`, and every other tool.
- Android, terminal, VS Code and Telegram clients: they ignore the new event and keep showing the result at the end. Android is the parity gap of this task and needs its own task.
- Exit code and duration in the terminal block.

## Notes

- `internal/tools/tools.go`: `BashCtx` (line 1044) writes stdout and stderr into a `limitedWriter`; the live tail needs a second writer and a callback that arrives through the context, as `ExtraEnv(ctx)` does.
- `internal/web/motor_web.go` (where tool events are emitted), `internal/web/index.html` (`toolCard`, `toolCardDone`, the `tool_call` / `tool_result` handlers around line 4139), `internal/web/app/tool-summary.js`.
- The output is decoded with `shell.DecodeOutput` at the end; the tail must not cut a multi-byte character.
