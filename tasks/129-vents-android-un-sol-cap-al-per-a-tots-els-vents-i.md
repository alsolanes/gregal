---
title: Vents Android: un sol capçal per a tots els vents i el vent reacciona amb el cos
status: todo
depends_on: ["124", "128"]
---

## Why

Same as 128 on Android: one header for every wind, and the wind reacts with its body.

## Acceptance criteria

- [ ] Every wind (Gregal included) has the same chat header: big face with mood, name, role chip in its colour, one overflow menu (Perfil, Fils, Desa a la wiki, Ordinador); tapping the face opens the profile. A Compose test.
- [ ] The wind nods on send, leans in with a small burst in its colour when a result arrives and shakes on error. Motion respects reduced motion (web: prefers-reduced-motion; Android: LocalWindMotion). A unit test of the event to reaction mapping.
- [ ] A failed turn shows as a wind bubble in the thread with a retry, ca+en. A Compose test. Robolectric frames or emulator screenshots (light, dark) in `output/ui-review/129/`.

## Out of scope

- Web (128).

## Notes

- Files: CompanyScreen.kt, VentsChatChrome.kt, WindFace.kt; strings in AppStrings.kt by hand.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
- From 124's audit: the doc editor toolbar stays Catalan when the app is in English (doc-page.js + Android bridge); fix here or note.
