# Office Add-in (Word, Excel and PowerPoint)

The task pane connects to a Gregal backend on the same machine. It can send
selected text as context and insert generated text at the selection or cursor.
The add-in uses Office APIs to edit documents instead of modifying Office
files externally.

## Requirements

- An Office deployment that supports task-pane add-ins and the Office.js APIs
  used by the manifest.
- Gregal running locally. The add-in manifest defaults to
  `http://127.0.0.1:8097`.
- Network access to load `office.js` from its Microsoft-hosted CDN.

Supported add-in capabilities and platform availability depend on the Office
deployment. Check its add-in requirements before installing.

## Sideload the Add-in

1. In Word, open **File → Options → Trust Center → Trust Center Settings →
   Trusted Add-in Catalogs**.
2. Add the repository's `internal/web/app/addin/` folder as a catalog and
   enable **Show in Menu**.
3. In the ribbon, open **Home → Add-ins → My Add-ins → Shared Folder** and
   choose **Gregal Agent**.
4. In the task pane, check the server URL, enter a token if the server requires
   one, and select **Check**.

If the backend uses another port, update `SourceLocation` in
`gregal-addin.xml` before loading the manifest. The task pane can also remember
a different backend URL.

## Use the Task Pane

Select text and choose a quick action such as **Rewrite**, **Correct**,
**Summarize**, **Shorten**, **Expand**, **More formal**, **To English**,
**To Catalan**, or **As bullets**. You can also enter a request and press
**Generate** (`Ctrl+Enter`). The selection can be included as context, and the
pane can be set to return only the result text.

The result is formatted in the pane. **Replace selection** inserts it over the
selected text; **Insert** places it at the cursor. Word uses `insertHtml` for
formatted text, including headings, lists, emphasis and tables. In Excel, a
pipe-delimited table can be inserted as a cell range starting at the active
cell. PowerPoint insertion is text-based. Respond to permission requests or
questions in the pane; **Stop** cancels the current turn.

## Read-Only Mode

The **Read only: do not edit the document** switch hides the insert and replace
actions and sends the turn in `chat` mode. The task pane still displays the
response. Server-side tool permissions remain the authority for other actions.
This mode is intended for shared documents where edits should be reviewed
before applying them.

## Boundaries

- PowerPoint support is text-based; it does not provide fine-grained slide
  editing.
- The pane connects to the loopback backend, but model requests are sent to the
  provider configured for the selected role. Review the
  [data-handling notes](getting-started.md) for details.
- `office.js` is loaded from Microsoft's CDN; the pane requires network access
  to load it.
- The manifest's default `SourceLocation` uses port 8097. If the desktop client
  selects another port, update the task pane's server URL.
