---
title: Avui: el temps, el calendari i el que segueixes, d'una ullada (API)
status: todo
depends_on: []
---

## Why

Today should open with what matters at a glance: today's weather where the person is, the next match of the teams they follow (or the next thing of anything else they follow) and today's calendar, without asking anyone.

## Acceptance criteria

- [ ] `GET /api/winds/avui` gains `glance:{weather?, events:[…], follows:[…]}`. `weather` = `{place, now_c, min_c, max_c, rain_mm, rain_hours:["HH"], code, sunrise, sunset}` for today from Open-Meteo (no key; geocoding the place name once, both cached ~30 min); `events` = today's and tomorrow's events from the person's connected calendars (`calendar_ics.go`), `{start, end?, title, calendar}`; `follows` = one item per thing followed. Go tests with fake HTTP servers for Open-Meteo and an ICS calendar.
- [ ] The place and the follows live in the Today settings (`/api/winds/avui/settings`): `place` (a city name) and `follows:[{id, kind:"team"|"other", name, wind?}]`, per account. Adding/removing a follow is a POST; a wind can also add one from chat with its tools («segueix el Barça»). Go tests, including two accounts.
- [ ] Each follow is kept fresh by the follow's wind (Gregal by default) with a daily read-only routine that searches the web and answers a small JSON `{title, when (RFC 3339 or empty), detail, url}` (e.g. «Barça – Madrid», next Saturday 21:00, competition); the last good answer is what `glance.follows` shows, with `updated`. A malformed answer keeps the previous one. Go test with the model stubbed.
- [ ] `docs/api-contract.md` updated. Nothing here sends notifications on its own.

## Out of scope

- Clients (106, 107).
- Paid sports APIs or any API key.

## Notes

- Existing: `internal/web/winds_avui.go` (tray, settings), `calendar_ics.go`, `connections.go` (`briefingBrief`, zone), routines in `jobs.go` (`windRoutineSystem`, `windOwnVoice`), Ponent's routine already used Open-Meteo successfully in the e2e run.
- Asked by the user on 2026-11-24: «a Avui hem de tenir la previsió del temps, next matches dels equips que seguim i si seguim alguna altra cosa també allà, o coses del calendari». It should feel alive and fun on the phone (user's direction for Vents), not a dashboard of grey boxes.
