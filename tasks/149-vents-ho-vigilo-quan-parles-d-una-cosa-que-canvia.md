---
title: Vents: «Ho vigilo?» quan parles d'una cosa que canvia
status: todo
depends_on: ["146"]
---

## Why

Watches are powerful but you have to ask for them; offering one at the right moment turns a question into ongoing work.

## Acceptance criteria

- [ ] After a chat turn about something that changes over time (price, release, availability, date), at most one `fyi` card «Vols que ho vigili?» with `source:"offer"` per account per day. Test with a fake model.
- [ ] Its action opens the chat with a prompt that, when sent, creates a watch with the existing `watch` tool. Test that the prompt leads to the watch being created.
- [ ] A topic dismissed or already watched is never offered again; calm mode never rings. Tests.

## Out of scope

- Creating the watch without the person's yes.

## Notes

- Files: internal/web/winds_avui.go, internal/web/winds_responsabilitats.go (watch tool), the post-turn hook next to startWindMemoryLearnInThread.
- Person, 2026-11-25: «busca vies de proactivitat». From task 143's research (Dots, Claude Duet, ChatGPT Pulse, Gemini, Limitless, Copilot Today, Apple; Grok Bot as the cautionary case: a «no» must really stop background work and every background read must show in Activity). App only, never Telegram.
