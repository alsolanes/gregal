---
title: "Desat (web): desar una resposta i consultar els desats"
status: todo
depends_on: [028]
---

## Why

On the web and the Windows desktop app the person can share a wind's reply but not keep it, and the «Desat» page only lists what the winds made, mixed with cited links. Saving has to be one click, and finding a saved recipe later has to be quick.

## Acceptance criteria

- [ ] Every wind reply (Gregal included) has a «Desa» / «Save» action next to share; it posts the reply markdown to `/api/notes` with `kind:"auto"` and turns into «Desat ✓» / «Saved ✓», which opens the note. A node test checks the request body for a reply with a recipe card.
- [ ] The «Desat» page starts with «Els teus desats» / «Your saved items»: a search box (`q`), chips per kind present (`kinds`) with localised names and icons, pinned first; each row shows title, kind and date. The winds' gallery (`renderSaved`) stays below it.
- [ ] Opening a saved item shows the note view, where the cards render as in the chat (the recipe's servings change). Delete, pin and tags are the notes' own.
- [ ] Every new string exists in Catalan and English.

## Out of scope

- The wind proposing to save (task 031).
- The Codi agent's chat.

## Notes

- Reply actions: `internal/web/app/share-msg.js` (`gregalShare.button`) and where it is attached in `winds-workspace.js`.
- Page: `savedView()` in `internal/web/app/winds-workspace.js` (line ~864) and `SAVED_TABS`; notes UI in `winds-notes.js` (`openNote`).
- Verify in the browser with the sandbox (memory: project-gregal-sandbox-testing).
