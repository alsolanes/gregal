---
title: Vents Android: una novetat d'un vent es nota al moment
status: todo
depends_on: []
---

## Why

A wind that does work on its own must be noticed on the phone: today a new novetat arrives and the home screen shows nothing, and the recents list is stale.

## Acceptance criteria

- [ ] With the app open, a new novetat from a wind makes that wind's face on the home strip and in the drawer show a badge and a short happy bounce, and a banner slides in from the top («Ponent t'ha escrit: …», tap opens that chat at the message, swipe dismisses). A Compose test that a novetat event shows the banner and the badge.
- [ ] The home recents list reloads when a novetat arrives, when a turn ends and when the app comes back to the foreground, and the newest thread is first. A unit test; also find why a finished Garbí thread was missing from recents in the e2e run (note the cause).
- [ ] With the app in the background, a novetat still produces a system notification with the wind's name and avatar colour, through the existing paths (FCM or WindWatch); verify on the emulator with a sandboxed server and note which path fired and how long it took.
- [ ] Strings in Catalan and English.

## Out of scope

- Web (099).
- Backend changes (097).

## Notes

- Files: `VentsStream.kt` (NovetatsLive), `WindWatch.kt`, `VentsStart.kt` (VentsStartPage, StartRecents), `CompanyScreen.kt` (home), `GregalViewModel.kt` (`loadWindThreads`, `windNews`).
- Found on 2026-11-24 testing end to end with the user's real models (sandboxed server, emulator): Ponent's daily routine produced an English markdown bulletin («# Barcelona — Daily Weather Bulletin»), and with the app open on the home screen nothing showed that Ponent had written.
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Android: gradle tests with `ANDROID_HOME` set; Strings.kt by hand; no `(?U)` regexes; Robolectric view models point at a fake server or `Conn("127.0.0.1", "9", "t")`, never 127.0.0.1:8097.
