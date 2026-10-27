# Chrome extension (Google Docs, Sheets and Apps Script)

A side panel in Chrome (or Edge) to talk to your winds about the page you
have open. It reads the document itself and puts the answer back: in a
Google Doc at the cursor, in a Sheets cell, in the Apps Script editor, or in
the field you are typing in on any other site. It uses the same Gregal as the
desktop app or your server, with the same API as the phone; nothing changes on
the server.

## Install

Chrome does not let any program install an extension by itself (only the Web
Store or company policies can), so it takes three clicks:

**From the desktop app:** Preferences → **Chrome extension** → **Prepare the
extension**. The app copies the extension to a fixed folder (refreshed on every
start, so updates reach it), opens `chrome://extensions` (or Edge's) and shows
the folder. Then turn on **Developer mode**, **Load unpacked**, choose that
folder and pin the Gregal icon.

**From the repository:** the same, choosing `extension/chrome/`.

## Connect

At the top of the settings (⚙) there is a **🖥️ Desktop app · 🌐 Server**
switch, like on the phone:

- **Desktop app** (the default): nothing to fill in. It connects to the Gregal
  app open on this computer, which keeps its backend on `127.0.0.1:8097` when
  that port is free.
- **Server:** the address, port and key it prints when it starts
  (`gregal --serve --addr 0.0.0.0`, or `https://your-domain` and no port).
  Chrome asks once for permission to reach that address.

Then pick the wind at the top: each one keeps its own memory and knowledge, so
use the one whose trade fits (a data wind for Sheets, a writer for Docs).

## What it reads

| Page | What the wind sees |
|---|---|
| Google Docs | The whole document (its text export) and your selection when the page exposes it. |
| Google Sheets | The current tab as CSV and the selected range. |
| Apps Script | The files open in the editor (or the visible lines if the editor is not reachable). |
| A PDF (Chrome's viewer) | The PDF is uploaded to Gregal and its text is the context; the message carries its path so the wind can read the whole file, scanned pages included, with `pdf_read` (see [PDF](pdf.md)). |
| Any other site | Your selection, or the main text of the page. |

Long documents are cut at 14,000 characters. Untick **Include the page** to
ask without it. Reading uses your own Google session in that browser: the
extension only asks Google for the document you have open.

## Putting the answer on the page

Every answer, and every code or formula block in it, has **Copy** and
**Insert**:

- **Docs:** inserted at the cursor, as if pasted.
- **Sheets:** pasted into the selected cell (tab-separated values fill a
  range).
- **Apps Script:** inserted at the cursor; **Replace the file** swaps the whole
  file for the block.
- **Other sites:** typed into the focused field.

If the page does not accept it, the text stays on the clipboard and the panel
says so: press Ctrl+V where you want it.

## Development

`logic.js` (address, stream, code blocks, message) is tested with
`node --test extension/chrome/logic.test.js`. `page.js` runs inside the tab
through `chrome.scripting`.
