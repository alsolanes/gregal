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
- Current project: `/api/sessions` lists every project's conversations, so Android's `SessionInfo` now carries `workspace` (already in the response) and `CodeOpening` keeps those whose workspace equals the open project's `cwd` (`ServerState.cwd`, the same source as the header and the drawer's project pill), ignoring trailing slashes. Conversations with an empty workspace (saved before the server recorded it) are shown, and so is everything while `cwd` is still unknown: hiding them would make old work vanish for no reason. The Conversations screen is unchanged and still lists all projects.
- The empty chat calls `vm.loadSessions(quiet = true)` keyed on the connection, so switching server reloads it; the quiet path skips the "couldn't load conversations" snackbar when offline. If the list was not cached yet, the composer moves up when it arrives.
- The opening scrolls (the content sits at the old 0.8/1.2 split via `Arrangement.aligned(BiasAlignment.Vertical(-0.2f))`), so in landscape the recents are not cut.
- `CodeOpening` takes `nowMs` for the dates; the test uses a fixed noon so it does not change after midnight.
- `CodeStartPageTest` writes `android/app/build/code-start/{dark,light,dark-small}.png`; dark-small is 360x560 dp, a 640 dp phone without status and top bars.
