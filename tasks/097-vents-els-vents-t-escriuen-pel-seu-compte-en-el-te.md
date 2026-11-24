---
title: Vents: els vents t'escriuen pel seu compte en el teu idioma i curt
status: todo
depends_on: []
---

## Why

When a wind writes on its own (routine, heartbeat, reminder), the message should read like a short note from a friend in the person's language, not a report: today a routine wrote an English markdown bulletin.

## Acceptance criteria

- [ ] The prompt of a wind's own turns (routines, heartbeats, reminders, follow-ups) asks for the person's interface language (config `lang`, or the account's) and a short message: one line with the news first, then at most a few lines of detail; no markdown headings. A Go test of the prompt built for each kind in `ca` and `en`.
- [ ] The novetat's preview text (`/api/winds/novetats` item `text` and the push/notification title) is the first line without markdown markers (`#`, `**`). A Go test with a reply that starts with a heading.
- [ ] A routine reply in another language than the person's is not rewritten (no extra model call): the prompt change is the fix. Note it in the task notes if a real run still answers in English.

## Out of scope

- Changing what routines do or when they run.
- Clients (they already show `text`).

## Notes

- Look at how routine/heartbeat prompts are built (`internal/web/winds_rutines.go`, `winds_heartbeat.go`, `winds_remind.go`) and where novetats store `text` (`winds_novetats.go`).
- Found on 2026-11-24 testing end to end with the user's real models (sandboxed server, emulator): Ponent's daily routine produced an English markdown bulletin («# Barcelona — Daily Weather Bulletin»), and with the app open on the home screen nothing showed that Ponent had written.
