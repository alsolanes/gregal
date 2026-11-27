---
title: Vents web: targetes per aprovar amb vista prèvia
status: done
depends_on: []
---

## Why

To delegate real work, the person approves what a wind is about to do with one tap, seeing exactly what it is: today ask/approve requests are plain text.

## Acceptance criteria

- [x] An `approve_request` in a wind chat renders as a card under the wind's perched face with its current step («Preparant l'enviament…»), a preview of the action (the email/message text, the page URL with a screenshot if the server sends one, the document title, the amount and payee if any) and two buttons, Permet / No; the decision goes through the existing answer route. A node test for each preview kind present in the payload.
- [x] After answering, the card collapses to one line («Has permès: …») and the wind's face reacts (nod / shake).
- [x] Strings in Catalan and English.

## Out of scope

- Android (103).
- New approval kinds on the server (only render what the payload already has; note gaps).

## Notes

- Files: `winds-workspace.js` (ask cards ~1029-1075, `loadAsks`), `/api/winds/pending` and `/api/winds/answer` in docs/api-contract.md.
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Web: `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server (never type into credential fields) with before/after screenshots in `output/ui-review/` (ignored).
- Done (web): the approval card (`paintAsks`, approve branch, in `winds-workspace.js`) shows the step, the question, a preview and Permet / No; answering goes through `answerAsk` → `/api/winds/answer` as before. Tests: `internal/web/app/winds-start.test.cjs` (the eight tests after «102:»). Screenshots before/after (en dark, ca light, mobile) in `output/ui-review/102/`, taken against a sandboxed server with `/api/winds/pending` simulated.
- What the payload carries (`windAskReq` in `internal/web/winds_ask.go`): `name` (tool), `kind`, `what`, `detail` (the first of command/path/file/dir/url/target/name/action in the args), `reason`. So the previews are: document (write/edit/patch + path), public site (`publish_public`, the address), page (an http(s) URL in `detail`) and command (the rest, the full `detail`). There is no separate "current step" field: the step line («Preparant el document…», «Al navegador…», «Preparant l'ordre…», «Preparant la publicació…») is derived from the preview kind.
- Gaps (out of scope here, server work): the payload has no screenshot, no email body/recipient (the `mail` tool is read-only anyway), no document title other than the file name, and no amount or payee. The card does not draw them.
- The verb buttons («Esborra'l», «Instal·la'l»…) became Permet / No as the criterion asks; `askPlan` no longer returns `yes`, and the old «Veure l'ordre» fold is gone (the command is the preview now, so `.vw-ask-cmd` CSS was removed).
- The one-line «Has permès: …» / «Has dit que no: …» is drawn in the thread just above the remaining cards once `/api/winds/answer` succeeds (`answerAsk` now returns whether it did); it goes away when the thread repaints with the wind's reply (it is not saved anywhere). The nod/shake is on the perched corner face, so it is not seen in Gregal's own thread or below 900 px, where there is no corner face; handoff, secret, connection and question cards are unchanged.
- Follow-up: `.vw-ask-plain` and `code.vw-ask-raw` in `winds-workspace.css` were already unused before this task.
- Follow-up: on a 390 px screen the wind chat header («T'espera» pill, Threads, Save to wiki) overflows; seen in the mobile screenshot, it was like that before.
- Follow-up: the comment on `Detail` in `internal/web/winds_ask.go` still mentions «Veure l'ordre», which the web card no longer has.
