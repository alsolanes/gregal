---
title: "Codi: el text de l'agent surt mentre s'escriu"
status: done
depends_on: []
---

## Why

In every web turn that is not plain chat, the server holds the model's text and reasoning until the model call ends (`llm.WithRecoverTruncation`, set in `internal/web/motor_web.go:87`), so that a truncated or interrupted answer can be retried without showing text twice. The price is that nothing appears while the model writes: in the sandbox a 1432-character answer arrived as one `token` event 100 ms before `run_completed`. It is the main reason Codi feels slower than opencode, Claude Code or Codex with the same model. Vents already stopped buffering for chats someone is watching (`internal/web/plan.go:136`).

## Acceptance criteria

- [x] In a web agent turn, `token` events are emitted while the model call is still streaming: a Go test with a fake provider that sends a content delta, holds the call open and then sends another sees the first `token` before the call returns.
- [x] When a model call is retried after text was already emitted because its stream closed before the final signal, the turn emits `steer_restart` before the new attempt, and the text of the discarded attempt is not in the final `assistant` text nor in the stored conversation. An answer cut by `finish_reason: length` needs no restart: in both the old and the new mode it is continued, not redone (`TestConduirTornConservaFragmentTallatIContinua` covers it on the live path).
- [x] Retries still happen as often as today: the existing truncation and interrupted-stream tests pass with the live path, and the fallback model is still tried after an interrupted attempt that had shown text.
- [x] `docs/api-contract.md` says that `steer_restart` is also sent when a model call is retried, and that clients must drop the live text of the current step when it arrives.

## Out of scope

- Vents turns nobody watches live (`plan.go` with `emit == nil`): they keep buffering. Terminal and Telegram turns do not go through `conduirTorn` and are not changed. A queued web run that nobody watches is now live too, which is harmless.
- Android: no change needed, it already clears its live text on `steer_restart` (`GregalViewModel.kt:2333`, `2454`).
- Showing tool-call arguments while they are written (task 012) and the progress cadence (task 010).

## Notes

- How it works: `llm.WithStreamRestart(ctx, fn)` (`internal/llm/length.go`). `chatStreamWithToolsOnce` (`internal/llm/client.go`) passes tokens through live, still retries an interrupted stream, and calls `fn` before the retry when the discarded attempt had shown text. `ChatStreamWithToolsFO` (`internal/llm/failover.go`) calls it before trying the fallback model. `internal/web/motor_web.go` passes an `fn` that emits the existing `steer_restart` event.
- A test changed because the requirement changed: `TestConduirTornRecoversInterruptedStepWithoutReplayingTools` (`internal/web/motor_web_test.go`) used to require that discarded text was never emitted; it now requires that the text left after applying `steer_restart` is the good one.
- **Client left behind: VS Code.** `vscode-gregal/src/run-client.ts` does not forward `steer_restart` and `vscode-gregal/media/chat.js` has no case for it: on a retried stream its bubble shows the discarded text followed by the redone one until the final `text` event replaces it (on a tool step it stays). It needs its own task.
- Replay and recovery, as asked: `internal/web/recovery.go` now drops accumulated tokens on `steer_restart` (test added); `internal/web/app/panels.js` (`renderActiveRunEvents`, used when returning to a running session) did paint the discarded attempt as its own bubble and is fixed (test added); `replay.go` skips tokens and `turnlog.go` only takes their timestamp, so neither is affected.
- Known rough edges, not fixed here:
  - `reintentaBuida` (`internal/llm/continua.go`) retries an empty reply outside the restart logic: a reply that is only whitespace or only an inline `<think>…</think>` block is shown and then followed by the retried answer with no `steer_restart`.
  - `extractThinkTags`, `RecoverCalls` and the repetition guard rewrite the text after its raw deltas went out, so inline `<think>` markup, DSML call markup or a repeated block is visible live until the `assistant` event replaces it; on a tool step it stays in the working bubble. Chat mode already had this.
  - The restart is announced at the start of the next attempt, after the backoff wait, so the discarded text stays on screen during the wait; after a third failed attempt it is never withdrawn before the error.
  - The web shows «Taking your message into account…» on an automatic retry (`internal/web/index.html`, `steer_restart` handler), though the user sent nothing.
  - `steer_restart` appears in the `/api/active` activity list as an entry with text `{}` (already true for the steer interrupt).
  - `TestAgentTurnEmitsTextWhileTheModelIsStillWriting` relies on the durable token buffer flushing (180 ms after the run starts, `serve.go:1744`); it passed every time here, but it is a timing assumption.
- Checked live in the sandbox (2026-11-20, `deepseek-v4.1-flash`): 80 `token` events spread over 16 s, the first one 16 s before `run_completed`, and their concatenation equal to the final text.
