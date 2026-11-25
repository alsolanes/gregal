---
title: Avui: les feines nocturnes es creen soles i la preparació d'una reunió marca l'avís llegit
status: todo
depends_on: ["147", "148"]
---

## Why

Follow-ups of 147 and 148: the open-threads job (`loops-<owner>`) only appears after the person opens Avui or saves its settings, and the meeting-prep notification carries no `card` id, so deciding the card in Avui does not mark the notification read (the same gap `local` cards have).

## Acceptance criteria

- [ ] On server start, every account that has winds gets its `loops-<owner>` job (and the prep job if a calendar is connected), without waiting for Avui to be opened; turning a job off keeps it off. A Go test that starts a server with an account and no jobs, and finds the job.
- [ ] Prep and local novetats carry the `card` id of their Avui card, so deciding the card marks the novetat read, as other cards do. A Go test for each.

## Out of scope

- Changing what the jobs do.

## Notes

- Files: internal/web/winds_avui_loops.go (syncLoopsJob), winds_avui_prep.go, winds_local.go, server start (serve.go / where jobs load), winds_novetats.go.
