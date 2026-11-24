---
title: Vents Android: el vent és un de sol i vola a dalt quan et mous pel xat
status: done
depends_on: []
---

## Why

In a one-to-one wind chat the wind must be a single character on screen, so it feels like one creature with you, and it should travel: close to you by the newest message, and up next to its name when you scroll back through the conversation.

## Acceptance criteria

- [x] In `WindConversation` there is exactly one `WindFace` of the wind on screen: the header (`VentsChatHeader`) does not draw its own face while the corner face exists, and the wind's bubbles have no per-message face (rotllana keeps them). A Compose test counting the faces in a wind chat and in a rotllana.
- [x] At the newest message the face rests at the bottom corner (088, with 100's life and celebrations and 103's nod/shake); when the person scrolls up it flies to the header slot next to the name, shrinking smoothly (a shared-element-style animation driven by the list state, one composable moved with graphicsLayer), and returns when they scroll back to the bottom. A Compose test that the list state moves the face between the two anchors.
- [x] With expressive motion off it jumps between the two places. Tapping it opens the wind's profile in both places.
- [x] Emulator screenshots or Robolectric frames of the flight in `output/ui-review/109/`.

## Out of scope

- Web (108).
- Group chats.

## Notes

- Files: `CompanyScreen.kt` (`WindConversation`, corner face, `rememberChatList`), `VentsChatChrome.kt` (`VentsChatHeader`), `WindFace.kt`, `VentsSpectacle.kt`.
- User feedback on 2026-11-24 after 087/088: «has volgut posar el vent a un lateral, però no està bé, llavors surt duplicat i no fa l'animació xula de ser sempre només un, i que aquest llavors vagi a la part de dalt quan ens movem». Today a wind chat shows the wind in the header, in the corner (087/088) and (web) next to its messages.
- Done (2026-11-24): the corner face is one `WindPresence` (`VentsChatChrome.kt`) composed last in `WindConversation`'s root box and moved only in `graphicsLayer` between two anchors measured with `onGloballyPositioned` (`WindPerch`): the invisible `wind-presence-corner` box in the list's bottom corner and the empty 44 dp `chat-header-face` slot that `VentsChatHeader` draws instead of its halo when it gets `faceSlot`. The target is `follow.behind` (`canScrollForward`, the same signal as «Últims»); progress is `animateFloatAsState` with a spring (damping .72, StiffnessMediumLow), so it turns around mid-flight, and `snap()` when `windMotionEnabled()` is false. The flight curves out 28 dp, tilts up to 10° and shrinks the face from 72 to 38 dp (the header halo's face size).
- One face only: no header face while the thread has messages; `WindTypingBubble` lost its 30 dp face and its `mood` parameter (this chat was its only caller; the chat face carries the mood; `WindVidaTest` call updated). `WindAskCards(onOwnAnswer = …)` draws no face for the chat's own wind on approve cards and answered lines, and answering makes the chat face nod (yes) or shake (no) for 0.9 s (103), only with motion. Asks from other winds keep their faces. Rotllana is unchanged (on Android its bubbles never had faces; the header keeps its stacked halo).
- `WindFace` gained `onClick` (tap = 100's cheer + profile; also for the esperit/cap/rosa/mapa packs) and a `WindFaceKey` semantics value (the accent) so tests can count a wind's faces.
- Tests: `WindFlightTest` (count in a wind chat with typing + approval and in a rotllana, anchors follow the list state, flight vs jump, tap in both places, nod/shake). Robolectric frames of the flight, 40 ms apart with motion on: `output/ui-review/109/flight-up-*.png` (output/ is gitignored, not committed).
- Not verified: on a device or emulator. On opening a long chat the face may start a short flight for a frame or two while `rememberChatList` scrolls to the end.
- Follow-up: with an empty thread the header halo, the welcome face and an approve card's face can still show together (the criterion only covers the corner-face case).
- Follow-up: `ChatPositions` is a process-wide object, so a Compose test that scrolls a chat leaks its position into later test classes (`WindFlightTest` clears it before and after each test).
