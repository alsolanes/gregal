---
title: Vents (web): el vent que rep un encàrrec sap de qui és i el fil diu «Encàrrec de X»
status: done
depends_on: []
---

## Why

When a wind hands work to another one with the `wind` tool (`ask`), the receiving wind is told "[Task handed over by the main agent]" and its thread saves «Encàrrec de Gregal: …», even when the one asking is another wind. Neither the wind nor the person can tell who really asked, and the message is drawn as a plain user bubble.

## Acceptance criteria

- [x] When wind A calls the `wind` tool with `action:"ask"` on wind B from A's 1:1 chat, the message B's turn receives names A ("Task handed over by A") instead of "the main agent", and B's thread saves «Encàrrec de A: …» / "Task from A: …"; called from Codi's main chat or from Gregal it stays as today.
- [x] `GET /api/winds/chat?wind=B` returns that message with `by:{id,name}` of the wind that asked (Gregal included); messages that were not handed over have no `by`.
- [x] In the web winds workspace (`winds-workspace.js`), a message with `by` is drawn as a hand-off row with the asker's face and «Encàrrec de A» instead of as a user bubble (node test).
- [x] `docs/api-contract.md` documents `by` on wind thread messages.

## Out of scope

- The asking side (task 162) and Android (task 161).
- The `all` action (it leaves no trace in B's thread, `windAskOnce`).
- Rotllana voices calling the tool: the caller is the group, not one wind; keep today's text there.

## Notes

- Caller: `runWindTool` (`internal/web/winds_specialists.go:160` on main) → `windDelegate` (line 328), `handoffTag` and `handoffShown` (lines 401–411, hardcode "the main agent" and «Gregal»). The 1:1 turn's wind id can come from the context (`windSteerKeyOf`, `internal/web/winds_permissions.go:57`, kind `"wind"`); check it is set on that path before relying on it.
- The message is stored as `llm.Message` (role user); the `by` field needs a place in the stored message, next to `SpeakerID`/`Kind`.
- Strings in English and Catalan. `winds.taskBy` in `i18n.js` means something else in English ("Task for {name}"), so do not reuse it.
- Read the code on `main`: this side session's checkout was on an old branch when the task was written.
- Android gap covered by task 161.
- Done: `windAsker` reads the asker from the turn's steer key (kind `"wind"`, set by the 1:1 stream turn). Gregal's chat, Codi's main chat and rotllanes (kind `"rotllana"`) all ask as Gregal, so their text stays as before and `by` is `{id:"gregal",name:"Gregal"}`.
- `by` is stored in the existing `SpeakerID`/`SpeakerName` of the handed-over user message (no new field in `llm.Message`); `petHistoryJSON` emits `by` for any user row with a speaker, and only `windDelegate` writes those.
- Old stored hand-offs (with the model tag, or with «Encàrrec de Gregal» text) have no `by` and keep showing as user bubbles.
- The web row is `handoffRow` in `winds-workspace.js` (tested in `internal/web/app/winds-handoff.test.cjs`); its wiring into `paintHistory` was not checked in a browser.
- Follow-up: a wind that hands work to a wind from inside a hand-off (`windDelegate` turn) has no steer key, so it still asks as Gregal.
- Follow-up: `/api/winds/transfer` export drops `by` on user messages (the import validator rejects speakers on user rows).
