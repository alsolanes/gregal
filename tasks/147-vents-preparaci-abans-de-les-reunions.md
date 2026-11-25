---
title: Vents: preparació abans de les reunions
status: done
depends_on: ["146"]
---

## Why

Pulse, Limitless and Duet prepare meetings; Gregal already reads the calendar and mail but does nothing with them before an event.

## Acceptance criteria

- [x] An event in a connected calendar that starts in 45-120 min, is not all-day and has a title, creates a `now` Avui card with `source:"prep"`, title «Prep: <title> · HH:MM» and a body written by a read-only turn (web, calendar, mail, memory). Go test with a fake calendar and a fake model.
- [x] At most one card per event and 3 per day; a second check does not repeat it; `notuseful` on a prep card stops prep for later events with the same title. Tests.
- [x] During quiet hours the card is held and does not ring; the contract lists `source:"prep"`. Test.

## Out of scope

- Writing to the calendar.
- A time chip in the clients (follow-up).

## Notes

- Files: new internal/web/winds_avui_prep.go; reuses winds_avui_glance.go (events), connections.go, the read-only loop of winds_responsabilitats.go, internal/jobs.
- Person, 2026-11-25: «busca vies de proactivitat». From task 143's research (Dots, Claude Duet, ChatGPT Pulse, Gemini, Limitless, Copilot Today, Apple; Grok Bot as the cautionary case: a «no» must really stop background work and every background read must show in Activity). App only, never Telegram.
- Done: a `interval` job `avui-prep-<owner>` (every 30 min, Gregal's wind) is created the first time the glance refresh sees a connected calendar (`refreshGlance`, winds_avui_glance.go); if the person turns it off it stays off. The 75-min window is wider than the interval, so no event is missed. Logic in internal/web/winds_avui_prep.go; dispatch in jobs.go; `avuiFromVoice` skips `prep` like `local` (the card is already made).
- One card per event = same card title (`Prep: <title> · HH:MM`) in the last 12 h. The 3/day cap counts prep cards created today. The «no» is derived from the cards: a prep card with `notuseful` mutes that title (case-insensitive). The cards file keeps 300 cards and drops the oldest closed ones first, so a very old «no» could be forgotten after many months.
- The prep is also said in the wind's thread (a novetat `source:"prep"`), so it rings like other voices; held in quiet hours and calm mode. Each run shows in Activity as a run of the job, with the card titles as output.
- Not done here: the reviewer subagent was not run by the implementer; no client change (the card renders like any other `now` card).
- Follow-up: the novetat of a prep (and of `local`) has no `card` id, so deciding the card in Avui does not mark the novetat read.
- Follow-up: a time chip for prep cards in the clients (out of scope).
