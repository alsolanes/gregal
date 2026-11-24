---
title: Vents Android: els personatges de plastilina es mouen al mòbil
status: todo
depends_on: ["112"]
---

## Why

Play the clay characters on Android, with the Canvas face as fallback.

## Acceptance criteria

- [ ] A Compose player draws the mood sprite sheet tinted with the wind's colour through the mask, crossfading moods; used for large faces (corner/flying, profile, home strip); small avatars keep the Canvas face. A Compose test of mood switching and the fallback.
- [ ] Celebration, nod/shake and the flight (100, 103, 109) still play. Motion off shows the first frame.
- [ ] No new dependency; sheets load from the APK assets (copied from `internal/web/app/characters/` at build time) or from the server.

## Out of scope

- Web (113).

## Notes

- Files: `WindFace.kt`, a new player file; gradle copy task for assets.
- User: «l'estètica ha de ser millor del que tenim, dots és molt millor, grok bot també» and chose both steps: redesign in code first, then a Blender-rendered clay version (free, GPL; renders are ours to redistribute).
