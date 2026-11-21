---
title: "Android: totes les pantalles amb la mateixa capçalera"
status: done
depends_on: ["034", "035", "036", "037"]
---

## Why

Most screens use `GregalTopBar` (transparent, `GregalType.title`, menu or back), but Webs, Notes, Automations, Saved, Conversations and Retro draw their own `TopAppBar` with other sizes (18 sp, 20 sp) and colours, so the app looks stitched together when moving through the drawer. In a wind conversation the text scrolls up behind the floating header and shows between the round buttons («Sources …» bleeding at the top).

## Acceptance criteria

- [x] Every screen reachable from the drawers uses `GregalTopBar` (or a documented exception), with the same title style; a test lists the screens and checks the title node uses the shared style or tag.
- [x] In a wind conversation the messages fade out under the floating header instead of showing behind it, in solid and glass themes; a screenshot shows it.
- [x] Screenshots of the changed screens, dark and light.

## Notes

- `GregalUi.kt` `GregalTopBar` (line ~74). Own top bars: `Webs.kt`, `Notes.kt` (4), `AutomatitzacionsScreen.kt`, `RetroScreen.kt`, `Screens.kt` ~674, `VentsActivity.kt`, `CompanyScreen.kt`. Starts after 034–037 are merged, which touch the same screens.
- Moved to `GregalTopBar`: Webs (top bar only, rows untouched for 036), Notes (list, agile note, open note, editor), Automations, Saved (`DesatScreen` in `VentsActivity.kt`), Retro (title «Retro» with the channel title as subtitle, instead of «#retro»), and the vents list header in `CompanyScreen.kt`. Their Scaffolds go from `Superficie` to transparent, so they sit on `GregalBg` like Work, Folder, Activity and the rest.
- The title carries the test tag `GregalTopBarTitle`; `TopBarsTest` walks every drawer screen and every Notes page and checks it.
- Exceptions: the Code chat header (`Screens.kt` ~674: the title is the model picker, centred, with frost) and the floating headers of wind and group conversations. They are conversations, not list screens.
- The back buttons of the open note, the note editor and Retro now say «Back» like every `GregalTopBar` (they said «Back to notes», «Cancel» and «Close»). `notes.back` was left unused and is removed in en and ca.
- Fade: `Modifier.fadeUnderChatHeader()` (`VentsChatChrome.kt`) masks the thread list with `BlendMode.DstIn`: empty down to the bottom of the buttons, back to full by `ChatHeaderInset`. A mask rather than a colour gradient because the glass background is not a flat colour. Applied to the wind and group (rotllana) conversation lists.
- Screenshots: `android/app/build/top-bars-look/` (`TopBarsLookTest`) and `android/app/build/chat-fade-look/` (`ChatHeaderFadeLookTest`, solid and glass).
- Follow-up: the vents list in `CompanyScreen.kt` keeps its `Superficie` Scaffold; on the transparent backdrop the «Mòbil» chip and the Inici/Avui/Xats segment almost vanish in light theme, so those controls need a stronger fill before it can match the other screens.
- Follow-up: with the shared 20 sp title (was 18 sp), «Automatitzacions» is cut at 360 dp next to its three actions.
- Follow-up: the Automations error card shows a raw «%1$s» («No s'ha pogut carregar l'activitat: %1$s»).
- Follow-up: `SharedChatScreens.kt` and `VentetChat.kt` use the same floating header and could take `fadeUnderChatHeader()` too.
