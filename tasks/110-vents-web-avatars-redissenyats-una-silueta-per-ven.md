---
title: Vents web: avatars redissenyats, una silueta per vent, ulls que parlen i un moviment per a cada estat
status: todo
depends_on: []
---

## Why

The winds must look as good as the best agent characters out there, with no new dependency: a stronger design and real motion design in the code we already have.

## Acceptance criteria

- [ ] The default pack («Cares») is redesigned: a small set of strong silhouettes (each wind keeps one, chosen from its style/id), tinted with the wind's colour with soft clay-like volume (light, rim, soft contact shadow), and large expressive eyes that carry most of the expression (eyelids, gaze, squint, happy arcs, wide). Before/after screenshots of six winds small (24–28 px) and large, dark and light, in `output/ui-review/110/`; small sizes stay crisp.
- [ ] Each mood has its own motion signature, not a loop pasted on: idle (calm breathing, curious glances, blinks), thinking (eyes up, slow tilt), searching/reading (eyes scan), working (energetic bob, determined squint), waiting (looks at the person, gentle lean), blocked/failed (droop), done (anticipation squash → hop → settle with a spring). Transitions between moods use springs (no snapping). A test per mood that its animation differs from the others and that transitions interpolate.
- [ ] The 101/100 life (blink, glance, squash on tap/hover, celebration) and 108/109 single flying face keep working on the new design. Reduced motion / motion off makes every face still.
- [ ] No new dependency; animation cost stays on transform/opacity (web: SVG + CSS/JS springs). The other packs are unchanged.

## Out of scope

- Android (111).
- Blender version (112-114).

## Notes

- Files: `winds.js` (`face`, Cares drawing ~575-618, moods), `winds.css`.
- Research on 2026-11-24 (no budget, open source only): Grok Bot's avatars are SVG paths moved by JavaScript (same tech as ours); what makes them good is design: one strong silhouette, expressive eyes carrying the expression, a distinct motion signature per state (idle calm and curious, thinking, working kicks into gear, waiting, blocked, done settles), springs and anticipation instead of constant loops, state detail on hover. Dots are 3D plush/clay characters with soft studio light. Our designs stay our own (no copies of their characters).
- User: «l'estètica ha de ser millor del que tenim, dots és molt millor, grok bot també» and chose both steps: redesign in code first, then a Blender-rendered clay version (free, GPL; renders are ours to redistribute).
