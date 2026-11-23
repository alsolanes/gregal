---
title: "Terminal (web i escriptori): un terminal de veritat al lateral del xat"
status: todo
depends_on: []
---

## Why

Some steps can only be done by the person: logins that open the browser (`gh auth login`), `sudo` passwords, installers that ask questions. The current Terminal view (`/api/exec`) runs commands as background processes with no stdin, so these hang. The person needs a normal interactive shell next to the chat, like the other coding tools have.

## Acceptance criteria

- [ ] A terminal icon in the chat header opens a panel at the side of the chat with an interactive shell (PowerShell on Windows, `$SHELL` or `/bin/sh` elsewhere) started in the session's `cwd`. A Go test opens the terminal over the new route, writes `echo hola` + Enter, and reads `hola` back.
- [ ] Typing, colours, Ctrl+C and interactive prompts work, because the shell runs in a real pseudo-terminal; resizing the panel resizes the PTY (`{type:"resize",cols,rows}`). A Go test sends a resize and checks the shell sees the new size (`stty size` / `$Host.UI.RawUI.WindowSize`).
- [ ] One terminal per session: closing the panel only hides it and reopening shows the same shell with its scrollback; the shell is killed when the session ends. A Go test that ending the session stops the process.
- [ ] When users are configured, only admin users can open it (403 for others, as the shell is not confined to `roots`); with no users and no token it is open for local use, like the other local routes. A Go test of the 403.
- [ ] The route is documented in `docs/api-contract.md` (`TestContracteAPIDocumentat` passes), and the panel's strings exist in Catalan and English in `i18n.js`.

## Out of scope

- Running commands from chat code blocks (task 065).
- Several terminals or tabs per session, split panes, themes, shell selection.
- The existing Terminal view and `/api/exec` stay as they are.
- Android (see Notes).

## Notes

- Transport: WebSocket with `golang.org/x/net/websocket` (already in `go.mod`): binary frames for the PTY bytes, JSON text frames for `resize`.
- New dependencies allowed: a Go PTY library with ConPTY support on Windows, `github.com/charmbracelet/x/xpty` (same family as the existing charmbracelet modules), or `github.com/aymanbagabas/go-pty` if xpty does not fit; and `@xterm/xterm` + `@xterm/addon-fit`, vendored minified in `internal/web/app/vendor/` with their licence added to `THIRD_PARTY_NOTICES.md` (like chart.js). If the vendored files trip the diff guard size limit, stop and ask instead of overriding it.
- What the person types is not run through the bash denylist: it is their own shell, not an agent tool. The denylist still applies to commands Gregal sends (065).
- Desktop gets it through the web app; nothing in `desktop/` should need changing.
- Side panel: 058 is building a side panel for documents. If it has landed, put the terminal in the same panel slot; otherwise add a minimal one and note the merge as a follow-up.
- Android gap: no terminal on Android in this task; a phone terminal over the same route would be its own task if wanted.
- Code to read first: `internal/web/exec.go`, `internal/agent` procs store, `internal/web/app/panels.js` (`term`), `internal/web/app/shell-layout.js`.
