---
title: Vents: el vent afluixa sol el que no t'interessa
status: done
depends_on: ["140"]
---

## Why

«Not useful» and «Not now» only reach the model as prompt text, so a watch can keep sounding. A «no» has to slow it down for real, deterministically.

## Acceptance criteria

- [x] With 3 cards from the same watch dismissed or marked `notuseful` within 7 days, that watch's `budget.messagesPerDay` is halved (minimum 1) and the act is written to its `log`. Go test through `POST /api/winds/avui` and `GET /api/winds/responsabilitats`.
- [x] At 5 dismissals the watch moves to `paused` and one Avui card appears («He deixat de mirar X») whose action resumes it. Test.
- [x] `useful` on a later card from the same watch resets the count; the heartbeat (`cor-*`) and reminders only get their budget lowered, never paused. Tests.
- [x] `docs/api-contract.md` describes the rule.

## Out of scope

- New client UI: the card uses the existing shape web and Android render.

## Notes

- Files: internal/web/winds_avui.go (POST handler), internal/web/winds_responsabilitats.go (budget, log, pause).
- Person, 2026-11-25: «busca vies de proactivitat». From task 143's research (Dots, Claude Duet, ChatGPT Pulse, Gemini, Limitless, Copilot Today, Apple; Grok Bot as the cautionary case: a «no» must really stop background work and every background read must show in Activity). App only, never Telegram.
- Count: cards of a responsabilitat (source `responsabilitat` → `job`; source `heartbeat` → `cor-<wind>`) dismissed or `notuseful` with `updated` in the last 7 days, after the last `useful` card of that responsabilitat in list order. Thresholds act once when crossed (3 → halve, 5 → pause or halve again). Logic in `respEase` (winds_responsabilitats.go) and `avuiNoes`/`avuiPauseCard` (winds_avui.go).
- «Reminders» read as watches whose only source is `agenda`: reminders (`remind` cards) have no responsabilitat nor budget, so their cards do not count.
- The pause card (source `pausa`) resumes the watch with a new deterministic POST `/api/winds/avui {id, action:"resume"}`. Web and Android render the card unchanged, but its chip (`{label,prompt}`) only sends the prompt to the wind's chat, and no wind tool resumes a responsabilitat: until a client calls `resume`, the chip does not resume it on its own.
- Follow-up: web (`winds-workspace.js` avuiCard) and Android should call `action:"resume"` for cards with source `pausa` instead of sending the chip's prompt.
