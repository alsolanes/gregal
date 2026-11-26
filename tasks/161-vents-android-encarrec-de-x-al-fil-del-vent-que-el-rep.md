---
title: Vents (Android): «Encàrrec de X» al fil del vent que el rep
status: done
depends_on: [160]
---

## Why

Task 160 makes the server say which wind handed a task to another one (`by` on the thread message). Gregal Vents still draws it as a message the user wrote.

## Acceptance criteria

- [x] The wind chat parser keeps `by:{id,name}` from `GET /api/winds/chat` (and the stream `done` history) on `PetMsg`; a message without `by` parses as today (unit test).
- [x] In `CompanyScreen.kt`, a message with `by` is drawn as «Encàrrec de A» / "Task from A" with A's face, not as a user bubble (Compose/unit test on the row choice).
- [x] The string exists in English and Catalan in `AppStrings.kt`.

## Out of scope

- The asking side (task 163). Server changes.

## Notes

- `PetMsg` in `android/app/src/main/java/xyz/solanes/gregal/GregalViewModel.kt:6631`.
- Edit `Strings.kt`/`AppStrings.kt` by hand; set `ANDROID_HOME` for the gradle tests.
- `by` lands on `PetMsg.by` (`MsgBy`, in `VentsHandoff.kt`); `HandoffRow` draws the asker's face (falls back to a face from id/name when the asker, e.g. Gregal, is not in the winds list) and strips «Encàrrec de A: » / "Task from A: " from the body, like `handoffRow` on the web. Tests: `HandoffTaskTest` (parser, strings, row) and `HandoffThreadTest` (WindConversation draws the row, with the «Nous» divider, instead of a user bubble).
- Follow-up: the offline thread cache (`VentsOffline.kt`) does not store `by`, so a hand-off read offline shows as a user bubble until the thread reloads.
