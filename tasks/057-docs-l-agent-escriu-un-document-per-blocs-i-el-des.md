---
title: "Docs: l'agent escriu un document per blocs i el descarregues en Word"
status: todo
depends_on: []
---

## Why

Gregal Docs are live documents that the agent and the person edit at the same time, and that leave as Office files only when exported. Today the agent can only produce a closed `.docx` with `office_create`, which has to be regenerated whole and is locked while Word has it open. This first slice gives the agent a project document made of blocks that it creates and edits one block at a time, and gives the person a Word download of it.

## Acceptance criteria

- [ ] A project-scoped document store: a document is `{id, title, kind:"doc", blocks:[{id, md, version, author, updated}], version, created, updated}`, one per file under the app data dir keyed by project root (like `backlog.AppFile`), safe for concurrent writers. Every block change bumps the block and document `version` and records `author` (`agent` or `user`). A Go test: two documents in two projects do not see each other; a replace bumps both versions and records the author; concurrent replaces on different blocks both survive.
- [ ] A native tool `doc` with `action`: `create` (`title`, optional `md` split into blocks at headings and blank lines), `list`, `read` (returns every block with its id and version), `insert` (`after` block id or start, `md`), `replace` (`block`, `md`) and `delete` (`block`). It is in `tools.Noms()`, runs without confirmation in the write modes, and is denied in the read-only modes like `office_edit`. A Go test through the agent policy: allowed in code mode, denied in chat/plan mode; and a tool test of create → insert → replace → delete → read.
- [ ] `GET /api/docs/{id}/export?format=docx` returns a valid `.docx` built from the blocks with the existing `ooxml.ParseMarkdown` + `ooxml.NewDocx` path and the active brand theme, named after the title. After `create`, the tool's result tells the agent the link `[title](/api/docs/{id}/export?format=docx)` to give the person. A Go test: a document with a heading, a list and a table exports to a docx that `ooxml` reads back with the same text; an id from another project returns 404.
- [ ] `docs/api-contract.md` documents the export route; `TestContracteAPIDocumentat` passes.

## Out of scope

- Any client UI (058, 061, 062), user edits and conflicts (059), the per-turn summary of user changes (060).
- Presentation (`deck` → pptx) and spreadsheet (`sheet` → xlsx) kinds.
- Undoing to an older block version: the history is recorded here, a restore UI is a follow-up.

## Notes

- Agreed with the user: a document belongs to the project (any chat in the project can open it); agent edits pass without confirmation because they are visible live and versioned, and are denied in read-only modes. This is a new tool, not a change to `office_edit`'s confirmation.
- One `doc` tool with actions follows the `backlog` tool, instead of four `doc_*` tools; every list that names tools must include it: `internal/tools/registre.go`, `internal/agent/loop.go` (spec, ~513 read-only list, ~650 read-only deny, dispatch), `autonomous_policy.go`, `finalize.go`, `specs_mode.go`, `project.go`, `internal/ordinador/agent.go` if computer agents should have it.
- Storage pattern: `internal/backlog/store.go` (`Root`, `AppFile`, per-root lock). A new package such as `internal/gdocs` (avoid `docs`, which is the documentation folder).
- Export: `internal/ooxml/markdown.go` (`ParseMarkdown`), `docx_build.go` (`NewDocx`), theme from the active brand as `office_create` does (`internal/tools/office.go`, `createDocx`). Download auth like `/api/winds/doc` (`internal/web/winds_docs.go`).
- Follow-up, not this diff: `deck` → pptx with `ooxml.DeckFromMarkdown`, `sheet` → xlsx with `internal/xlsxdoc`.
