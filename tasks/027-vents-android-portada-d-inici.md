---
title: "Vents (Android): una portada d'inici en lloc del fil etern amb Gregal"
status: todo
depends_on: []
---

## Why

Same as task 026, on the phone, where the user mostly lives: the Vents home is the endless thread with Gregal (B14) and the other winds are hidden in the drawer.

## Acceptance criteria

- [ ] Opening Vents shows a start page: the greeting, the composer, a row with the faces of the winds and the recent conversations (`/api/winds/threads`), each with title, wind and date. A Compose test renders it with fake data.
- [ ] Sending from the start page creates a new Gregal session (`windSessionNew`) and opens it with the message sent; the old Gregal conversation stays in the recent list. A ViewModel test with a fake server checks the session is created before the message is sent.
- [ ] Tapping a face opens a new conversation with that wind; tapping a recent conversation opens it. Back from a conversation returns to the start page.
- [ ] «New conversation» in the drawer goes to the start page.

## Out of scope

- Web: task 026.
- Servers without the Gregal wind keep the current home (the «Ventet»).

## Notes

- `android/app/src/main/java/xyz/solanes/gregal/CompanyScreen.kt`: `VentsHome` (line 119); the B14 branch (around line 320, `homeThread && gregalWind != null`) shows `GregalFloatingHeader` and Gregal's thread. `GregalViewModel.newWindSession` (line 5456) and `GregalApi` (`windSessionNew`, line 820) already exist; `/api/winds/threads` may need a client call.
- `Drawer.kt`: «Nova conversa» calls `vm.openVentsPanel(null)`.
- Verify with screenshots in dark and light, like `ModeSwitcherLookTest` and `CodeChatLookTest`.
