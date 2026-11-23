---
title: "Gregal Docs (Android): els teus documents des del calaix"
status: done
depends_on: ["070"]
---

## Why

The same Docs home as on the web, on the phone: open your documents without going through a chat, and start a new one.

## Acceptance criteria

- [x] The drawer has a «Gregal Docs» entry that opens a Docs home with «Els meus documents» and, when the current chat has a project, «Aquest projecte»; tapping a document opens the existing document screen with the right scope. A unit test of the list parsing and of the scope each item opens with.
- [x] «Document nou» calls `POST /api/docs` and opens the new document with an empty block ready to type in; any document with no blocks also shows that empty block. A unit test of the request and of the empty-document state.
- [x] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Presentations and spreadsheets (074, 077). Renaming or deleting documents.

## Notes

- `Docs.kt`, `DocScreen.kt`, `Screens.kt`, `MainActivity.kt` (drawer), `LiveDocTest.kt`.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
- Done: the drawer entry is in `Drawer.kt` (not `MainActivity.kt`): under «Eines» in the Codi drawer, and above «Els teus vents» in the Vents drawer, like the web button that sits above the winds menu. It opens `Screen.Docs` with no document (`vm.openDocsHome()`); `DocScreen` is then the home («Document nou», «Els meus documents», «Aquest projecte · nom»), and with a document open it shows the document and the same lists below. The old «Documents del projecte» list and its strings (`docs.title`, `docs.list`, `docs.empty`) are gone.
- Scope: `DocRef` now carries `kind` and `scope`; `DocsJson.list(raw, scope)` stamps each item with the scope it was listed with, and `doc_changed` keeps its own `scope` (so a wind's card, 066, also opens with `scope=mine`). `docPath(ref, sub)` builds the read and stream paths, `docExportPath` adds the scope, and `LiveDoc.scope` (`@Transient`, set from the ref after reading) goes on every block POST/PUT/DELETE.
- «Aquest projecte» shows when `docsProjectName(cwd, project, simpleVents)` is not null: the Codi area with a session `cwd`, as on the web (not in Vents). The name is `project`, or the `cwd`.
- «Document nou» sends `docCreateRequest(title)` (`POST /api/docs?scope=mine` `{title, kind:"doc"}`; an empty title becomes «Document sense títol»), opens the answer with scope `mine` and, once read, opens a new-block editor at the top with the cursor in it. `LiveDoc.blank` (a text document with no blocks and no editor) shows «Escriu aquí…»; tapping it opens the same editor. It comes back when the editor closes empty or the last block is deleted.
- Decks (072): listed with their kind («Presentació»); opening one shows a plain notice (`docs.deckLater`) instead of the blocks and the Word download. 074 replaces that.
- Tested with `LiveDocTest` (5 new tests) and the whole Android unit suite (986 tests, 0 failures). Not tried on a device or emulator with a running server.
- Follow-up: if «Els meus documents» fails to load, «Aquest projecte» is not fetched either, and the notice is `docs.error` («No s’ha pogut obrir el document»), which reads oddly for a list.
- Follow-up: an unknown kind (a sheet, 077) shows its raw kind in the list and the deck notice when opened.
- Follow-up: the view-model wiring (passing `"mine"` to the list, the write flag after «Document nou») has no test; only the pure functions it uses do.
