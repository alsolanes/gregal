---
title: "Docs: a cada torn l'agent sap què has canviat al document"
status: done
depends_on: ["057"]
---

## Why

The person should be able to write in the document and then say in the chat «I have reworked the introduction, adapt the rest», without explaining what changed. The agent needs to know which blocks the person edited since it last looked.

## Acceptance criteria

- [x] At the start of each turn, for every document the session's agent has read or written, if blocks changed with `author: user` after the version the agent last saw, the turn carries a short note listing them: document title, and per block whether it was edited, inserted or deleted, with the new text of edited and inserted blocks cut to a limit. The note goes to the model only, not into the visible transcript. A Go test: agent writes a document, user replaces one block and deletes another, the next turn's model input has a note naming both; a turn with no user changes has no note.
- [x] After that turn the agent's seen version moves forward, so the same changes are not reported twice. A Go test of two consecutive turns.
- [x] Documents the session never touched are not reported, even if the person edits them. A Go test.

## Out of scope

- Any UI. Changes made by the agent itself are never reported back to it.

## Notes

- Depends only on 057: the store records `author` on every change, so the test can write user changes through the store. In use it shows its value once 059 lets people edit.
- Find where the agent loop adds per-turn context to the model input (`internal/agent/loop.go`) and add the note there; do not put it in the stored history.
- Done: the chat turns run in the motor (`Torn`, `internal/agent/motor.go`), not in `loop.go`'s `Loop`. The note is computed once at the start of the turn (`notaDocs` in `doc_tool.go`, next to where hooks add their context) and appended to the system prompt in `histAmbSys`, so every step of the turn sees it and `Torn.Hist` (what clients store and draw) never has it. It starts with `agent.MarcaDocs`; each edited or inserted block shows its first 300 runes on one line.
- Tombstones (added here, as agreed): `gdocs.Doc.Deleted` keeps each deleted block with the version, author and time of the delete; `Store.Delete` now takes an author. `Block.Since` is the version a block was created at, which tells «edited» from «inserted»; blocks from before this change have `since` 0 and read as existing.
- Seen record for 059: `gdocs.Store.Seen(session)` returns `doc id → last version seen`, `Store.MarkSeen(session, id, v)` moves it forward only. One JSON per session under `<docs dir>/seen/<hash of session>.json`. The `doc` tool marks it after every successful create/read/insert/replace/delete; the turn note marks it for the documents it reports. A session "touched" a document exactly when it has an entry here.
- The session key is the one tools run with: the motor uses `Sessio` (or `agent` when empty), the tool `session` after the same fallback. They match in the web (`s.id` both) and headless runs (empty both).
- Follow-up: the TUI passes `Sessio: "tui"` to the motor but runs tools with session "" (→ `agent`), and Telegram passes `tgScope(chatID)` but runs tools with ""; there the note never fires until the two keys agree.
- Follow-up: tombstones and the `seen/` session files are never pruned.
- Follow-up: the note marks the documents as seen when the turn starts, so if the first model call fails (provider error, cancel) those changes are not told again next turn. Marking after the first successful step would close that.
- Follow-up: only a block's latest author counts: if the person edits a block and an agent in another session rewrites it afterwards, this session hears of neither change.
- Follow-up: subagents and headless runs without `Sessio` share the key `agent`, so one of them can use up a note meant for another.
