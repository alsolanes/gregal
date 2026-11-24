---
title: Avui Android: el temps, el calendari i el que segueixes a dalt de tot
status: todo
depends_on: ["105"]
---

## Why

Today should open with what matters at a glance: today's weather where the person is, the next match of the teams they follow (or the next thing of anything else they follow) and today's calendar, without asking anyone.

## Acceptance criteria

- [ ] Android's Today screen (`VentsAvui.kt`) opens with the same glance: weather card, next events, one card per follow, with the same data and wording as the web. A Compose test per card from a sample `glance`.
- [ ] Add/remove follows and set the place from the screen. A unit test with a fake server.
- [ ] It feels alive on the phone: cards spring in, the weather icon animates gently, match countdowns tick; all off with expressive motion off.
- [ ] Strings in Catalan and English.

## Out of scope

- Web (106).
- A home-screen widget (possible follow-up).

## Notes

- Files: `VentsAvui.kt`, `GregalViewModel.kt` (avui load), `GregalApi.kt`.
- Asked by the user on 2026-11-24: «a Avui hem de tenir la previsió del temps, next matches dels equips que seguim i si seguim alguna altra cosa també allà, o coses del calendari». It should feel alive and fun on the phone (user's direction for Vents), not a dashboard of grey boxes.
- Android: gradle tests with `ANDROID_HOME` set; AppStrings/Strings by hand; no `(?U)`; Robolectric view models point at a fake server or `Conn("127.0.0.1", "9", "t")`.
- BLOCKED (2026-11-24): everything is built and the `*Avui*` gradle tests pass (AvuiGlanceTest, AvuiGlanceUiTest), but the diff guard rejects the size: 725 changed non-test lines, limit 400. The new `VentsAvuiGlance.kt` alone is about 590 lines. A possible split: (a) data, view model, API, wiring, strings and plain cards (weather, calendar, follows, inline editors); (b) "alive": spring-in, the animated Canvas sky (`GlanceSky`), the ticking countdown and the live pulse.
- Shape follows the merged contract of 105: `sunrise`/`sunset` "HH:MM"; all-day events are midnight→midnight RFC 3339; `follow` POST without `wind` (the server defaults to Gregal).
- The place and follow editors are inline cards, not dialogs: an `OutlinedTextField` in an `AlertDialog` never got idle under Robolectric (AppNotIdleException).
- When the glance has no weather, the place invite shows and the old weather-wind card (if any) stays below it.
- Frames for review: `output/ui-review/107/` (dark with motion at 16/140/400/1500 ms, light, empty light), written by AvuiGlanceUiTest.
- Not verified: on a real phone or emulator, against the real 105 server, or the copy against the web (106 was not merged when this was written).
