---
title: "Gregal Docs (web): un botó obre els teus documents i en crees de nous"
status: done
depends_on: ["066"]
---

## Why

Gregal Docs only opens from a chat when the agent writes a document. The person should also be able to use it as an ordinary editor inside the app: open their documents from a button and start a new one themselves, with no agent involved.

## Acceptance criteria

- [x] The app (web and desktop) has a «Gregal Docs» button in the sidebar, in both Codi and Vents, that opens a Docs home with two sections: «Els meus documents» (`scope=mine`) and, when a Codi project is open, «Aquest projecte». Each lists title, kind and last change, newest first, and opens the document in the existing editor. A node test of the home with and without a project.
- [x] «Document nou» creates an empty text document in «Els meus documents» through `POST /api/docs` `{title, kind:"doc"}` (author `user`) and opens it with the cursor in a first empty block. A Go test of the route (scope, author, 400 for an unknown kind) and a node test that a new document opens in edit mode.
- [x] A document with no blocks can be written in: the editor shows an empty block to type into, on any document, not only new ones. A node test.
- [x] Every string in Catalan and English; `docs/api-contract.md` documents `POST /api/docs`.

## Out of scope

- Presentations and spreadsheets (073, 076 add their «nou» buttons to this home). Android (071).
- Renaming, deleting or moving documents.

## Notes

- Agreed with the user: the home shows «Els meus documents» (the person's own folder, the same root the winds use, 066) and «Aquest projecte» in Codi; what the button creates goes to «Els meus documents».
- Editor and panel: `internal/web/app/docs-panel.js`, `doc-view.js`, `doc-page.js`; the empty-document gap is a follow-up of 059 and 063.
- Sidebar: `internal/web/index.html`, `internal/web/app/shell-layout.js`.
- Done: `POST /api/docs` `{title, kind:"doc"}` lives in `handleDocs` (`handleDocCreate`, docs_api.go) and uses `docStore`, so it honours `?scope=mine` like every docs route; the client always sends `scope=mine`. Only `kind:"doc"` is accepted (400 otherwise, also for an empty title or bad JSON); 073/076 can widen it for decks and sheets.
- «author user»: a `gdocs.Doc` has no author of its own and the new document has no blocks, so `st.Create(title, "", AuthorUser)` leaves nothing to observe. `TestNewDocGoesToMyDocuments` checks that the first block written into it through the block route is `author: user`.
- Button: `#docsHomeBtn` in the rail (`.rail-main` of index.html), which shell-layout.js already moves into the sidebar in both Codi and Vents (visible above the winds menu in Vents). It opens the home inside the docs panel (`docs-panel.js` `home()`; rendering and «Document nou» in the new UMD `app/doc-home.js`). «Aquest projecte» shows when `gregalProduct === 'codi'`, the view is not the winds one and the session has a `cwd`.
- The panel now carries a `scope` per open document (list, stream, document, export link and block routes). «Obre en una finestra» is hidden for «Els meus documents»: `/app/doc/{id}` and `desktop/doc-window.js` do not pass `scope`.
- Empty documents (`doc-view.js`): a writable text document with no blocks shows a `.doc-empty` block («Escriu aquí…»); clicking it, or the panel after «Document nou», opens a new-block editor at the start (`open(null, '')`). It comes back when the editor closes empty or the last block is deleted, and goes when a block arrives. Not for decks (they are not edited as markdown).
- Checked in the browser (sandboxed server under `%TEMP%/g070`, synthetic project, fake provider): the button in Codi and in Vents, the home with and without «Aquest projecte», «Document nou» opening with the cursor in the first block, Enter/Ctrl+Enter saving to `scope=mine`, the document listed in «Els meus documents» afterwards. Not checked in the packaged desktop app; no desktop file changed.
- `node --test desktop/*.test.js`: 73 pass, 1 fails (`verify-release.test.js`: `Cannot find module 'electron-updater'`, the worktree has no `desktop/node_modules`); unrelated to this change.
- Follow-up: `doc_changed` with `scope:"mine"` (wind documents, 066) still opens the panel without the scope, so it 404s; `changed(d)` could pass `d.scope` to `open`.
- Follow-up: the doc window (`/app/doc/{id}`, desktop `gregal:open-doc`) should take `scope` so personal documents can open in their own window.
- Follow-up: the panel's «Descarrega en Word» link always asks for `format=docx`, also for a deck (400).
