---
title: Vents: personatges de plastilina fets amb Blender, renderitzats en fotogrames
status: todo
depends_on: ["110"]
---

## Why

For a Dots-level plush/clay look, render our characters in Blender (free) ahead of time and play them as sprite sheets, with the wind's colour applied at runtime.

## Acceptance criteria

- [ ] A Blender Python pipeline in `tools/characters/` (run with `blender -b -P build.py`) models the 110 silhouettes as soft clay/plush bodies with eyes, lights them like a studio, animates every mood from 110 (same timing/signatures) and renders each mood as a short loop of frames (≤ 128 px, WebP with alpha) in a neutral body tone plus a colour mask, so one render serves every wind colour.
- [ ] Output: `internal/web/app/characters/<silhouette>/<mood>.webp` sprite sheets plus a `manifest.json` (frames, fps, loop, size, mask). Total size budget stated in the notes and kept (aim < 1.5 MB for all).
- [ ] Rendering is reproducible: same script → same frames (seeded); a README in `tools/characters/` says how to re-render and change a design. A test (Python or Go) checks the manifest matches the files.
- [ ] Licence: the renders and scripts are the project's (Blender GPL does not apply to outputs); note it.

## Out of scope

- Players (113, 114).

## Notes

- Needs Blender installed on this machine (user installs it, e.g. `winget install BlenderFoundation.Blender`).
- Research on 2026-11-24 (no budget, open source only): Grok Bot's avatars are SVG paths moved by JavaScript (same tech as ours); what makes them good is design: one strong silhouette, expressive eyes carrying the expression, a distinct motion signature per state (idle calm and curious, thinking, working kicks into gear, waiting, blocked, done settles), springs and anticipation instead of constant loops, state detail on hover. Dots are 3D plush/clay characters with soft studio light. Our designs stay our own (no copies of their characters).
- User: «l'estètica ha de ser millor del que tenim, dots és molt millor, grok bot també» and chose both steps: redesign in code first, then a Blender-rendered clay version (free, GPL; renders are ours to redistribute).
