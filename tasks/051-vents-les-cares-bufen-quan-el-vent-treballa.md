---
title: "Vents: les cares bufen quan el vent treballa"
status: todo
depends_on: ["045", "046"]
---

## Why

The Cares pack (the current faces) gets the one thing only Gregal has: while a wind works, it blows a few breath lines toward the wind's direction, as in «Cares que bufen» in `mockups/vents-avatars.html`. Today the working state is the same as everyone's.

## Acceptance criteria

- [ ] Web: a Cares face in the `working`, `thinking`, `searching`, `reading` or `writing` mood shows three breath lines leaving the face toward the opposite of the wind's direction (045's function), and two short lines below 28 px. Idle shows none. A node test with the fake DOM.
- [ ] Android: `WindFace` with the Cares pack does the same, in the same direction as the web. A Compose test.
- [ ] The lines stay inside the face's own box plus a margin that doesn't cover the wind's name in list rows. A node test checks the overlay's box, and a screenshot of the sidebar is enough on Android.
- [ ] The lines move only while the wind works; when the mood goes back to idle, they disappear. A test on each platform.

## Out of scope

- Changing the face drawing, the kits or the other moods.

## Notes

- Web: `face()`/`setMood()` in `internal/web/app/winds.js`. Android: `WindFace.kt`, `drawKit`/mood handling.
- Reference: `bufen()` in `mockups/vents-avatars.html`. The `working` mood's cog stays for packs other than Cares; in Cares the breath replaces it.
- This touches both clients in one diff because each side is small (well under 200 lines). If it grows, split it into web and Android.
