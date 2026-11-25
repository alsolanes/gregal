---
title: Vents Android: xat més professional, la resposta creix sense salts i tipografia polida
status: blocked
depends_on: ["133", "141"]
---

## Why

Same as 144 on Android: make the wind chat feel professional; check whether the reply bubble is rebuilt or resized in jumps while streaming and swapped when the final reply arrives, and fix it.

## Acceptance criteria

- [x] Streaming is stable: the pending reply is one item with a stable key that grows in place (no item replaced, no layout jump of items above, face animation not restarted), and the final reply keeps that key so the list does not swap it. A Compose test that the pending and final item share the key and that earlier items keep their positions.
- [x] The same calmer message style as the web (Notes): wind replies as flowing text with a small face and name at the start of a run, the person's messages as a subtle bubble, one reading width and type scale, consistent paragraph/list/code spacing, colour only on face, name and accents; light and dark. Robolectric frames or emulator screenshots in `output/ui-review/145/`.
- [ ] The list stays pinned to the end while streaming if the person was there, and does not jump when they scrolled up. A test.

## Out of scope

- Web (144).

## Notes

- Files: CompanyScreen.kt (message list, pending bubble), VentsChatChrome.kt, markdown rendering on Android.
- Tasks 133 and 141 touch CompanyScreen.kt; start after they are merged.
- Reference look (Claude, ChatGPT, Dots): the assistant's reply is plain flowing text on the page (no heavy bubble), with a small face and name at the start of a run; the person's message is a subtle rounded bubble aligned right; reading width about 68-72ch; one text scale (15-16 px body, line-height about 1.6, headings and lists with calm spacing); colour used sparingly (the wind's colour on the face, name and accents, not on the whole bubble); streaming text grows in place with no reflow above it.
- Person, 2026-11-25: «explora què podem fer perquè els xats es vegin una mica millor, més pro. No sé si és alguna cosa de fluïdesa, com per exemple quan genera el text que sembla que va recreant la bombolla del missatge i repintant-la perquè es veu com fa salts, o no sé si és el tema de les bombolles, el color, la tipografia... però no acaba de ser professional i no sé massa el perquè».
- Found (2026-11-25): the jump was real. The «escrivint» dots were a separate item (`key = "typing"`) removed when the first token arrived, and the reply appeared as a new item; besides, the first frame of the paced text is empty, so the row was lower than the dots and a thread at its end dropped ~20 dp. `WindReplyInPlaceTest` reproduces it on the old code (no row holds both the dots and the text; the sent message moved 20 px).
- Done: `withPendingReply` puts the waiting reply into the message rows (key = index), so dots, live text and final reply are one row and one node; while `streaming` the row only grows (`heightIn(min = tallest)`). Style: replies have no tinted surface (the `tint` parameter is gone), max `GregalChat.readingMax` (560 dp, about 70 letters), `chatBody` 16/26 sp; `WindByline` at the start of each run (name in `windNameColor`, 40 % of the wind colour into the ink, AA on the chat wash in every theme), with a 24 dp face in groups; your message is an ink veil (6 % light, 10 % dark; `UserBubble` removed); paragraphs are split into blocks (`mdParagraphs`) so paragraphs, lists, code and tables share one 12 dp gap; code blocks 13/20 sp on the same veil. Frames before/after, light and dark, 1:1 and group, in `output/ui-review/145/` (gitignored, from `WindChatLookTest`).
- Blocked, criterion 3: Android deliberately does the opposite of «pinned to the end» after you send. `ChatFollowState` (ChatGPT style) lifts your sent message to the top and lets the reply grow below it without moving, even past the screen, with the «↓» pill; `ChatPinTest` («mentre la resposta creix la llista no es mou encara que passi de la pantalla») asserts it. Following the end means changing that design and that test, which is the person's call. What holds and is tested (`WindReplyInPlaceTest` «si has pujat a llegir…»): a growing reply does not move you when you scrolled up. In a thread you did not send in, the list does not follow the end either (unchanged).
- Test changed because the requirement changed: `WindFlightTest.aRotllanaKeepsTheFaceInItsHeader` asserted exactly one face of the wind in a group chat; criterion 2 adds the small face beside the name at each run, so it now asserts one face in the header plus one per byline.
- Follow-up: `windBubbleColor` no longer tints anything in the app (only `WindChatColorStateTest` uses it); remove both in their own task.
- Follow-up: a `doc_changed` during a reply inserts the card before the live row, which shifts the live row to the next index (key) and remounts it; keying rows by message identity would avoid it.
- Follow-up: group chats (`RotllanaConversation`) still show their waiting dots as a separate centred item, without the grow-only row; the same `withPendingReply` treatment would apply there.
