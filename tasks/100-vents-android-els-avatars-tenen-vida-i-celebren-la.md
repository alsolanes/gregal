---
title: Vents Android: els avatars tenen vida i celebren la feina feta
status: done
depends_on: []
---

## Why

The user wants Vents to be fun on the phone: the avatars should feel alive and the UI should move with intent.

## Acceptance criteria

- [x] Idle avatars blink and glance now and then (randomised, a few seconds apart), and react when tapped (squash and spring back with a small sound-free wiggle). A Compose test with the clock advanced that frames differ over time with motion on and stay identical with motion off.
- [x] When a turn ends well, the wind celebrates once: a short confetti/sparkle burst in its colour around the corner avatar and the header face; on a failed turn, a small sad shrug instead. A test that the burst runs on done and not on error.
- [x] New messages spring in (scale + fade, ~200 ms), the typing bubble breathes, and the work card slides up. All respect expressive motion off and the system animator scale.
- [x] No jank: the animations are drawn in Canvas/graphicsLayer without recomposing the list each frame (note how it was checked).

## Out of scope

- Web (101).
- New avatar art.

## Notes

- Files: `WindFace.kt`, `WindVolume.kt`, `VentsSpectacle.kt` (WindHalo), `CompanyScreen.kt` (WindConversation, corner face), `VentsHomeParts.kt`.
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Android: gradle tests with `ANDROID_HOME` set; Strings.kt by hand; no `(?U)` regexes; Robolectric view models point at a fake server or `Conn("127.0.0.1", "9", "t")`, never 127.0.0.1:8097.
- Done (Android): Cares faces blink on a seeded schedule (`windBlinkTimes`, every 2-4.5 s) and glance twice per 12 s loop (`windGlances`), different per wind; a tap now adds a short wiggle to the existing squash/spring. `windTurnCue` (VentsSpectacle.kt) is applied to every WindFace pack: confetti/sparkles in the wind's colour on a change into DONE, a shrug on a change into FAILED, nothing for a face that is shown already done, nothing with expressive motion or the system animator scale off. WindConversation now gives the corner face (now tappable) and the header face FAILED when the turn ends with an error, DONE when it ends well.
- Messages: BubbleIn is fade 200 ms + scale spring from the speaker's corner, gated on `windMotionEnabled()` (it used LocalWindMotion only, so the system animator scale did not stop it). The typing bubble breathes (graphicsLayer scale); WindWorkCard slides up with a spring (graphicsLayer translation/alpha).
- No jank, how it was checked: every per-frame value (blink, glance, wiggle, burst, shrug, breath, rise) is read inside `graphicsLayer {}` or a draw lambda, never in composition, so a frame only redraws the layer; AnimatedVisibility with fade+scale also animates in its layer. Checked by reading the code paths, not with a profiler or on a device.
- Tests: `WindVidaTest` renders frames in Robolectric with the clock advanced. Review frames were rendered to `output/ui-review/100/` (not committed).
- Not verified on a device or emulator; the header face is 38 dp inside a clipped pill, so its burst is cut to the pill.
- Follow-up: blinking/glancing is in the Cares pack only; Esperit, Cap, Rosa and Mapa get the turn cue but not the new blink schedule.
- Follow-up: list-sized avatars (< 64 dp, not tappable) stay still when idle, as before; making every list avatar blink would mean one running animation per row.
- Also touched outside the listed files: `GregalWorkStrip.kt` (WindWorkCard slide-up). In CompanyScreen.kt only BubbleIn, the corner face and the header mood argument of WindConversation changed (098/103 edit the same file).
