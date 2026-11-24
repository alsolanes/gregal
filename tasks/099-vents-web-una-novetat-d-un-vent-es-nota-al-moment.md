---
title: Vents web: una novetat d'un vent es nota al moment
status: todo
depends_on: []
---

## Why

A wind that does work on its own must be noticed on the web and desktop: today a new novetat arrives and the home screen shows nothing, and the recents list is stale.

## Acceptance criteria

- [ ] With Vents open, a new novetat (polled by `watchNews`) makes that wind's face in the sidebar and the start faces show a badge and a short bounce, and a toast slides in («Ponent t'ha escrit: …») that opens the chat at the message. A node test that a new novetat shows the toast and the badge.
- [ ] On desktop (Electron) and in a browser tab that is not visible, a system notification is shown (Notification API when permitted; the desktop app's native notification if it has one); asking for permission happens once, from a clear button, never on load. A node test of the decision (visible tab → toast, hidden → notification).
- [ ] The recents/conversations list refreshes when a novetat arrives.
- [ ] Strings in Catalan and English.

## Out of scope

- Android (098).

## Notes

- Files: `winds.js` (`watchNews` ~268), `winds-workspace.js` (start faces, nav), desktop `main.js`/`preload.js` if a native notification bridge exists.
- Found on 2026-11-24 testing end to end with the user's real models (sandboxed server, emulator): Ponent's daily routine produced an English markdown bulletin («# Barcelona — Daily Weather Bulletin»), and with the app open on the home screen nothing showed that Ponent had written.
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Web: `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server (never type into credential fields) with before/after screenshots in `output/ui-review/` (ignored).
