---
title: Vents web: avatars redissenyats, una silueta per vent, ulls que parlen i un moviment per a cada estat
status: done
depends_on: []
---

## Why

The winds must look as good as the best agent characters out there, with no new dependency: a stronger design and real motion design in the code we already have.

## Acceptance criteria

- [x] The default pack («Cares») is redesigned: a small set of strong silhouettes (each wind keeps one, chosen from its style/id), tinted with the wind's colour with soft clay-like volume (light, rim, soft contact shadow), and large expressive eyes that carry most of the expression (eyelids, gaze, squint, happy arcs, wide). Before/after screenshots of six winds small (24–28 px) and large, dark and light, in `output/ui-review/110/`; small sizes stay crisp.
- [x] Each mood has its own motion signature, not a loop pasted on: idle (calm breathing, curious glances, blinks), thinking (eyes up, slow tilt), searching/reading (eyes scan), working (energetic bob, determined squint), waiting (looks at the person, gentle lean), blocked/failed (droop), done (anticipation squash → hop → settle with a spring). Transitions between moods use springs (no snapping). A test per mood that its animation differs from the others and that transitions interpolate.
- [x] The 101/100 life (blink, glance, squash on tap/hover, celebration) and 108/109 single flying face keep working on the new design. Reduced motion / motion off makes every face still.
- [x] No new dependency; animation cost stays on transform/opacity (web: SVG + CSS/JS springs). The other packs are unchanged.

## Out of scope

- Android (111).
- Blender version (112-114).

## Notes

- Files: `winds.js` (`face`, Cares drawing ~575-618, moods), `winds.css`.
- Research on 2026-11-24 (no budget, open source only): Grok Bot's avatars are SVG paths moved by JavaScript (same tech as ours); what makes them good is design: one strong silhouette, expressive eyes carrying the expression, a distinct motion signature per state (idle calm and curious, thinking, working kicks into gear, waiting, blocked, done settles), springs and anticipation instead of constant loops, state detail on hover. Dots are 3D plush/clay characters with soft studio light. Our designs stay our own (no copies of their characters).
- User: «l'estètica ha de ser millor del que tenim, dots és molt millor, grok bot també» and chose both steps: redesign in code first, then a Blender-rendered clay version (free, GPL; renders are ours to redistribute).
- Design (2026-11-24), for 111 to match on Android. All in `winds.js` `face()` (Cares) and `winds.css`; other packs untouched.
  - Silhouettes from `style[2]`: `o` pebble/mochi (`pebble` path: broad shoulders, flat settled base, 69×62 in the 80 box), `s` the same pebble plus the gust curl (the curl is now part of the shading clip), `n` the cloud as before.
  - Clay volume: same layers, more matte: sheen r .7R at .3, the specular is a soft broad blob (rx .27R, ry .16R, peak .7) instead of a glint, core shadow a little lighter (.5/.38/.18). Every tone is `mix()` of the wind colour; only white, blush and tongue are fixed.
  - Eyes at the same centres (30.4/49.6, y 38.4, so kit glasses still fit), dark ink ellipses sized by `style[0]`: p 5.8×8, b 7.2×8.6, e 6.2×7.8 with a four-point star catchlight, m 6.8×5.8 set 1.6 lower (calm). Under 28 px ×1.12/×1.08, one bold catchlight, no iris glow or second catchlight. Large/regular: a soft glow of the wind colour at the bottom of the eye and a small second catchlight.
  - Each eye is clipped to itself and holds an upper lid (body paint + 1.2 crease line in `mix(fill,#1B1530,.45)`, moved by `--wf-lid` 0..1 of the eye, slanted by `--wf-brow`, mirrored on the right eye) and a lower lid ellipse that rises with `--wf-smile` into happy arcs. The old closed-arc "joy" eyes are gone.
  - Poses per mood (`poses` in winds.js) set as CSS vars on the svg: x, y, tilt, sx, sy (body, `.wf-pose`), gx, gy (gaze, `.wf-aim`), lid, brow, smile. They transition with a damped spring (ζ .55, 1.55 Hz, .9 s, ~12 % overshoot) written as CSS `linear()`. A face redrawn in a new mood (the chat corner replaces its face) starts from that wind's last pose at that size (`key|large`) and springs after two frames; under reduced motion it takes the pose at once and nothing moves.
  - Motion signatures (loops on `.wf-figure`/`.wf-gaze` over the pose): idle breathe + glances + blinks; thinking slow 6 s tilt with eyes drifting up; searching peers forward and scans fast (1.2 s); reading line scan; writing bob; working hard .6 s squash-bob with stepping feet; discussing converse; waiting (new mood: the chat's «t'espera») leans toward the person, wide eyes looking out-down, slow 4 s lean; failed droops (sinks, squashes, sad lids) and sighs every 6 s; done anticipation squash → stretched hop → landing squash → wobble to rest (1.1 s, once per transition) with happy-arc eyes; news hello.
- Screenshots and frame strips (before/after, 24/28/48/120 px, dark and light, moods, idle→working, working→done, idle→failed in place, reduced motion) in `output/ui-review/110/` (ignored by git, on the task's worktree).
- Not verified in the real app: only in a static harness page with the real winds.js/winds.css; the chat corner flight (108) was not exercised with the new faces.
- Follow-up: the lids and lower lids are painted with the body gradient but sit above the shade/sheen layers, so they read a touch lighter than the face around them; a matching overlay would hide that.
- `waiting` is now a real face mood. The chat corner (winds-workspace.css) already leans its waiting face −8° with its own loop, so winds.css drops the face's own tilt and lean loop inside `.vw-wind-corner`; elsewhere the face leans itself.
- Follow-up: the comment in winds-workspace.js (cornerMood, «waiting no és una cara de winds.js») is now stale, and the single-wind owner face there never passes `waiting`; left alone because task 108 is changing that file.
