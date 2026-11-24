---
title: Vents: l'activitat de tots els vents en una sola llista (API)
status: done
depends_on: []
---

## Why

What the winds did in the background is spread over several lists (delegated tasks, news, routine runs, reminders). A timeline needs one list, the same for web and Android.

## Acceptance criteria

- [x] `GET /api/winds/activity?wind=<id>&before=<ts>&limit=N` returns `{items:[{at, wind, kind, title, status, ref}]}`, newest first, merging delegated tasks (`/api/winds/tasks`), news (`/api/winds/novetats`), routine runs (`/api/winds/rutines/runs`) and heartbeat/reminder runs (`last_at`/`last_status` of `/api/winds/programats`); without `wind`, all of the person's winds. A Go handler test with one item of each kind, ordering and `before` paging.
- [x] Only the caller's winds and items: another account's items never appear. A Go test with two accounts.
- [x] `limit` defaults to 50 and is capped at 200; a bad `before` answers 400. Go tests.
- [x] The route is in `docs/api-contract.md`.

## Out of scope

- The clients (092, 093).
- Storing new kinds of activity.

## Notes

- Routes and handlers: `internal/web/winds_router.go` (`/api/winds/tasks`, `/programats`, `/rutines/runs`), contract lines 190-260.
- Run `go test ./internal/web -run Activity` while iterating.
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
- Implemented in `internal/web/winds_activity.go`, tests in `internal/web/winds_activity_list_test.go` (`winds_activity_test.go` already existed for other wind tests).
- Kinds and status: `task` (running/done/failed/cancelled/interrupted, ref = task id, at = created), `news` (unread/read, ref = `n-…`), `routine` (run status ok/error, ref = job id, at = finished), `heartbeat` and `reminder` (job `last_status`, ref = job id, at = `last_at`).
- A delegated task has no wind field: it belongs to the first seat (sorted by seat id) whose label is the name of one of the account's winds, or to the `wind` asked for if it worked on it. Tasks with no seat matching a wind are left out.
- `before` takes the `at` of an item (RFC 3339, must be URL-encoded because of the `+` offset) or unix seconds; strict `<`, so two items with the very same `at` across a page boundary would lose one.
- Follow-up: heartbeat jobs (`wind-<id>`) are saved without `wind`/`wind_owner`, so between two accounts with the same wind id the heartbeat is shared, as in `/api/winds/automations` and `/programats` today.
- Fixed on `fix/vents-batecs-propietari`: with accounts, a job of a wind with no `wind` (the old `wind-<id>` heartbeat) shows to no account in `/activity`, `/automations` and `/programats` (`canSeeWindJob` in `jobs.go`); single-user servers see it as before. It shows again once its owner saves it (POST `/api/jobs` stamps `wind`/`wind_owner`). Follow-up: the job id `wind-<id>` is global, so two accounts with the same wind id still share one heartbeat job; the owner stamp only decides who sees it.
- Fixed on `fix/vents-batecs-per-compte`: with accounts a heartbeat's job id is `wind-<wind>@<account>` (`ownerWindJobID`; without accounts still `wind-<wind>`). POST `/api/jobs/save` maps `wind-<wind>`, or no id with `kind:"event"` and `wind`, to the account's own id, and answers 404 for another account's. On start, `migrateWindJobs` moves an owned `wind-<wind>` job to `wind-<wind>@<owner>` (schedule kept); its old runs keep the old `job_id`, so `/api/jobs/runs?job=` starts fresh for it (`/programats` stats still count them). Ownerless `wind-<wind>` jobs are now hidden from `/api/jobs`, run and delete too when there are accounts. Android finds its heartbeat by `wind`+`kind` (`JobItem.isHeartbeatOf`) and uses the returned id; the web never built the id. Reminders (`remind-<wind>-<nanos>`) already carry the owner and a nanosecond stamp, so they were left as they are.
- Fixed on `fix/vents-batec-arriba-app`: an ownerless `wind-<wind>` heartbeat on a server with accounts used to run on the root server as a plain job (root store has no such wind): no chat message, no novetat, no reschedule. Now `migrateWindJobs` (end of `New`, with `windOwners`) gives it to the one account that has the wind (`wind-<wind>@<account>`, schedule kept), leaves it alone if only the shared state has it, and turns it off (logged) if several have it or none. If the account already has its own heartbeat, the old one is deleted (this now also applies to owned `wind-<wind>` jobs, which used to stay duplicated). `executeJob` never runs a `wind-…` job as a plain job: without the wind it records an error run ("the heartbeat's wind is not in this account"). Heartbeat runs now carry `wind`/`wind_owner`.
