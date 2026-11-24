---
title: Vents web: un sol capçal per a tots els vents i el vent reacciona amb el cos
status: todo
depends_on: ["125", "126", "127"]
---

## Why

Gregal's header differs from the other winds', and the wind does not react to what happens: errors land as red text under the composer.

## Acceptance criteria

- [ ] Every wind (Gregal included) has the same header: big face with its mood, name, a role chip in the wind's colour and one «⋯» menu (Perfil, Fils, Desa a la wiki, Ordinador); tapping the face opens the profile. A node test that Gregal and another wind render the same structure.
- [ ] The wind reacts with its body: a nod when you send, a lean-in with a small particle burst in its colour when a result arrives, a shake on error (reuse `vw-corner-nod`, `vw-corner-shake`, `.vw-burst`). Motion respects reduced motion (web: prefers-reduced-motion; Android: LocalWindMotion). A node test for which reaction each event triggers.
- [ ] A failed turn shows as a bubble in the thread spoken by the wind («No he pogut arribar-hi · Torna-ho a provar», with a retry) instead of red text under the composer; ca+en. A node test.
- [ ] The corner wind no longer gets cut at the right edge or covers the last message. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/128/`.

## Out of scope

- Android (129).

## Notes

- Files: `.vw-top` and streamChat in winds-workspace.js, winds-workspace.css, i18n.js.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
