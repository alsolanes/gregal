---
title: Retocs: color del vent a les notificacions i títols antics d'encàrrecs
status: done
depends_on: []
---

## Why

Two leftovers from today's round that are visible to the person.

## Acceptance criteria

- [x] Android: a wind's notification (WindNotifications / WindWatch / push) is tinted with the wind's own colour and shows its avatar (a small rendered WindFace bitmap as the large icon), not the colour of the message kind. A unit test.
- [x] Old handoff threads saved before c3b8a10f show «Encàrrec de Gregal: …» (ca) / «Task from Gregal: …» (en) in the threads list and in the chat instead of the raw «[Task handed over by the main agent] …» prefix, without rewriting files (display-time). A Go test for the thread title and the history read.

## Out of scope

- Clay kits.

## Notes

- Files: Android `WindNotifications`, `WindWatch.kt`, `VentsPush.kt`; Go `internal/web/winds_threads.go` (`threadTitle`), wind history load (`winds_web.go`), `winds_specialists.go` (`handoffShown`).
- Go: the old prefix is rewritten in memory when a wind history loads (`windHistoryFor`), so the title, the chat and the model all see «Encàrrec de Gregal: …». The file is not migrated, but the next turn in that thread saves the in-memory form.
- Android: `WindNotifications.veu` (the path WindWatch and VentsPush both use) looks the wind up in the origin's offline `winds.json` copy. With no copy (local origin, a wind not cached yet) the notice keeps the kind's colour and badge.
- Android: the Canvas body of `WindFace` moved into `WindFigure.draw` (DrawScope) so `windFaceBitmap` can draw a still face without a window. It draws the Cares figure or the minairó; the esperit / cap / rosa / mapa packs and the clay faces fall back to the Cares figure in the notification.
- Follow-up: notifications for winds using the esperit, cap, rosa or mapa packs show the Cares figure, not their own pack.
- Follow-up: `MascotMinimalTest.defaultFaceHasSoftVolumeAndTwoLargeSimpleEyes` already fails on a93836c7 (white cheek highlight at size 96 trips the "no white glare" check); `softContactShadowSitsUnderTheBodyOnly` also failed once on the base.
