---
title: Vents Android: els personatges de plastilina es mouen al mòbil
status: done
depends_on: ["112"]
---

## Why

Play the clay characters on Android, with the Canvas face as fallback.

## Acceptance criteria

- [x] A Compose player draws the mood sprite sheet tinted with the wind's colour through the mask, crossfading moods; used for large faces (corner/flying, profile, home strip); small avatars keep the Canvas face. A Compose test of mood switching and the fallback.
- [x] Celebration, nod/shake and the flight (100, 103, 109) still play. Motion off shows the first frame.
- [x] No new dependency; sheets load from the APK assets (copied from `internal/web/app/characters/` at build time) or from the server.

## Out of scope

- Web (113).

## Notes

- Files: `WindFace.kt`, a new player file; gradle copy task for assets.
- User: «l'estètica ha de ser millor del que tenim, dots és molt millor, grok bot també» and chose both steps: redesign in code first, then a Blender-rendered clay version (free, GPL; renders are ours to redistribute).
- Done 2026-11-24. `WindClay.kt`: `WindClayFace` plays `<silhouette>/<mood>.webp` (silhouette from the style's third letter via the manifest; HAPPY plays `idle`, TALKING `discussing`). Each sheet is decoded once into an LruCache (24 MB); the frame number is a state read only in the Canvas draw, so a face redraws at 12 fps without recomposing. Looping sheets use `withInfiniteAnimationFrameNanos` (like an infinite transition: test idling does not wait for them); `done`/`failed` play once and hold. Paused while the lifecycle is below STARTED, resumed on the same frame. Tint: in a layer, white, the mask upscaled and tinted with `SrcIn`, then the sheet with `Modulate` (the same blend on every API level; `Multiply` maps to a different PorterDuff mode below API 29). Mood change crossfades 220 ms over the old sheet's last frame. Motion off: frame 0, no crossfade.
- `WindFace(clay = true)` / `WindHalo(clay = true)` opt in: the chat's flying face (109), the profile's big face, and the home strip (`StartFaces` in VentsStart.kt). The rest keep the Canvas face. Today has no large face (its cards use a 30 dp avatar), so nothing there changed. The 100 burst/shrug, the 103 nod/shake and the 109 flight are modifiers on the face and work unchanged on the clay one.
- Fallback to the Canvas face when the manifest, the mood or a file is missing, and when the face wears its kit (`showKit`/large figure with a `windKits` specialty): the clay winds have no kits yet and `WindPresenceTest.theCornerWindWearsItsKit` requires the kit on the chat face. So a crew wind with a specialty still shows the Canvas face in its chat.
- Gradle: `copyWindCharacters` copies `internal/web/app/characters/` into a generated assets dir added with `androidComponents.onVariants { sources.assets.addGeneratedSourceDirectory(...) }` (a plain `srcDir` + `dependsOn` broke `generateReleaseLintVitalReportModel`). No files duplicated in git. Robolectric tests read the real sheets from the merged assets.
- APK size: `assembleRelease -Pgregal.plotsPreview -Pgregal.abi=arm64-v8a` went from 22,185,000 to 23,427,214 bytes (+1.24 MB; 67 files, 1,232,366 bytes, WebP is stored almost uncompressed).
- Tests: `WindClayTest` (mood switching and silhouettes, small avatars and a missing folder keep the Canvas face, motion on plays / off shows frame 0, red and blue tint through the mask, the 100 burst on a clay face and none without motion). Review frames: `output/ui-review/114/` (idle, working, waiting, done, failed, crossfade; three silhouettes in two colours next to a 40 dp Canvas avatar).
- Not verified: on a real device (no emulator used): first-decode jank (decoding is synchronous on first use of a sheet, a few ms), memory, and how the crossfade reads (in the frames it is a short double exposure).
- Not loaded from the server: the sheets only come from the APK assets.
- Follow-up: clay kits (or draw the kit over the clay figure) so crew winds with a specialty get the clay face in their chat too.
- Follow-up: `HomeFacesRow` (VentsHomeParts.kt) and `VentsCrew` (CompanyScreen.kt) have no callers.
- Follow-up: `WindVidaTest.aFailedTurnShrugsWithoutConfetti` failed once in a wide run and passed on rerun (Canvas face only, untouched here); looks flaky.
