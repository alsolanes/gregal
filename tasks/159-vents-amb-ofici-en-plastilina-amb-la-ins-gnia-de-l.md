---
title: Vents amb ofici en plastilina, amb la insígnia de l'ofici
status: done
depends_on: []
---

## Why

A wind with a trade (Tramuntana, «webs i codi») kept the code-drawn face wherever the others are clay, so on the home strip one wind is clay and the next a drawing. The user chose (2026-11-26): clay for every wind, the trade shown as a small round badge in the figure's bottom corner, on the web and on Android.

## Acceptance criteria

- [x] Web: a large face of a wind that wears a trade kit swaps to its clay sheet like any other wind, and every frame drawn carries a round badge with the trade's icon in the bottom-right corner; a wind without a trade has no badge. `characters.test.cjs` «a wind with a trade plays its clay sheet with the trade badge over every frame»
- [x] Web: the trade chip («<> webs i codi») and the clay badge use the same icon paths, defined once (`gregalWinds.kitIcon` in winds.js). `winds-start.test.cjs` «the pill shows the kit’s icon…» checks the chip draws that path.
- [x] The large figure of an empty one-wind chat is clay too, on the web and on Android (both still drew it, for every wind). `winds-start.test.cjs` «an empty chat with one wind shows that wind large in clay», `VentsExperienceTest.anEmptyWindChatShowsTheWindInClay`.
- [x] Android: a large clay face of a wind with a trade kit is its clay sheet (no Canvas fallback) and draws the trade badge; without its sheets it keeps the Canvas figure with its kit, as before. `WindClayTest.aWindWithATradeIsClayWithItsBadge`, `withoutItsSheetsAWindWithATradeKeepsItsCanvasFigure`.

## Out of scope

The clay sheets themselves, the trade's full drawing (headphones, cap, bag…) on clay, the small list faces (no kit), the minairó and the pelut (no kit).

## Notes

- Web: `internal/web/app/characters.js` (play, draw), `winds.js` (icon paths), `winds-workspace.js` (`kitIcon` uses them). Android: `WindFace.kt` (the `wearsKit` rule), `WindClay.kt` (`WindClayFace`), the icons are `windKitIcon` in `VentsHomeParts.kt`.
- Requirement change: the web test «kits and the other avatar packs stay code-drawn» described the old rule; its kit half changes to the badge.
- Checked in the browser (home strip, chat header, empty chat) and on emulator-5554 (empty chat). At 40 px on the web home strip the badge is small; the trade chip under it names the trade anyway.
- Android's chat header face (small, Canvas for every wind) is unchanged.
