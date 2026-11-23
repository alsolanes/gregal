---
title: "Documents nous: l'agent tria el document viu"
status: done
depends_on: []
---

## Why

Asked «fes un word amb text inventat» in Codi mode (checked by the user on 2026-11-23 with DeepSeek V4.1 Flash), the agent called `office_create`, whose name matches the request, and the person got a closed `informe-inventat.docx` instead of a live document. A new text document should be a live document that exports to Word; `office_create` stays for what a live document cannot do. The winds get the same rule in 066.

## Acceptance criteria

- [x] The `doc` and `office_create` tool descriptions say the same rule: a new text document (report, letter, notes, a «Word») is a live document with `doc`, and the reply gives its Word export link; `office_create` is for templates, presentations, spreadsheets and documents the person attached. A Go test that both descriptions carry the rule, so a later edit cannot drop it from one of them.
- [x] The Codi agent asked «fes un word amb text inventat» in a project calls `doc`, not `office_create`, and its reply carries the export link; asked for a presentation it still calls `office_create`. Checked against a real model (the user's case used DeepSeek V4.1 Flash) in a sandbox; the commands and the before/after tool calls go in the notes.

## Out of scope

- Presentation and spreadsheet live documents.
- Removing `office_create` or changing what it does.

## Notes

- Agreed with the user: the live document is the default for new text documents; `office_create` keeps templates, pptx, xlsx and existing files.
- Texts: `internal/agent/doc_tool.go` (`doc` spec), `internal/agent/loop.go` (`office_create` spec, ~191), `agent.OfficeDeliveryRules`, and the office skills the `office_create` description tells the agent to load (`office-docx`). The winds' brief (`windDocsBrief`) is 066's.
- Sandbox for the live check: isolate `GREGAL_DATA_DIR` and the auth/memory dirs as in the user's sandbox testing notes.
- Wind quality eval: the gqual sandbox and its `run.py` question battery (user memory); evals code in `internal/evals`.
- Done: one rule in both descriptions, «a new text document (a report, a letter, notes, «a Word»)» goes to `doc` and the reply gives the Word link. In `doc_tool.go` it replaces 072's «Prefer it over office_create when the person wants a document or a new presentation», which now says only the presentation part. In `office_create` (`loop.go`) the `docx:` clause starts with the rule, written like 072's `pptx:` clause: a docx with `office_create` only from a .dotx/.docx template or a document the person attached. Presentations and spreadsheets keep their own clauses (072's deck one, 075's sheet one), so the criterion's «office_create is for templates, presentations, spreadsheets…» is not repeated in the docx clause. Test: `TestElsDocumentsDeTextNousVanADoc` (`internal/agent/doc_tool_test.go`).
- Criterion 2 vs 072: the criterion says a presentation «still calls `office_create`»; since 072 (the base of this branch) a new presentation goes to `doc` with kind "deck", and the before run already did that. The check below verifies that a presentation is unchanged by this task (still `doc` deck), not that it calls `office_create`.
- Real-model check (2026-11-23), `zen` + `deepseek-v4.1-flash`, sandbox `%TEMP%/g069` (`GREGAL_DATA_DIR`, `GREGAL_AUTH`, `GREGAL_MEMORY`, `HOME`, `USERPROFILE` all there; config copied from the 066 sandbox; project folder `proj` with a synthetic README, not a git repo). Binaries `go build -o %TEMP%/g069/gregal069-before.exe .` at 1770affb and `gregal069-after.exe` with this change. Command, from `proj`: `gregal069-<x>.exe --config ../config.yaml --mode code --auto-approve --no-session --max-steps 25 --output stream-json -p "<prompt>"`.
  - Before, «fes un word amb text inventat»: `read`+`skill`, `office_create`, `office_render`, `send_file` (a closed `Hortet_guia_inventada.docx`; the user's bug reproduced).
  - After, same prompt: `read`+`office_read`, `doc` (create), `doc` (read); reply «[Descarrega el Word](/api/docs/…/export?format=docx)». Rerun in a clean folder: `doc` only, reply starting «Fet: [El poble de Vilanera - text inventat](/api/docs/…/export?format=docx)».
  - Presentation, «fes una presentació de tres diapositives sobre l'hortet»: before `read`+`office_read`, then 5× `doc`; after `read`, then 5× `doc`; both replies carry `/api/docs/…/export?format=pptx`.
  - Run through `-p` (same Codi mode and tool specs as a web Codi session), not through the web client.
- Follow-up: `OfficeDeliveryRules` (`mode.go`) still says «create each complete file in one office_create call», and the `office-docx` skill's «Creating» section starts with `office_create`; neither mentions `doc`. The real-model check passed without changing them.
