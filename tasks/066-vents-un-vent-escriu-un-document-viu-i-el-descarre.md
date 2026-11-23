---
title: "Vents: un vent escriu un document viu i el descarregues en Word"
status: todo
depends_on: []
---

## Why

Gregal Docs (057–063) only reach the agent of the Codi mode. Asked «crea un Word» in Vents, a wind still makes a closed `.docx` with `office_create`, with no live document to watch or edit. Winds have no project: their documents live in the person's wind documents folder (`windDocsDir`, per user). This task gives winds the `doc` tool rooted there, and lets the docs routes reach those documents.

## Acceptance criteria

- [ ] In a wind chat turn the `doc` tool is offered and allowed, and every document it creates or edits lives in the gdocs store rooted at the person's wind documents folder (`windDocsDir`), never in the server's working project. A Go test: a wind turn's `doc create` lands in that root; the same call from the Codi agent lands in the project.
- [ ] Two accounts never see each other's wind documents: with accounts, the root is per user (`windStateDir`). A Go test with two users.
- [ ] The docs routes (`/api/docs`, `/api/docs/{id}`, `/stream`, `/export`, and the block routes of 059) take `?scope=mine` to use the caller's wind documents root instead of the session's project. A Go test: a wind document is listed, read and exported with `scope=mine`, and is a 404 without it.
- [ ] The wind stream emits `doc_changed` `{id, title, scope:"mine"}` when a wind's `doc` call creates or changes a document, like `motor_web.go` does for Codi turns. A Go test.
- [ ] The winds' documents brief (`windDocsBrief`) gives the rule of 069: a new text document is a live document with `doc` and the reply gives its Word export link; `office_create` is for templates, presentations, spreadsheets and attached documents. A Go test that the brief carries it; a wind asked «crea un Word amb …» calls `doc`, checked once against a real model with the commands in the notes.
- [ ] `docs/api-contract.md` documents `scope=mine` and the `scope` field of `doc_changed`.

## Out of scope

- Opening the document in the web panel (067) or on Android (068).
- The Codi agent's choice between `doc` and `office_create` (069).

## Notes

- Agreed with the user: a wind's documents live in the person's wind documents folder, the same folder where winds save their files today.
- Wind tool policy: `internal/web/winds_docs.go` (`withWindDocs`, `windDocsOf`, `windDocTool`, `windDocDecide`), applied in `internal/web/plan.go` (~319, ~468). Wind turns run in a read-only mode, where the policy denies `doc` and `specs_mode.go` hides it; winds need it allowed the way the wind document tools are.
- The `doc` tool runs with the turn's `dir` (`internal/agent/loop.go`, `case "doc"` → `docExec(dir, …)`); in a wind turn it must use `windDocsOf(ctx)`. `gdocs.Open` uses `backlog.Root`, which returns the folder itself when it is not a git repo.
- Wind turns: `internal/web/winds_stream.go` (`windDocsCtx` at ~354 and ~889, `windDocsBrief`), `winds_decisions.go:470`, `winds_specialists.go:357`.
- Store resolution for the routes: `docStore` in `internal/web/docs_api.go`.
- The per-session seen record (060) and the conflict guard (059) need a session key for wind turns; use the one the wind tool calls already run with, and say which in the notes.
