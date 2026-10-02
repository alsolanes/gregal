# Compatibility Matrix

| Client | Implementation | Turn contract | Fallback when v2 is unavailable |
|---|---|---|---|
| TUI | Implemented | `POST /api/v2/runs` + `/api/v2/events` (SSE when `event_stream` is available) | Runs the local engine if it cannot discover the shared service |
| Desktop/web | Implemented | Runs, durable events, streaming and interactive payloads | `/api/agent` SSE |
| Android | Implemented | Uses v2 only with `runs`, `durable_events`, `events` or `event_stream`, and `interactive_events`; sends an explicit `mobil` session | `/api/agent` SSE, including approvals and questions |
| VS Code | Implemented | Runs plus polling durable events and interactive payloads | `/api/agent` SSE |
| Telegram | Implemented | Local engine with `runs.Queue`, session `tg:<chatID>` and run/context cancellation | Does not use local HTTP |

## Required Negotiation

Before enabling v2, a client must query `GET /api/health` and validate
`status: "ok"`, `protocol`, `instance_id` and the capabilities it needs. The
minimum for durable interactive turns is:

```text
runs = true
durable_events = true
(events = true or event_stream = true)
interactive_events = true
```

`interactive_events` is `true` only when the durable event log preserves the
safe payload of an approval or question. Clients must not reconstruct an
interaction from `text`; if this capability is missing, they should fall back
to `/api/agent` SSE, which keeps `key`, `call_id` and `options` available while
the request is live.

## Shared v2 Contract

- `POST /api/v2/runs` queues `{task, images?, mode?, idempotency_key?}` and
  returns `{run:{id,state,session,workspace,...}}`.
- `GET /api/v2/events?after=&limit=&session=` is durable and returns
  `cursor,next`.
- `GET /api/v2/events/stream` replays the same events over SSE, accepts `after`
  or `Last-Event-ID`, and sends heartbeats.
- `POST /api/v2/runs/{id}/cancel` identifies the exact run to cancel.
- `approve_request` preserves `{key,call_id,name,args}` and `question_request`
  preserves `{key,call_id,query,options}` in `payload`.
- `X-Gregal-Session` and `session` must identify the same conversation. Without
  the header, the server keeps using `default` for backward compatibility.

## Model Compatibility

Any OpenAI-compatible backend with `/v1/chat/completions` supports chat. The
`/api/agent` and v2 runs require `tool_calls`; `read_image` requires text plus
`image_url`; model listing requires `/v1/models`.
