---
title: Vents web: els avatars tenen vida i celebren la feina feta
status: done
depends_on: []
---

## Why

The user wants Vents to be fun on web and desktop: the avatars should feel alive and the UI should move with intent.

## Acceptance criteria

- [x] Idle faces blink and glance now and then (randomised), and react on hover/click (squash and spring back). A node test that idle faces get the blink animation and that `prefers-reduced-motion` removes it.
- [x] When a turn ends well the corner wind and the header face celebrate once (sparkle burst in the wind's colour); a failed turn gets a small shrug. A node test that done adds the celebration class once and error the shrug.
- [x] New messages spring in, the pending bubble breathes, side panes slide in; all disabled under reduced motion.
- [x] Animations use CSS transforms/opacity only (no layout thrash).

## Out of scope

- Android (100).

## Notes

- Files: `winds.js` (faces, moods), `winds.css` (keyframes 275-336, reduced motion 383), `winds-workspace.js` / `.css` (chat, corner face).
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Web: `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server (never type into credential fields) with before/after screenshots in `output/ui-review/` (ignored).
- Done (2026-11-24): blink/glance/squash live in a `prefers-reduced-motion:no-preference` block in `winds.css`; winds.js only sets a per-wind `--wf-look` (glance side and reach from the seed, stable across redraws). Loops stay at 12 s so `--wf-phase` keeps continuity.
- Celebration: `state.cheer` is set when a chat turn returns (done, or shrug when `failed` is non-empty or the request throws) and consumed by the next `render()` through `cheerOn()` on `.vw-wind-corner`, the solo `.vw-owner` and Gregal's header face. Group chats have no single face and get nothing. Sparkles are `.vw-burst` spans in `--vw-wind` / `--g-accent`.
- Visual check: static page with before/after frames in `output/ui-review/101/` (not committed). Not checked against a running server or the Electron app, and not on a light theme.
- Follow-up: the message spring-in uses the old `.vw-run-last:last-of-type` rule, so it plays again on every full `render()` of the chat, not only for messages that are actually new.
- Follow-up: a turn with `failed` entries still sets `done`, so the corner shows the happy «done» face while it shrugs.
