---
title: Vents web: Feina amb targetes que s'obren al lloc i una guia més lleugera
status: done
depends_on: ["125", "126", "127"]
---

## Why

Feina's running items are plain rows, «El que han fet els vents» overlaps Feina › Fet, and the guide is slow and oversized.

## Acceptance criteria

- [x] Running items in Feina are cards with the owner's face that expand in place to show steps and links (animated; Motion respects reduced motion (web: prefers-reduced-motion; Android: LocalWindMotion).); «El que han fet els vents» merges into Fet with no duplicate entry point. A node test.
- [x] The «Què pot fer» guide is a sheet with 3 cards and «Més», cards staggered about 40 ms (about 250 ms total). A node test for the 3-card default. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/136/`.

## Out of scope

- Android Feina (note gaps).

## Notes

- Files: winds-workspace.js (Feina, guide `.vw-dialog.vw-guide`), winds-workspace.css.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
- Done: Feina › Ara draws each running task (and the live turn) as a `.vw-feina-card` (`workCard()` in winds-workspace.js): the owner's face (first agent of the task; for the live turn the turn's wind, else Gregal), the title, «En marxa» and a chevron. The head is a button with `aria-expanded`; the body opens in place (grid rows 0fr→1fr, .22 s, only under `prefers-reduced-motion: no-preference`) and is `inert` while closed. It shows the last 5 steps (task events through `logLine`, or the turn's `workSteps`) and links: up to 4 files the task saved (opened with `gregalWindRunner.openResult`) and «Obre la feina» / «Open the task» (the live turn keeps «Mira-ho ›»). The card is tinted with the owner's colour (`--vw-card`). Tests in winds-start.test.cjs.
- Done: the gallery of what the winds made (`renderSaved`) moved from Desat to Feina › Fet, under the done tasks; Desat no longer has «El que han fet els vents» (its `.vw-saved-made` CSS is gone). The gallery's empty line now reads «Quan un vent et faci un document, una web o un gràfic, també sortirà aquí.» so it fits under Fet.
- Done: the guide (vents-guide.js, where `.vw-dialog.vw-guide` is built) opens with the first 3 topics and «Més» / «More», which adds the rest in place (stagger restarts, focus moves to the first new card). Cards animate .17 s with 40 ms steps (3 cards ≈ 250 ms), the sheet rises in .2 s and the bloom lasts .25 s (was 2.1 s), all under no-preference only. Wide screens: an 880 px dialog with the 3 cards in one row; ≤600 px: a bottom sheet. Tests in vents-guide.test.cjs.
- Screenshots (not committed): `output/ui-review/136/` before/after `feina`, `feina-open`, `fet`, `desat`, `guide`, `guide-more`, light/dark, 1280/390; sandbox on port 18930 with tasks and saved items mocked.
- Android gap: Feina on Android still has plain running rows, and its guide (GregalGuide.kt) still shows every topic at once; where Android shows what the winds made was not checked.
- Follow-up: the Activitat tab («El que els vents han fet mentre no hi eres») still partly overlaps Fet (finished work shows in both); merging it is its own decision.
- Follow-up: in one sandbox run before the change, the saved gallery chips showed Catalan («Feina feta», «avui») on an English page; not seen after, not investigated.
- Follow-up: vents-guide.js's header comment still says the guide opens from the ✦ header button; since 128 it is the «⋯» menu.
