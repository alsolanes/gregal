---
title: Vents Android: avatars amb més volum
status: todo
depends_on: []
---

## Why

The user wants the wind avatars to look more three-dimensional. «Cares» already has volume (radial gradients, rim light, sheen, ground shadow); the other packs are flatter.

## Acceptance criteria

- [ ] `EsperitFace`, `CapFace` and `RosaFace` (`WindFace.kt`) get the volume of `WindVolume.kt` (radial gradient, rim, shine, coloured contact shadow), as the default face has. A unit test that each pack draws through the volume helper (e.g. a draw-call recorder or a bitmap probe like `WindEsperitTest`).
- [ ] The gravity tilt (`rememberWindLean`) also moves the highlight of these packs when expressive motion is on.
- [ ] Every pack stays readable at the smallest list size: emulator screenshots of every pack, small and large, dark and light.
- [ ] With expressive motion off nothing moves.

## Out of scope

- Web (094).
- The map pack.

## Notes

- Files: `WindFace.kt` (`EsperitFace` 528, `CapFace` 585, `RosaFace` 649), `WindVolume.kt` (`rememberWindLean` 166). Tests: `WindEsperitTest`, `WindCapRosaTest`.
- Android: run gradle tests with `ANDROID_HOME` set; edit `Strings.kt` by hand (do not run `regen-strings.py`); check on the emulator (`Medium_Phone_API_36`, x86_64 build with `-Pgregal.abi=x86_64`) with a screenshot in the notes. Regex: no `(?U)` (Android's ICU refuses it; `UserAvatarTest` guards this).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
