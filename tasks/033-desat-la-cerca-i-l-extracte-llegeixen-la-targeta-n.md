---
title: "Desat: la cerca i l'extracte llegeixen la targeta, no el JSON"
status: done
depends_on: [028]
---

## Why

A saved reply keeps its card as a JSON fence (task 028). Today the note search matches the fence syntax («rega» finds every recipe through `gregal-recipe`, «title» finds every card) and the list excerpt shows the raw JSON, so the saved items are hard to scan and search. Found while doing task 029.

## Acceptance criteria

- [x] `GET /api/notes?q=` matches the text inside a card (title, ingredient names, steps, tip…) but not the fence name nor the JSON keys: `q=llenties` finds a recipe card about lentils, `q=rega` and `q=title` do not find it. Go test.
- [x] The list `excerpt` and the search `snippet` of a note with a card show the card's text (e.g. «Llenties estofades · 400 g llenties…»), never `{` or `"title"`. Go test.
- [x] Notes without cards keep the same excerpt, snippet and ranking (the existing notes tests pass unchanged).

## Out of scope

- The note body as stored and as returned by `?id=`: it stays the original markdown.
- Client changes.

## Notes

- `internal/web/notes.go`: `noteExcerpt`, `searchNotes`, `noteSnippet`; `noteCardFence` in `notes_kind.go`.
