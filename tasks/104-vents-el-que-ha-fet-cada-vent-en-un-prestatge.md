---
title: Vents: el que ha fet cada vent, en un prestatge
status: todo
depends_on: []
---

## Why

What a wind produced (documents, sites, files, links) gets lost in the chat. Each wind should keep a shelf of its deliverables, so delegating real work leaves something you can find and share.

## Acceptance criteria

- [ ] `GET /api/winds/outputs?wind=<id>` lists the wind's deliverables newest first: Gregal Docs it created or changed (with kind and export link), published sites/share links and files it wrote, each `{at, kind, title, url}`; only the caller's. A Go test with one of each and two accounts.
- [ ] Web: the wind's profile/side area shows the shelf (cards with kind icon, title, date, open/share). A node test.
- [ ] Android: the wind's profile shows the same shelf; tapping opens the doc screen, the link or the file. A Compose test.
- [ ] docs/api-contract.md updated.

## Out of scope

- Publishing new kinds of output.

## Notes

- Sources: docs (`internal/gdocs` author/agent, `doc_changed` events), share links (`/share/...` used by `runWindTool` tests), the wind's working folder.
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Web: `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server (never type into credential fields) with before/after screenshots in `output/ui-review/` (ignored).
- Android: gradle tests with `ANDROID_HOME` set; Strings.kt by hand; no `(?U)` regexes; Robolectric view models point at a fake server or `Conn("127.0.0.1", "9", "t")`, never 127.0.0.1:8097.
