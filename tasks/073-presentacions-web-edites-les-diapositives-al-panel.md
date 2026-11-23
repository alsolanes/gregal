---
title: "Presentacions (web): edites les diapositives al panell"
status: done
depends_on: ["070", "072"]
---

## Why

A live presentation has to be seen as slides and edited in the app, next to the chat or from the Docs home, like a text document.

## Acceptance criteria

- [x] The panel (and its own window, `/app/doc/{id}`) shows a `deck` as 16:9 slides drawn from their fields in the brand colours, one under another, and applies `block` events in place. A node test that insert/replace/delete events change the right slide.
- [x] Clicking a slide opens a form for its layout (title, bullets or body, the two columns, image URL, notes); saving sends the slide with its `base_version` and a 409 keeps the person's fields with the notice, as in 059. «Diapositiva nova» adds one after the current one with a layout chosen from a list; a slide can be deleted. A node test of open, save, conflict, add and delete.
- [x] The Docs home (070) has «Presentació nova», and a deck's panel has «Descarrega en PowerPoint» using `format=pptx`. A node test.
- [x] Every string in Catalan and English; a screenshot of a three-slide deck in dark and light themes in the notes.

## Out of scope

- Editing chart data in the form (charts made by the agent are shown and exported, not edited here). Android (074).

## Notes

- Reuse the editing rules and the conflict handling of `internal/web/app/doc-view.js`; the slide preview only needs to be faithful enough to recognise the layout, the exported pptx is the reference.
- Desktop: any new module must be in `desktop/package.json` `build.files` (`release.test.js` checks it).
- Done, server: `POST /api/docs` takes `kind:"deck"` (`CreateDeck`); `POST /api/docs/{id}/blocks` takes `{after, slide}` and `PUT …/blocks/{block}` `{slide, base_version}` (`InsertSlide` / `ReplaceSlide`). `gdocs.ErrInvalid` marks kind mismatches (`sameKind`) and bad slides (`checkSlide`: unknown layout, chart that `CheckChart` refuses); `docWriteErr` answers 400 for it, so md sent to a deck or a sheet, and a slide sent to a doc, are 400 now (they were 500). The message is unchanged for the agent's tool. `docs/api-contract.md` updated.
- Done, web: new UMD `internal/web/app/deck-view.js` (`window.gregalDeckView.view(box, doc, render, edit)`, same shape as doc-view's), loaded by `index.html` and by the doc window page (`handleDocPage`). `docs-panel.js` picks it for `kind: deck`; `blockEdits` sends `md` for a string and `slide` for an object, and carries `tx`. The download link follows the kind (`format=pptx`, «Descarrega en PowerPoint» / "Download as PowerPoint"; anything else stays docx). No desktop file changed: the desktop window loads the same page from the server.
- Slides are drawn with the colours of ooxml's `default` pptx theme (title/closing slides on Primary, section on Dark, Accent rule under titles), not the app theme: a slide is white in both app themes, like PowerPoint. Sizes use container query units (`cqw`), so the slide scales with the panel. A project image path cannot load in the browser, so the image layout shows the path; only `http(s)` URLs show as images. A chart shows as its title in italics.
- Form: fields per layout follow `ooxml.buildSlide` (title/section/closing: title, subtitle; content: title, body, bullets; two_columns/comparison: the two headings and columns; big_number: number, caption; image: image URL, body; title_only: title; blank: nothing), plus notes always. Saving sends the whole slide: the stored slide with the form's fields on top, so fields the form does not show (chart, table) stay. Like 059: an untouched form saves nothing, a network error keeps the form, a 409 shows the notice and the slide as it is now above the person's fields, and the next Save overwrites against the version shown. Delete is in the form and gets the same 409 handling. «Diapositiva nova» posts `{layout}` after the last slide opened (or the last slide; at the start of an empty deck) and opens the new slide's form. A new presentation from the home opens empty with the «Diapositiva nova» bar.
- Review fix: the form's buttons are inside the slide, so a Cancel (or an untouched Save) click reached the slide after the form had closed and opened it again. The slide ignores clicks that start inside a form or a link (like doc-view's link check); tested in node and checked in headless Chrome. The Go test was split in two for gocyclo.
- Bullet lines keep their leading spaces on purpose: the pptx builder reads two spaces as one nesting level (`SlideSpec.bodyMD`); the preview shows them flat.
- `doc-view.js` still refuses to make an empty deck writable (`doc.kind !== 'deck'`), though the panel no longer hands decks to it; left as is, with its test.
- Tests: `internal/web/app/deck-view.test.cjs` (render, events, form per layout, save, click on a form button, 409, network error, add, empty deck, delete with 409), `docs-panel.test.cjs` (a deck uses the slide view, slide bodies with scope, pptx link and label), `doc-home.test.cjs` («Presentació nova», `newDoc` with `kind:'deck'`, deck-view.js loaded by index.html and the doc page), Go `TestNewDeckAndItsSlidesFromThePanel` (create, insert, replace, 409) and `TestDeckRefusesEditsOfAnotherKind` (400 for md in a deck and an unknown layout).
- Changed test: `doc-view.test.cjs` checked that docs-panel.js contained the literal `/export?format=docx`; the format now follows the kind, so it checks `/export?format=' + format` and the PowerPoint string. Word downloads stay covered by `docs-panel.test.cjs` (`format=docx&name=Carta.docx`).
- Screenshots (synthetic deck «Llançament 2027», three slides: title, content with bullets, two columns) taken in a sandboxed server (`%TEMP%/g073`, isolated HOME/data, fake provider) in the doc window `/app/doc/{id}?scope=mine`, with headless Chrome: `output/t073/dark.png`, `light.png`, and `dark-form.png`, `light-form.png` with the content slide's form open (`output/` is git-ignored, so they are not committed). Also checked live in the Browser pane: saving a title, «Diapositiva nova» with Xifra gran after the current slide, deleting a slide; the document on the server matched.
- `node --test desktop/*.test.js`: 74 pass, 1 fails (`verify-release.test.js`, `Cannot find module 'electron-updater'`, no `desktop/node_modules` in the worktree), as in 070; no desktop file changed.
- Not checked: the packaged desktop app; the panel next to a chat in the full app (only the doc window page, which loads the same `docs-panel.js`).
- Follow-up: the turn note (`notaDocs`, internal/agent/doc_tool.go) prints `blk.MD`, empty for slides; it should show the slide title.
- Follow-up: a sheet's panel still links «Descarrega en Word» with `format=docx` (400); 076 can add `sheet: ['xlsx', …]` to `DOWNLOADS` in docs-panel.js.
- Follow-up: a slide's layout cannot be changed from the form; delete and add one with another layout.
- Follow-up: clicking another slide while a form is open after a 409 drops the person's fields (same as 059's follow-up for text blocks).
- Follow-up: write errors in the block routes that are not kind or slide errors (a cell name like `1A`, a duplicate sheet name) still answer 500 (075's note).
