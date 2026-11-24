---
title: Vents Android: la conversa d'un vent agafa el seu color i en mostra l'estat
status: todo
depends_on: []
---

## Why

A wind's conversation looks the same for every wind: neutral bubbles on a neutral background, with the wind's colour only in its face. The wind should feel like a character with its own colour, and the person should see at a glance whether it is working, waiting for them or done.

## Acceptance criteria

- [ ] In `WindConversation` (`CompanyScreen.kt`) the screen background gets a soft wash of the wind's colour and `WindBubble` the wind's bubbles a tint of it, in dark and light; text contrast of wind bubbles is at least 4.5:1 for the squad preset colours and `#3D7BFF`. A unit test of the colour mix.
- [ ] `VentsChatHeader` shows the wind's state as one label: working (the wind is in `windBusy`), waiting for you (an entry in `windAsks` for that wind), done (after a turn ends until the person sends a message or leaves), nothing otherwise. A unit test of the pure state function for the four cases.
- [ ] The same rule as the web (085): same states, same wording keys.
- [ ] Strings in Catalan and English in `Strings.kt`.

## Out of scope

- Web (085).
- The avatar outside the frame (088).
- Rotllana conversations.

## Notes

- Files: `CompanyScreen.kt` (`WindConversation` 1174, `WindBubble` 1921, background `Superficie` ~1244), `VentsChatChrome.kt` (`VentsChatHeader` 69), `VentsHomeParts.kt` (`WindTypingBubble` already tints at 16%), `GregalViewModel.kt` (`windBusy` 3544, `windAsks` 912, `windLive` 3552).
- Android: run gradle tests with `ANDROID_HOME` set; edit `Strings.kt` by hand (do not run `regen-strings.py`); check on the emulator (`Medium_Phone_API_36`, x86_64 build with `-Pgregal.abi=x86_64`) with a screenshot in the notes. Regex: no `(?U)` (Android's ICU refuses it; `UserAvatarTest` guards this).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
