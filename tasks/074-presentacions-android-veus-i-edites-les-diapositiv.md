---
title: "Presentacions (Android): veus i edites les diapositives"
status: done
depends_on: ["071", "072"]
---

## Why

Parity with the web: a live presentation opens on the phone as slides, updates live and can be edited slide by slide.

## Acceptance criteria

- [x] The document screen shows a `deck` as 16:9 slide cards drawn from their fields and applies `block` events in place. A unit test of applying events to a deck.
- [x] Tapping a slide edits its fields for its layout; saving sends `base_version`, a 409 keeps the person's fields with the notice; a slide can be added after the current one (layout from a list) or deleted. A unit test of the request each action builds and of the conflict state.
- [x] The Docs home has «Presentació nova», and a deck has «Descarrega en PowerPoint» (`format=pptx`), saved like other downloads. A unit test of the export link.
- [x] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Editing chart data.

## Notes

- `Docs.kt`, `DocScreen.kt`, `LiveDocTest.kt`; keep the deck state pure and testable like `LiveDoc`.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
- Done: the state is pure, in `Docs.kt`. `DocBlock.slide` is a `JsonObject`, so fields the form does not show (chart, table) survive a save untouched, and `LiveDoc.apply` already applies deck events. `slideLayouts` and `slideFields(layout)` follow deck-view.js (`FIELDS`, notes always); `slideText` / `slideBullets` read a slide for the card.
- On a deck, `LiveDoc.open(id)` opens the slide form: `DocEdit.slide` is the slide as opened and `fields` the form, and `LiveDoc.current` remembers the last slide opened. `saveRequest()` sends `PUT …/blocks/{id}` `{slide, base_version}` with the whole slide: the stored one (after a 409, the agent's current one) with the form on top; empty fields are dropped, bullet lines keep their leading spaces (nesting in the pptx). Untouched, nothing is sent. `deleteRequest()` sends `DELETE …?base_version=N`. `saved()` reuses the text-block rules: a 409 keeps the fields and sets `conflict`; a 200 without `block` (the DELETE answer) removes the slide.
- «Diapositiva nova»: a layout picked from a dropdown (default «Contingut») and the button, below the slides. `slideAfter` is the last slide opened if it still exists, otherwise the last one (the start of an empty deck). `addSlideRequest(after, layout)` posts `{after, slide:{layout}}`; `slideAdded()` inserts the answer and opens its form. Like the web, an open form is closed when the new slide arrives.
- UI (`DocScreen.kt`): «Descarrega en PowerPoint», then one 16:9 card per slide in the colours of the pptx default theme (as docs-panel.js: white slides, title/closing on #1F4E79, section on #1F2A44, accent rule under titles), «Encara no hi ha cap diapositiva.» when empty, the form in place of the tapped slide (layout name, one field per layout field, Desa / Cancel·la / Esborra la diapositiva; after a 409 «L’agent ha canviat aquesta diapositiva» and the current slide above). Body text is plain, not rendered markdown; an image path is shown as text; a chart as its title in italics.
- «Presentació nova» is next to «Full de càlcul nou» on the home screen; an empty title becomes «Presentació sense títol». `docExportPath` builds `format=pptx` for a deck. The `docs.deckLater` notice and its strings are gone.
- Tested with `LiveDocTest` (6 new tests) and the whole Android unit suite (1004 tests, 0 failures). Not tried on a device or emulator with a running server: the slide cards, the form and the dropdown have not been seen running.
- Follow-up: while a slide form is open, tapping another slide does nothing (as with text blocks); the web closes the form and opens the other one. Cancel is the way out.
- Follow-up: if the person opens a slide and types while a «Diapositiva nova» request is in flight, those fields are dropped when the new slide's form opens.
- Follow-up: a failed delete says «No s’ha pogut desar la diapositiva», not a delete-specific message.
- Follow-up: an `http(s)` image URL is shown as text; the web shows it as an image.
- Follow-up: a slide's layout cannot be changed from the form (same as the web, 073).
