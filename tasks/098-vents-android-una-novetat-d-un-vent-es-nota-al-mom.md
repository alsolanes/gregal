---
title: Vents Android: una novetat d'un vent es nota al moment
status: blocked
depends_on: []
---

## Why

A wind that does work on its own must be noticed on the phone: today a new novetat arrives and the home screen shows nothing, and the recents list is stale.

## Acceptance criteria

- [x] With the app open, a new novetat from a wind makes that wind's face on the home strip and in the drawer show a badge and a short happy bounce, and a banner slides in from the top («Ponent t'ha escrit: …», tap opens that chat at the message, swipe dismisses). A Compose test that a novetat event shows the banner and the badge.
- [x] The home recents list reloads when a novetat arrives, when a turn ends and when the app comes back to the foreground, and the newest thread is first. A unit test; also find why a finished Garbí thread was missing from recents in the e2e run (note the cause).
- [ ] With the app in the background, a novetat still produces a system notification with the wind's name and avatar colour, through the existing paths (FCM or WindWatch); verify on the emulator with a sandboxed server and note which path fired and how long it took.
- [x] Strings in Catalan and English.

## Out of scope

- Web (099).
- Backend changes (097).

## Notes

- Blocked (098): criteria 1, 2 and 4 are done and tested; criterion 3 (background notification on the emulator) failed. WindWatch fired but rang nothing; see below.
- Files: `VentsStream.kt` (NovetatsLive), `WindWatch.kt`, `VentsStart.kt` (VentsStartPage, StartRecents), `CompanyScreen.kt` (home), `GregalViewModel.kt` (`loadWindThreads`, `windNews`).
- Found on 2026-11-24 testing end to end with the user's real models (sandboxed server, emulator): Ponent's daily routine produced an English markdown bulletin («# Barcelona — Daily Weather Bulletin»), and with the app open on the home screen nothing showed that Ponent had written.
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Android: gradle tests with `ANDROID_HOME` set; Strings.kt by hand; no `(?U)` regexes; Robolectric view models point at a fake server or `Conn("127.0.0.1", "9", "t")`, never 127.0.0.1:8097.
- Done (098): new novetats are detected in `refreshWindNews` against the pending set of the previous read (`WindNewsLogic.arrived`; the first read after connecting only sets the baseline, so old news never banners). The newest one becomes `windArrival`: `WindArrivalBanner` (VentsArrival.kt) on the start page, `NewsBadge` + `NewsBounce` on the start strip and in the simple Vents drawer (winds with news are listed first there). Tap = `openWindArrival` (MessageFocus + open chat); swipe up or sideways dismisses; it hides itself after 8 s; TalkBack has a «Dismiss» custom action. Tests: `VentsArrivalTest`.
- Recents: reloaded on a new novetat and at every turn end (`endWindReply`) in the ViewModel, on foreground by `LifecyclePoll(conn)` in the home (was `LaunchedEffect(Unit)`), and sorted newest first by parsed instant (`newestFirst`). The foreground reload has no test of its own: it reuses `LifecyclePoll`, the same helper NovetatsLive uses.
- Recents root cause (Garbí thread missing): the server only writes a wind session to disk at the end of the turn (`saveWindHistoryTo`, winds_web.go), and `recentWindThreads` skips sessions with 0 messages. The home loaded recents once, when it was composed, and right after `startWindConversation` created the session, so both times the thread was still empty. When Garbí finished while you stayed on the home, nothing reloaded the list, so the finished thread never appeared until you left and came back.
- Emulator (AVD Medium_Phone_API_36, release x86_64, sandboxed server with a fake provider on 10.0.2.2:18898, novetats made with remind jobs through /api/jobs/save + /api/jobs/run): with the app open the banner «Ponent wrote to you: …», the «1» badge on Ponent and the new Ponent thread in recents showed up within 2.4 s of the run (event stream). Background: push is off on the sandbox (`enabled=false`), so only WindWatch can fire. Background NOT verified: with the app sent home, WindWatch passes fired at 17:03:57 ( the first pass on this server, which by design only sets the cursor), 17:18:57 and 17:33:57 (logcat shows WindWatchService starting and stopping within ~100 ms each time), but no system notification appeared for the novetat created at 17:04:16, and the host showed no connection from the emulator to the server after the 17:33:57 pass. So the pass seems to return before asking `/api/winds/novetats` (maybe `Prefs.host/token` are empty for key sign-in, or `NotificationOrigins.remote` is null). Not diagnosed: release build, no logs. The criterion stays unchecked.
- Follow-up: the banner only shows on the Vents start page; inside another chat or in Codi a novetat only shows as the drawer badge/dot.
- Follow-up: `DrawerRow` shows the raw count while `NewsBadge` shows «9+» from ten on.
- Follow-up: `recentWindThreads` (winds_threads.go) sorts by the RFC 3339 string; mixed offsets would sort wrong and it cuts to 12 after that sort (097).
- Follow-up: diagnose why WindWatch rang nothing in the background on the emulator (see above), with a debug build and logs.
- Follow-up: WindWatch's first pass on a new server/account (`desDe == 0`) only marks the cursor, so a novetat that arrives between connecting and that first pass never rings.
