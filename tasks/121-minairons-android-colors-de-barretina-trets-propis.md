---
title: Minairons Android: colors de barretina, trets propis i el mateix disseny que la web
status: done
depends_on: ["120"]
---

## Why

With every wind drawn as a minairó, they look too alike; each should be recognisable at a glance while staying clearly a Catalan minairó.

## Acceptance criteria

- [x] Android draws the same barretina colour and traits as the web for the same wind (same palette, same deterministic pick, same override from the wind's style). A unit test with the web's expected picks for a fixed list of ids (copy them from 120's test).
- [x] Align the Android minairó with the web design from 117/120: fold side by wind direction (east winds lean left, west right), proportions (beard narrower than the body, eyes ×0.84), band width, accessories at the same sizes. Robolectric frames side by side with the web screenshots in `output/ui-review/121/`.
- [x] The wind editor/profile on Android offers the same barretina colour override. A test.

## Out of scope

- Web (120).

## Notes

- Files: `WindFace.kt` (minairó ~682-870), wind editor screens. Task 118's notes say the Android design predates 117.
- Barretina palette (curated, each keeps the wide black band so it still reads as a barretina): vermell clàssic #C9342C, morada #6E2F6B (the traditional purple barretina), plus a few that stay folk-like and readable on every body colour: verd fosc, mostassa/ocre, blau marí, granat. Pick per wind deterministically from its id, avoiding a barretina hue too close to the wind's own colour (fall back to the next).
- User (2026-11-24): «busca forma de diferenciar una mica els minairons, amb colors diferents de barretines per exemple». Earlier: «la cinta negra de baix de les barretines, així se sabrà que és més català» (done in 137ae81b: black band #17141A).

### Done (121)

- `WindFace.kt`: `minairoLook(style, body)` now follows winds.js: fold from the bearing (seed for N/S), beard length from the mouth style, gear `seed % 3`, barretina from the palette (`minairoBarretines`, id hash walk, 30° hue gap, or the 4th style letter as is), brows/moustache/glasses from the seed. `drawMinairo` uses the web's barretina and band paths (PathParser), the web beard outline (narrower than the body), eyes at .84 at (40 ± .34·rx, cy − 10), white brows, the three moustaches, glasses, and the web accessories at their sizes, shown from 28 dp as on the web. The mood motion (tip bob, beard sway, moustache curl, brows with the lids) is kept.
- `WindStyle.barretina` holds the 4th letter; `windStyleOf` reads it (unknown letters are automatic) and `encode()` writes it. The editor (new wind and edit from the profile, same `WindEditor`) shows a «Barretina» picker (Automàtica + six) while the wind is drawn as a minairó; an AI suggestion keeps the chosen barretina.
- Tests: `WindMinairoTest` (web picks table, fold/beard, 4th letter, labels), `WindEditorMinimalTest.aMinairoPicksItsBarretinaSavedAsTheFourthLetterOfItsStyle`. Two existing `WindMinairoTest` tests changed with the requirement: `each wind gets its own minairo` now feeds mouth styles (beard comes from the mouth, 4 lengths), and the 24 dp test looks for the wind's barretina colour instead of red.
- Screens (not committed): `output/ui-review/121/sizes-dark.png`, `sizes-light.png` (Android, same 8 winds as 120's sheet) next to `web-sizes.png`, `web-big.png` (copied from 120). Colours, traits, fold sides and accessories match.
- The seed comes from `windId` (id, or the slug of the name). The web keys an unsaved wind by its raw name, so only the new-wind preview can differ; Android's preview matches what the wind looks like once saved.
- Follow-up: the Android minairó body is always the round ball (R 25); the web uses the shape letter (round, plump, slim) for its body.
- Follow-up: `MascotMinimalTest.defaultFaceHasSoftVolumeAndTwoLargeSimpleEyes` and `softContactShadowSitsUnderTheBodyOnly` fail on base 4f48cedf already (Cares face, untouched here); main fixed them in 45fc7c6d.
- Follow-up: the open mouth still uses the face ink, not the web's red-based mouth ink.
