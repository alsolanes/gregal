---
title: Vents web: el xat i la feina del vent, un al costat de l'altre
status: done
depends_on: []
---

## Why

While a wind works, the person should see what it is doing next to the conversation, not in a window that hides the chat: the computer it drives, the document it writes, the page it opens.

## Acceptance criteria

- [x] When a wind's turn uses the computer (an `activity` event of an ordinador tool, or the handoff card), the wind chat opens a side pane in `#workbench` with the live computer view (the `/api/ordinador/live` polling that `openLocalComputer` uses today in a modal), instead of the modal dialog; the person can close it. A node test that the pane opens on that event and not on others.
- [x] A `doc_changed` keeps opening the Gregal Docs pane next to the chat (as today), and only one work pane shows at a time: opening the computer pane closes the docs pane and the other way round. A node test.
- [x] On screens narrower than 900 px the work pane opens over the chat with a close button, as the docs pane does today.
- [x] The remote viewer (`/api/ordinador/viewer`, a new window) keeps working for remote use.

## Out of scope

- Android (090).
- Changing what the ordinador can do.

## Notes

- Files: `winds-workspace.js` (`openComputer` 981, `openLocalComputer` 985, `streamChat` 400-434 handles `activity` and `doc_changed`), `docs-panel.js` (`window.gregalDocs`), `#workbench` in `index.html` (612, 1734-1754).
- Web: node tests with `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server in the browser pane, dark and light theme, with before/after screenshots in `output/ui-review/` (ignored).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
- Done: `openLocalComputer` is now a pane (`.vw-computer-pane`) in `#workbench`. It opens on the first `activity` of `browser`/`viewer` in a turn (the same names as `stepLabel`), and on every handoff `approve_request`/`question_request`. The docs pane and the computer pane tell each other with a `gregal:workpane` document event (`detail` `docs` or `computer`), and the other one closes. The `dialog` modal and its `.vw-dialog.vw-live-computer` rule are gone.
- `openComputer` (remote viewer in a new window, falling back to the pane on `no_viewer`) is unchanged. If the pane opens on its own for an account with a remote computer, `/api/ordinador/live` answers 409 "remote computer" and the pane offers «Open the computer» (the viewer). That flow was not run against a real remote computer.
- Below 900 px the pane is fixed over the chat (`inset:92px 0 0 auto`, the docs pane values), checked in headless screenshots at 1280 and 800 px, dark and light, with the stream and `/api/ordinador/live` simulated (`output/ui-review/089/`).
- Follow-up: the docs pane switches to its overlay at `max-width:1000px` and the computer pane at 900 px, so between 900 and 1000 px they behave differently.
- Follow-up done (fix/vents-web-detalls): the computer pane switches to its overlay at `max-width:1000px`, the same as the docs pane. Test: `winds-start.test.cjs` «the docs pane and the computer pane switch to an overlay at the same width».
