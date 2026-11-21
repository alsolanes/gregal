---
title: "Android: la cara del vent ja no tapa les llistes"
status: done
depends_on: []
---

## Why

The floating wind face (`WindFab`) sits bottom-left on about ten screens (Webs, Notes, Desat, Automatitzacions, Work, Backlog, Jobs, Sessions…) and covers list content and buttons: in the Webs screenshot it hides the «Open» of a card. It only takes you to Vents, which the drawer's Codi | Vents switch already does. Its one piece of information, that a wind is working or has news, is not shown anywhere else.

## Acceptance criteria

- [x] No screen shows the floating wind face any more; `WindFab` and its placement in `MainActivity` are removed.
- [x] The «Vents» half of the drawer's Codi | Vents switch shows a small dot while a wind is busy or has news (`vm.windBusy`, `vm.windNews`), with a content description that says so, in English and Catalan. A Compose test checks the dot shows with news and not without.
- [x] Screenshots of the drawer, dark and light, with and without the dot.

## Out of scope

- Headers of the screens (task 038).

## Notes

- `MainActivity.kt` around line 394; `CompanyScreen.kt` `WindFab` (line ~2193); `ModeSwitcher.kt`, `ModeSwitcherLookTest.kt`.
- Built: `WindFab` (CompanyScreen.kt) and its placement in `MainActivity` are gone. `ModeSwitcher` takes a `ventsDot: Boolean`; both drawers (`GregalDrawerContent` and `SimpleVentsDrawer` in Drawer.kt) pass `windBusy.isNotEmpty() || windNews.isNotEmpty()`. The dot is 7 dp, `Sea`, after the «Vents» label, with content description `app.mode.ventsAttention` («Un {vent} treballa o té novetats» / «A {vent} is working or has news», in AppStrings.kt).
- Tests: `ModeSwitcherLookTest` checks the dot appears next to «Vents» when set and is absent otherwise, and writes `android/app/build/mode-look/drawer-{dark,light}[-dot].png`. `ModeSwitcherTest` only got the new argument.
- Not tested: the wiring from `vm.windBusy` / `vm.windNews` to the drawer (no test drives the whole drawer with a VM), and the screen reader reading the description in a real device. The busy/news state inside the face (thinking/news mood) is no longer shown outside Vents; only the dot remains.
- The dot shows the same for busy and news; it does not distinguish which wind.
