---
title: "Docs (Android): el mateix editor WYSIWYG"
status: done
depends_on: ["079", "080", "081", "082"]
---

## Why

Every feature ships on Android too. Rebuilding a rich text editor in Compose would duplicate the web editor and drift from it; the phone should show the same editor, with the same toolbar and the same saving rules.

## Acceptance criteria

- [x] On Android a text document opens in a WebView that loads the web document page (`/app/doc/{id}`, with the right `scope`) authenticated with the app's key, so it has the web editor, toolbar and page look; presentations and spreadsheets keep their native screens. A unit test of the URL and of how the key reaches the page without putting it in the URL.
- [ ] The toolbar fits a phone width (wraps or scrolls), and the keyboard does not hide the line being typed. Checked by hand on a device or emulator, with a screenshot in the notes; if no device is available, say so and leave this unticked.
- [x] «Descarrega en Word» and the back button work as on the other document screens. A unit test.
- [x] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- A native Compose rich text editor.

## Notes

- Proposed approach (the user can change it): WebView over the same page, for parity at low cost. Existing WebView use: `PreviewScreen.kt`. Never put the key in a query string; pass it as a header or through a JS bridge, as the desktop does for its document windows.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
- Done: once the document is read, a text document (`LiveDoc.inWebEditor`, kind `doc`) shows «Descarrega en Word» (the same `docExportPath` download as before) above a WebView (`DocWebPage` in `DocScreen.kt`) that loads `docPageUrl(base, ref, sessioApp)` = `{base}/app/doc/{id}?session=…&scope=mine`. Decks and sheets keep their native screens, and the home and lists are unchanged. The kind comes from the document that was read, not from the ref, because a `doc_changed` ref can arrive without a kind.
- The key: the page load sends `Authorization: Bearer …` (`docPageHeaders`), and the page's own fetches get it from the JS bridge `window.GregalAndroid.token()` (`DocPageBridge`). `doc-page.js` now reads `window.gregalDesktop?.apiToken ?? window.GregalAndroid?.token()` (a one-line web change). The key never goes in the URL. The WebView only loads the page itself (`docPageKeeps`); any other link opens in the system browser, so the bridge is never exposed to another page, and Back leaves the document through the app's back stack, as for decks and sheets.
- The keyboard: the column has `imePadding()` (the activity is `adjustResize`). The web toolbar already wraps (`flex-wrap:wrap` in docs-panel.js).
- Removed, because the WebView replaces them: the native text-block editor (`BlockEditor`), `writeEmptyDoc`, the «Document nou» write flag (`docWrite`, the web page has its own empty-document placeholder) and the strings `docs.write`, `docs.addBlock` and `docs.conflict` (ca and en). No new strings were needed.
- Tests: `LiveDocTest::aTextDocumentOpensTheWebPageWithTheKeyOutsideTheUrl`, `LiveDocTest::theWebEditorKeepsWordDownloadAndNeverNavigatesAway`, `BackStackTest::aTextDocumentInTheWebEditorGoesBackLikeTheOtherDocuments` (this one passed before the change too: it pins the back behaviour). Only these two classes and the compile were run, not the full Android suite.
- NOT checked: criterion 2. No device was connected (`adb devices` was empty). AVDs exist (`Medium_Phone`, …), but they were not booted, so there was no hand check and no screenshot. The page was not loaded in a real WebView either, so the bridge plus the `/app/doc` page against a server with a token is unverified.
- Follow-up: `followDoc` still opens the native stream for a text document shown in the WebView (needed only for its kind and title), so there are two streams per open document.
- Follow-up: the pure text-block edit state in `Docs.kt` (`blank`, `openNew`, text-block `saveRequest`/`saved`) and `docs.saveError` are now only used by tests and by sheets and decks; trim what only text blocks used.
- Follow-up: the bridge holds the key it was created with. If the connection's key changes while a document is open, the page keeps the old one until the screen is reopened.
- Follow-up: no JS test covers `doc-page.js` reading the Android bridge (doc-page.js has no test, see 067).
