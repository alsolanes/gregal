---
title: "Desat (Android): la proposta de desar del vent"
status: todo
depends_on: [030, 031]
---

## Why

Task 031 on the phone: the wind's ```` ```gregal-keep ```` marker has to become a tap to save, not a block of JSON.

## Acceptance criteria

- [ ] `richBlocks` parses ```` ```gregal-keep {"title","kind"} ```` into its own block; the chat draws it as the «Desar «títol»?» chip, and a bad marker draws nothing. A unit test covers the parsing.
- [ ] Tapping the chip saves the reply without the marker, with that title and kind (same call as task 030), and it becomes «Desat ✓». A ViewModel test with a fake server checks the body.
- [ ] `Share.kt` drops the marker from shared text. A unit test checks it.
- [ ] Strings in Catalan and English.

## Out of scope

- Prompt changes (task 031).

## Notes

- `Md.kt` (`Rich`, `richBlocks`, ~line 381), `RichCards.kt`, `Share.kt`, chat drawing in `CompanyScreen.kt` ~2096.
