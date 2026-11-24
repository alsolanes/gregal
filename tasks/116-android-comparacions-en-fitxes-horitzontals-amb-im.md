---
title: Android: comparacions en fitxes horitzontals amb imatge
status: done
depends_on: ["115"]
---

## Why

When someone asks to compare things (products, places, options, tools, libraries…), a row of cards with a picture or logo for each one reads far better than a table, in Codi and in Vents alike.

## Acceptance criteria

- [x] `CompareCards.kt` renders `gregal-compare` as a `LazyRow` with snap and a peek of the next card: image or logo on top (the resolved `image`; `logo` from the server; initial as fallback), name, aligned facts, pick badge, verdict below; 2–8 items. A Compose test with 2 and 6 items.
- [x] Works in Code-mode chats as well as wind chats (same Markdown renderer). A test.
- [x] Cards spring in and the row scrolls smoothly; motion off keeps it static.

- [x] The election example works end to end: items with `points` and two images render as a swipeable one-card-per-page carousel with page dots; a test with three parties.

## Out of scope

- Web and server (115).

## Notes

- Existing: ```gregal-compare {title, items:[{name, image_query, facts:{Trait:value}, pick}], verdict}``` taught in `internal/companion/winds.go` `CardsGuide` (winds and Gregal only, «2-3 things side by side»); images resolved in `internal/web/winds_card_images.go` (Commons, English names) only for wind replies; rendered by `internal/web/app/md.js` `compareCard` (grid) and Android `CompareCards.kt` (Row).
- Asked by the user on 2026-11-24: «quan algun usuari demana (al mode codi o vents) una comparació, que si té sentit el llm pugui decidir fer fitxes scrollable horitzontals amb la seva imatge o logo o el que sigui».
- Android: gradle tests with ANDROID_HOME; no `(?U)`.
- User example (2026-11-24): «un resum dels programes electorals dels partits que es presenten a les eleccions, una fitxa per partit amb una foto o dues, fent swipe right poder veure el següent i així (o swipe left)». So an item can also carry `images` (up to 2: e.g. the party logo via `logo_domain` and the candidate's photo via `image_query`) and `points` (3–6 short summary bullets), and when items carry `points` the block renders as a full-width paged carousel: one card per page, swipe left/right (and arrow keys/buttons on desktop), page dots, the verdict/sources after. Short fact comparisons keep the compact snapping row. Neutral wording for political content: summarise each programme from its own sources (`url` per item), no pick on parties.
- Done (Android): `CompareItem` now has `photos` (`image` + `images`, http(s) only, max 2), `logo`, `url` and `points` (max 6); up to 8 items. Without `points`: `LazyRow` with snap, card width 72% (two items share the width), facts aligned by measuring each value at the card width (`minLines`), pick badge on the picture. With `points`: `HorizontalPager`, up to two pictures (photos, then the logo, fitted on white), bullets, facts, the item's `url` host as a link, page dots (tap to jump), verdict and sources after. No picture or a failed download shows the initial. Spring-in uses `windMotionEnabled()`; motion off is static (pixel-identical frames).
- Tests: `CompareCardsTest` (parsing) and `CompareCardsUiTest` (row with 2 and 6 items, aligned facts, three-party pager, Codi `AssistantResponse`, wind `WindBubble`, motion off/on, frames in `output/ui-review/116/`). Robolectric has no network, so the frames show the initial or an empty picture slot instead of real photos; not checked on a device.
- Assumes the server (115) sends resolved `image`, `images` and `logo` URLs; `logo_domain` and `image_query` are ignored by the app.
- Follow-up: `Viewer.kt` only collects the first photo of each compare item; the second photo of a pager card is not in the viewer list.
- Follow-up: `springIn` is now duplicated in `VentsAvuiGlance.kt` and `CompareCards.kt`; one shared modifier would do.
