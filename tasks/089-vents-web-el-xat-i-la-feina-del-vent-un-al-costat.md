---
title: Vents web: el xat i la feina del vent, un al costat de l'altre
status: todo
depends_on: []
---

## Why

While a wind works, the person should see what it is doing next to the conversation, not in a window that hides the chat: the computer it drives, the document it writes, the page it opens.

## Acceptance criteria

- [ ] When a wind's turn uses the computer (an `activity` event of an ordinador tool, or the handoff card), the wind chat opens a side pane in `#workbench` with the live computer view (the `/api/ordinador/live` polling that `openLocalComputer` uses today in a modal), instead of the modal dialog; the person can close it. A node test that the pane opens on that event and not on others.
- [ ] A `doc_changed` keeps opening the Gregal Docs pane next to the chat (as today), and only one work pane shows at a time: opening the computer pane closes the docs pane and the other way round. A node test.
- [ ] On screens narrower than 900 px the work pane opens over the chat with a close button, as the docs pane does today.
- [ ] The remote viewer (`/api/ordinador/viewer`, a new window) keeps working for remote use.

## Out of scope

- Android (090).
- Changing what the ordinador can do.

## Notes

- Files: `winds-workspace.js` (`openComputer` 981, `openLocalComputer` 985, `streamChat` 400-434 handles `activity` and `doc_changed`), `docs-panel.js` (`window.gregalDocs`), `#workbench` in `index.html` (612, 1734-1754).
- Web: node tests with `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server in the browser pane, dark and light theme, with before/after screenshots in `output/ui-review/` (ignored).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
