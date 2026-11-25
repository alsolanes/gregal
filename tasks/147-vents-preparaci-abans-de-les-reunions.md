---
title: Vents: preparació abans de les reunions
status: todo
depends_on: ["146"]
---

## Why

Pulse, Limitless and Duet prepare meetings; Gregal already reads the calendar and mail but does nothing with them before an event.

## Acceptance criteria

- [ ] An event in a connected calendar that starts in 45-120 min, is not all-day and has a title, creates a `now` Avui card with `source:"prep"`, title «Prep: <title> · HH:MM» and a body written by a read-only turn (web, calendar, mail, memory). Go test with a fake calendar and a fake model.
- [ ] At most one card per event and 3 per day; a second check does not repeat it; `notuseful` on a prep card stops prep for later events with the same title. Tests.
- [ ] During quiet hours the card is held and does not ring; the contract lists `source:"prep"`. Test.

## Out of scope

- Writing to the calendar.
- A time chip in the clients (follow-up).

## Notes

- Files: new internal/web/winds_avui_prep.go; reuses winds_avui_glance.go (events), connections.go, the read-only loop of winds_responsabilitats.go, internal/jobs.
- Person, 2026-11-25: «busca vies de proactivitat». From task 143's research (Dots, Claude Duet, ChatGPT Pulse, Gemini, Limitless, Copilot Today, Apple; Grok Bot as the cautionary case: a «no» must really stop background work and every background read must show in Activity). App only, never Telegram.
