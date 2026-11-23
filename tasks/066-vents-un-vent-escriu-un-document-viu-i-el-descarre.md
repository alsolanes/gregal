---
title: "Vents: un vent escriu un document viu i el descarregues en Word"
status: done
depends_on: []
---

## Why

Gregal Docs (057–063) only reach the agent of the Codi mode. Asked «crea un Word» in Vents, a wind still makes a closed `.docx` with `office_create`, with no live document to watch or edit. Winds have no project: their documents live in the person's wind documents folder (`windDocsDir`, per user). This task gives winds the `doc` tool rooted there, and lets the docs routes reach those documents.

## Acceptance criteria

- [x] In a wind chat turn the `doc` tool is offered and allowed, and every document it creates or edits lives in the gdocs store rooted at the person's wind documents folder (`windDocsDir`), never in the server's working project. A Go test: a wind turn's `doc create` lands in that root; the same call from the Codi agent lands in the project.
- [x] Two accounts never see each other's wind documents: with accounts, the root is per user (`windStateDir`). A Go test with two users.
- [x] The docs routes (`/api/docs`, `/api/docs/{id}`, `/stream`, `/export`, and the block routes of 059) take `?scope=mine` to use the caller's wind documents root instead of the session's project. A Go test: a wind document is listed, read and exported with `scope=mine`, and is a 404 without it.
- [x] The wind stream emits `doc_changed` `{id, title, scope:"mine"}` when a wind's `doc` call creates or changes a document, like `motor_web.go` does for Codi turns. A Go test.
- [x] The winds' documents brief (`windDocsBrief`) gives the rule of 069: a new text document is a live document with `doc` and the reply gives its Word export link; `office_create` is for templates, presentations, spreadsheets and attached documents. A Go test that the brief carries it; a wind asked «crea un Word amb …» calls `doc`, checked once against a real model with the commands in the notes.
- [x] `docs/api-contract.md` documents `scope=mine` and the `scope` field of `doc_changed`.

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
- Done: in a wind turn whose context has a documents folder (`windDocsCtx`), `doc` is allowed before the shared policy (so also in read-only wind modes) and runs through `windDocExec` (`winds_docs.go`) with `windDocsOf(ctx)` as dir. Turns without that folder (the review turn of `readOnlyWindCtx`) still get the policy's deny. The tool spec is already in `windSpecs` (it is deferred behind `load_tool`, like the office tools).
- `windDocExec` rewrites the tool's export link to `/export?scope=mine&format=…` (no change to `doc_tool.go`, which task 072 is editing), and emits `doc_changed` `{id, title, scope:"mine"}` through the loop's emit; `forwardWindAsk` passes it to every wind SSE stream. `emitDocChanged` now takes the root and scope; Codi passes `s.sessionCwd()` and "".
- Session key for the seen record and the conflict guard in wind turns: `s.id`, the server session the wind tool calls already run with (`windToolExec(ctx, s.id, …)`).
- Routes: `docStore(w, r)` uses `s.windDocsDir()` with `?scope=mine`; that covers list, document, stream, export, the block routes and `/app/doc/{id}`.
- Real-model check (2026-11-23), OpenCode Go `zen` + `deepseek-v4.1-flash`, sandbox under `%TEMP%/g066` (`GREGAL_DATA_DIR`, `GREGAL_AUTH`, `GREGAL_MEMORY`, `HOME`, `USERPROFILE` all there, own binary `go build -o %TEMP%/g066/gregal066.exe .`, config with the four roles on zen and `verify.mode: off`): `gregal066.exe --config config.yaml --serve --addr 127.0.0.1:18766`, `POST /api/winds {"name":"Mestral",…}`, `POST /api/winds/chat/stream {"wind":"mestral","message":"crea un Word amb una carta curta de benvinguda per a un nou veí de l escala"}`. The wind loaded and called `doc` (3 steps: `load_tool`, `doc`, reply; no `office_create`), the stream sent `doc_changed {"id":"t2qy355soeup","scope":"mine",…}`, the reply carried `[Carta de benvinguda a l'escala](/api/docs/t2qy355soeup/export?scope=mine&format=docx)`; that link downloaded a docx (200), without `scope=mine` it was a 404, and `/api/docs` of the project stayed empty.
- Review fix (privacy): personal documents open with `gdocs.OpenAt(windDocsDir)`, the exact folder, never `backlog.Root`; with the config inside a git repo every person's wind folder resolved to the repo root, so people and the project shared one store. `TestWindDocsStayPerUserInsideAGitRepo` reproduced it (the project listed Anna's document) before the fix. Wind `doc` calls now run `agent.DocExecIn` on that store instead of the generic tool path, and only `create`'s link is rewritten to `scope=mine`.
- Follow-up: wind turns do not get the 060 turn note (`notaDocs`): it runs in the motor, and wind turns use `agent.Loop` in `plan.go`.
- Follow-up: `doc list` in a wind says «No documents in this project.»; the wording is the project's.
