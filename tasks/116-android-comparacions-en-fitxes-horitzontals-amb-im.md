---
title: Android: comparacions en fitxes horitzontals amb imatge
status: todo
depends_on: ["115"]
---

## Why

When someone asks to compare things (products, places, options, tools, libraries…), a row of cards with a picture or logo for each one reads far better than a table, in Codi and in Vents alike.

## Acceptance criteria

- [ ] `CompareCards.kt` renders `gregal-compare` as a `LazyRow` with snap and a peek of the next card: image or logo on top (the resolved `image`; `logo` from the server; initial as fallback), name, aligned facts, pick badge, verdict below; 2–8 items. A Compose test with 2 and 6 items.
- [ ] Works in Code-mode chats as well as wind chats (same Markdown renderer). A test.
- [ ] Cards spring in and the row scrolls smoothly; motion off keeps it static.

## Out of scope

- Web and server (115).

## Notes

- Existing: ```gregal-compare {title, items:[{name, image_query, facts:{Trait:value}, pick}], verdict}``` taught in `internal/companion/winds.go` `CardsGuide` (winds and Gregal only, «2-3 things side by side»); images resolved in `internal/web/winds_card_images.go` (Commons, English names) only for wind replies; rendered by `internal/web/app/md.js` `compareCard` (grid) and Android `CompareCards.kt` (Row).
- Asked by the user on 2026-11-24: «quan algun usuari demana (al mode codi o vents) una comparació, que si té sentit el llm pugui decidir fer fitxes scrollable horitzontals amb la seva imatge o logo o el que sigui».
- Android: gradle tests with ANDROID_HOME; no `(?U)`.
