---
title: Vents Android: Avui com un resum del dia, una targeta per vent
status: done
depends_on: ["124", "134"]
---

## Why

Same as 134 on Android.

## Acceptance criteria

- [x] Avui leads with one swipeable card per wind that did or proposes something today, skipping idle winds. A Compose test.
- [x] Weather and match cards get the same small illustrations as the web. Robolectric frames or emulator screenshots (light, dark) in `output/ui-review/135/`.

## Out of scope

- Web (134).

## Notes

- Files: Android Avui screen (task 107), the comparison pager on Android.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
- Built before the web (134): the Android rules are the reference. One page per wind (`VentsAvuiBrief.kt`, `AvuiBriefLogic.briefs`): «Ha fet» is its finished work (card source `feina:*` or `job`, open or done today), «Proposa» the first open card that is not that one; the button is the first action of the proposal, else of the finished work. Winds with neither are skipped. The pager reuses `HorizontalPager` and the comparison pager's dots, now `PagerDots` in `CompareCards.kt`.
- Art: the weather wind (`weatherWind`) gets the animated sky of the glance (`GlanceSky`, now internal) when `glance.weather` is there; a wind that follows a team gets `MatchArt` (pitch and bouncing ball in its colour), which also replaces the football icon on the team follow cards. Drawn in Compose, no image assets.
- Data: everything comes from GET /api/winds/avui; novetats were not needed. Missing field: the cards carry no explicit «did vs proposes» kind, so it is inferred from `source`; a `kind` field would let web and Android agree without that rule.
- AvuiTrayUiTest «fetTreuLaTargetaIElsBotonsDecideixen» clicked «Explica'm el resultat» by text; the brief now shows the same button, so the test targets the one inside `avui-card-c2` (same assertion).
- Frames (Robolectric, light and dark, one per page): `output/ui-review/135/avui-{garbi,oratge,xaloc}-{dark,light}.png`, written by `AvuiBriefUiTest.pintaElResumEnFoscIEnClar` (output/ is git-ignored). No emulator screenshots.
- Follow-up: the web (134) should use the same did/proposes rule so both clients show the same brief.
