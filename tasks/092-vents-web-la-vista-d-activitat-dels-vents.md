---
title: Vents web: la vista d'activitat dels vents
status: todo
depends_on: ["091"]
---

## Why

The person should see what each wind did while they were away, in one place, as on agent products that show an activity view.

## Acceptance criteria

- [ ] Vents has an «Activitat» view (a tab in the wind's profile and an entry for all winds) that reads `/api/winds/activity` and shows a timeline grouped by day: the wind's face, the time, the title, a status chip (done, failed, waiting) and, when `ref` points to a chat, a document or a run, a link that opens it. A node test of the grouping and the item rendering.
- [ ] Scrolling to the end loads older items with `before`. A node test.
- [ ] An empty list shows a short explanation, and a failed load a message with retry.
- [ ] Strings in Catalan and English.

## Out of scope

- Android (093).

## Notes

- Files: `vents-activity.js` (automations and saved, used in the wind profile tabs, `winds-workspace.js:673`), `winds-workspace.js` (`todayView` 465-523).
- Web: node tests with `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server in the browser pane, dark and light theme, with before/after screenshots in `output/ui-review/` (ignored).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
