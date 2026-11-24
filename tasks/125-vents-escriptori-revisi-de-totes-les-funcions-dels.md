---
title: Vents escriptori: revisió de totes les funcions dels vents i botons sense mode fosc
status: todo
depends_on: []
---

## Why

The person sees buttons on the desktop app that are not in dark mode and look odd (the browser pane, for example), and reports that some wind features do not work on desktop (2026-11-24).

## Acceptance criteria

- [ ] With the dark theme (and each dark-ish theme: dark, gpt, claude, opencode, mistral, gemini — check which are dark in index.html), no control on any page shows a light background or light-only colours: browser pane (toolbar, Go, Add to chat, the empty shell — `.browser-shell` has `background:#fff`), terminal, docs, wind chat and its panes, settings, dialogs. A node/CSS test that fails on hard-coded light colours in the touched rules (extend the identitat test pattern if it fits). Before/after screenshots in `output/ui-review/125/`.
- [ ] Every wind feature on desktop is exercised once (Playwright against a sandbox server, and the packaged Electron app for Electron-only paths like dialogs) and listed in the task notes as works / fixed / still broken: wind list, create/edit, avatar theme and per-wind pack and barretina, chat stream, approvals, activity, shelf, Avui, routines, wiki, docs pane, computer pane, comparisons pager, shared chats, notifications.
- [ ] Each bug found that is fixable in under ~80 lines is fixed with a test; bigger ones become notes with a proposed task.

## Out of scope

- Android (124).
- The doc side pane toggle (126).

## Notes

- Files: internal/web/index.html (themes 30-160, browser CSS 536-554), internal/web/app/polish.css, winds*.js, docs-panel.js.
- Playwright from ~/AppData/Local/npm-cache/_npx/31e32ef8478fbf80/node_modules with system Chrome; `_electron` for the packaged app. Sandbox: isolated GREGAL_DATA_DIR etc (memory: project-gregal-sandbox-testing); g090 fake provider on 18790 can be reused.
- Never touch the person's real config or sessions.
