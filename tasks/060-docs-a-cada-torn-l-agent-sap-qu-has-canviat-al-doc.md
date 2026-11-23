---
title: "Docs: a cada torn l'agent sap què has canviat al document"
status: todo
depends_on: ["057"]
---

## Why

The person should be able to write in the document and then say in the chat «I have reworked the introduction, adapt the rest», without explaining what changed. The agent needs to know which blocks the person edited since it last looked.

## Acceptance criteria

- [ ] At the start of each turn, for every document the session's agent has read or written, if blocks changed with `author: user` after the version the agent last saw, the turn carries a short note listing them: document title, and per block whether it was edited, inserted or deleted, with the new text of edited and inserted blocks cut to a limit. The note goes to the model only, not into the visible transcript. A Go test: agent writes a document, user replaces one block and deletes another, the next turn's model input has a note naming both; a turn with no user changes has no note.
- [ ] After that turn the agent's seen version moves forward, so the same changes are not reported twice. A Go test of two consecutive turns.
- [ ] Documents the session never touched are not reported, even if the person edits them. A Go test.

## Out of scope

- Any UI. Changes made by the agent itself are never reported back to it.

## Notes

- Depends only on 057: the store records `author` on every change, so the test can write user changes through the store. In use it shows its value once 059 lets people edit.
- Find where the agent loop adds per-turn context to the model input (`internal/agent/loop.go`) and add the note there; do not put it in the stored history.
