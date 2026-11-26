---
title: Vents (Android): a quin vent es pregunta i què ha respost
status: done
depends_on: [162]
---

## Why

Task 162 makes the server say which wind is being asked (`winds` on the activity event) and keep its answer in the thread (`kind:"consult"`). Gregal Vents still shows «Consultant un especialista» and would draw the answers as ordinary replies.

## Acceptance criteria

- [x] The live progress label for the `wind` tool reads «Preguntant a Llevant» / "Asking Llevant" (or «Preguntant a N vents») when the event carries `winds`, and stays «Consultant un especialista» without it (unit test on `windProgressLabel`).
- [x] The progress row shows the consulted wind's face next to the label.
- [x] A run of `kind:"consult"` messages is folded like `debate`, with each wind's face and name (unit test like the one for `debateRunStart`).
- [x] New strings in English and Catalan in `AppStrings.kt`.

## Out of scope

- Server changes. The receiving side (task 161).

## Notes

- Label: `android/app/src/main/java/xyz/solanes/gregal/Winds.kt:125-137`; fold: `debateRunStart` in `CompanyScreen.kt:2439`.
- Set `ANDROID_HOME` for the gradle tests.
- `WindLive.asking` holds the consulted winds resolved against the app's wind list when the `activity` event arrives (only in the wind chat streams; the rotllana streams do not carry `winds`). With two or more, the label counts them and the progress row shows the first one's face.
- The fold reuses `debateRunStart`/`DebateHeader` (now also for `kind:"consult"`). The «Nous» divider goes above the folded header when the first unread message is inside the closed run.
- Tests: `WindConsultTest.kt` (parsing, label, face in the progress row through `WindConversation`, fold, divider).
- Follow-up: long-pressing an unfolded consult answer shares it under the asking wind's name, not the consulted one's.
