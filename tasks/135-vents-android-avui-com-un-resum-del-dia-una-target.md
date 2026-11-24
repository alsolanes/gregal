---
title: Vents Android: Avui com un resum del dia, una targeta per vent
status: todo
depends_on: ["124", "134"]
---

## Why

Same as 134 on Android.

## Acceptance criteria

- [ ] Avui leads with one swipeable card per wind that did or proposes something today, skipping idle winds. A Compose test.
- [ ] Weather and match cards get the same small illustrations as the web. Robolectric frames or emulator screenshots (light, dark) in `output/ui-review/135/`.

## Out of scope

- Web (134).

## Notes

- Files: Android Avui screen (task 107), the comparison pager on Android.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
