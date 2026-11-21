---
title: "Android: un sol selector Codi | Vents, al calaix"
status: done
depends_on: []
---

## Why

In v1.89.0 (Vents 1.95.0) the Android app can switch between Codi and Vents from five places: the selector at the top of the drawer and four icons inside the screens (the Vents home header, the Vents top bar, the Codi top bar and the wind chat header). The user finds the in-screen icons strange, and the one in the Vents home header pushes the wind's face off centre. The drawer selector itself leaves both options crammed on the left of a full-width pill.

## Acceptance criteria

- [x] Codi and Vents are switched only from the drawer selector: no screen shows a mode-switch icon (the composable, its parameter and its two strings are gone, so the compiler enforces it).
- [x] On the Vents home header the face is centred: one button on each side (menu and computer). Checked in a screenshot only: a test would have to render the whole home screen.
- [x] The drawer selector is two equal halves that fill its width, each with its icon and label centred, and the selected one in semibold (`ModeSwitcherLookTest`, plus a screenshot in `build/mode-look/`).

## Out of scope

- The glass style (the next step of the plan) and the Codi chat look.
- Web and desktop: they have no in-screen switch icons of this kind.

## Notes

- Removed: `ModeSwitchIcon` (`ModeSwitcher.kt`), the `onModeSwitch` parameter of `VentsChatHeader`, `CenteredChatHeader` and `GregalFloatingHeader` (`VentsChatChrome.kt`), its callers in `CompanyScreen.kt` and `Screens.kt`, and the strings `app.mode.toVents` / `app.mode.toCode` in both languages.
- Side effect on the Vents home header of a server without the gregal wind (`CenteredChatHeader`, `CompanyScreen.kt:332`): the switch icon took the right slot and hid the computer button; it is back. Other wind chats never had the icon.
- Checked: `ModeSwitcherLookTest` (equal halves and centred content, both failing before the change), the whole Android unit suite, and the screenshot. Not checked on a phone.
