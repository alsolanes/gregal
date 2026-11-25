---
title: Vents Android: xat més professional, la resposta creix sense salts i tipografia polida
status: todo
depends_on: ["133", "141"]
---

## Why

Same as 144 on Android: make the wind chat feel professional; check whether the reply bubble is rebuilt or resized in jumps while streaming and swapped when the final reply arrives, and fix it.

## Acceptance criteria

- [ ] Streaming is stable: the pending reply is one item with a stable key that grows in place (no item replaced, no layout jump of items above, face animation not restarted), and the final reply keeps that key so the list does not swap it. A Compose test that the pending and final item share the key and that earlier items keep their positions.
- [ ] The same calmer message style as the web (Notes): wind replies as flowing text with a small face and name at the start of a run, the person's messages as a subtle bubble, one reading width and type scale, consistent paragraph/list/code spacing, colour only on face, name and accents; light and dark. Robolectric frames or emulator screenshots in `output/ui-review/145/`.
- [ ] The list stays pinned to the end while streaming if the person was there, and does not jump when they scrolled up. A test.

## Out of scope

- Web (144).

## Notes

- Files: CompanyScreen.kt (message list, pending bubble), VentsChatChrome.kt, markdown rendering on Android.
- Tasks 133 and 141 touch CompanyScreen.kt; start after they are merged.
- Reference look (Claude, ChatGPT, Dots): the assistant's reply is plain flowing text on the page (no heavy bubble), with a small face and name at the start of a run; the person's message is a subtle rounded bubble aligned right; reading width about 68-72ch; one text scale (15-16 px body, line-height about 1.6, headings and lists with calm spacing); colour used sparingly (the wind's colour on the face, name and accents, not on the whole bubble); streaming text grows in place with no reflow above it.
- Person, 2026-11-25: «explora què podem fer perquè els xats es vegin una mica millor, més pro. No sé si és alguna cosa de fluïdesa, com per exemple quan genera el text que sembla que va recreant la bombolla del missatge i repintant-la perquè es veu com fa salts, o no sé si és el tema de les bombolles, el color, la tipografia... però no acaba de ser professional i no sé massa el perquè».
