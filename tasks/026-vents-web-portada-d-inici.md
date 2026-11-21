---
title: "Vents (web): una portada d'inici en lloc del fil etern amb Gregal"
status: done
depends_on: []
---

## Why

Since B14 the Vents home is the thread with Gregal: every time you open Vents you continue the same conversation, which grows forever, and the other winds are only in the sidebar. The user wants a start page, like ChatGPT or Claude, where each new topic is a new conversation and the winds are in sight (decided 2026-11-21).

## Acceptance criteria

- [x] Opening Vents (or «New conversation») shows the start page, not a thread: the greeting, the composer, a row with the faces of the winds and the recent conversations (`/api/winds/threads`), newest first, each with its title, wind and date.
- [x] Sending from the start page creates a new Gregal session (`POST /api/winds/sessions`) and sends the message there; the page becomes that conversation. The previous Gregal conversation is untouched and listed among the recent ones.
- [x] Tapping a wind's face opens a new conversation with that wind; tapping a recent conversation opens it (`/api/winds/session/open`).
- [x] Gregal still has the context of earlier conversations through its memory: a node test or a Go test shows a new session does not clear the wind's memory or notes.

## Out of scope

- Android: task 027.
- Moving the existing Gregal history into separate conversations: the old thread stays as one conversation.
- Automatic titles beyond what `/api/winds/threads` gives today (the first thing the person asked).

## Notes

- `internal/web/app/winds-workspace.js`: `home()` (around line 423) already builds a start page (greeting, composer, crew strip, prompts, «Avui»), but returns early into `talkTo(gregal)` unless `state.homeClassic`. Start there; the example prompts and the «Avui» feed may stay or go, decide with screenshots.
- API: `docs/api-contract.md` lines 194–196 (`/api/winds/sessions`, `/api/winds/threads`, `/api/winds/session/open`). No server change is expected; if one is needed, it is a separate small task.
- The desktop app loads the same web client, so it gets the change too.
- What was built: `home()` in `winds-workspace.js` no longer opens Gregal's thread when the Gregal wind exists. The start page shows the faces (Gregal first, then every wind), the greeting, the composer and the recent conversations from `/api/winds/threads` (title, wind, date via `gregalNotes.dayText`). The example prompts show only when there are no recent conversations. The orb, the squad pills, the crew strip, the group demo and the «Avui» feed are left out of this start page: the faces row replaces the orb and the strip, sending goes to Gregal whatever the squad is, and «Today» has its own entry in the sidebar. Screenshots (dark, light, en, ca, mobile) were taken against a sandboxed server with synthetic winds and threads. Servers without the Gregal wind keep the old start page.
- Sending from the start page creates the Gregal session (`POST /api/winds/sessions`), opens it, and only then sends; the recipient picker is hidden there, as it was in the Gregal thread. If creating the session fails, nothing is sent and the text stays in the composer. While a turn of this client is running, sending from the start page opens that conversation with the text kept, and tapping the busy wind's face opens its running conversation instead of a new one. A recent conversation opens through `/api/winds/session/open` (only a `talkTo` read while that wind is answering).
- Tests: `internal/web/app/winds-start.test.cjs` loads the whole `winds-workspace.js` with a small fake DOM; `TestWindNewSessionKeepsMemory` in `internal/web/winds_sessions_test.go` (it passed before the change too: no server change was needed). Not covered: a busy turn started from another device (only this client's turn counts as busy), the desktop app (same web client, not opened).
- Follow-up: the header of the start page still says «Vents / Avui» («Winds / Today»), inherited from the old home with the «Avui» feed; it could say «Nova conversa».
- Follow-up: the sidebar's recent conversations (`soloButton`) still open with a `GET /api/winds/chat?session=` read instead of `/api/winds/session/open`, so the wind's active session does not follow them as it does from the start page.
