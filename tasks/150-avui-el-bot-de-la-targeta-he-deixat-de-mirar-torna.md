---
title: Avui: el botó de la targeta «He deixat de mirar» torna a engegar la vigilància (web i Android)
status: todo
depends_on: ["146"]
---

## Why

Task 146 pauses a watch after 5 dismissals and shows a card («He deixat de mirar: …») whose action resumes it through `POST /api/winds/avui` with `action:"resume"`. The clients render card actions as a prompt to the wind's chat, so pressing it does not resume anything.

## Acceptance criteria

- [ ] Web/desktop: a card with source `pausa` shows a «Torna-hi» / «Resume» button that calls `action:"resume"` for that card, then the card leaves and a short confirmation shows; an error shows a message. A node test.
- [ ] Android: the same in Avui. A test against a fake server.

## Out of scope

- Other new card actions.

## Notes

- Files: vents-avui-glance.js / winds-workspace.js Avui cards, Android VentsAvui.kt; contract row of /api/winds/avui in docs/api-contract.md (task 146).
