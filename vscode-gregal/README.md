# Gregal for Visual Studio Code

A coding companion in your sidebar, connected to your own Gregal server.
Use Code to work on a change, Inspect to explore without editing, or Chat to
talk through an idea. Follow tools and delegated agents as they work.

On Windows, a single PowerShell command installs both the VS Code extension
and backend, then starts the backend:

```powershell
irm https://raw.githubusercontent.com/alsolanes/gregal/main/scripts/install-vscode.ps1 | iex
```

## Install for a user

1. Download the latest published `gregal-X.Y.Z.vsix` and the Gregal backend
   for the user's operating system from [GitHub Releases](https://github.com/alsolanes/gregal/releases).
2. In VS Code, open **Extensions** → **…** → **Install from VSIX…**, select
   the downloaded VSIX, then reload VS Code. Alternatively, run
   `code --install-extension gregal-X.Y.Z.vsix` in a terminal.
3. On Windows, download and start the backend in one PowerShell command:

   ```powershell
   irm https://github.com/alsolanes/gregal/releases/latest/download/gregal-windows-amd64.exe -OutFile "$env:LOCALAPPDATA\gregal.exe"; & "$env:LOCALAPPDATA\gregal.exe" --serve
   ```

   The default server address is `http://127.0.0.1:8097`. Configure a model
   provider before the first request.
4. Open a folder in VS Code and select the Gregal icon in the Activity Bar.

The extension and backend are separate downloads. To connect to a Gregal
server on another machine, set `gregal.baseUrl` to that server's reachable URL
and configure its token with **Gregal: Set server token securely** when needed.
The VSIX can also be installed on remote VS Code extension hosts; the backend
must then be reachable from that host.

## Start

1. Start your backend with `gregal --serve`.
2. Open the Gregal icon in the Activity Bar, or run **Gregal: Open chat**
   (`Ctrl+Alt+G`, or `Cmd+Alt+G` on macOS).
3. Use **Add selection or file** to attach code explicitly. The chip shows the
   file and selection range; remove it or uncheck context to leave it out.
4. Write a request. Enter sends; Shift+Enter inserts a new line.

The sidebar follows your VS Code colors, fonts, contrast and reduced-motion
preferences. Code blocks can be copied, tool output can be expanded, and
**Review changes** opens VS Code Source Control. Tool approvals and questions
use native VS Code pickers. **Stop** cancels the run on the backend.
Use the history button beside the title to restore a saved conversation. Its
project must be open in this VS Code window so the restored chat stays scoped
to the right workspace.
Inline completions are enabled by default and use the configured code model.
They send a bounded prefix and suffix around the cursor to the configured
Gregal server; disable `gregal.inlineCompletions.enabled` to turn them off.

## Connection

`gregal.baseUrl` defaults to `http://127.0.0.1:8097`. For authenticated servers,
use **Gregal: Set server token securely**. Tokens are stored in VS Code
SecretStorage and associated with that server URL. The old `gregal.token`
setting remains readable for compatibility, but is deprecated.

The default `gregal.session` value (`vscode`) creates a separate session for
each workspace. Setting a custom session ID explicitly shares that backend
session. A run captures its URL, token, session and workspace at submission;
changing settings does not redirect a pending approval or cancellation.

The backend must be able to access the selected workspace path. This extension
runs in the extension host, including remote SSH/container hosts. Virtual
workspaces and untrusted workspaces are not supported.

## Editor actions

- **Gregal: Ask about the selection or file** attaches the current code and
  opens the composer for review.
- **Gregal: Explain the selection** sends that context in read-only Inspect mode.
- **Gregal: Undo the agent's changes** lists backend checkpoints and confirms
  the restoration. Undo cannot overlap a run in this extension.
- **Gregal: Browse GitHub issues and pull requests** provides read-only queries.
- **Gregal: New conversation** starts a fresh backend session without deleting
  files or previous backend conversations.
- **Gregal: Accept selected diff lines (stage)** adds the selected lines from a
  Git diff to the index.
- **Gregal: Reject selected diff lines** discards the selected diff lines after
  confirmation. Open a Git diff and select the changed lines first.
- Diagnostic quick fixes offer **Ask Gregal to fix this diagnostic** and
  **Ask Gregal to explain this diagnostic**. The fix uses Code mode; the
  explanation uses read-only Inspect mode. Both attach a short code range and
  the diagnostic text.

## Current limits

Git line actions use VS Code's built-in selected-range staging and revert
commands. Inline completion is opportunistic: cancelled requests stop at the
backend, and provider errors do not interrupt typing. The local sidebar retains
a bounded recent transcript; the backend owns the complete conversation. Code
context is capped at 24,000 characters plus ten diagnostics.

Modern backends use durable events and exact run cancellation. Older servers
use SSE after the selected workspace is set on that session. If a submission's
acknowledgement is delayed, stopping waits for acknowledgement or idempotency
recovery so that the correct backend run can be cancelled.

## Development

```sh
npm ci
npm run typecheck
npm test
npm run compile
npm run package
```

The extension ships without runtime npm dependencies. Its Webview loads only
packaged resources, applies a strict Content Security Policy, and renders
model text through DOM nodes rather than injecting HTML.
