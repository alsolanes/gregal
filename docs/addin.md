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

## Install the Add-in

**From the desktop app (Windows):** Preferences → **Word add-in** → **Install
in Word**. The app asks the running backend for the manifest, writes the real
backend address into it (the backend uses another port when 8097 is taken),
saves it under the app's data folder (`word-addin/gregal-addin.xml`) and
registers it under `HKCU\Software\Microsoft\Office\16.0\WEF\Developer`,
the key Microsoft's own tools use to load add-ins locally. No shared folder or
Trust Center step is needed. Then close Word completely, open it again and
choose **Gregal Agent** in **Home → Add-ins** (or **More add-ins → My add-ins →
Developer add-ins**). On every start the app rewrites the manifest with the
current port. **Remove from Word** deletes the registry value.

**Without the desktop app**, register the manifest by hand. Save
`http://127.0.0.1:<port>/app/addin/gregal-addin.xml` from your running
backend (change `8097` in it if you use another port) and run:

```
reg add "HKCU\Software\Microsoft\Office\16.0\WEF\Developer" /v 64C79419-6A50-4552-BF4F-075D1C5F849D /t REG_SZ /d "C:\path\to\gregal-addin.xml" /f
```

The older route also works: share a folder on the network, add it in Word
under **File → Options → Trust Center → Trust Center Settings → Trusted
Add-in Catalogs** with **Show in Menu**, and pick the add-in under **Shared
Folder**.

In the task pane, check the server URL, enter a token if the server requires
one, and select **Check**.

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
- The repository manifest uses port 8097. The desktop app's one-click install
  rewrites it with the real port; a manual install must be edited by hand.
