---
title: "Android: la portada de Codi, com la de Vents, amb les converses recents"
status: done
depends_on: []
---

## Why

The Vents start page (task 027) shows the winds' faces and the recent conversations around the composer. The empty Codi chat is only a greeting, a glow and the composer: to go back to yesterday's work you open the drawer, then Conversations. The two homes should feel like the same app.

## Acceptance criteria

- [x] The empty Codi chat shows, under the composer, up to five recent code conversations of the current project (`vm.sessions`, newest first) with title and date, in the same style as the Vents start page recents (reuse or share the row, don't copy it); tapping one resumes it. With none, the page stays as today.
- [x] A Compose test with fake sessions: the rows show, newest first, and a tap resumes that session.
- [x] Screenshots, dark and light.

## Notes

- `Screens.kt` ~757 (`msgs.isEmpty()` opening); `VentsStart.kt` `StartRecents` and `threadWhen`; sessions are loaded by `vm.loadSessions()` and resumed the way `SessionsScreen` does it.
- Done: the empty Codi chat is now `CodeOpening` (Screens.kt), testable without the ViewModel. It takes `vm.sessions` (already newest first from `/api/sessions`), shows the first five and resumes with `vm.resumeSession(name)` like SessionsScreen. `VentsStart.kt` now has `StartRecents(lang) { ... }` (header and column) and `RecentRow(title, detail, onClick, lead)`, used by both start pages; Codi rows have no leading face. No new strings: the header reuses `app.vents.start.recent` (EN and CA).
- While the composer is focused or has text the recents hide, as on the Vents page, so the keyboard layout is the same as before.
- The empty chat calls `vm.loadSessions()` when it opens; if the list was not cached yet, the composer moves up when it arrives.
- `CodeStartPageTest` writes `android/app/build/code-start/{dark,light,dark-small}.png`; dark-small is 360x560 dp, a 640 dp phone without status and top bars.
- Follow-up: `/api/sessions` is per user, not per project, and Android's `SessionInfo` has no `workspace`; "of the current project" holds only as far as `vm.sessions` does. Filtering by the open project needs `workspace` in the model.
- Follow-up: if `loadSessions` fails (offline), the empty Codi chat now shows the "couldn't load conversations" snackbar.
