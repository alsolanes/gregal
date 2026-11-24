---
title: Vents Android: la feina del vent en una targeta que s'obre
status: todo
depends_on: []
---

## Why

While a wind works, the person should see what it is doing next to the conversation, not in a window that hides the chat: the computer it drives, the document it writes, the page it opens.

## Acceptance criteria

- [ ] While a wind's turn uses the computer, a work card shows pinned above the composer in `WindConversation` with a live thumbnail (as `GregalWorkStrip` does for Gregal today, now for every wind); tapping it opens `OrdinadorSheet`. A Compose test that the card appears for a wind with computer activity and not without.
- [ ] When a `doc_changed` arrives during the turn, the same pinned card shows the document (kind icon and title) and opens it, besides the chat card it already adds. A unit test.
- [ ] The card hides when the turn ends and the work has not been opened; the chat cards stay.
- [ ] Strings in Catalan and English in `Strings.kt`.

## Out of scope

- Web (089).
- A side-by-side layout on tablets.

## Notes

- Files: `GregalWorkStrip.kt` (40), `OrdinadorSheet.kt` (199), `CompanyScreen.kt` (`WindConversation`, `GregalWorkStrip` used at ~1366), `DocScreen.kt` (`DocCard`), `GregalViewModel.kt` (`doc_changed` at 927-935).
- Android: run gradle tests with `ANDROID_HOME` set; edit `Strings.kt` by hand (do not run `regen-strings.py`); check on the emulator (`Medium_Phone_API_36`, x86_64 build with `-Pgregal.abi=x86_64`) with a screenshot in the notes. Regex: no `(?U)` (Android's ICU refuses it; `UserAvatarTest` guards this).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
