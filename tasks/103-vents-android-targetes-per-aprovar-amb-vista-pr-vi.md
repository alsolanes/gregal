---
title: Vents Android: targetes per aprovar amb vista prèvia
status: todo
depends_on: []
---

## Why

To delegate real work, the person approves what a wind is about to do with one tap, seeing exactly what it is: today ask/approve requests are plain text.

## Acceptance criteria

- [ ] In `WindConversation`, an approval ask renders as a card with the wind's perched face and step, a preview of the action from the payload (text, URL/screenshot, document, amount/payee) and Permet / No buttons, through the existing answer call. A Compose test per preview kind.
- [ ] After answering, the card collapses to one line and the face nods/shakes.
- [ ] An approval that arrives while the app is in the background shows a notification with Permet / No actions that answer without opening the app (if the existing notification code supports actions; otherwise note the gap).
- [ ] Strings in Catalan and English.

## Out of scope

- Web (102).

## Notes

- Files: `WindAsks.kt` (81, 93), `WindNotifications`, `GregalViewModel.kt` (`windAsks`).
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Android: gradle tests with `ANDROID_HOME` set; Strings.kt by hand; no `(?U)` regexes; Robolectric view models point at a fake server or `Conn("127.0.0.1", "9", "t")`, never 127.0.0.1:8097.
