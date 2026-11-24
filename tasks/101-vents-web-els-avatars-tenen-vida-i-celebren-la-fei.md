---
title: Vents web: els avatars tenen vida i celebren la feina feta
status: todo
depends_on: []
---

## Why

The user wants Vents to be fun on web and desktop: the avatars should feel alive and the UI should move with intent.

## Acceptance criteria

- [ ] Idle faces blink and glance now and then (randomised), and react on hover/click (squash and spring back). A node test that idle faces get the blink animation and that `prefers-reduced-motion` removes it.
- [ ] When a turn ends well the corner wind and the header face celebrate once (sparkle burst in the wind's colour); a failed turn gets a small shrug. A node test that done adds the celebration class once and error the shrug.
- [ ] New messages spring in, the pending bubble breathes, side panes slide in; all disabled under reduced motion.
- [ ] Animations use CSS transforms/opacity only (no layout thrash).

## Out of scope

- Android (100).

## Notes

- Files: `winds.js` (faces, moods), `winds.css` (keyframes 275-336, reduced motion 383), `winds-workspace.js` / `.css` (chat, corner face).
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Web: `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server (never type into credential fields) with before/after screenshots in `output/ui-review/` (ignored).
