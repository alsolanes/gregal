---
title: Vents: el resum del matí continua el que va quedar obert ahir
status: todo
depends_on: ["146"]
---

## Why

Pulse follows up what you talked about; Gregal's morning brief only reads calendar, mail and memory, and what stayed open in chats is lost.

## Acceptance criteria

- [ ] One hour before `quiet_to`, threads with activity in the last 24 h are checked for open loops (a deterministic marker in the wind's last message or a short classifier); at most 3 get a read-only search turn. Go test with a fake model.
- [ ] Each finding is an `fyi` card with `source:"loop"`, the thread name and an action that opens that thread; with nothing found there is no card; a dismissed loop never comes back. Tests.
- [ ] No more than 3 model turns per night per account. Test.

## Out of scope

- Writing into the thread itself.

## Notes

- Files: new internal/web/winds_avui_loops.go; winds_avui.go; the thread store / windChatTurns.
- Person, 2026-11-25: «busca vies de proactivitat». From task 143's research (Dots, Claude Duet, ChatGPT Pulse, Gemini, Limitless, Copilot Today, Apple; Grok Bot as the cautionary case: a «no» must really stop background work and every background read must show in Activity). App only, never Telegram.
