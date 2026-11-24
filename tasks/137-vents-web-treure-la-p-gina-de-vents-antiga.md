---
title: Vents web: treure la pàgina de vents antiga
status: todo
depends_on: ["125", "126", "127", "128", "130", "132", "134", "136"]
---

## Why

The legacy wind page (`#ventsPage .winds-page`, the «Fem lloc…» desk) is hidden with `display:none!important` but its markup and code still ship and run.

## Acceptance criteria

- [ ] The legacy markup is removed from index.html and the winds.js code used only by it is deleted; everything still used by winds-workspace.js (faces, packs, editor) stays. All node and Go tests pass; a test that `.winds-page` is gone.
- [ ] The app loads with no console errors on every Vents page in light and dark (Playwright run noted in the task).

## Out of scope

- Any visual change.

## Notes

- Files: index.html, winds.js, winds-workspace.css:8.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
