---
title: Vents Android: permisos amb el color del vent i la cara a cada missatge
status: todo
depends_on: ["129", "130"]
---

## Why

Same as 130 on Android.

## Acceptance criteria

- [ ] Approval cards show the wind's face, an edge in its colour, a one-line reason and the Allow button in the wind's colour with checked contrast. A test.
- [ ] Wind messages show a small face beside the first bubble of a run; one bubble rule per speaker. A Compose test. Robolectric frames or emulator screenshots (light, dark) in `output/ui-review/131/`.

## Out of scope

- Web (130).

## Notes

- Files: WindAsks.kt, CompanyScreen.kt and the chat bubbles.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
