---
title: Vents Android: la conversa d'un vent agafa el seu color i en mostra l'estat
status: done
depends_on: []
---

## Why

A wind's conversation looks the same for every wind: neutral bubbles on a neutral background, with the wind's colour only in its face. The wind should feel like a character with its own colour, and the person should see at a glance whether it is working, waiting for them or done.

## Acceptance criteria

- [x] In `WindConversation` (`CompanyScreen.kt`) the screen background gets a soft wash of the wind's colour and `WindBubble` the wind's bubbles a tint of it, in dark and light; text contrast of wind bubbles is at least 4.5:1 for the squad preset colours and `#3D7BFF`. A unit test of the colour mix.
- [x] `VentsChatHeader` shows the wind's state as one label: working (the wind is in `windBusy`), waiting for you (an entry in `windAsks` for that wind), done (after a turn ends until the person sends a message or leaves), nothing otherwise. A unit test of the pure state function for the four cases.
- [x] The same rule as the web (085): same states, same wording keys.
- [x] Strings in Catalan and English in `Strings.kt`.

## Out of scope

- Web (085).
- The avatar outside the frame (088).
- Rotllana conversations.

## Notes

- Files: `CompanyScreen.kt` (`WindConversation` 1174, `WindBubble` 1921, background `Superficie` ~1244), `VentsChatChrome.kt` (`VentsChatHeader` 69), `VentsHomeParts.kt` (`WindTypingBubble` already tints at 16%), `GregalViewModel.kt` (`windBusy` 3544, `windAsks` 912, `windLive` 3552).
- Android: run gradle tests with `ANDROID_HOME` set; edit `Strings.kt` by hand (do not run `regen-strings.py`); check on the emulator (`Medium_Phone_API_36`, x86_64 build with `-Pgregal.abi=x86_64`) with a screenshot in the notes. Regex: no `(?U)` (Android's ICU refuses it; `UserAvatarTest` guards this).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
- Done 2026-11-24: `windWash` (6 % of the wind colour over the surface), `windBubbleColor` (16 %, like the typing dots) and the pure `windChatState` live in `VentsChatChrome.kt`; `WindBubble` takes an optional `tint` (the washed background) that only `WindConversation` passes, so rotllana, shared and Ventet chats keep their look. The state goes in the header's existing subtitle line. Tests: `WindChatColorStateTest` (contrast of ink and text over the bubble for every `GregalTemes` theme in light and dark, squad presets plus `#3D7BFF`; the four states; strings in both languages).
- Rule for the web (085) to match: keys `winds.state.working` / `winds.state.waiting` / `winds.state.done` (added to `Strings.kt`, which mirrors `i18n.js`, so 085 should add the same keys there); waiting wins over working when the wind asks during a running turn; done clears when a new turn starts or the chat is left.
- Not verified: no emulator screenshot (left to the visual check step), so the look in dark and light is only covered by the contrast test.
- Follow-up: «Fet» does not clear when a message is only queued offline (no turn starts); the web rule should decide whether sending alone clears it.
