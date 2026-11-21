---
title: "Vents (Android): una portada d'inici en lloc del fil etern amb Gregal"
status: done
depends_on: []
---

## Why

Same as task 026, on the phone, where the user mostly lives: the Vents home is the endless thread with Gregal (B14) and the other winds are hidden in the drawer.

## Acceptance criteria

- [x] Opening Vents shows a start page: the greeting, the composer, a row with the faces of the winds and the recent conversations (`/api/winds/threads`), each with title, wind and date. A Compose test renders it with fake data.
- [x] Sending from the start page creates a new Gregal session (`windSessionNew`) and opens it with the message sent; the old Gregal conversation stays in the recent list. A ViewModel test with a fake server checks the session is created before the message is sent.
- [x] Tapping a face opens a new conversation with that wind; tapping a recent conversation opens it. Back from a conversation returns to the start page.
- [x] «New conversation» in the drawer goes to the start page.

## Out of scope

- Web: task 026.
- Servers without the Gregal wind keep the current home (the «Ventet»).

## Notes

- `android/app/src/main/java/xyz/solanes/gregal/CompanyScreen.kt`: `VentsHome` (line 119); the B14 branch (around line 320, `homeThread && gregalWind != null`) shows `GregalFloatingHeader` and Gregal's thread. `GregalViewModel.newWindSession` (line 5456) and `GregalApi` (`windSessionNew`, line 820) already exist; `/api/winds/threads` may need a client call.
- `Drawer.kt`: «Nova conversa» calls `vm.openVentsPanel(null)`.
- Verify with screenshots in dark and light, like `ModeSwitcherLookTest` and `CodeChatLookTest`.
- What was built: `VentsStart.kt` (`VentsStartPage` on top of the existing `VentsOpening`: faces, recent conversations, greeting, suggestions, composer), `GregalViewModel.startWindConversation` / `resumeWindThread` / `loadWindThreads`, `GregalApi.windThreads`. The conversation opens only once the new session is the selected one, so its history load cannot bring back the old thread (`WindStartConversationTest` mimics the screen's load and failed with the old order). While a wind is busy, a new conversation is not started: tapping its face opens the running one, and the composer keeps the text. Offline, the message goes to the usual offline queue of the current thread. If creating the session fails, the text goes back to the composer.
- The old Gregal-home mode of `WindConversation` (`home = true`: floating header, opening glow, greeting) is removed; the live computer strip (`GregalWorkStrip`) now shows in any conversation with Gregal. The face on the start page opens Gregal's profile (`WindProfileDialog`) and thinks while Gregal works.
- Drawer: «Nova conversa» goes to the start page even from inside a conversation; the «Gregal» row opens Gregal's current conversation.
- Not covered by tests: back from a conversation returning to the start page (checked by reading: `openVentsChat(null)`), the busy and offline paths. Not checked on a phone.
