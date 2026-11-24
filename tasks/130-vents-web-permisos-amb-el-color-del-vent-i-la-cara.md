---
title: Vents web: permisos amb el color del vent i la cara a cada missatge
status: todo
depends_on: ["125", "126", "128"]
---

## Why

Permission cards and messages do not carry the wind's identity.

## Acceptance criteria

- [ ] Approval cards (`.vw-ask`) show the wind's face, an edge or tint in `--vw-wind`, a one-line reason, and the Allow button in the wind's colour with text contrast checked (as windShare does). A node test for the contrast pick.
- [ ] Wind messages show a small face (20 px) beside the bubble, only on the first of a run; one bubble rule per speaker, not by content length. A node test. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/130/`.

## Out of scope

- Android (131).

## Notes

- Files: winds-workspace.js/.css (`.vw-ask`, `.vw-message`), wind-face helpers in winds.js.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
