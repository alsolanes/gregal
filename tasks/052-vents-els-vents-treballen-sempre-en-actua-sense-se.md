---
title: "Vents: els vents treballen sempre en Actua, sense selector de mode al xat"
status: todo
depends_on: []
---

## Why

In a wind's chat, Android shows the «Actua» pill (the Pregunta / Actua / Planifica mode). The user doesn't want it (2026-11-22): a wind can decide on its own, act on what's safe and ask only for what matters. The mode is also global and wind turns inherit it (`windTurnMode`). If you choose «Pregunta» or «Planifica» in Codi, the winds become read-only, on the web too, with nothing on screen to explain why. The mode is a Codi tool. The approval cards («Vols que…?») and the access level (task 018) are what keep winds safe, and they stay.

## Acceptance criteria

- [ ] A wind turn always runs in Act (`code`), whatever the global mode is: with the global mode set to `plan`, `chat`, `inspect`, `goal`, `debug` or `autonomous`, the wind can write inside the project under the same rule as today. A Go test table over every mode.
- [ ] The access level and the hard gates still apply as in task 018. With «Demana abans de canviar» (`AskEdits`), a wind that wants to write still asks; sending mail, publishing, the calendar, deleting and pushing still ask. The 018 decision table tests pass unchanged.
- [ ] Android: the wind and rotllana chats show no mode pill, and their «+» menu has no Pregunta / Actua / Planifica entries, whether «Vents net» is on or off. Codi's chat keeps its selector. A Compose test for each chat.
- [ ] Choosing a mode in Codi no longer changes how winds work, and nothing in Vents changes Codi's mode. Today, setting the mode from a wind chat calls the global `/api/mode`. A test.
- [ ] `docs/api-contract.md` and `docs/modes.md` say that wind turns always run in Act and only follow the access level. `docs/vents-ux.md` drops the line saying the mode pill «stays in the +».

## Out of scope

- Codi's mode selector and the access level (padlock) UI: unchanged.
- The cases specific to winds (the rotllana where only the first speaker writes, review requests, shared chats that only search, the study folder). They keep their own scope and must not come from the global mode. Check that none of them relied on it.
- The web: Vents shows no mode pill there already. The server change fixes the web too.

## Notes

- Server: `windTurnMode` in `internal/web/winds_permissions.go` (~94), called from ~65. Today it keeps read-only modes (`agent.ReadOnlyMode`: chat, inspect, goal, plan) and debug, and turns autonomous into code. The rule: `actRule` in `internal/agent/loop.go`; `WindAutoAllow` in `internal/agent/autonomous_policy.go`.
- Android: `showModePill = !vm.simpleVents.value` in `CompanyScreen.kt` (~1387 and ~1669), `onMode = { vm.setMode(it) }` (~1381, ~1664), and the «+» menu entries in `VentsChatChrome.kt` (~408–418). `vm.setMode` posts to the global `/api/mode`.
- This is a behaviour change that every client sees. The web needs no UI change, but the contract must say it.
- Strings: if an Android string becomes unused, delete it from `AppStrings.kt` in both languages. Set `ANDROID_HOME` for the gradle tests.
