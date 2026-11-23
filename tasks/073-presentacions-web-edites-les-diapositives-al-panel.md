---
title: "Presentacions (web): edites les diapositives al panell"
status: todo
depends_on: ["070", "072"]
---

## Why

A live presentation has to be seen as slides and edited in the app, next to the chat or from the Docs home, like a text document.

## Acceptance criteria

- [ ] The panel (and its own window, `/app/doc/{id}`) shows a `deck` as 16:9 slides drawn from their fields in the brand colours, one under another, and applies `block` events in place. A node test that insert/replace/delete events change the right slide.
- [ ] Clicking a slide opens a form for its layout (title, bullets or body, the two columns, image URL, notes); saving sends the slide with its `base_version` and a 409 keeps the person's fields with the notice, as in 059. «Diapositiva nova» adds one after the current one with a layout chosen from a list; a slide can be deleted. A node test of open, save, conflict, add and delete.
- [ ] The Docs home (070) has «Presentació nova», and a deck's panel has «Descarrega en PowerPoint» using `format=pptx`. A node test.
- [ ] Every string in Catalan and English; a screenshot of a three-slide deck in dark and light themes in the notes.

## Out of scope

- Editing chart data in the form (charts made by the agent are shown and exported, not edited here). Android (074).

## Notes

- Reuse the editing rules and the conflict handling of `internal/web/app/doc-view.js`; the slide preview only needs to be faithful enough to recognise the layout, the exported pptx is the reference.
- Desktop: any new module must be in `desktop/package.json` `build.files` (`release.test.js` checks it).
