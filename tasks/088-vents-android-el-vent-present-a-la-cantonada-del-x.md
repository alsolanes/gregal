---
title: Vents Android: el vent present a la cantonada del xat, animat pel que fa
status: todo
depends_on: ["086"]
---

## Why

The wind should feel present next to the person, not only as a small face in each row: a larger avatar that sits partly outside the chat's frame and moves with what the wind is doing.

## Acceptance criteria

- [ ] In `WindConversation` the wind's avatar (`WindFace`, active pack) sits at a bottom corner, about 72 dp, partly over the edge of the message list, above the composer; it does not show in rotllana conversations. A Compose test that the node exists in a wind chat and not in a rotllana.
- [ ] Its mood follows the state from 086 (working, waiting, done), with the existing aura while working (`windShowsAura`). A unit test of the state → mood mapping.
- [ ] It never covers the composer or the last message: the list gets bottom padding while it is shown. Screenshot on the emulator.
- [ ] With expressive motion off (`windMotionEnabled`) it does not animate.

## Out of scope

- Web (087).

## Notes

- Files: `CompanyScreen.kt` (`WindConversation`), `WindFace.kt` (`WindFace` 203, `windShowsAura` 170, `windMotionEnabled` 91), `VentsSpectacle.kt` (`WindHalo` 35).
- Android: run gradle tests with `ANDROID_HOME` set; edit `Strings.kt` by hand (do not run `regen-strings.py`); check on the emulator (`Medium_Phone_API_36`, x86_64 build with `-Pgregal.abi=x86_64`) with a screenshot in the notes. Regex: no `(?U)` (Android's ICU refuses it; `UserAvatarTest` guards this).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
