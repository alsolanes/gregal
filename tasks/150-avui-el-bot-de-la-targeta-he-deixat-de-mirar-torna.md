---
title: Avui: el botó de la targeta «He deixat de mirar» torna a engegar la vigilància (web i Android)
status: done
depends_on: ["146"]
---

## Why

Task 146 pauses a watch after 5 dismissals and shows a card («He deixat de mirar: …») whose action resumes it through `POST /api/winds/avui` with `action:"resume"`. The clients render card actions as a prompt to the wind's chat, so pressing it does not resume anything.

## Acceptance criteria

- [x] Web/desktop: a card with source `pausa` shows a «Torna-hi» / «Resume» button that calls `action:"resume"` for that card, then the card leaves and a short confirmation shows; an error shows a message. A node test.
- [x] Android: the same in Avui. A test against a fake server.

## Out of scope

- Other new card actions.

## Notes

- Files: vents-avui-glance.js / winds-workspace.js Avui cards, Android VentsAvui.kt; contract row of /api/winds/avui in docs/api-contract.md (task 146).
- Web: `resumeButton(card, status, onResumed)` in vents-avui-glance.js (exposed on `window.gregalAvuiGlance`); `avuiCard` in winds-workspace.js uses it for source `pausa` instead of the prompt chip. The confirmation or error goes to the page `status` line, which survives the tray repaint. Test: vents-avui-glance.test.cjs «Torna-hi d’una vigilància pausada…».
- Android: a `pausa` card shows «Torna-hi»/«Resume» (tag `avui-resume`) that calls `avuiAct(id, "resume")`; `AvuiLogic.apply` treats `resume` like `done`, and the view model reverts on failure and says it through `notify` (snackbar). The brief at the top (`WindBrief.act`) no longer offers the `pausa` chip. Tests: AvuiGlanceTest (fake server), AvuiTrayUiTest.laVigilanciaPausadaEsRepren. New strings `app.avui.resume`, `resumed`, `resumeError` (ca+en).
- docs/api-contract.md: one clause on how clients show the `pausa` card.
- Extension asked by the coordinator (task 148's `loop` cards): an action with `thread` opens that conversation of the card's wind instead of sending a prompt. Web: `avuiCard` calls `openThread(w, a.thread)` (POST `/api/winds/session/open`); test winds-start.test.cjs «a Today card action with a thread opens that conversation…». Android: `AvuiAction.thread` (DTO and model), `avuiRun` calls `resumeWindThread`, which also covers the brief's button; test AvuiGlanceTest.unaAccioAmbFilObreAquellaConversa (fake server, checks the history is read with `session=`).
- This branch starts at d26c038b, before task 148: the clause added to the `/api/winds/avui` contract row is on the same line 148 changed, so merging into integ/docs-fix will conflict on that line. Keep both: 148's `thread?` and this task's `pausa` clause.
