---
title: "Presentacions: l'agent fa diapositives vives i les descarregues en PowerPoint"
status: done
depends_on: []
---

## Why

Gregal Docs should also cover presentations. A live presentation is a document of kind `deck` whose blocks are slides; it is edited slide by slide and exports to PowerPoint with the deck builder Gregal already has.

## Acceptance criteria

- [x] The store accepts `kind:"deck"`: each block is one slide, stored as the fields of `ooxml.SlideSpec` (layout, title, bullets or body, two columns, image, chart, notes), with the same per-block version, author, conflicts and stream events as text blocks. A Go test: create a deck, replace one slide, an invalid layout is rejected.
- [x] The `doc` tool creates and edits decks (`create` with `kind:"deck"`, `insert`/`replace` with a slide), and `read` shows each slide's fields. A tool test of create → insert → replace → read.
- [x] `GET /api/docs/{id}/export?format=pptx` builds the presentation with `ooxml` (`DeckSpec`) and the active brand theme; a text document asked for `pptx`, or a deck for `docx`, is a 400. A Go test that the exported pptx reads back with the slides' titles in order.
- [x] The `doc` and `office_create` descriptions send new presentations to `doc` with `kind:"deck"`; `office_create` keeps `.pptx` templates and files the person attached. A Go test that both descriptions say so.
- [x] `docs/api-contract.md` documents the deck block shape and `format=pptx`.

## Out of scope

- Editing slides in a client (073, 074). Free placement of boxes on a slide.

## Notes

- Agreed with the user: slides are edited per slide by layout and fields, rendered as a real slide; no dragging of boxes.
- `internal/ooxml/pptx_deck.go` (`DeckSpec`, `SlideSpec`, `ChartSpec`, `ParseDeckSpec`), `pptx_build.go`; the existing `office_create` pptx path in `internal/tools/office.go` (`createPptx`).
- Store and tool: `internal/gdocs`, `internal/agent/doc_tool.go`; export route `internal/web/docs_api.go`.
- If this does not fit under the diff guard, split the export into its own task.
- Done: a block gets `slide *ooxml.SlideSpec` (`omitempty` on its fields now, so stored and read slides show only what is set); `md` stays empty in a deck. Kinds are explicit constants `gdocs.KindDoc` / `KindDeck`: `CreateDeck(title)` makes an empty deck, `InsertSlide` / `ReplaceSlide` share the private `insert` / `replace` with `Insert` / `Replace`, and `sameKind` refuses markdown in a deck and slides in a doc. 075 can add `KindSheet` the same way.
- A slide's `layout` must be one of the builder's kinds (`ooxml.SlideLayout`, over `layoutKinds`); empty is refused too, since a live deck has no template layouts to name.
- `doc` tool: `create` with `kind:"deck"` (md refused: a deck starts empty), `insert`/`replace` with `slide` (an object); `read` prints each slide as compact JSON. The create output still starts `Created document <id> ` so `emitDocChanged` opens the panel.
- Export: without `format` the document's own format; images in slides resolve against the project root; builder warnings (a missing image) are logged, not returned.
- Follow-up: `PUT`/`POST /api/docs/{id}/blocks` only take `md`; on a deck they fail with 500 (kind error not mapped to 400). 073/074 need a `slide` body and a 400 there.
- Follow-up: the turn note (`notaDocs`) prints `blk.MD`, so a slide the person changed shows with no text; it should show the slide title.
- Follow-up: the web/desktop/Android panels draw a deck's blocks as empty markdown until 073/074.
- Follow-up: a model that sends `slide` as a JSON string instead of an object gets a JSON error for the whole call; `office_create` repairs that with `llm.FixJSON`.
- Review fix: a slide's chart is checked with `ooxml.CheckChart` (the rules `AddChart` applies) when it is stored, so a chart with no data or with series and categories of different lengths is refused to the tool instead of breaking the whole pptx download.
- Follow-up: slide image paths (and `![](path)` in the docx export) can name any image on disk, absolute or with `..`; confine them to the document's root in a task of its own.
