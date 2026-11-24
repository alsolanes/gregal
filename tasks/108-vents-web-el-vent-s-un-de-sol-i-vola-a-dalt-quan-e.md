---
title: Vents web: el vent és un de sol i vola a dalt quan et mous pel xat
status: todo
depends_on: []
---

## Why

In a one-to-one wind chat the wind must be a single character on screen, so it feels like one creature with you, and it should travel: close to you by the newest message, and up next to its name when you scroll back through the conversation.

## Acceptance criteria

- [ ] In a one-to-one wind chat there is exactly one face of the wind on screen: the header no longer draws its own face and the wind's messages no longer have a face per row (group chats keep per-message faces to tell speakers apart). A node test counting faces in a wind chat and in a rotllana.
- [ ] At the newest message the face rests at the bottom corner (where 087 put it, keeping its moods, celebration and nod/shake); when the person scrolls up past a threshold it flies to the header slot next to the wind's name, shrinking smoothly (one element animated with transform, following the scroll or a spring), and flies back when they return to the bottom. A node test that scroll position switches the face between the two anchors.
- [ ] With reduced motion it jumps between the two places without the flight. On narrow screens (< 900 px, where the corner was hidden) the face lives in the header and still is only one.
- [ ] Screenshots/frames of the flight in dark and light in `output/ui-review/108/`.

## Out of scope

- Android (109).
- Group chats.

## Notes

- Files: `winds-workspace.js` (`owner` header face, `paintHistory` row faces, 087's corner `cornerMood`, 101's celebrations, 102's nod/shake), `winds-workspace.css`.
- User feedback on 2026-11-24 after 087/088: «has volgut posar el vent a un lateral, però no està bé, llavors surt duplicat i no fa l'animació xula de ser sempre només un, i que aquest llavors vagi a la part de dalt quan ens movem». Today a wind chat shows the wind in the header, in the corner (087/088) and (web) next to its messages.
