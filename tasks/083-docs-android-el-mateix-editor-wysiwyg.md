---
title: "Docs (Android): el mateix editor WYSIWYG"
status: todo
depends_on: ["079", "080", "081", "082"]
---

## Why

Every feature ships on Android too. Rebuilding a rich text editor in Compose would duplicate the web editor and drift from it; the phone should show the same editor, with the same toolbar and the same saving rules.

## Acceptance criteria

- [ ] On Android a text document opens in a WebView that loads the web document page (`/app/doc/{id}`, with the right `scope`) authenticated with the app's key, so it has the web editor, toolbar and page look; presentations and spreadsheets keep their native screens. A unit test of the URL and of how the key reaches the page without putting it in the URL.
- [ ] The toolbar fits a phone width (wraps or scrolls), and the keyboard does not hide the line being typed. Checked by hand on a device or emulator, with a screenshot in the notes; if no device is available, say so and leave this unticked.
- [ ] «Descarrega en Word» and the back button work as on the other document screens. A unit test.
- [ ] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- A native Compose rich text editor.

## Notes

- Proposed approach (the user can change it): WebView over the same page, for parity at low cost. Existing WebView use: `PreviewScreen.kt`. Never put the key in a query string; pass it as a header or through a JS bridge, as the desktop does for its document windows.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
