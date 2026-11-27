---
title: Vents: «Ho vigilo?» quan parles d'una cosa que canvia
status: done
depends_on: ["146"]
---

## Why

Watches are powerful but you have to ask for them; offering one at the right moment turns a question into ongoing work.

## Acceptance criteria

- [x] After a chat turn about something that changes over time (price, release, availability, date), at most one `fyi` card «Vols que ho vigili?» with `source:"offer"` per account per day. Test with a fake model.
- [x] Its action opens the chat with a prompt that, when sent, creates a watch with the existing `watch` tool. Test that the prompt leads to the watch being created.
- [x] A topic dismissed or already watched is never offered again; calm mode never rings. Tests.

## Out of scope

- Creating the watch without the person's yes.

## Notes

- Files: internal/web/winds_avui.go, internal/web/winds_responsabilitats.go (watch tool), the post-turn hook next to startWindMemoryLearnInThread.
- Person, 2026-11-25: «busca vies de proactivitat». From task 143's research (Dots, Claude Duet, ChatGPT Pulse, Gemini, Limitless, Copilot Today, Apple; Grok Bot as the cautionary case: a «no» must really stop background work and every background read must show in Activity). App only, never Telegram.
- Done: `startWindOffer` (internal/web/winds_avui_offer.go) runs after each one-to-one chat turn, next to `startWindMemoryLearnInThread` (winds_stream.go). A side-model call (`windOfferLoop`, stubbed in tests) answers `<topic> :: <url>` or NONE; the card is `fyi`, source `offer`, title «Vols que ho vigili?», `why` = topic, `body` = URL, one action whose prompt is «Vigila això i avisa'm si canvia: <topic> (<url>)». Contract updated in docs/api-contract.md.
- Deterministic gates: the model is only asked when the exchange cites a URL (a watch needs a public page, and the prompt must carry it so that sending it creates the watch), and only if no offer was made today; the URL it returns must be one of the exchange. A topic or URL of any earlier offer (any state, not only dismissed) or of a watch not `done` (objective or source URL; substring match on normalized text) is never offered; the model is also told those. The cap and the dedupe are checked again under the Avui lock before saving.
- «Calm mode never rings»: an offer never posts a novetat or push in any mode (the person is in the chat when it appears); in calm mode or quiet hours the card has `held:true`.
- Not verified: the e2e test uses a fake chat model that calls `watch` when the user message asks to watch a URL; whether a real model calls `watch` from that prompt is not tested.
- Clients: web and Android render it like any other card with a prompt chip; no client change was made (not checked in the apps).
