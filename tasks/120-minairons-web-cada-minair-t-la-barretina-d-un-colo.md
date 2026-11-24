---
title: Minairons web: cada minairó té la barretina d'un color i trets propis
status: todo
depends_on: []
---

## Why

With every wind drawn as a minairó, they look too alike; each should be recognisable at a glance while staying clearly a Catalan minairó.

## Acceptance criteria

- [ ] Each minairó's barretina takes a colour from the curated palette (notes), chosen deterministically per wind and never too close to its body colour; the black band stays. A node test over many ids: deterministic, spread across the palette, contrast with the body.
- [ ] Small per-wind traits on top of 117's (beard length, fold side, accessory): eyebrow shape (straight, arched, bushy), a moustache style, and rarely round glasses — readable at 48 px, dropped below 28 px. Screenshots of 8 different minairons at 24/48/120 px, dark and light, in `output/ui-review/120/`.
- [ ] The wind editor (Lliure mode and the profile) lets the person override the barretina colour from the palette; saved per wind in the wind's style. A node test.

## Out of scope

- Android (121).
- Clay (122).

## Notes

- Files: `winds.js` (minairó drawing ~666-740, `face`, wind editor), `winds.css`.
- Barretina palette (curated, each keeps the wide black band so it still reads as a barretina): vermell clàssic #C9342C, morada #6E2F6B (the traditional purple barretina), plus a few that stay folk-like and readable on every body colour: verd fosc, mostassa/ocre, blau marí, granat. Pick per wind deterministically from its id, avoiding a barretina hue too close to the wind's own colour (fall back to the next).
- User (2026-11-24): «busca forma de diferenciar una mica els minairons, amb colors diferents de barretines per exemple». Earlier: «la cinta negra de baix de les barretines, així se sabrà que és més català» (done in 137ae81b: black band #17141A).
