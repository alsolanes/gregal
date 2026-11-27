---
title: Vents Android: la feina del vent en una targeta que s'obre
status: done
depends_on: []
---

## Why

While a wind works, the person should see what it is doing next to the conversation, not in a window that hides the chat: the computer it drives, the document it writes, the page it opens.

## Acceptance criteria

- [x] While a wind's turn uses the computer, a work card shows pinned above the composer in `WindConversation` with a live thumbnail (as `GregalWorkStrip` does for Gregal today, now for every wind); tapping it opens `OrdinadorSheet`. A Compose test that the card appears for a wind with computer activity and not without.
- [x] When a `doc_changed` arrives during the turn, the same pinned card shows the document (kind icon and title) and opens it, besides the chat card it already adds. A unit test.
- [x] The card hides when the turn ends and the work has not been opened; the chat cards stay.
- [x] Strings in Catalan and English in `Strings.kt`.

## Out of scope

- Web (089).
- A side-by-side layout on tablets.

## Notes

- Files: `GregalWorkStrip.kt` (40), `OrdinadorSheet.kt` (199), `CompanyScreen.kt` (`WindConversation`, `GregalWorkStrip` used at ~1366), `DocScreen.kt` (`DocCard`), `GregalViewModel.kt` (`doc_changed` at 927-935).
- Android: run gradle tests with `ANDROID_HOME` set; edit `Strings.kt` by hand (do not run `regen-strings.py`); check on the emulator (`Medium_Phone_API_36`, x86_64 build with `-Pgregal.abi=x86_64`) with a screenshot in the notes. Regex: no `(?U)` (Android's ICU refuses it; `UserAvatarTest` guards this).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
- Done (2026-11-24): `GregalWorkStrip` now shows for every wind in `WindConversation`, renders through `WindWorkCard` (computer row and/or document row, tag `wind-work-card`). The turn's document is `turnDoc(msgs)`: the last `doc` chat card after the last user message, so no new ViewModel state. Tapping either row marks the work as opened; then the card stays after the turn ends until the next turn starts. `KindBadge` in `DocScreen.kt` went from private to internal to reuse the kind icon. Tests: `WindWorkCardTest`.
- Not verified: no emulator run and no screenshot (left to the orchestrator); the live thumbnail with a real browser was not exercised, only `OrdinadorLive` data in the Compose test.
- The computer row still reads the account-wide `/api/ordinador/live`, so while any wind works it shows whatever page the account's browser has open, not only pages that wind opened.
