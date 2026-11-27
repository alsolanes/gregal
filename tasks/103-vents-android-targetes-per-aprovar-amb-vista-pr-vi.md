---
title: Vents Android: targetes per aprovar amb vista prèvia
status: done
depends_on: []
---

## Why

To delegate real work, the person approves what a wind is about to do with one tap, seeing exactly what it is: today ask/approve requests are plain text.

## Acceptance criteria

- [x] In `WindConversation`, an approval ask renders as a card with the wind's perched face and step, a preview of the action from the payload (text, URL/screenshot, document, amount/payee) and Permet / No buttons, through the existing answer call. A Compose test per preview kind.
- [x] After answering, the card collapses to one line and the face nods/shakes.
- [x] An approval that arrives while the app is in the background shows a notification with Permet / No actions that answer without opening the app (if the existing notification code supports actions; otherwise note the gap).
- [x] Strings in Catalan and English.

## Out of scope

- Web (102).

## Notes

- Files: `WindAsks.kt` (81, 93), `WindNotifications`, `GregalViewModel.kt` (`windAsks`).
- The user's direction (2026-11-24): «ha de ser divertit, hi ha d'haver animacions dels avatars i de la UI, han de ser força proactius… ha de ser una cosa xula de tenir al mòbil». Inspiration (spirit only, our own designs): Grok Bot's avatar motion per state, Muse's approval cards, Dots' activity/outputs panel.
- Android: gradle tests with `ANDROID_HOME` set; Strings.kt by hand; no `(?U)` regexes; Robolectric view models point at a fake server or `Conn("127.0.0.1", "9", "t")`, never 127.0.0.1:8097.
- Done (2026-11-24): `ApproveCard` in `WindAsks.kt` now has the wind's face perched on its top edge (the ask's `wind`, else the conversation's wind, passed from `WindConversation`), the step (`windProgressLabel` of the ask's tool + «Espera la teva resposta»), a preview from `askPreview` and Permet / No. Answered approvals stay as one line («Has permès: …» / «Has dit que no: …») with a small face that bobs (yes) or sways (no) for 0.9 s when wind motion is on. Tests: `WindAsksTest` (url, doc, text, collapse/nod/shake).
- Preview kinds come only from what `/api/winds/pending` sends (`what`, `detail`, `name`): **url** (first http(s) URL in `detail`/`what`: host + URL), **doc** (`write|edit|patch <path>`: file + folder), **text** (any other non-command `detail`/`what`). Commands (bash, install, download, delete, system) keep the folded «Veure l'ordre».
- Gap: the payload has no screenshot, no message/email body (only `windAskArgsHint`/`windAskArgsFull`, which pick command/path/url/name/action) and no amount/payee, so those previews are not rendered. Follow-up: the server should add structured preview fields (e.g. `preview:{text,url,shot,doc,amount,payee}`) to `windAskReq` and the contract; then web (102) and Android can show them.
- The buttons are now Permet / No as the criterion asks, so `ApprovePlan.yes` (the verb, «Esborra'l») went away and `ApprovePlanTest` no longer asserts it (it asserts the publish_public question instead).
- Gap (criterion 3): the existing notification code does not support actions. A background approval reaches the phone as a novetat (`source:"torn"`, FCM `{k:"novetat",id}` → `WindNotifications.veu`), which carries no decision key, and neither `GregalNotifications` nor `WindNotifications` builds notification actions or has a receiver for them. Follow-up: carry the decision key in the «torn» novetat, add Permet / No actions with a receiver that calls `/api/winds/answer` with the stored session.
- Follow-up: `FeinesScreen` calls `WindAskCards` without a wind; the face shows there only when the ask carries `wind` and that wind is in `vm.winds`.
