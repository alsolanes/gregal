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
