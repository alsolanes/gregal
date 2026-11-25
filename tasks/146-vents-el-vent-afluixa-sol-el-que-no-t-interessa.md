---
title: Vents: el vent afluixa sol el que no t'interessa
status: todo
depends_on: ["140"]
---

## Why

«Not useful» and «Not now» only reach the model as prompt text, so a watch can keep sounding. A «no» has to slow it down for real, deterministically.

## Acceptance criteria

- [ ] With 3 cards from the same watch dismissed or marked `notuseful` within 7 days, that watch's `budget.messagesPerDay` is halved (minimum 1) and the act is written to its `log`. Go test through `POST /api/winds/avui` and `GET /api/winds/responsabilitats`.
- [ ] At 5 dismissals the watch moves to `paused` and one Avui card appears («He deixat de mirar X») whose action resumes it. Test.
- [ ] `useful` on a later card from the same watch resets the count; the heartbeat (`cor-*`) and reminders only get their budget lowered, never paused. Tests.
- [ ] `docs/api-contract.md` describes the rule.

## Out of scope

- New client UI: the card uses the existing shape web and Android render.

## Notes

- Files: internal/web/winds_avui.go (POST handler), internal/web/winds_responsabilitats.go (budget, log, pause).
- Person, 2026-11-25: «busca vies de proactivitat». From task 143's research (Dots, Claude Duet, ChatGPT Pulse, Gemini, Limitless, Copilot Today, Apple; Grok Bot as the cautionary case: a «no» must really stop background work and every background read must show in Activity). App only, never Telegram.
