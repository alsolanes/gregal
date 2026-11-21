---
title: "Desat: una nota pot ser un element desat amb tipus"
status: done
depends_on: []
---

## Why

The person wants to keep recipes, tips, plans, charts and webs that Gregal or a wind made, and find them again easily. Notes already are a per-account markdown store with search, tags, pins, both clients and a `notes` tool the winds use, so a saved item is a note with a `kind`; no second store.

Format: the note body is the reply's markdown as it was, cards included as their fences (```` ```gregal-recipe {…} ````, ```` ```chart ````, ```` ```gregal-web ````…), so the existing renderers (md.js, `richBlocks` in Md.kt) draw it again, the recipe keeps its servings scaler, and search finds the text inside the JSON.

## Acceptance criteria

- [x] `POST /api/notes {action:"save", body, kind}` stores `kind` in the front matter and returns it; `kind` is one of `recipe, tip, plan, route, compare, facts, result, chart, web, text` (400 otherwise). Omitted on an update, it is kept.
- [x] A save with `kind:"auto"` takes the kind from the first card fence of the body (`gregal-recipe` → `recipe`, `chart`/`gregal-plot` → `chart`, `gregal-web` → `web`, …) and `text` when there is none; the title defaults to that card's `title` before the first line.
- [x] `GET /api/notes?kind=recipe` returns only those notes, `?kind=*` every note with a kind, and the list response adds `kinds:[{name,count}]`. Existing notes without `kind` behave as before.
- [x] `docs/api-contract.md` (the `/api/notes` rows) describes `kind`, `?kind=` and `kinds`.

## Out of scope

- Any client change (tasks 029 and 030).
- The winds' `notes` tool: it already searches every note; it does not set `kind`.

## Notes

- `internal/web/notes.go`: `Note`, `noteHead`, `noteInput`, `saveNoteOnly`, the list handler and its counts (`notebooks`, `tags`). Tests in `internal/web/notes_test.go` (`notesReq`).
- Card fence names: `internal/web/app/md.js` around line 397 and `CardsGuide` in `internal/companion/winds.go`.
