---
title: Minairons de plastilina: la barretina pren el color de cada minairó
status: todo
depends_on: ["120", "121"]
---

## Why

The clay minairó should carry each minairó's barretina colour too, not always red.

## Acceptance criteria

- [ ] `tools/characters/build.py` renders the minairó barretina (tube and crown, not the black band) into a separate mask (e.g. `<mood>.cap.webp`), in a neutral tone, so clients tint it with the minairó's barretina colour; the manifest says so; the size budget holds (note the total). Re-render only the minairó.
- [ ] The web player (`characters.js`) and Android player (`WindClay.kt`) tint the cap mask with the wind's barretina colour (same pick as 120/121), body mask with the wind colour as now. Tests in both.
- [ ] Contact sheets with 6 differently coloured clay minairons in `output/ui-review/122/`.

## Out of scope

- Kits on clay faces.

## Notes

- See `tools/characters/README.md` and task 119's notes.
- Barretina palette (curated, each keeps the wide black band so it still reads as a barretina): vermell clàssic #C9342C, morada #6E2F6B (the traditional purple barretina), plus a few that stay folk-like and readable on every body colour: verd fosc, mostassa/ocre, blau marí, granat. Pick per wind deterministically from its id, avoiding a barretina hue too close to the wind's own colour (fall back to the next).
- User (2026-11-24): «busca forma de diferenciar una mica els minairons, amb colors diferents de barretines per exemple». Earlier: «la cinta negra de baix de les barretines, així se sabrà que és més català» (done in 137ae81b: black band #17141A).
