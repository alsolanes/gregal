---
title: Vents web: la conversa d'un vent agafa el seu color i en mostra l'estat
status: todo
depends_on: []
---

## Why

A wind's conversation looks the same for every wind: neutral bubbles on a neutral background, with the wind's colour only in its face. The wind should feel like a character with its own colour, and the person should see at a glance whether it is working, waiting for them or done.

## Acceptance criteria

- [ ] In a wind's chat (`winds-workspace.js` `chatView`), the page gets a soft wash of the wind's colour (a radial or top gradient at low opacity behind the messages) and the wind's bubbles a tint of its colour, readable in dark and light themes: text contrast of the wind bubbles is at least 4.5:1 in both themes for the eight squad preset colours and Gregal's `#3D7BFF`. A node test of the colour mix function over those colours.
- [ ] The header shows the wind large (the existing `face(w, true, mood)`) with one state label: «Treballant…» while the wind has a turn running (`state.pending` / `running(w)`), «T'espera» when it has an unanswered ask (`state.asks` for that wind), «Fet» after a turn ends until the person sends a message or leaves the chat, and nothing otherwise. A node test of the pure state function for the four cases.
- [ ] Gregal's own thread keeps its current glow (`.wind-workspace.vw-gregal::before`) but takes Gregal's colour, so it follows the same rule.
- [ ] Strings in Catalan and English (`i18n.js`).

## Out of scope

- Android (086).
- The avatar outside the frame (087).
- Group chats (rotllana) keep their current look.

## Notes

- Files: `internal/web/app/winds-workspace.js` (`render` 148, `owner` 143, `chatView` 959, `paintHistory` 1125, `paintPending` 363), `winds-workspace.css` (messages 716-741, Gregal glow 764-778).
- Wind colour: `Wind.Color` (`internal/companion/winds.go`), web `color()` in `winds.js:31`; squad presets in `internal/web/winds_squads.go`.
- Today the wind bubbles use `--surface-hi` and the typing dots `--vw-accent` (the app accent).
- Web: node tests with `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server in the browser pane, dark and light theme, with before/after screenshots in `output/ui-review/` (ignored).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
