---
title: Android: els vents de plastilina sense requadre al voltant
status: todo
depends_on: []
---

## Why

On Android (seen on an API 36 emulator, v1.101.0-vents) every clay wind in the home faces row sits in a dark square with a lighter edge. drawClay in WindClay.kt fills its layer white and relies on BlendMode.Modulate to keep the clay's alpha; on the phone's GPU the layer outside the clay stays partly opaque.

## Acceptance criteria

- [ ] A clay wind drawn over a dark background leaves the background untouched outside the wind: no square, no edge. A Robolectric test checks the corners and edges of a clay face over a dark Box.
- [ ] The body, the barretina and the clay keep their colours (the existing WindClayTest colour tests still pass), and on the emulator the faces row shows no square.

## Out of scope

- Web (the canvas player cuts to the sheet's alpha already).

## Notes

- File: android/app/src/main/java/xyz/solanes/gregal/WindClay.kt (drawClay). The Robolectric software renderer does not reproduce the GPU square, so the test guards the result and the emulator screenshot is the proof of the fix.
