---
title: "Docs (Android): edites un bloc des del mòbil"
status: todo
depends_on: ["059", "062"]
---

## Why

The person should be able to fix a sentence or add a paragraph from the phone while the agent keeps writing, the same as on the web.

## Acceptance criteria

- [ ] Tapping a block in the document screen edits its markdown in place; «Desa» / «Save» sends `PUT /api/docs/{id}/blocks/{block}` with the block's version, «Afegeix un bloc» / «Add a block» inserts below, and saving an empty block deletes it. A unit test of the request each action builds.
- [ ] A 409 shows the current text and «L'agent ha canviat aquest bloc» / «The agent changed this block», keeping the person's text. A unit test of the conflict state.
- [ ] While a block is being edited, `block` events for that block do not replace the editor; others still apply. A unit test.
- [ ] Every string in Catalan and English in `Strings.kt`, edited by hand.

## Out of scope

- Rich formatting toolbar.

## Notes

- Same editing rules as the web panel in 059; reuse its strings where the wording is the same.
