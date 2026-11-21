---
title: "Desat (web): el vent proposa desar el que val la pena"
status: todo
depends_on: [029]
---

## Why

The person forgets to save. When Gregal or a wind gives something they will want again (a recipe, a tip to reuse, a plan to follow, a dashboard), it should offer to keep it, and saving stays the person's choice.

## Acceptance criteria

- [ ] The winds' prompt (`CardsGuide`, also used by Gregal and the group chat) tells them to end such a reply with ```` ```gregal-keep {"title","kind"} ```` and only then (not for chat, news or one-off answers). A Go test checks the guide has the marker and the kinds.
- [ ] md.js renders the marker as a «Desar «títol»?» / «Save «title»?» chip under the reply; the chip saves the reply without the marker, with that title and kind (same request as task 029's action), and becomes «Desat ✓». A node test covers render and request; a bad JSON marker shows nothing.
- [ ] The share text (`share-msg.js` `plainText`) and the saved body never contain the marker.
- [ ] The prompt also tells winds that saved things are notes with a kind, so «la recepta de llenties que vam desar» is a `notes` search.

## Out of scope

- Android (task 032).
- Saving on its own without the person's tap.

## Notes

- `internal/companion/winds.go` `CardsGuide`; `internal/web/app/md.js` card switch (~line 397); `Share.kt` mirrors `plainText` on Android and must strip the marker too in task 032.
