---
title: "Vents (Android): el pack Mapa de vent"
status: todo
depends_on: ["046", "049"]
---

## Why

Android parity with task 049: the Mapa de vent pack.

## Acceptance criteria

- [ ] `mapa` appears in Preferences → Avatars and in the editor's picker (Free mode), and `WindFace` draws it when chosen. A Robolectric test.
- [ ] The streaks flow in the same direction as on the web for the same wind. A unit test on the simulation step.
- [ ] Still when `LocalWindMotion` is off, unless the wind is working. A Compose test.
- [ ] A list of 30 winds scrolls without jank on a mid-range phone. A Macrobenchmark or a frame-timing note in the task is enough.

## Notes

- `WindFace.kt`, with a Compose `Canvas` and a shared frame clock (`withFrameNanos`) only while the face is visible.
- Reference: `mapa()`/`step()` in `mockups/vents-avatars.html`.
