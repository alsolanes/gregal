---
title: "Presentacions: l'agent fa diapositives vives i les descarregues en PowerPoint"
status: todo
depends_on: []
---

## Why

Gregal Docs should also cover presentations. A live presentation is a document of kind `deck` whose blocks are slides; it is edited slide by slide and exports to PowerPoint with the deck builder Gregal already has.

## Acceptance criteria

- [ ] The store accepts `kind:"deck"`: each block is one slide, stored as the fields of `ooxml.SlideSpec` (layout, title, bullets or body, two columns, image, chart, notes), with the same per-block version, author, conflicts and stream events as text blocks. A Go test: create a deck, replace one slide, an invalid layout is rejected.
- [ ] The `doc` tool creates and edits decks (`create` with `kind:"deck"`, `insert`/`replace` with a slide), and `read` shows each slide's fields. A tool test of create → insert → replace → read.
- [ ] `GET /api/docs/{id}/export?format=pptx` builds the presentation with `ooxml` (`DeckSpec`) and the active brand theme; a text document asked for `pptx`, or a deck for `docx`, is a 400. A Go test that the exported pptx reads back with the slides' titles in order.
- [ ] The `doc` and `office_create` descriptions send new presentations to `doc` with `kind:"deck"`; `office_create` keeps `.pptx` templates and files the person attached. A Go test that both descriptions say so.
- [ ] `docs/api-contract.md` documents the deck block shape and `format=pptx`.

## Out of scope

- Editing slides in a client (073, 074). Free placement of boxes on a slide.

## Notes

- Agreed with the user: slides are edited per slide by layout and fields, rendered as a real slide; no dragging of boxes.
- `internal/ooxml/pptx_deck.go` (`DeckSpec`, `SlideSpec`, `ChartSpec`, `ParseDeckSpec`), `pptx_build.go`; the existing `office_create` pptx path in `internal/tools/office.go` (`createPptx`).
- Store and tool: `internal/gdocs`, `internal/agent/doc_tool.go`; export route `internal/web/docs_api.go`.
- If this does not fit under the diff guard, split the export into its own task.
