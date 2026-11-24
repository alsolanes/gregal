---
title: Vents web: coherència (llistes, ordinador/navegador, salutació, Desat, accent, Avui buit)
status: todo
depends_on: ["125", "126"]
---

## Why

Small things that make Vents feel unfinished or contradictory.

## Acceptance criteria

- [ ] Numbered lists in wind messages are readable: the number does not cover the text (`winds-workspace.css` ~1551 overrides the padding set ~1319). A node/CSS test.
- [ ] Ordinador and navegador are named consistently: the button and the panel it opens say the same thing, and the sidebar «Ordinador» opens it without cutting the home faces row. Codi and Vents greet with the same words at the same hour (one helper). A node test for each.
- [ ] Desat has one title (no tab + H1 + H2 repeating), Feina's tabs sit under its title with one text scale; the primary button keeps one brand accent in light and dark; «Ajusta el de demà» looks the same in both themes; «Tots els vents» leads with the roster and the tidy-up becomes a small chip.
- [ ] Avui's empty cards look intentional (no grey smudge: `.vw-glance-card::before` uses `--accent`), no orphan «Segueix…», and on mobile the cards show a scroll cue. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/127/`.

## Out of scope

- Android (its own review in 124).
- Bigger redesigns (128-137).

## Notes

- Files: winds-workspace.js/.css, vents-avui-glance.js, i18n.js (ca+en), index.html (no hard-coded hex: identitat test).
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
