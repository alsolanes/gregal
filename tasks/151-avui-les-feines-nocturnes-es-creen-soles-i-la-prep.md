---
title: Avui: les feines nocturnes es creen soles i la preparació d'una reunió marca l'avís llegit
status: done
depends_on: ["147", "148"]
---

## Why

Follow-ups of 147 and 148: the open-threads job (`loops-<owner>`) only appears after the person opens Avui or saves its settings, and the meeting-prep notification carries no `card` id, so deciding the card in Avui does not mark the notification read (the same gap `local` cards have).

## Acceptance criteria

- [x] On server start, every account that has winds gets its `loops-<owner>` job (and the prep job if a calendar is connected), without waiting for Avui to be opened; turning a job off keeps it off. A Go test that starts a server with an account and no jobs, and finds the job.
- [x] Prep and local novetats carry the `card` id of their Avui card, so deciding the card marks the novetat read, as other cards do. A Go test for each.

## Out of scope

- Changing what the jobs do.

## Notes

- Files: internal/web/winds_avui_loops.go (syncLoopsJob), winds_avui_prep.go, winds_local.go, server start (serve.go / where jobs load), winds_novetats.go.
- Done: `ensureNightJobs` (winds_avui_loops.go) runs from `StartBackground` (serve.go) when Gregal's background is on: for the default account and every configured user (through the same `wind-routines` session the scheduler uses) it calls `syncLoopsJob` and, with a calendar connected, `ensurePrepJob`. An account without winds gets no loops job (`syncLoopsJob` already skipped it).
- `syncLoopsJob` now keeps `Enabled` (and `LastAt`) of an existing job when it moves it: before, a job turned off came back on when quiet_to changed (settings POST) or would have at start.
- Card ids: `windMessageLinks.Card` carries the card a voice already made to `recordNovetatMsgHold` (new `made` parameter). Prep passes its card. «A prop teu» makes several cards and one novetat, so the novetat carries the first card: deciding another of those cards does not mark it read.
- Contract updated in docs/api-contract.md (loops job at start and stays off, prep at start, `card` on prep and local novetats).
- Follow-up: a local novetat could be marked read when any of its cards is decided (it would need a list of card ids on the novetat, a contract change).
