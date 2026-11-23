---
title: "Gregal Docs (web): un botó obre els teus documents i en crees de nous"
status: todo
depends_on: ["066"]
---

## Why

Gregal Docs only opens from a chat when the agent writes a document. The person should also be able to use it as an ordinary editor inside the app: open their documents from a button and start a new one themselves, with no agent involved.

## Acceptance criteria

- [ ] The app (web and desktop) has a «Gregal Docs» button in the sidebar, in both Codi and Vents, that opens a Docs home with two sections: «Els meus documents» (`scope=mine`) and, when a Codi project is open, «Aquest projecte». Each lists title, kind and last change, newest first, and opens the document in the existing editor. A node test of the home with and without a project.
- [ ] «Document nou» creates an empty text document in «Els meus documents» through `POST /api/docs` `{title, kind:"doc"}` (author `user`) and opens it with the cursor in a first empty block. A Go test of the route (scope, author, 400 for an unknown kind) and a node test that a new document opens in edit mode.
- [ ] A document with no blocks can be written in: the editor shows an empty block to type into, on any document, not only new ones. A node test.
- [ ] Every string in Catalan and English; `docs/api-contract.md` documents `POST /api/docs`.

## Out of scope

- Presentations and spreadsheets (073, 076 add their «nou» buttons to this home). Android (071).
- Renaming, deleting or moving documents.

## Notes

- Agreed with the user: the home shows «Els meus documents» (the person's own folder, the same root the winds use, 066) and «Aquest projecte» in Codi; what the button creates goes to «Els meus documents».
- Editor and panel: `internal/web/app/docs-panel.js`, `doc-view.js`, `doc-page.js`; the empty-document gap is a follow-up of 059 and 063.
- Sidebar: `internal/web/index.html`, `internal/web/app/shell-layout.js`.
