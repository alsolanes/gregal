---
title: "Presentacions (Android): veus i edites les diapositives"
status: todo
depends_on: ["071", "072"]
---

## Why

Parity with the web: a live presentation opens on the phone as slides, updates live and can be edited slide by slide.

## Acceptance criteria

- [ ] The document screen shows a `deck` as 16:9 slide cards drawn from their fields and applies `block` events in place. A unit test of applying events to a deck.
- [ ] Tapping a slide edits its fields for its layout; saving sends `base_version`, a 409 keeps the person's fields with the notice; a slide can be added after the current one (layout from a list) or deleted. A unit test of the request each action builds and of the conflict state.
- [ ] The Docs home has «Presentació nova», and a deck has «Descarrega en PowerPoint» (`format=pptx`), saved like other downloads. A unit test of the export link.
- [ ] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Editing chart data.

## Notes

- `Docs.kt`, `DocScreen.kt`, `LiveDocTest.kt`; keep the deck state pure and testable like `LiveDoc`.
- Run gradle tests with `ANDROID_HOME` set; do not run `regen-strings.py`.
