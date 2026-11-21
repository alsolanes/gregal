---
title: "Vents (web): una portada d'inici en lloc del fil etern amb Gregal"
status: todo
depends_on: []
---

## Why

Since B14 the Vents home is the thread with Gregal: every time you open Vents you continue the same conversation, which grows forever, and the other winds are only in the sidebar. The user wants a start page, like ChatGPT or Claude, where each new topic is a new conversation and the winds are in sight (decided 2026-11-21).

## Acceptance criteria

- [ ] Opening Vents (or «New conversation») shows the start page, not a thread: the greeting, the composer, a row with the faces of the winds and the recent conversations (`/api/winds/threads`), newest first, each with its title, wind and date.
- [ ] Sending from the start page creates a new Gregal session (`POST /api/winds/sessions`) and sends the message there; the page becomes that conversation. The previous Gregal conversation is untouched and listed among the recent ones.
- [ ] Tapping a wind's face opens a new conversation with that wind; tapping a recent conversation opens it (`/api/winds/session/open`).
- [ ] Gregal still has the context of earlier conversations through its memory: a node test or a Go test shows a new session does not clear the wind's memory or notes.

## Out of scope

- Android: task 027.
- Moving the existing Gregal history into separate conversations: the old thread stays as one conversation.
- Automatic titles beyond what `/api/winds/threads` gives today (the first thing the person asked).

## Notes

- `internal/web/app/winds-workspace.js`: `home()` (around line 423) already builds a start page (greeting, composer, crew strip, prompts, «Avui»), but returns early into `talkTo(gregal)` unless `state.homeClassic`. Start there; the example prompts and the «Avui» feed may stay or go, decide with screenshots.
- API: `docs/api-contract.md` lines 194–196 (`/api/winds/sessions`, `/api/winds/threads`, `/api/winds/session/open`). No server change is expected; if one is needed, it is a separate small task.
- The desktop app loads the same web client, so it gets the change too.
