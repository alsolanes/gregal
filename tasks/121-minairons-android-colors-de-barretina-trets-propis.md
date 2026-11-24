---
title: Minairons Android: colors de barretina, trets propis i el mateix disseny que la web
status: todo
depends_on: ["120"]
---

## Why

With every wind drawn as a minairó, they look too alike; each should be recognisable at a glance while staying clearly a Catalan minairó.

## Acceptance criteria

- [ ] Android draws the same barretina colour and traits as the web for the same wind (same palette, same deterministic pick, same override from the wind's style). A unit test with the web's expected picks for a fixed list of ids (copy them from 120's test).
- [ ] Align the Android minairó with the web design from 117/120: fold side by wind direction (east winds lean left, west right), proportions (beard narrower than the body, eyes ×0.84), band width, accessories at the same sizes. Robolectric frames side by side with the web screenshots in `output/ui-review/121/`.
- [ ] The wind editor/profile on Android offers the same barretina colour override. A test.

## Out of scope

- Web (120).

## Notes

- Files: `WindFace.kt` (minairó ~682-870), wind editor screens. Task 118's notes say the Android design predates 117.
- Barretina palette (curated, each keeps the wide black band so it still reads as a barretina): vermell clàssic #C9342C, morada #6E2F6B (the traditional purple barretina), plus a few that stay folk-like and readable on every body colour: verd fosc, mostassa/ocre, blau marí, granat. Pick per wind deterministically from its id, avoiding a barretina hue too close to the wind's own colour (fall back to the next).
- User (2026-11-24): «busca forma de diferenciar una mica els minairons, amb colors diferents de barretines per exemple». Earlier: «la cinta negra de baix de les barretines, així se sabrà que és més català» (done in 137ae81b: black band #17141A).
