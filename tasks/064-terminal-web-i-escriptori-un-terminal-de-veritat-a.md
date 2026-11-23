---
title: "Terminal (web i escriptori): un terminal de veritat al lateral del xat"
status: done
depends_on: []
---

## Why

Some steps can only be done by the person: logins that open the browser (`gh auth login`), `sudo` passwords, installers that ask questions. The current Terminal view (`/api/exec`) runs commands as background processes with no stdin, so these hang. The person needs a normal interactive shell next to the chat, like the other coding tools have.

## Acceptance criteria

- [x] A terminal icon in the chat header opens a panel at the side of the chat with an interactive shell (PowerShell on Windows, `$SHELL` or `/bin/sh` elsewhere) started in the session's `cwd`. A Go test opens the terminal over the new route, writes `echo hola` + Enter, and reads `hola` back.
- [x] Typing, colours, Ctrl+C and interactive prompts work, because the shell runs in a real pseudo-terminal; resizing the panel resizes the PTY. A Go test sends a resize and checks the shell sees the new size (`stty size` / `$Host.UI.RawUI.WindowSize`).
- [x] One terminal per session: closing the panel only hides it and reopening shows the same shell with its scrollback; the shell is killed when the session ends. A Go test that ending the session stops the process.
- [x] When users are configured, only admin users can open it (403 for others, as the shell is not confined to `roots`); with no users and no token it is open for local use, like the other local routes. A Go test of the 403.
- [x] The route is documented in `docs/api-contract.md` (`TestContracteAPIDocumentat` passes), and the panel's strings exist in Catalan and English in `i18n.js`.

## Out of scope

- Running commands from chat code blocks (task 065).
- Several terminals or tabs per session, split panes, themes, shell selection.
- The existing Terminal view and `/api/exec` stay as they are.
- Android (see Notes).

## Notes

- As built: no new dependencies. The repo already had its own PTY (`internal/pty`, ConPTY on Windows) and VT emulator (`internal/vterm`) from the Kanban mode, so the shell reuses them and the panel reuses the Kanban key encoding (`kanban.js`) instead of xterm.js. Transport is SSE read with `fetch` plus JSON POSTs, like `/api/kanban/*`: the API only accepts the `Authorization` header, which a browser WebSocket cannot send. POSTs must be `application/json`, so another site cannot drive the shell with a simple POST in local mode.
- `internal/vterm` gained scrollback (`History`, last 2000 lines that scroll off the main screen) and `internal/pty` on Windows clears the inherited "ignore Ctrl+C" flag before starting the child; without it Ctrl+C did nothing when the server was started in a new process group (also fixes Ctrl+C in Kanban cards).
- The shell lives in the existing side tool dock's Terminal tab, above the `/api/exec` process list; the header gets a `>_` icon for it.
- What the person types is not run through the bash denylist: it is their own shell, not an agent tool. The denylist still applies to commands Gregal sends (065).
- Desktop gets it through the web app; nothing in `desktop/` changed.
- Split in two commits (backend, then web panel) to stay under the diff guard's 400 lines per change.
- Follow-up: 058 landed its documents panel as a separate `#docsPanel` beside the tool dock; moving it into the tool dock as one more tab would leave a single side panel.
- Follow-up: `clear` with ED 3 empties the server history, but an open panel keeps showing the old lines until it reconnects.
- Android gap: no terminal on Android; a phone terminal over the same routes would be its own task if wanted.
