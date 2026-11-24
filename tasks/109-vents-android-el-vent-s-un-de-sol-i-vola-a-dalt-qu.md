---
title: Vents Android: el vent és un de sol i vola a dalt quan et mous pel xat
status: todo
depends_on: []
---

## Why

In a one-to-one wind chat the wind must be a single character on screen, so it feels like one creature with you, and it should travel: close to you by the newest message, and up next to its name when you scroll back through the conversation.

## Acceptance criteria

- [ ] In `WindConversation` there is exactly one `WindFace` of the wind on screen: the header (`VentsChatHeader`) does not draw its own face while the corner face exists, and the wind's bubbles have no per-message face (rotllana keeps them). A Compose test counting the faces in a wind chat and in a rotllana.
- [ ] At the newest message the face rests at the bottom corner (088, with 100's life and celebrations and 103's nod/shake); when the person scrolls up it flies to the header slot next to the name, shrinking smoothly (a shared-element-style animation driven by the list state, one composable moved with graphicsLayer), and returns when they scroll back to the bottom. A Compose test that the list state moves the face between the two anchors.
- [ ] With expressive motion off it jumps between the two places. Tapping it opens the wind's profile in both places.
- [ ] Emulator screenshots or Robolectric frames of the flight in `output/ui-review/109/`.

## Out of scope

- Web (108).
- Group chats.

## Notes

- Files: `CompanyScreen.kt` (`WindConversation`, corner face, `rememberChatList`), `VentsChatChrome.kt` (`VentsChatHeader`), `WindFace.kt`, `VentsSpectacle.kt`.
- User feedback on 2026-11-24 after 087/088: «has volgut posar el vent a un lateral, però no està bé, llavors surt duplicat i no fa l'animació xula de ser sempre només un, i que aquest llavors vagi a la part de dalt quan ens movem». Today a wind chat shows the wind in the header, in the corner (087/088) and (web) next to its messages.
