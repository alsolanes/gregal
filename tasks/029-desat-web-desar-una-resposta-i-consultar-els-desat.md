---
title: "Desat (web): desar una resposta i consultar els desats"
status: done
depends_on: [028]
---

## Why

On the web and the Windows desktop app the person can share a wind's reply but not keep it, and the «Desat» page only lists what the winds made, mixed with cited links. Saving has to be one click, and finding a saved recipe later has to be quick.

## Acceptance criteria

- [x] Every wind reply (Gregal included) has a «Desa» / «Save» action next to share; it posts the reply markdown to `/api/notes` with `kind:"auto"` and turns into «Desat ✓» / «Saved ✓», which opens the note. A node test checks the request body for a reply with a recipe card.
- [x] The «Desat» page starts with «Els teus desats» / «Your saved items»: a search box (`q`), chips per kind present (`kinds`) with localised names and icons, pinned first; each row shows title, kind and date. The winds' gallery (`renderSaved`) stays below it.
- [x] Opening a saved item shows the note view, where the cards render as in the chat (the recipe's servings change). Delete, pin and tags are the notes' own.
- [x] Every new string exists in Catalan and English.

## Out of scope

- The wind proposing to save (task 031).
- The Codi agent's chat.

## Notes

- Reply actions: `internal/web/app/share-msg.js` (`gregalShare.button`) and where it is attached in `winds-workspace.js`.
- Page: `savedView()` in `internal/web/app/winds-workspace.js` (line ~864) and `SAVED_TABS`; notes UI in `winds-notes.js` (`openNote`).
- Verify in the browser with the sandbox (memory: project-gregal-sandbox-testing).
- Done: `gregalShare.saveButton` / `saveReply` (share-msg.js) next to share on every wind reply in `paintHistory`; «Desat ✓» opens the note through `openSaved` → Notes view with `opts.open`.
- Done: `gregalNotes.saved()` (winds-notes.js) draws «Els teus desats» at the top of the Desat page (`kind=*`, `q`, chips from `kinds`, pinned first from the server); names and icons in `KIND_NAMES`/`KIND_ICONS`. The winds' gallery is below under «El que han fet els vents».
- The note view now binds only `input.md-check` as tasks: before, a recipe's ingredient checkboxes (`gc-check`) were indexed as text tasks and ticking one rewrote and repainted the note.
- Verified in a sandbox server with a stub model: save from chat, «Desat ✓» opens the note, servings 2→3 rescale, chips and search filter, pinned tip first.
- Follow-up: `q` matches inside card fences, so searching «rega» also finds every recipe (it matches «gregal-recipe»); the note excerpt in Notes also shows the raw card JSON. Both come from notes.go.
- Follow-up: Android parity is task 030.
- Review fixes: share-msg.js remembers saved replies by their text (`savedNote`), because `paintHistory` rebuilds the buttons and the history comes back from the server as new objects; a repainted reply starts as «Desat ✓» and a second click opens the note instead of saving a duplicate. The memory lasts for the page; after a reload «Desa» shows again.
- The kind chips show only when there are two kinds or more (`savedChips`): with one kind they would not filter anything.
