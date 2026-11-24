---
title: Vents escriptori: el document del vent al lateral s'obre i es tanca quan vols
status: done
depends_on: []
---

## Why

The document a wind writes opens in a side pane next to the chat; the person wants to open and close it whenever they like (2026-11-24), not only when the wind opens it.

## Acceptance criteria

- [x] In a wind chat that has a document (or computer view), a toggle button in the chat header opens and closes the side pane at will; closing does not lose the document, and reopening shows the same one. A node test.
- [x] When the wind changes the document again (`doc_changed`) while the pane is closed by the person, the pane stays closed and the toggle shows a dot that there is something new; it opens only if the person never closed it in this chat. A node test.
- [x] The pane width can be dragged and is remembered (localStorage), with a minimum for chat and pane; narrow screens (<900 px) keep the overlay with close. Screenshots in `output/ui-review/126/`.

## Out of scope

- Android (the doc card already opens and closes).
- Dark-mode fixes (125).

## Notes

- Files: winds-workspace.js (streamChat doc_changed handling, openComputer), docs-panel.js (`window.gregalDocs`), `#workbench` in index.html. Task 089 built the pane.
- Done: `winds-workspace.js` keeps, per chat (thread id), the last thing the side pane showed (`doc` with id and scope, or `computer`), whether the person closed it, and whether there is something new. The toggle (`.vw-pane-toggle`, the rightmost button of the chat header, also in Gregal's own header) has `aria-pressed`, and `data-fresh` draws the dot; it reopens the same document with `gregalDocs.open(id, scope)` or the computer pane.
- `docs-panel.js`: only its × (the person) dispatches `gregal:docsclosed`; the closes caused by the computer pane (`gregal:workpane`) and by a session switch no longer go through the × and do not dispatch it. «Open in a window» still goes through the ×, so it counts as the person closing the pane.
- Width: a grip (`.vw-pane-grip`, `role=separator`, arrow keys move it 24 px) on the left edge of both panes sets `--work-pane-w` on `#workbench`, saved in `localStorage` `gregal_work_pane_w`; the pane keeps 340 px and the chat 420 px (also in CSS with `max-width:calc(100% - 420px)`, so a narrower window does not crush the chat). The grip is hidden on the overlay, which stays at `max-width:1000px` (the breakpoint both panes already shared after 089), not 900 px.
- Strings use the file's own `t(ca, en)` / `tx(ca, en)`, like the rest of `winds-workspace.js`, not `i18n.js` keys.
- Screenshots: `output/ui-review/126/` (not committed), 1280 px and 800 px, dark and light, sandbox server with the wind stream and the document simulated.
- Follow-up: the computer pane still opens on its own on a turn that uses the browser even if the person closed the side pane in that chat; only `doc_changed` respects the person's close.
- Follow-up: the grip separator has no `aria-valuenow`.
- Follow-up: the side pane is shared by all chats, the record is per chat: switching to another chat leaves the pane open, so that chat's toggle shows pressed and closing it closes the other chat's document (and marks this chat as closed by the person).
