---
title: "Vents (web i escriptori): el document del vent s'obre al panell"
status: todo
depends_on: ["066"]
---

## Why

With 066 a wind writes live documents, but the Vents chat does not open them. The person should see the document next to the wind's chat, edit it, and unpin it into its own window, as in Codi.

## Acceptance criteria

- [ ] When the wind chat receives `doc_changed` with `scope:"mine"`, the web app opens the document in the side panel next to the wind's chat (or brings it forward if already open), the same panel as 058. A node test of the event handling in the wind chat.
- [ ] The panel opened for a wind document reads, streams, edits (059) and exports with `scope=mine` on every call, and its list of documents shows the person's wind documents. A node test that a wind-scoped panel builds every URL with `scope=mine`, and a Codi panel never does.
- [ ] «Obre en una finestra» on a wind document opens `/app/doc/{id}?scope=mine` (desktop window and browser), and edits sync between the window and the panel. A test of the URL; checked by hand in a desktop build, with that build's commit in the notes.
- [ ] Every string in Catalan and English.

## Out of scope

- Android (068). Changing how the agent chooses between `doc` and `office_create` (069).

## Notes

- Panel and view: `internal/web/app/docs-panel.js`, `doc-view.js`, page `doc-page.js`; desktop window `desktop/doc-window.js` (it must stay in `desktop/package.json` `build.files`, which `release.test.js` checks).
- Wind chat clients: `internal/web/app/winds.js`, `winds-workspace.js`; check the panel's container exists in the Vents view, not only in Codi's `#workbench`.
- The hand check runs in a packaged desktop build (portable), not in the dev server: 061 only failed there.
