---
title: Vents: l'activitat de tots els vents en una sola llista (API)
status: todo
depends_on: []
---

## Why

What the winds did in the background is spread over several lists (delegated tasks, news, routine runs, reminders). A timeline needs one list, the same for web and Android.

## Acceptance criteria

- [ ] `GET /api/winds/activity?wind=<id>&before=<ts>&limit=N` returns `{items:[{at, wind, kind, title, status, ref}]}`, newest first, merging delegated tasks (`/api/winds/tasks`), news (`/api/winds/novetats`), routine runs (`/api/winds/rutines/runs`) and heartbeat/reminder runs (`last_at`/`last_status` of `/api/winds/programats`); without `wind`, all of the person's winds. A Go handler test with one item of each kind, ordering and `before` paging.
- [ ] Only the caller's winds and items: another account's items never appear. A Go test with two accounts.
- [ ] `limit` defaults to 50 and is capped at 200; a bad `before` answers 400. Go tests.
- [ ] The route is in `docs/api-contract.md`.

## Out of scope

- The clients (092, 093).
- Storing new kinds of activity.

## Notes

- Routes and handlers: `internal/web/winds_router.go` (`/api/winds/tasks`, `/programats`, `/rutines/runs`), contract lines 190-260.
- Run `go test ./internal/web -run Activity` while iterating.
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
