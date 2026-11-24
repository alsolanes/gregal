---
title: Retocs: color del vent a les notificacions i títols antics d'encàrrecs
status: todo
depends_on: []
---

## Why

Two leftovers from today's round that are visible to the person.

## Acceptance criteria

- [ ] Android: a wind's notification (WindNotifications / WindWatch / push) is tinted with the wind's own colour and shows its avatar (a small rendered WindFace bitmap as the large icon), not the colour of the message kind. A unit test.
- [ ] Old handoff threads saved before c3b8a10f show «Encàrrec de Gregal: …» (ca) / «Task from Gregal: …» (en) in the threads list and in the chat instead of the raw «[Task handed over by the main agent] …» prefix, without rewriting files (display-time). A Go test for the thread title and the history read.

## Out of scope

- Clay kits.

## Notes

- Files: Android `WindNotifications`, `WindWatch.kt`, `VentsPush.kt`; Go `internal/web/winds_threads.go` (`threadTitle`), wind history load (`winds_web.go`), `winds_specialists.go` (`handoffShown`).
