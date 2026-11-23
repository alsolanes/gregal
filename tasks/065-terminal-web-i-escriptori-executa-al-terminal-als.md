---
title: "Terminal (web i escriptori): «Executa al terminal» als blocs d'ordres del xat"
status: todo
depends_on: ["064"]
---

## Why

When Gregal needs the person to run something it cannot do itself, or shares terminal instructions, the person now has to copy the command and paste it somewhere. A button next to the command should run it in the side terminal, where any interaction it needs (a password, a browser login) just happens.

## Acceptance criteria

- [ ] Code blocks tagged `bash`, `sh`, `shell`, `zsh`, `powershell`, `pwsh`, `ps1`, `cmd` or `console` show a terminal-icon button next to «Copy», labelled «Executa al terminal» / «Run in terminal»; other languages do not. A node test of the rendered HTML for a `bash` and a `python` block, both in the live render (`md.js`) and in saved conversations (`code-blocks.js`).
- [ ] Clicking it opens the session's terminal panel (starting the shell if needed) and runs the block's text there directly, followed by Enter; a multi-line block runs line by line as typed. A node test that the click sends the block text + `\r` to the terminal.
- [ ] Commands Gregal sends are checked against the same policy as the agent's `bash` (`tools.ClassifyWith` with the session's allow/deny lists): a denied command is not sent and the person sees «Ordre bloquejada: …» / «Command blocked: …». A Go test that a denied command sent through the route is refused and never reaches the shell.
- [ ] In `$ cmd` / `PS> cmd` console blocks the prompt prefix is stripped and output lines (without a prompt) are not sent. A node test.
- [ ] Strings in Catalan and English in `i18n.js`; `docs/api-contract.md` describes how a command is sent to the terminal.

## Out of scope

- The terminal itself (064).
- Asking for an extra confirmation before running: the click is the person's confirmation.
- Detecting which commands need interaction.
- Android (see Notes).

## Notes

- Files: `internal/web/app/md.js` (line ~408, where `copybtn` is rendered), `internal/web/app/code-blocks.js`, the terminal panel from 064.
- The policy check belongs on the server (a `{type:"run",cmd}` frame or a small POST on the terminal route), not only in the client, so a modified client cannot skip it.
- Android gap: Android has no terminal (064), so the button is web/desktop only; Android keeps «Copy».
- No new dependencies.
