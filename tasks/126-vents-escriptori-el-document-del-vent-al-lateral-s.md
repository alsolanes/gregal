---
title: Vents escriptori: el document del vent al lateral s'obre i es tanca quan vols
status: todo
depends_on: []
---

## Why

The document a wind writes opens in a side pane next to the chat; the person wants to open and close it whenever they like (2026-11-24), not only when the wind opens it.

## Acceptance criteria

- [ ] In a wind chat that has a document (or computer view), a toggle button in the chat header opens and closes the side pane at will; closing does not lose the document, and reopening shows the same one. A node test.
- [ ] When the wind changes the document again (`doc_changed`) while the pane is closed by the person, the pane stays closed and the toggle shows a dot that there is something new; it opens only if the person never closed it in this chat. A node test.
- [ ] The pane width can be dragged and is remembered (localStorage), with a minimum for chat and pane; narrow screens (<900 px) keep the overlay with close. Screenshots in `output/ui-review/126/`.

## Out of scope

- Android (the doc card already opens and closes).
- Dark-mode fixes (125).

## Notes

- Files: winds-workspace.js (streamChat doc_changed handling, openComputer), docs-panel.js (`window.gregalDocs`), `#workbench` in index.html. Task 089 built the pane.
