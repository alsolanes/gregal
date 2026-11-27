---
title: Vents (web): el xat diu a quin vent es pregunta i què ha respost
status: done
depends_on: []
---

## Why

When a wind asks another one (`wind` tool, `ask` or `all`), the chat only shows «Consultant un especialista» with no name or face, and the answer it got back is lost: the person sees the final reply without knowing who helped or what they said.

## Acceptance criteria

- [x] During a wind chat stream, the `activity` event for the `wind` tool carries `winds:[id]` with the consulted winds (one for `ask`, the ones asked for `all`); only ids of existing winds are sent, never the tool arguments.
- [x] In the web winds workspace the step reads «Preguntant a Llevant» / "Asking Llevant" with Llevant's face (and «Preguntant a N vents» for `all`) instead of «Consultant un especialista» (node test).
- [x] After the turn, the asking wind's thread (`GET /api/winds/chat?wind=A`) holds, before A's reply, one message `kind:"consult"` per wind that answered, with that wind as `speaker_id`/`speaker_name` and its answer as `text`.
- [x] The web draws a run of `consult` messages folded, like `debate`, with each wind's face and name (node test).
- [x] `docs/api-contract.md` documents `winds` on the activity event and `kind:"consult"`.

## Out of scope

- The receiving side (task 160) and Android (task 163).
- Rotllana and crew threads (they have their own `debate` messages).
- Changing who gets consulted or when.

## Notes

- Activity: `windActivity` (`internal/web/winds_stream.go:38`) and `activity("tool", tool)` in `internal/web/plan.go:264`, which on purpose never exposes arguments; the consulted ids must come from `runWindTool` (`internal/web/winds_specialists.go:160`) after resolving them against the wind store.
- Step label: `stepLabel` in `internal/web/app/winds-workspace.js:464 on main`; fold: `debateRun` (line 1387).
- Strings in English and Catalan.
- If the diff runs over 400 lines, split the folded answers (criteria 3–4) into their own task.
- Android gap covered by task 163.
- Done: the `wind` step is announced from `runWindTool` once the winds are resolved (hook in the context, `winds_consult.go`), so the `tool` activity for `wind` is sent once, with `winds`; `list` or an unknown wind still gets the step, without `winds`.
- Consult rows are skipped by `windChatTurns`: the asking wind already had the answers as the tool result, and they must not come back as its own words.
- Only the normal end of a turn saves them; a turn that parks on a decision (winds_ask.go) or fails drops them.
- Tests: `internal/web/winds_consult_test.go`, and two tests in `internal/web/app/winds-start.test.cjs` (search «162»).
- Follow-up: Android (task 163) shows `consult` rows as plain messages from the asking wind until it folds them.
