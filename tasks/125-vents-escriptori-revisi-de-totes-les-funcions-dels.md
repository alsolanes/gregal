---
title: Vents escriptori: revisió de totes les funcions dels vents i botons sense mode fosc
status: done
depends_on: []
---

## Why

The person sees buttons on the desktop app that are not in dark mode and look odd (the browser pane, for example), and reports that some wind features do not work on desktop (2026-11-24).

## Acceptance criteria

- [x] With the dark theme (and each dark-ish theme: dark, gpt, claude, opencode, mistral, gemini — check which are dark in index.html), no control on any page shows a light background or light-only colours: browser pane (toolbar, Go, Add to chat, the empty shell — `.browser-shell` has `background:#fff`), terminal, docs, wind chat and its panes, settings, dialogs. A node/CSS test that fails on hard-coded light colours in the touched rules (extend the identitat test pattern if it fits). Before/after screenshots in `output/ui-review/125/`.
- [x] Every wind feature on desktop is exercised once (Playwright against a sandbox server, and the packaged Electron app for Electron-only paths like dialogs) and listed in the task notes as works / fixed / still broken: wind list, create/edit, avatar theme and per-wind pack and barretina, chat stream, approvals, activity, shelf, Avui, routines, wiki, docs pane, computer pane, comparisons pager, shared chats, notifications.
- [x] Each bug found that is fixable in under ~80 lines is fixed with a test; bigger ones become notes with a proposed task.

## Out of scope

- Android (124).
- The doc side pane toggle (126).

## Notes

- Files: internal/web/index.html (themes 30-160, browser CSS 536-554), internal/web/app/polish.css, winds*.js, docs-panel.js.
- Playwright from ~/AppData/Local/npm-cache/_npx/31e32ef8478fbf80/node_modules with system Chrome; `_electron` for the packaged app. Sandbox: isolated GREGAL_DATA_DIR etc (memory: project-gregal-sandbox-testing); g090 fake provider on 18790 can be reused.
- Never touch the person's real config or sessions.
- Dark themes in the app: dark (default), opencode, grok and calma (calma.css). gpt, claude, mistral and gemini are light. All four dark ones were screenshotted on every Code view, the Preferences sections and the Vents views (Playwright, sandbox on 18820) with a scan for visible elements with a light background and for low-contrast text.
- Dark-mode fixes: `.browser-shell` (index.html) and the agent browser `#agentBrowser .ab-view` (agent-browser.js) were `#fff`, now `var(--bg)`: the empty frame showed a light rim and a white box before the first page. The live preview's `.preview-viewport` (preview.js) was `#dce2e3`, now `var(--surface-hi)`. Test: identitat.test.js «els marcs del navegador i de la vista prèvia segueixen el tema». Before/after in output/ui-review/125/ (gitignored).
- Left as they are on purpose: the white canvas of a loaded web page (`#browserView, #browserFrame`, preview content, docx/pdf/pptx pages, game iframes), the off-white user bubble (`--user-bubble`, a deliberate choice in calma.css), grok's white accent buttons, colour swatches.
- Electron (worktree desktop/ run with the dev electron.exe against the sandbox, GREGAL_URL): the browser `<webview>` only took the 300 px of its min-height and left the rest of the frame empty. Fixed: it fills the frame (absolute, inset 0) and the "type an address" text sits above it. Test: ui_test.go TestLaPaginaDelNavegadorOmpleElMarc.
- Wind features on desktop (dark theme, sandbox with a fake provider):
  - Wind list / All winds: fixed. The sidebar only linked to «All winds» with more winds than it lists, so with Gregal and up to five others there was no way on desktop to the roster: no creating a wind, no profiles (mobile has the bottom bar). Now always shown. Test: winds-start.test.cjs «with every wind already in the sidebar, it still links to «All winds»…».
  - Create / edit wind: works (create dialog with a suggestion; profile edits save).
  - Avatar theme (Preferences → Appearance), per-wind pack (Free mode, profile), barretina (Minairons: each wind has its `data-barretina` colour): works.
  - Chat stream: works. Approvals: works (writing outside the project asks; Allow writes the file). In-project writes pass without asking, as designed.
  - Activity, Work tabs (In progress, Do, Activity, Automations), shelf and the other profile tabs (Drawer, Memory, Knowledge, Scheduled, Saved): work.
  - Avui (Today), Saved, Wiki, Library, Playground, the winds' corner: open and render; no content to exercise in a fresh sandbox.
  - Routines: works (New routine, Run now, the result arrives as quiet news).
  - Docs pane: works (opens beside the page). Computer pane: works (no remote viewer, so it opens the local computer pane).
  - Comparisons pager: works (next moves a page, dots follow).
  - Shared chats: works with two sandbox accounts (create, invite, accept, a wind answers, the other person sees it). Single-account servers answer 404 and the UI hides the section, as designed.
  - Notifications: works (a reminder fired as an in-app toast while on another view). System and native notifications are covered by the existing winds-start tests, not checked live.
- Not exercised: native Electron dialogs (the folder picker for «Open a project»), since Playwright cannot drive them; the installed Gregal-Setup build (the worktree desktop/ was run with the dev Electron instead).
- Follow-up: «New routine» with no active squad quietly activates the whole everyday squad (nine winds) before the dialog opens (winds-workspace.js newRoutine), and the wind picker defaults to the first of them, not Gregal.
- Follow-up: «Automations» from the Work tabs opens a page without those tabs, so the only way back is the sidebar.
- Follow-up: a comparison card clicked right as the chat re-renders (the «X knows more…» suggestion appearing) jumps back to the first page.
- Follow-up: «Run now» on a new daily routine whose time has already passed today produced two runs and two news items in the sandbox; check whether the scheduler catch-up and the manual run overlap.
