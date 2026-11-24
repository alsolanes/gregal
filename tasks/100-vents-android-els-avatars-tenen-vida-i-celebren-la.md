---
title: Vents Android: els avatars tenen vida i celebren la feina feta
status: todo
depends_on: []
---

## Why

The user wants Vents to be fun on the phone: the avatars should feel alive and the UI should move with intent.

## Acceptance criteria

- [ ] Idle avatars blink and glance now and then (randomised, a few seconds apart), and react when tapped (squash and spring back with a small sound-free wiggle). A Compose test with the clock advanced that frames differ over time with motion on and stay identical with motion off.
- [ ] When a turn ends well, the wind celebrates once: a short confetti/sparkle burst in its colour around the corner avatar and the header face; on a failed turn, a small sad shrug instead. A test that the burst runs on done and not on error.
- [ ] New messages spring in (scale + fade, ~200 ms), the typing bubble breathes, and the work card slides up. All respect expressive motion off and the system animator scale.
- [ ] No jank: the animations are drawn in Canvas/graphicsLayer without recomposing the list each frame (note how it was checked).

## Out of scope

- Web (101).
- New avatar art.

## Notes

- Files: `WindFace.kt`, `WindVolume.kt`, `VentsSpectacle.kt` (WindHalo), `CompanyScreen.kt` (WindConversation, corner face), `VentsHomeParts.kt`.
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Android: gradle tests with `ANDROID_HOME` set; Strings.kt by hand; no `(?U)` regexes; Robolectric view models point at a fake server or `Conn("127.0.0.1", "9", "t")`, never 127.0.0.1:8097.
