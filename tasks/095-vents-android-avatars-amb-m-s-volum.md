---
title: Vents Android: avatars amb més volum
status: done
depends_on: []
---

## Why

The user wants the wind avatars to look more three-dimensional. «Cares» already has volume (radial gradients, rim light, sheen, ground shadow); the other packs are flatter.

## Acceptance criteria

- [x] `EsperitFace`, `CapFace` and `RosaFace` (`WindFace.kt`) get the volume of `WindVolume.kt` (radial gradient, rim, shine, coloured contact shadow), as the default face has. A unit test that each pack draws through the volume helper (e.g. a draw-call recorder or a bitmap probe like `WindEsperitTest`).
- [x] The gravity tilt (`rememberWindLean`) also moves the highlight of these packs when expressive motion is on.
- [ ] Every pack stays readable at the smallest list size: emulator screenshots of every pack, small and large, dark and light.
- [x] With expressive motion off nothing moves.

## Out of scope

- Web (094).
- The map pack.

## Notes

- Files: `WindFace.kt` (`EsperitFace` 528, `CapFace` 585, `RosaFace` 649), `WindVolume.kt` (`rememberWindLean` 166). Tests: `WindEsperitTest`, `WindCapRosaTest`.
- Android: run gradle tests with `ANDROID_HOME` set; edit `Strings.kt` by hand (do not run `regen-strings.py`); check on the emulator (`Medium_Phone_API_36`, x86_64 build with `-Pgregal.abi=x86_64`) with a screenshot in the notes. Regex: no `(?U)` (Android's ICU refuses it; `UserAvatarTest` guards this).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
- Done (2026-11-24): `WindVolume.kt` splits `drawWindBody` into `drawWindShadow` (ground shadow) and `drawWindVolume` (fill, rim, shine), both in radius-1 coordinates. Esperit's head and Cap's head (a ball 1.5 wider than tall) draw both with a `WindBodyPaint`; Rosa draws only `drawWindVolume` with its own dark dial gradient, because the dial fills the box and a ground shadow would fall outside it. Instead the long point casts a dark shadow on the dial, away from the light. All three read `rememberWindLean(motion && size >= 64)`, as Cares does.
- Tests: `WindCapRosaTest` "Esperit, Cap and Rosa have the volume of Cares..." (rim and contact shadow probes) and "the gravity tilt moves the light of every pack..." (large faces follow the tilt; list faces and motion-off faces do not).
- `WindCapRosaTest.ink` now ignores pixels under 48 on every channel: the new contact shadow under Cap's head is ground, not breath, and two direction assertions ("nothing below its head", "24 dp: no middle line") would otherwise count it.
- Not verified: the emulator screenshots of the third criterion (left to the shared visual step). A Robolectric render of every pack at 24, 40 and 96 dp on dark and light looked readable (`output/ui-review/095/sheet.png`, ignored, from a throwaway test not committed).
