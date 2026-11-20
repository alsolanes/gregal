---
title: "Codi: el senyal de vida del model s'actualitza cada segon"
status: done
depends_on: []
---

## Why

While the model reasons or builds a tool call, the character counters and the reasoning tail only move every 3 seconds (`intervalProgres` in `internal/web/progres.go`). A line that stands still for 3 seconds reads as a hung agent.

## Acceptance criteria

- [x] While a model call is receiving and a count changes, a `progress` update is emitted at most once per second (`TestProgresTornThrottleIEstats` in `internal/web/progres_test.go`, with a fake clock: nothing 300 ms after the last one, an update at 1 s).
- [x] `docs/api-contract.md` states the new cadence in the `progress` section.

## Out of scope

- New fields in `progress`, and any client change: web, Android and the others already repaint on every `progress` event.
- Tool-call previews (task 012) and command output (task 011).

## Notes

- `internal/web/progres.go`: `intervalProgres` is now one second. `TestProgresTornThrottleIEstats` changed because the requirement changed (its steps were written for 3 seconds).
- **A criterion was dropped.** The task first asked for the durable log (`/api/v2/events`) to keep recording at most one `progress` every 3 seconds. That cannot hold: the web client and Android do not read `/api/agent` by default (it is only their fallback for servers without v2), they read the durable log (`runs.js` → `/api/v2/events/stream`, Android → `/api/v2/events`), so an update the durable log does not record never reaches them. `recordProgress` (`internal/web/serve.go`) therefore records every update, now up to one per second during a model call. For scale: the same log already takes a `token` entry every 180 ms while the model writes (`serve.go:1744`).
- Follow-up: the event store (`events.jsonl`, `internal/events/store.go`) is never pruned or rotated, is read whole into memory when opened and keeps one offset per event; this change triples the `progress` share of its growth. It deserves its own task; `progress` and `token` rows of finished runs are the first candidates to drop.
- The cadence test changed to 900 ms / 100 ms / 900 ms steps so it fails for any interval outside (900 ms, 1 s].
