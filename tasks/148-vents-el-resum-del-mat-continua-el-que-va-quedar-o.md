---
title: Vents: el resum del matí continua el que va quedar obert ahir
status: done
depends_on: ["146"]
---

## Why

Pulse follows up what you talked about; Gregal's morning brief only reads calendar, mail and memory, and what stayed open in chats is lost.

## Acceptance criteria

- [x] One hour before `quiet_to`, threads with activity in the last 24 h are checked for open loops (a deterministic marker in the wind's last message or a short classifier); at most 3 get a read-only search turn. Go test with a fake model.
- [x] Each finding is an `fyi` card with `source:"loop"`, the thread name and an action that opens that thread; with nothing found there is no card; a dismissed loop never comes back. Tests.
- [x] No more than 3 model turns per night per account. Test.

## Out of scope

- Writing into the thread itself.

## Notes

- Files: new internal/web/winds_avui_loops.go; winds_avui.go; the thread store / windChatTurns.
- Person, 2026-11-25: «busca vies de proactivitat». From task 143's research (Dots, Claude Duet, ChatGPT Pulse, Gemini, Limitless, Copilot Today, Apple; Grok Bot as the cautionary case: a «no» must really stop background work and every background read must show in Activity). App only, never Telegram.
- Scheduling: a daily job `loops-<owner>` (Wind = the account's first wind) at quiet_to − 1 h in the person's zone, converted to server time like `syncLocalJob`. It is created or moved by GET `/api/winds/avui` and by a settings POST (`syncLoopsJob`, winds_avui_loops.go); dispatched in `executeJob` (jobs.go) and its run shows in Activity like any job run.
- Open loop = deterministic marker on the wind's last turn (`windChatTurns`): its last line ends in «?» or it contains one of `loopMarkers` (pendent, demà, quan sàpigues, ja em diràs, pending, tomorrow, let me know, once you know). No classifier.
- State per account in `winds-avui-loops.json`: `{night, turns, seen}`. Turns are reserved before the model runs (an error or a second run the same night still counts). Every checked loop (thread + hash of the last wind message) goes to `seen` (last 200), so it is never looked at again, found or not, dismissed or not; new wind text in the thread is a new loop.
- The card action carries a new `thread` field (`avuiAction.Thread`, session name) instead of a prompt; `why` is «From the conversation: <first user message>». Contract updated in docs/api-contract.md.
- Follow-up: web (`winds-workspace.js` avuiCard) and Android do not handle `actions[].thread` yet; until they do, the «Open the conversation» chip has no prompt and does nothing. They should switch to that wind's session.
- Follow-up: the job is only created once the person opens Avui (or saves its settings); a server-side ensure at startup would cover accounts that never open the tray.
