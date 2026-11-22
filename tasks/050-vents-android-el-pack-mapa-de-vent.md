---
title: "Vents (Android): el pack Mapa de vent"
status: in-progress
depends_on: ["046", "049"]
---

## Why

Android parity with task 049: the Mapa de vent pack.

## Acceptance criteria

- [x] `mapa` appears in Preferences → Avatars and in the editor's picker (Free mode), and `WindFace` draws it when chosen. A Robolectric test.
- [x] The streaks flow in the same direction as on the web for the same wind. A unit test on the simulation step.
- [x] Still when `LocalWindMotion` is off, unless the wind is working. A Compose test.
- [ ] A list of 30 winds scrolls without jank on a mid-range phone. A Macrobenchmark or a frame-timing note in the task is enough.

## Notes

- `WindFace.kt`, with a Compose `Canvas` and a shared frame clock (`withFrameNanos`) only while the face is visible.
- Reference: `mapa()`/`step()` in `mockups/vents-avatars.html`.

### What was built (2026-11-22)

- `mapa` joins `windPacks`, so it is valid for the theme (`avatarThemeOf`) and a wind's own avatar, and appears in Preferences › Avatars and in the editor's Free-mode picker with no picker change. String `winds.avatarMapa` («Mapa de vent» / «Wind map», the web's texts) in `AppStrings.kt`.
- `WindMap.kt`: `mapRandom`, `mapStart`, `mapStep` mirror winds.js (the same PRNG bits on Kotlin `Int`, the seed from the wind's id hash, 90 particles, `phi = bearing + 180°`, ±0.6 rad swirl, 0.0055 / 0.013 per step, fade 0.1 / 0.16, `BlendMode.Plus` for `lighter`, the same stroke width rule). The picture is an `ImageBitmap` at the face's pixel size (min 48) with the web's still frame (50 silent steps, then 20 painted); the face clips it to a circle and adds the gloss, rim and mood badge.
- `WindStyle.windId` carries what seeds the picture (`windId(id, name)`, split out of `windDirection`: the id, or the slug of the name for an unsaved wind), filled by `Wind.faceStyle` and the editor preview, so the preview is the picture the saved wind gets.
- Motion: as Esperit, the map steps once per frame (`withFrameNanos` in a `LaunchedEffect`, so only composed faces step) when the Cares face would move (`LocalWindMotion`, the animator scale, not a small non-tappable face) or while the wind works (then faster); otherwise it is the still frame.
- `CrewProposalTest` compares `faceStyle` exactly; its expectation now includes `windId = "ratxa"`. Stricter, not weaker.

### Tests

- `WindMapTest`: start and step values pinned from node with the web's functions (Ponent, Bit, the unsaved «Lupa Ràpida»), the streak direction (Ponent east, Tramuntana down), Preferences posts `{theme:"mapa"}`, the Free editor saves `avatar: "mapa"`, `WindFace` draws the map from the theme and from the wind's own avatar (the same picture), and with motion off an idle map is still while a working one moves.

### Not verified

- Criterion 4 is not met, so `status` stays `in-progress`: no frame timing was measured on a phone (none was available). Each moving face steps 90 particles and redraws a software bitmap that is uploaded on every frame; with 30 faces moving at once that is the main jank risk. Idle faces in lists under 64 dp do not move (as Esperit), so a list of idle winds only pays for the still frame when each row is first composed.
- Rendered only in Robolectric (Tramuntana, Ponent, Bit working, Gregal: the streaks run down, east and south-west as on the web); not seen on a device. The colours use Compose `lerp` like the rest of `WindFace`, which is not the web's sRGB `mix`, so the tints can differ slightly.
- Follow-up: faces stop stepping off screen only in lazy lists; in a plain scrolling `Column` every composed map keeps stepping. Which wind lists are lazy was not checked.
- Follow-up: the bitmap capture helper is duplicated in `WindEsperitTest` and `WindMapTest`.
- Rebased onto 048: `mapa` joins `cap` and `rosa` in `windPacks` and the `WindFace` dispatch, and the full Android suite passes after the 051 list-face fix.
