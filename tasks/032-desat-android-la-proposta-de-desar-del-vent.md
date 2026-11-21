---
title: "Desat (Android): la proposta de desar del vent"
status: done
depends_on: [030, 031]
---

## Why

Task 031 on the phone: the wind's ```` ```gregal-keep ```` marker has to become a tap to save, not a block of JSON.

## Acceptance criteria

- [x] `richBlocks` parses ```` ```gregal-keep {"title","kind"} ```` into its own block; the chat draws it as the «Desar «títol»?» chip, and a bad marker draws nothing. A unit test covers the parsing.
- [x] Tapping the chip saves the reply without the marker, with that title and kind (same call as task 030), and it becomes «Desat ✓». A ViewModel test with a fake server checks the body.
- [x] `Share.kt` drops the marker from shared text. A unit test checks it.
- [x] Strings in Catalan and English.

## Out of scope

- Prompt changes (task 031).

## Notes

- `Md.kt` (`Rich`, `richBlocks`, ~line 381), `RichCards.kt`, `Share.kt`, chat drawing in `CompanyScreen.kt` ~2096.
- `Rich.Keep(title, kind)` surt de `withKeep` (Md.kt): la marca es treu abans de `splitMd` i, si s'entén, va al final dels blocs; `withoutKeep` és la regla compartida amb `ShareText.plain` i `saveReply`. La marca s'endú les línies en blanc del davant, perquè el cos desat sigui la resposta tal com era (sense `trim`, a diferència del web).
- `saveReply(text, title = "", kind = "auto")` recorda el que s'ha desat a la sessió (`savedReplies`, per cos sense marca): «Desa» del full i el xip diuen «Desat ✓» tots dos, com el `saved` de share-msg.js. Un segon toc al xip desat obre el desat.
- El xip només surt on `WindBubble` rep `vm` (converses de vent i rotllanes a CompanyScreen.kt); a la resta de llocs (VentetChat, Desat, wiki) `Rich.Keep` no pinta res.
- Follow-up: VentetChat.kt i SharedChatScreens.kt no passen `vm` a `WindBubble`, així que no hi surt el xip.
