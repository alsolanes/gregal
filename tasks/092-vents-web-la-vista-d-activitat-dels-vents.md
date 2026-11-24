---
title: Vents web: la vista d'activitat dels vents
status: done
depends_on: ["091"]
---

## Why

The person should see what each wind did while they were away, in one place, as on agent products that show an activity view.

## Acceptance criteria

- [x] Vents has an «Activitat» view (a tab in the wind's profile and an entry for all winds) that reads `/api/winds/activity` and shows a timeline grouped by day: the wind's face, the time, the title, a status chip (done, failed, waiting) and, when `ref` points to a chat, a document or a run, a link that opens it. A node test of the grouping and the item rendering.
- [x] Scrolling to the end loads older items with `before`. A node test.
- [x] An empty list shows a short explanation, and a failed load a message with retry.
- [x] Strings in Catalan and English.

## Out of scope

- Android (093).

## Notes

- Files: `vents-activity.js` (automations and saved, used in the wind profile tabs, `winds-workspace.js:673`), `winds-workspace.js` (`todayView` 465-523).
- Web: node tests with `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server in the browser pane, dark and light theme, with before/after screenshots in `output/ui-review/` (ignored).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
- Done: `renderActivity` in `vents-activity.js` (exported with `groupByDay`, `statusChip`, `activityLink`); «Activitat» is a tab of Feina (`WORK_TABS`, view `activitat`) and of the wind profile. Tests: `internal/web/app/vents-activity.test.cjs`.
- Status chips: done (done, ok, read), failed (failed, error, cancelled, interrupted), waiting (running «En marxa», unread «Per llegir»). Links: a task opens the task view; news, heartbeat and reminder open the wind's chat (where they spoke); a routine run shows its output inline from `/api/winds/rutines/runs?id=` (only the last 20 runs are kept, older ones say so). No kind points to a document today.
- Strings are inline `tx(ca, en)` like the rest of `vents-activity.js` and `t(ca, en)` in `winds-workspace.js`, not keys in `i18n.js`.
- Older pages: an IntersectionObserver on the foot plus a «Més antigues» button for keyboard users.
- Follow-up: «Obre el xat» opens the wind's current chat, not the exact message (`message_id` from `/api/winds/novetats?id=`), and does not mark the news read.
