---
title: Comparacions en fitxes horitzontals amb imatge, també al mode Codi (servidor i web)
status: done
depends_on: []
---

## Why

When someone asks to compare things (products, places, options, tools, libraries…), a row of cards with a picture or logo for each one reads far better than a table, in Codi and in Vents alike.

## Acceptance criteria

- [x] The Code-mode agent's system prompt also teaches `gregal-compare` (only when a comparison really helps; never for code diffs or one-line answers), and the guide allows 2–8 items, each with `name`, `image_query` or `image`, optional `logo_domain` (e.g. `nodejs.org`, for brands/products/software) and `url`, `facts` with the same traits, optional `pick`, plus `verdict`. A Go test of both prompts.
- [x] Images are resolved for compare cards in Code-mode replies too, not only winds: `image_query` → a photo (as today), `logo_domain` → a logo URL from a free, keyless source (e.g. the site's favicon/apple-touch-icon via a server-side fetch or a public favicon service, cached), never an invented URL; a missing image falls back to the item's initial. Go tests with fake HTTP servers.
- [x] Web: `gregal-compare` renders as a horizontally scrollable row of cards (scroll-snap, swipe/trackpad/arrow buttons on desktop, a peek of the next card), each with its image or logo on top, name, facts aligned across cards (same trait order), a «tria» badge on the pick, and the verdict below the row; works in Codi and Vents, dark and light, under reduced motion. A node test with 2 and 6 items.
- [x] docs/api-contract.md mentions the block fields if the contract lists card blocks.

- [x] The election example works end to end: items with `points` and two images render as a swipeable one-card-per-page carousel with page dots; a test with three parties.

## Out of scope

- Android (116).

## Notes

- Existing: ```gregal-compare {title, items:[{name, image_query, facts:{Trait:value}, pick}], verdict}``` taught in `internal/companion/winds.go` `CardsGuide` (winds and Gregal only, «2-3 things side by side»); images resolved in `internal/web/winds_card_images.go` (Commons, English names) only for wind replies; rendered by `internal/web/app/md.js` `compareCard` (grid) and Android `CompareCards.kt` (Row).
- Asked by the user on 2026-11-24: «quan algun usuari demana (al mode codi o vents) una comparació, que si té sentit el llm pugui decidir fer fitxes scrollable horitzontals amb la seva imatge o logo o el que sigui».
- Web: node tests; screenshots from a sandboxed server (never type into credential fields) in `output/ui-review/115/`.
- User example (2026-11-24): «un resum dels programes electorals dels partits que es presenten a les eleccions, una fitxa per partit amb una foto o dues, fent swipe right poder veure el següent i així (o swipe left)». So an item can also carry `images` (up to 2: e.g. the party logo via `logo_domain` and the candidate's photo via `image_query`) and `points` (3–6 short summary bullets), and when items carry `points` the block renders as a full-width paged carousel: one card per page, swipe left/right (and arrow keys/buttons on desktop), page dots, the verdict/sources after. Short fact comparisons keep the compact snapping row. Neutral wording for political content: summarise each programme from its own sources (`url` per item), no pick on parties.
- Done (2026-11-24). Final block format after the server: each item `{name, image?, images?, logo?, url?, facts?, points?, pick?}`; `image` is the photo (Commons, from `image_query` or the English name), `images` at most 2 http(s) URLs given by the model, `logo` the site icon found from `logo_domain`. `image_query` and `logo_domain` are removed. Same names as Android (116).
- Guide: `companion.CompareGuide`, used by `CardsGuide` (winds) and by `sysPromptParts` (every web Codi mode, ~1 KB).
- Logos: `internal/web/card_logos.go` reads the site's home page (4 s timeout), ranks apple-touch-icon > biggest icon > /favicon.ico, and keeps the first one that answers 200 with an `image/*` type. In-memory cache per domain (misses too, not timeouts). `logoSiteURL` is the test hook.
- Code mode: `conduirTorn` passes the `assistant` text and the final reply through `codeCompareImages` (only ```gregal-compare blocks), memoised so the shown and saved text are the same.
- Web: `.gx-row` (snapping row, peek of the next card, facts in a CSS subgrid so the same trait lines up across cards) or `.gx-paged` (one card per page, up to two images, page dots). Arrow buttons only with a fine pointer; smooth scroll only without reduced motion. Strings `card.prev`/`card.next` in ca and en.
- api-contract.md does not list card block fields (only the note kinds), so it is unchanged.
- The diff is over the guard's 400 lines (581 non-test lines after the SSRF hardening), accepted by the coordinator.
- `TestLocalRunBringsSourcedCardsToTodayAndVoiceRespectsQuiet` fails on the base commit too; not touched.
- Follow-up: the public clip page (`clip.css`) has no styles for the new compare row/pages and does not load the arrow handler outside md.js (md.js wires it, so arrows work; only the look is missing).
- Follow-up: `handleChat` (the legacy `/api/chat` path) does not resolve compare images; only `runTurn`/`conduirTorn` does.
- Logo fetch hardened against SSRF (logo_domain comes from the model): only dotted host names (no IPs, no localhost or single-label names), http(s) on ports 80/443, at most 3 redirects each re-checked (`logoRedirect`), and every connection goes through `logoDialContext` (resolves once, refuses loopback, RFC1918/ULA, link-local, multicast, unspecified and CGNAT, dials the checked IP) plus a `net.Dialer` Control that re-checks at connect time. Page reads capped at 512 KB; the icon check reads no body. Tests inject `logoClient`, `logoLookup` and `logoDial`. The server fetches no other compare-card image URLs (photos come from the Commons search).
