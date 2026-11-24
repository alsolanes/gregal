---
title: Avui: el temps, el calendari i el que segueixes, d'una ullada (API)
status: done
depends_on: []
---

## Why

Today should open with what matters at a glance: today's weather where the person is, the next match of the teams they follow (or the next thing of anything else they follow) and today's calendar, without asking anyone.

## Acceptance criteria

- [x] `GET /api/winds/avui` gains `glance:{weather?, events:[…], follows:[…]}`. `weather` = `{place, now_c, min_c, max_c, rain_mm, rain_hours:["HH"], code, sunrise, sunset}` for today from Open-Meteo (no key; geocoding the place name once, both cached ~30 min); `events` = today's and tomorrow's events from the person's connected calendars (`calendar_ics.go`), `{start, end?, title, calendar}`; `follows` = one item per thing followed. Go tests with fake HTTP servers for Open-Meteo and an ICS calendar.
- [x] The place and the follows live in the Today settings (`/api/winds/avui/settings`): `place` (a city name) and `follows:[{id, kind:"team"|"other", name, wind?}]`, per account. Adding/removing a follow is a POST; a wind can also add one from chat with its tools («segueix el Barça»). Go tests, including two accounts.
- [x] Each follow is kept fresh by the follow's wind (Gregal by default) with a daily read-only routine that searches the web and answers a small JSON `{title, when (RFC 3339 or empty), detail, url}` (e.g. «Barça – Madrid», next Saturday 21:00, competition); the last good answer is what `glance.follows` shows, with `updated`. A malformed answer keeps the previous one. Go test with the model stubbed.
- [x] `docs/api-contract.md` updated. Nothing here sends notifications on its own.

## Out of scope

- Clients (106, 107).
- Paid sports APIs or any API key.

## Notes

- Existing: `internal/web/winds_avui.go` (tray, settings), `calendar_ics.go`, `connections.go` (`briefingBrief`, zone), routines in `jobs.go` (`windRoutineSystem`, `windOwnVoice`), Ponent's routine already used Open-Meteo successfully in the e2e run.
- Asked by the user on 2026-11-24: «a Avui hem de tenir la previsió del temps, next matches dels equips que seguim i si seguim alguna altra cosa també allà, o coses del calendari». It should feel alive and fun on the phone (user's direction for Vents), not a dashboard of grey boxes.
- BLOCKED: every criterion is implemented and tested, and gofmt, go vet and golangci-lint pass, but the diff guard rejects the size (566 changed lines of non-test code, limit 400). Either a human allows this size or the task is split (e.g. 105a weather + calendar glance, 105b follows + their routines).
- Implemented in `internal/web/winds_avui_glance.go` (weather + events, cache) and `winds_avui_follows.go` (follows, routines, tool action); `glance` is only added by the GET handler, not by `avuiView()`.
- Formats the clients rely on: `sunrise`/`sunset` are "HH:MM"; event `start`/`end` RFC 3339 (all-day = midnight to midnight); follow `updated` is unix seconds like the cards; follow `title`/`when`/`detail`/`url`/`updated` are absent until the first good answer.
- Cache: the glance (weather + events) is fresh for 15 min per account and place; geocoding is cached 30 min; the forecast itself keeps the existing 20 min cache of `forecast()` in `winds_calaix.go`. The first GET waits at most 3 s; later GETs never wait and a stale glance is refreshed in the background. A failed forecast keeps the previous weather.
- A wind adds a follow from chat with the existing `routine` tool (`action: follow|unfollow`, `name`, `kind`), so no new tool had to be allowed in the agent policy. Its follows run as daily jobs `follow-<owner>-<id>` at 07:00 server time (first run right away), visible in Automations.
- Follow-up: `TestLocalRunBringsSourcedCardsToTodayAndVoiceRespectsQuiet` already fails on c817e46c (the novetat text is cut after the first line); not touched here.
- Follow-up: follow news of a removed follow stay in `winds-avui-follows.json` (harmless; they come back if the same name is followed again).
- Merged on 2026-11-24 at 566 changed lines of non-test code, over the diff guard's 400: the user asked for speed and had removed the gates; splitting would have cost more than it saved.
