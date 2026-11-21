---
title: "Desat (web): el vent proposa desar el que val la pena"
status: done
depends_on: [029]
---

## Why

The person forgets to save. When Gregal or a wind gives something they will want again (a recipe, a tip to reuse, a plan to follow, a dashboard), it should offer to keep it, and saving stays the person's choice.

## Acceptance criteria

- [x] The winds' prompt (`CardsGuide`, also used by Gregal and the group chat) tells them to end such a reply with ```` ```gregal-keep {"title","kind"} ```` and only then (not for chat, news or one-off answers). A Go test checks the guide has the marker and the kinds.
- [x] md.js renders the marker as a «Desar «títol»?» / «Save «title»?» chip under the reply; the chip saves the reply without the marker, with that title and kind (same request as task 029's action), and becomes «Desat ✓». A node test covers render and request; a bad JSON marker shows nothing.
- [x] The share text (`share-msg.js` `plainText`) and the saved body never contain the marker.
- [x] The prompt also tells winds that saved things are notes with a kind, so «la recepta de llenties que vam desar» is a `notes` search.

## Out of scope

- Android (task 032).
- Saving on its own without the person's tap.

## Notes

- `internal/companion/winds.go` `CardsGuide`; `internal/web/app/md.js` card switch (~line 397); `Share.kt` mirrors `plainText` on Android and must strip the marker too in task 032.
- The chip is `.gc-keep` inside `.gc-keep-row`, appended after the rendered reply by md.js; it is only visible under `.gc-sayable` (like the card actions) and `gregalShare.keepChips(body, getText, openNote)` wires it in winds-workspace.js. An unknown kind becomes `auto`; a marker without a title or with bad JSON is dropped.
- The «Desa» button and the chip share `saveReply`, keyed by the text without the marker, so saving with one marks the other as saved on the next repaint (not immediately).
- Follow-up: debate voices (`debateRun` in winds-workspace.js) render with md.js inside `.gc-sayable` but have no save buttons, so a marker there would show a chip that does nothing.
- Follow-up: the public clip (`/api/winds/clip`) and "send to someone" still carry the raw marker in the text; the clip page hides the chip (no `.gc-sayable`) but the stored text keeps it.
- Follow-up: the `notes` tool search does not match `kind`; the prompt points winds to a plain notes search by title/text.
