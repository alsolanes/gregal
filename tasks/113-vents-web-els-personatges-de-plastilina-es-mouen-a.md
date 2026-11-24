---
title: Vents web: els personatges de plastilina es mouen a la web
status: todo
depends_on: ["112"]
---

## Why

Play the clay characters on the web and desktop, with the code-drawn face as fallback.

## Acceptance criteria

- [ ] A small player draws the right mood sheet on a canvas, tinting the body with the wind's colour through the mask, switching moods with a short crossfade; it is used wherever a wind face is large (corner/flying face, profile, start faces); tiny list avatars keep the SVG. A node test of mood switching and the fallback when a sheet is missing.
- [ ] Celebration, nod/shake and the flight (101, 102, 108) still play on top. Reduced motion shows the first frame.
- [ ] No new dependency.

## Out of scope

- Android (114).

## Notes

- Files: `winds.js` face entry points, a new `characters.js`.
- User: «l'estètica ha de ser millor del que tenim, dots és molt millor, grok bot també» and chose both steps: redesign in code first, then a Blender-rendered clay version (free, GPL; renders are ours to redistribute).
