---
title: Vents web: una novetat d'un vent es nota al moment
status: done
depends_on: []
---

## Why

A wind that does work on its own must be noticed on the web and desktop: today a new novetat arrives and the home screen shows nothing, and the recents list is stale.

## Acceptance criteria

- [x] With Vents open, a new novetat (polled by `watchNews`) makes that wind's face in the sidebar and the start faces show a badge and a short bounce, and a toast slides in («Ponent t'ha escrit: …») that opens the chat at the message. A node test that a new novetat shows the toast and the badge.
- [x] On desktop (Electron) and in a browser tab that is not visible, a system notification is shown (Notification API when permitted; the desktop app's native notification if it has one); asking for permission happens once, from a clear button, never on load. A node test of the decision (visible tab → toast, hidden → notification).
- [x] The recents/conversations list refreshes when a novetat arrives.
- [x] Strings in Catalan and English.

## Out of scope

- Android (098).

## Notes

- Files: `winds.js` (`watchNews` ~268), `winds-workspace.js` (start faces, nav), desktop `main.js`/`preload.js` if a native notification bridge exists.
- Found on 2026-11-24 testing end to end with the user's real models (sandboxed server, emulator): Ponent's daily routine produced an English markdown bulletin («# Barcelona — Daily Weather Bulletin»), and with the app open on the home screen nothing showed that Ponent had written.
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Web: `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server (never type into credential fields) with before/after screenshots in `output/ui-review/` (ignored).
- Done (2026-11-24): `watchNews(listener)` in `winds.js` now also polls with the tab hidden and calls the listener only with novetats it had not seen before (the ones already waiting when the page opens only show on the faces, as before); `gregalWinds.hasNews(id)` says whether a wind has one unread. `winds-workspace.js` passes the listener: refreshes `/api/winds/threads` (bar and start-page recents), puts a `.vw-news-dot` on the wind's face in the bar and on the start page, bounces it (`.vw-bounce`, 0.9 s, off with reduced motion) and shows a toast (`.vw-toasts` in `body`, max 3, 12 s) that opens the chat at the message through `openActivity` (same path as Activity). Hidden tab (on desktop also a window without focus): desktop uses `gregalDesktop.notify` (falls back to the toast if it returns false, e.g. focused window); the web uses `Notification` when granted. The toast offers «Avisa'm quan no miri» / "Notify me when I'm away" once (`localStorage gregal.winds.notifyAsked`), only on the web and while permission is `default`. Tests: `internal/web/winds.test.js` (watchNews), `internal/web/app/winds-start.test.cjs` (toast, badge, bounce, recents, decision, permission, Catalan). Screenshots in `output/ui-review/099/`.
- Follow-up: the desktop's native notification (`gregal:notify` in `desktop/main.js`) has no click handler, so clicking it does not open the message; it would need an IPC event back to the page.
- Follow-up: the web `Notification` click focuses the tab and opens the message, but it was not checked in a real browser with permission granted (headless Chrome keeps it `default`).
- Follow-up: a toast is under an open modal `<dialog>` (top layer), e.g. the first-run «What do you want to call your helpers?» dialog; it shows once the dialog closes.
- Follow-up: once Vents has loaded, the toast also shows while the user is in another part of the app (Codi, Racó); clicking it switches back to Vents. If the user is already in that wind's chat they still get the bounce and the toast.
- Follow-up: `internal/web/identitat.test.js` («cap color escrit a mà…») already fails at a4237220 on hex colours in `index.html`; not touched here.
