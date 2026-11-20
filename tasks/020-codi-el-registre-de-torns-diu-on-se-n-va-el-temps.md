---
title: "Codi: el registre de torns diu on se'n va el temps"
status: done
depends_on: []
---

## Why

Long Codi turns feel slower than other coding agents, and `turns.jsonl` cannot say why: agent turns record the total time, steps and tokens, but not how much of it was the model, the tools or waiting for the user. In the real log (2026-11-09 to 2026-11-18, 23 turns over two minutes) the time per model call goes from 7 s to 160 s with the same amount of generated text (300–900 completion tokens per call) and a healthy prompt cache (85–96 %), so the difference is somewhere this file does not measure. Wind turns already record `model_ms` and `tools_ms`.

## Acceptance criteria

- [x] An agent turn's line in `turns.jsonl` has `model_ms` (time inside the model calls of its steps), `tools_ms` (time running tools) and `wait_ms` (time blocked on an approval or a question to the user). A part that took no measurable time is left out of the line, like every other zero field. Go tests: a real turn with a provider that takes 150 ms per call shows at least 300 ms of `model_ms`, the parts never add up to more than `total_ms`, and time between a model call and the tools (automatic review) is in neither.
- [x] A turn that waits for an approval counts that wait in `wait_ms` and not in `model_ms` or `tools_ms` (Go test through the server: the agent asks to write a file and the approval comes 500 ms later). A wait that happens while tools run, as when a subagent asks for permission, goes back to `tools_ms` when it is answered.
- [x] The fields are documented in `docs/performance.md` and where the record is defined, and the existing turn-log tests pass.

## Out of scope

- Making anything faster. This task only measures.
- Per-tool timings, and the time of background calls (scout, review, bets, diagnostics after an edit): if `total_ms − model_ms − tools_ms − wait_ms` turns out large in real turns, that is the next task.
- Wind, rotllana and tempesta records.

## Notes

- How it works (`internal/web/turnlog.go`): `turnLog.emitter` already saw every event of an agent turn. A `progress` event with phase `model` or `tools` opens that part; the model part closes when the call returns (`speed`, `thinking`, `assistant` or `tool_call`) and the tools part at the first `tool_result`; everything else is nobody's time. Waits are not inferred from events: `waitApproval` and `waitQuestion` (`internal/web/serve.go`) mark them through the turn log carried in the context, and they nest, because two subagents can ask at once. The agent loop is untouched.
- What is in the rest and not in any part: the automatic reviewer and side bets, `pre_tool` and `stop` hooks, the final summary and step-extension calls (phase `review`), compaction, the scout, and the work between steps. `OrdreSintesi` and `OrdreAmplia` are model calls and count in `model_calls`, but not in `model_ms`.
- A model call whose `speed` event is not sent (a failed call, or one without usage or timing) stays open until the next `tool_call` or phase, so a little review time can still land in `model_ms` there.
- Retry backoff waits of a model call count as model time.
- While one subagent waits for an approval, the tool time of its siblings running in parallel is booked as `wait_ms`, not `tools_ms`: the turn is one timeline.
- The subagent case is tested by calling `waiting()` while the tools part is open, not through a real `delegate` call; that the context reaches the delegate approver was checked by reading (`internal/agent/delegate.go:228`, `:378`).
- How the numbers in "Why" were obtained: `~/.local/share/gregal/turns.jsonl`, agent turns with `total_ms >= 120000`. Records before 2026-11-09 have no `model_calls` and their `prompt_tokens` is not comparable.
- Reference for the base speed: `gregal --eval` on the bench in `%TEMP%\gspeed` takes about 4 s per step with `deepseek-v4.1-flash` (2026-11-18), so the loop itself is not the slow part.
- Seen on 2026-11-20 in the sandbox with reasoning on: a trivial prompt spent most of its time reasoning before the first word. Whether the default reasoning effort is higher than what other agents use with the same model was not checked.
- Next step once a few days of real turns are logged: read the split for turns over two minutes and see which part explains the 7 s to 160 s per call.
