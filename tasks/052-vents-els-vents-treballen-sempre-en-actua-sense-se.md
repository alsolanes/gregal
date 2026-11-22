---
title: "Vents: els vents treballen sempre en Actua, sense selector de mode al xat"
status: done
depends_on: []
---

## Why

In a wind's chat, Android shows the «Actua» pill (the Pregunta / Actua / Planifica mode). The user doesn't want it (2026-11-22): a wind can decide on its own, act on what's safe and ask only for what matters. The mode is also global and wind turns inherit it (`windTurnMode`). If you choose «Pregunta» or «Planifica» in Codi, the winds become read-only, on the web too, with nothing on screen to explain why. The mode is a Codi tool. The approval cards («Vols que…?») and the access level (task 018) are what keep winds safe, and they stay.

## Acceptance criteria

- [x] A wind turn always runs in Act (`code`), whatever the global mode is: with the global mode set to `plan`, `chat`, `inspect`, `goal`, `debug` or `autonomous`, the wind can write inside the project under the same rule as today. A Go test table over every mode.
- [x] The access level and the hard gates still apply as in task 018. With «Demana abans de canviar» (`AskEdits`), a wind that wants to write still asks; sending mail, publishing, the calendar, deleting and pushing still ask. The 018 decision table tests pass unchanged.
- [x] Android: the wind and rotllana chats show no mode pill, and their «+» menu has no Pregunta / Actua / Planifica entries, whether «Vents net» is on or off. Codi's chat keeps its selector. A Compose test for each chat.
- [x] Choosing a mode in Codi no longer changes how winds work, and nothing in Vents changes Codi's mode. Today, setting the mode from a wind chat calls the global `/api/mode`. A test.
- [x] `docs/api-contract.md` and `docs/modes.md` say that wind turns always run in Act and only follow the access level. `docs/vents-ux.md` drops the line saying the mode pill «stays in the +».

## Out of scope

- Codi's mode selector and the access level (padlock) UI: unchanged.
- The cases specific to winds (the rotllana where only the first speaker writes, review requests, shared chats that only search, the study folder). They keep their own scope and must not come from the global mode. Check that none of them relied on it.
- The web: Vents shows no mode pill there already. The server change fixes the web too.

## Notes

- Server: `windTurnMode` in `internal/web/winds_permissions.go` (~94), called from ~65. Today it keeps read-only modes (`agent.ReadOnlyMode`: chat, inspect, goal, plan) and debug, and turns autonomous into code. The rule: `actRule` in `internal/agent/loop.go`; `WindAutoAllow` in `internal/agent/autonomous_policy.go`.
- Android: `showModePill = !vm.simpleVents.value` in `CompanyScreen.kt` (~1387 and ~1669), `onMode = { vm.setMode(it) }` (~1381, ~1664), and the «+» menu entries in `VentsChatChrome.kt` (~408–418). `vm.setMode` posts to the global `/api/mode`.
- This is a behaviour change that every client sees. The web needs no UI change, but the contract must say it.
- Strings: if an Android string becomes unused, delete it from `AppStrings.kt` in both languages. Set `ANDROID_HOME` for the gradle tests.

### What was built (2026-11-22)

- Server: `windLoopScope()` no longer reads `s.mode` and always gives `code`. `interactiveWindCtx` takes that scope as it is, and `windTurnMode` is gone. Read-only wind turns are unchanged: `readOnlyWindCtx` (a rotllana voice after the first, a review) still gives `inspect`, the study still writes only in its practice folder, and `plan.go` still gives `inspect` to turns with no wind scope (Pla, heartbeat, routines).
- Crews: they also depended on the global mode. With Codi in Pregunta or Planifica, `crewChatAllowed` sent «fes una web» to the normal chat, and starting, resuming or answering a crew returned 403 «crew is unavailable in read-only mode» without saying why. That is the same problem this task fixes, so `crewMayResume` and the mode check in `crewChatAllowed` are gone. No client handled that 403. Deeper down, `runTeamFlow` and `runTeamSeat` also forced every seat to read-only with `serverReadOnlyMode`: now that applies only to Codi's own team, not to a crew of winds (`run.winds` / `automaticReview`).
- Android: `VentsComposer` no longer has `onMode` or `showModePill`. The mode pill above the field and the Actua / Planifica / Pregunta entries in «+» are gone. Only the wind chat and the rotllana passed `onMode` (the Ventet uses the same composer without it); Codi has its own composer. `RotllanaConversation` is now `internal`, like `WindConversation`, so it can be tested. The `app.mode.act/plan/ask` strings stay: `ModesAccessTest` requires them and Codi builds `app.mode.<mode>` on the fly (`Screens.kt`).
- Docs: `modes.md` (the mode is Codi's), `api-contract.md` (`/api/mode`, `/api/winds/chat`, the rotllana and the notes tool), `vents-ux.md` (no mode in the wind chats).

### Tests

- Go: `TestWindTurnsActWhateverTheCodiMode` (every mode × «Demana abans de canviar» / «Aprova les edicions»: the turn is `code`, a write inside the project asks or goes ahead according to the access level, and `s.mode` does not change), `TestWindWritesInTheProjectWhateverTheCodiMode` (a real wind chat writes with Codi in chat, plan, inspect, goal, debug and autonomous), `TestCrewStartsWhateverTheCodiMode` (it starts, asks, takes the answer and finishes with Codi in plan, chat and inspect, and Codi keeps its mode), `TestCrewEntryRouteIgnoresTheCodiMode` (`/api/winds/intent` → `work`), `TestCrewSeatWritesWhateverTheCodiMode` (a crew builder seat writes with Codi in plan; Codi's own team does not).
- Android: `VentsNoModeTest` (the wind chat and the rotllana chat, with Vents net on and off: no mode in the chat or in «+», and «+» still opens).
- Tests changed because the requirement changed: `TestWindTurnModePreservesReadOnlyModes`, `TestWindReadOnlySessionCannotWriteThroughOrdinaryWindRoute`, `TestCrewStartEndpointHonorsReadOnlyMode` and `TestCrewReadOnlyEntryRoutesRejectStartAndKeepChatInConversation` checked that Codi's read-only mode made the winds read-only. They became the tests above. `TestWindReadOnlySessionCannotScheduleReminderThroughTool` (now `TestReadOnlyWindTurnCannotScheduleReminderThroughTool`) and `TestReadOnlyWindCannotRunClassifiedSafeShellThatWrites` now drive a read-only wind turn (`readOnlyWindCtx`) instead of Codi in Pregunta, so they still cover that protection. `composerWithToolsExposesModeWorkspaceAndPlusMenu` no longer presses the mode pill. The 018 tests (`permisos_comuns_test.go`) are unchanged and pass.

Follow-up: the padlock in the wind chats (`PermissionDialog` → `/api/access`) shares the access level with Codi, and `/api/access` also sets Codi's mode (`code` / `autonomous`). With Codi in Pregunta, choosing an access level from Vents switches Codi to Actua. The task left the padlock out of scope.
