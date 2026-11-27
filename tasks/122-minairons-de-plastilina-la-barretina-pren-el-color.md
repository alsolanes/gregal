---
title: Minairons de plastilina: la barretina pren el color de cada minairó
status: done
depends_on: ["120", "121"]
---

## Why

The clay minairó should carry each minairó's barretina colour too, not always red.

## Acceptance criteria

- [x] `tools/characters/build.py` renders the minairó barretina (tube and crown, not the black band) into a separate mask (e.g. `<mood>.cap.webp`), in a neutral tone, so clients tint it with the minairó's barretina colour; the manifest says so; the size budget holds (note the total). Re-render only the minairó.
- [x] The web player (`characters.js`) and Android player (`WindClay.kt`) tint the cap mask with the wind's barretina colour (same pick as 120/121), body mask with the wind colour as now. Tests in both.
- [x] Contact sheets with 6 differently coloured clay minairons in `output/ui-review/122/`.

## Out of scope

- Kits on clay faces.

## Notes

- See `tools/characters/README.md` and task 119's notes.
- Barretina palette (curated, each keeps the wide black band so it still reads as a barretina): vermell clàssic #C9342C, morada #6E2F6B (the traditional purple barretina), plus a few that stay folk-like and readable on every body colour: verd fosc, mostassa/ocre, blau marí, granat. Pick per wind deterministically from its id, avoiding a barretina hue too close to the wind's own colour (fall back to the next).
- User (2026-11-24): «busca forma de diferenciar una mica els minairons, amb colors diferents de barretines per exemple». Earlier: «la cinta negra de baix de les barretines, així se sabrà que és més català» (done in 137ae81b: black band #17141A).

### Done (122)

- `build.py`: the minairó's barretina tube and crown are neutral clay (`NEUTRAL`, like the body) and carry a `cap` object property; a third render pass (`render(..., "cap")`, only for the silhouettes in `CAPS`) points the mask material at it and writes `minairo/<mood>.cap.webp` at `maskSize`, made like the body mask. The black band stays black and outside both masks. The manifest gains `"cap": "<silhouette>/<mood>.cap.webp"` and `"caps": ["minairo"]`. Only the minairó was re-rendered (`--only minairo`, about 2.5 min); its `.mask.webp` files came out byte-identical.
- Size: pebble 317,382, cloud 300,120, curl 354,220, minairo 398,018 (of which the 11 cap masks are 26,926), manifest 740; total 1,370,480 bytes (was 1,403,872: the grey barretina compresses better than the red one). Under the 1.5 MB of `TestCharactersMinairoWithinBudget`.
- Web: `winds.js` puts its pick (`barretinaOf`) on the minairó svg as `data-barretina`; `characters.js` reads it and, for a silhouette in `caps`, tints the cap mask with it after the body mask (the sheet key and the last-sheet look include it). Android: `WindClay.sheet` loads the cap mask when the manifest lists the silhouette in `caps` (a missing cap file falls back to the drawn face, like a missing mask); `drawClay` tints it with `minairoLook(style, accent).cap`, which `WindFace` passes to `WindClayFace`.
- Tests: Go `TestCharactersManifestMatchesSheets` (checks the cap sheets of `caps`) and `TestCharactersMinairoWithinBudget` (the manifest has the cap mask); node `characters.test.cjs` «a minairó tints its barretina with its own colour through the cap mask…» and `wind-face.test.cjs` (the svg's `data-barretina` is the pick); Android `WindClayTest.aClayMinairoWearsItsOwnBarretinaColour` and `aClayMinairoInARedBarretinaHasNoBlueCap`.
- Review sheets (not committed): `output/ui-review/122/r1/minairons-128|40-dark|light.png` (six body and barretina pairs from 120's table, every mood, Blender composite), plus the usual `contact-*` and `motion-*`; `output/ui-review/122/web-minairons-dark|light.png`: six minairons in the real `characters.js` in Chrome (Playwright, harness in `%TEMP%/g122/shot.cjs`).
- Not verified: Android on a device (only Robolectric pixels); the full web app (only the player on a bare page).
- Follow-up: where the barretina meets the body, an edge pixel can take both tints (both half-size masks keep the max of each 2x2).
