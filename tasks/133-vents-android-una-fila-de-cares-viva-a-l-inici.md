---
title: Vents Android: una fila de cares viva a l'inici
status: done
depends_on: ["124", "132"]
---

## Why

Same as 132 on Android (Android has no pati: faces row only).

## Acceptance criteria

- [x] The home faces row scrolls with snap and a fade, shows every wind, faces breathe out of sync and show a busy dot; tapping a face animates it into the chat header (shared element). Motion respects reduced motion (web: prefers-reduced-motion; Android: LocalWindMotion). A Compose test. Robolectric frames or emulator screenshots (light, dark) in `output/ui-review/133/`.

## Out of scope

- Web (132).
- A pati on Android.

## Notes

- Files: Vents home screen, WindFace.kt.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
- From 124's audit: each tap on a wind on the Android home opens a new empty session; tapping should resume the wind's latest thread (or the wind's main chat). Fix with a test.

### Done (2026-11-25)

- `VentsStart.kt`: the faces row is a `LazyRow` with `rememberSnapFlingBehavior(…, SnapPosition.Start)` (the default snaps to the centre), a mask fade on the edge that still has faces, every wind (reachable by scrolling), a 3.6 s breath per face offset by index (`startFaceBreath`) and a dot in the wind's colour plus «Treballant…» as state description for winds in `vm.windBusy`. Breath and flight go through `windMotionEnabled()` (LocalWindMotion + system animator scale).
- Shared element: home and chat are an if/else in `CompanyScreen`, not one `AnimatedContent`, so a Compose `SharedTransitionLayout` would have meant re-indenting ~300 lines. Instead the tapped face's bounds go to `WindConversation(landFrom)`, and `FaceLanding` flies a copy from there to the header face slot (shrinking 48 → 38 dp). If the thread has messages, the corner face (109) waits hidden in the header and then springs down to its corner as usual; with an empty thread the header face appears where the copy landed.
- Tapping a wind on the home now calls `vm.openWindFromHome`: its newest thread from `/api/winds/threads`, or its usual chat if it has none; no `POST /api/winds/sessions`.
- Tests: `VentsStartFacesTest` (every wind, snap, busy dot, tap bounds, breath out of sync, flight, no flight without motion) and `WindStartConversationTest` › `tocar un vent a la portada repren el seu ultim fil, no en crea un de nou`. Robolectric frames (light, dark, scrolled, mid-flight) in `output/ui-review/133/` (git-ignored). The flight was not checked on the emulator.
- Note: Robolectric tests that scroll a `LazyRow` hang until OOM with `mainClock.autoAdvance = false` (waitForIdle never settles); these tests turn auto-advance on before scrolling.
