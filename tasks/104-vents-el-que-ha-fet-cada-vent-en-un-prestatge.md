---
title: Vents: el que ha fet cada vent, en un prestatge
status: done
depends_on: []
---

## Why

What a wind produced (documents, sites, files, links) gets lost in the chat. Each wind should keep a shelf of its deliverables, so delegating real work leaves something you can find and share.

## Acceptance criteria

- [x] `GET /api/winds/outputs?wind=<id>` lists the wind's deliverables newest first: Gregal Docs it created or changed (with kind and export link), published sites/share links and files it wrote, each `{at, kind, title, url}`; only the caller's. A Go test with one of each and two accounts.
- [x] Web: the wind's profile/side area shows the shelf (cards with kind icon, title, date, open/share). A node test.
- [x] Android: the wind's profile shows the same shelf; tapping opens the doc screen, the link or the file. A Compose test.
- [x] docs/api-contract.md updated.

## Out of scope

- Publishing new kinds of output.

## Notes

- Sources: docs (`internal/gdocs` author/agent, `doc_changed` events), share links (`/share/...` used by `runWindTool` tests), the wind's working folder.
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Web: `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server (never type into credential fields) with before/after screenshots in `output/ui-review/` (ignored).
- Android: gradle tests with `ANDROID_HOME` set; Strings.kt by hand; no `(?U)` regexes; Robolectric view models point at a fake server or `Conn("127.0.0.1", "9", "t")`, never 127.0.0.1:8097.
- The shelf is a per-account record (`<wind state dir>/wind-outputs.json`, newest first, at most 500 entries) written by the wind tool dispatcher in `plan.go` (`noteWindOutput`) after a successful `doc` create/insert/replace/set_cells/delete, `publish` (a `/share/<id>/` site or a named web) or a write tool in the wind's documents folder (`pdf_create`, `pdf_edit`, `office_create`, `office_edit`). Titles, kinds and links are resolved when listing, so deleted docs and files and revoked or expired links drop out.
- Only 1:1 wind chat turns are recorded (the context carries the wind through `withWindSteerKey(..., "wind", id)`). Nothing made before this change is on the shelf.
- Web: new `internal/web/app/vents-shelf.js` (loaded from `index.html`), a «Prestatge / Shelf» tab in the wind profile; live docs open at `/app/doc/<id>?scope=mine`, files download with the app key (attach.js), sites and links have «Comparteix / Share».
- Android: `WindShelf.kt`, a «Prestatge / Shelf» tab in `WindProfileScreen.kt`; a doc opens the Docs screen (`vm.openDoc`), a file goes through the app's server-doc download, sites and links open in the browser and share with the system sheet. Strings added by hand to `AppStrings.kt` (ca + en).
- The non-test diff is exactly at the 400-line guard limit.
- Follow-up: `/api/winds/saved?wind=` (the «Desat» tab) already shows what a wind made by scanning its chat links; the two tabs overlap and could be merged into one.
- Follow-up: background turns (delegated tasks, heartbeats, routines) and rotllanes/crews do not record outputs yet, because their context does not say which wind is working.
