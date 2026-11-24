---
title: Vents Android: una fila de cares viva a l'inici
status: todo
depends_on: ["124", "132"]
---

## Why

Same as 132 on Android (Android has no pati: faces row only).

## Acceptance criteria

- [ ] The home faces row scrolls with snap and a fade, shows every wind, faces breathe out of sync and show a busy dot; tapping a face animates it into the chat header (shared element). Motion respects reduced motion (web: prefers-reduced-motion; Android: LocalWindMotion). A Compose test. Robolectric frames or emulator screenshots (light, dark) in `output/ui-review/133/`.

## Out of scope

- Web (132).
- A pati on Android.

## Notes

- Files: Vents home screen, WindFace.kt.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
