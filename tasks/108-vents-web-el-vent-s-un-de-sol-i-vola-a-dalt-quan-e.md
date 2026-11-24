---
title: Vents web: el vent és un de sol i vola a dalt quan et mous pel xat
status: done
depends_on: []
---

## Why

In a one-to-one wind chat the wind must be a single character on screen, so it feels like one creature with you, and it should travel: close to you by the newest message, and up next to its name when you scroll back through the conversation.

## Acceptance criteria

- [x] In a one-to-one wind chat there is exactly one face of the wind on screen: the header no longer draws its own face and the wind's messages no longer have a face per row (group chats keep per-message faces to tell speakers apart). A node test counting faces in a wind chat and in a rotllana.
- [x] At the newest message the face rests at the bottom corner (where 087 put it, keeping its moods, celebration and nod/shake); when the person scrolls up past a threshold it flies to the header slot next to the wind's name, shrinking smoothly (one element animated with transform, following the scroll or a spring), and flies back when they return to the bottom. A node test that scroll position switches the face between the two anchors.
- [x] With reduced motion it jumps between the two places without the flight. On narrow screens (< 900 px, where the corner was hidden) the face lives in the header and still is only one.
- [x] Screenshots/frames of the flight in dark and light in `output/ui-review/108/`.

## Out of scope

- Android (109).
- Group chats.

## Notes

- Files: `winds-workspace.js` (`owner` header face, `paintHistory` row faces, 087's corner `cornerMood`, 101's celebrations, 102's nod/shake), `winds-workspace.css`.
- User feedback on 2026-11-24 after 087/088: «has volgut posar el vent a un lateral, però no està bé, llavors surt duplicat i no fa l'animació xula de ser sempre només un, i que aquest llavors vagi a la part de dalt quan ens movem». Today a wind chat shows the wind in the header, in the corner (087/088) and (web) next to its messages.
- Done (web). The one element is `.vw-wind-corner`, now inside the header (`owner()`, in `.vw-owner-slot` next to the name). It is drawn at 96 px and shrinks to 44 px at the top. `.vw-corner-spot` is an empty, zero-height row between the thread and the composer that marks the corner. `flyCorner()` measures the spot against the slot on scroll and through a ResizeObserver on the thread, then sets `data-at="corner|top"` and `--vw-fly-x/y`. The CSS hides the spot below 900 px and when a work pane is open below 1200 px. The spot then has no size, so the wind stays at the top.
- The flight sets `translate` (x), `transform: translateY` (y) and `scale` on the container. The face keeps its own `transform` for moods, the 101 celebration and the 102 nod/shake. x and y run on different timings, so the wind travels in an arc. Going up it rises and then slides over; coming down it slides and then lands with a small spring (`linear()`). CSS transitions start a new flight from wherever the wind is when you turn back mid-flight. The transition only exists under `prefers-reduced-motion: no-preference`, and only once the first position has been set (`data-fly`), so opening a chat never plays a flight.
- It flies up once you are more than 160 px from the bottom.
- Gregal's own thread: its face stays large in its header and the rows lose their faces too. The pending row in a one-to-one chat has no face either.
- Requirement change in tests: the 101 test "corner wind and header face celebrate" now expects one cheer and one burst, because those two faces are now the same element.
- Frames: `output/ui-review/108/` (gitignored): `{dark,light}-1280-1-up-*`, `-3-down-*`, `-4-interrupt-*`, `-trail-{up,down}` (frames blended to show the arc), `-820-*` (narrow), `dark-reduced-1280-*`, `dark-1280-gregal-thread`. Taken against a sandboxed server with `/api/winds*` intercepted for a 36-message conversation.
- Follow-up: in a one-to-one chat the rows no longer have a face column, but `.vw-ask-done` (58 px), `.vw-choices` (46 px) and `.vw-handoff`/`.vw-suggest` still have left margins sized for one; they may look slightly indented next to the bubbles.
- Follow-up: Android (109).
