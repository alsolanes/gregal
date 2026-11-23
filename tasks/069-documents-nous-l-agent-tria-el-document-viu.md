---
title: "Documents nous: l'agent tria el document viu"
status: todo
depends_on: []
---

## Why

Asked «fes un word amb text inventat» in Codi mode (checked by the user on 2026-11-23 with DeepSeek V4.1 Flash), the agent called `office_create`, whose name matches the request, and the person got a closed `informe-inventat.docx` instead of a live document. A new text document should be a live document that exports to Word; `office_create` stays for what a live document cannot do. The winds get the same rule in 066.

## Acceptance criteria

- [ ] The `doc` and `office_create` tool descriptions say the same rule: a new text document (report, letter, notes, a «Word») is a live document with `doc`, and the reply gives its Word export link; `office_create` is for templates, presentations, spreadsheets and documents the person attached. A Go test that both descriptions carry the rule, so a later edit cannot drop it from one of them.
- [ ] The Codi agent asked «fes un word amb text inventat» in a project calls `doc`, not `office_create`, and its reply carries the export link; asked for a presentation it still calls `office_create`. Checked against a real model (the user's case used DeepSeek V4.1 Flash) in a sandbox; the commands and the before/after tool calls go in the notes.

## Out of scope

- Presentation and spreadsheet live documents.
- Removing `office_create` or changing what it does.

## Notes

- Agreed with the user: the live document is the default for new text documents; `office_create` keeps templates, pptx, xlsx and existing files.
- Texts: `internal/agent/doc_tool.go` (`doc` spec), `internal/agent/loop.go` (`office_create` spec, ~191), `agent.OfficeDeliveryRules`, and the office skills the `office_create` description tells the agent to load (`office-docx`). The winds' brief (`windDocsBrief`) is 066's.
- Sandbox for the live check: isolate `GREGAL_DATA_DIR` and the auth/memory dirs as in the user's sandbox testing notes.
- Wind quality eval: the gqual sandbox and its `run.py` question battery (user memory); evals code in `internal/evals`.
