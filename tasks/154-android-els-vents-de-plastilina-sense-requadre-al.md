---
title: Android: els vents de plastilina sense requadre al voltant
status: done
depends_on: []
---

## Why

On Android (seen on an API 36 emulator, v1.101.0-vents) every clay wind in the home faces row sits in a dark square with a lighter edge. drawClay in WindClay.kt fills its layer white and relies on BlendMode.Modulate to keep the clay's alpha; on the phone's GPU the layer outside the clay stays partly opaque.

## Acceptance criteria

- [x] A clay wind drawn over a dark background leaves the background untouched outside the wind: no square, no edge. A Robolectric test checks the corners and edges of a clay face over a dark Box.
- [x] The body, the barretina and the clay keep their colours (the existing WindClayTest colour tests still pass), and on the emulator the faces row shows no square.

## Out of scope

- Web (the canvas player cuts to the sheet's alpha already).

## Notes

- File: android/app/src/main/java/xyz/solanes/gregal/WindClay.kt (drawClay). The Robolectric software renderer does not reproduce the GPU square, so the test guards the result and the emulator screenshot is the proof of the fix.
- Two attempts: Multiply + DstIn inside the GPU layer still left the square on the API 36 emulator. What fixed it: each sheet is tinted once per colour in a software canvas (cached, 16 MB LRU) and drawn as a plain image, no layer. Verified on the emulator (faces row, header).
- motionOffHoldsTheStillFrameWithTheProp compared two faces pixel for pixel; the tinted sheet scaled at two places differs in one pixel by one level, so it now allows at most 4 pixels at most 2 levels apart. A different frame still fails it.
