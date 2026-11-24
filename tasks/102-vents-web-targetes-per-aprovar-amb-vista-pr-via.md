---
title: Vents web: targetes per aprovar amb vista prèvia
status: todo
depends_on: []
---

## Why

To delegate real work, the person approves what a wind is about to do with one tap, seeing exactly what it is: today ask/approve requests are plain text.

## Acceptance criteria

- [ ] An `approve_request` in a wind chat renders as a card under the wind's perched face with its current step («Preparant l'enviament…»), a preview of the action (the email/message text, the page URL with a screenshot if the server sends one, the document title, the amount and payee if any) and two buttons, Permet / No; the decision goes through the existing answer route. A node test for each preview kind present in the payload.
- [ ] After answering, the card collapses to one line («Has permès: …») and the wind's face reacts (nod / shake).
- [ ] Strings in Catalan and English.

## Out of scope

- Android (103).
- New approval kinds on the server (only render what the payload already has; note gaps).

## Notes

- Files: `winds-workspace.js` (ask cards ~1029-1075, `loadAsks`), `/api/winds/pending` and `/api/winds/answer` in docs/api-contract.md.
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Web: `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server (never type into credential fields) with before/after screenshots in `output/ui-review/` (ignored).
