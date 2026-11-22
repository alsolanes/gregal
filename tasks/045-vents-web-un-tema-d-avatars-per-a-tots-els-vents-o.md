---
title: "Vents (web): un tema d'avatars per a tots els vents, o lliure per a cadascun, amb l'Esperit de vent"
status: todo
depends_on: []
---

## Why

Wind avatars look like everyone else's (a ball with a face). The user wants avatar packs (Cares, Esperit de vent, Cap de vent, Mapa de vent, Rosa de rellotge), seen in `mockups/vents-avatars.html` (2026-11-22). They want to choose one theme for every wind, or a free mode where each wind uses any pack. This task builds the choice and the first new pack, Esperit de vent; the other packs come in 047 and 049.

## Acceptance criteria

- [ ] A wind has an `avatar` field (`cares`, `esperit`; empty means `cares`) and the account has an avatar theme (`cares`, `esperit` or `free`; `cares` by default). `GET /api/winds` returns `avatarTheme` and each wind's `avatar`. `POST /api/winds` saves `avatar` and keeps it when omitted. `POST /api/winds/avatar-theme` `{theme}` saves the theme, and an unknown pack is 400. Go tests for each case, including two accounts not seeing each other's theme.
- [ ] In the Vents preferences, «Avatars» offers Cares, Esperit de vent and Free. With a pack chosen, every face in Vents is drawn with that pack. With Free, each wind uses its own `avatar`. The wind editor shows the pack picker only in Free mode; with a theme set, it says which theme is in use. Node tests with the fake DOM for both modes.
- [ ] Changing the theme keeps each wind's own `avatar`, so going back to Free brings back every wind's choice. A Go test.
- [ ] Esperit de vent: the face with two wind tails pointing to where the wind comes from, and eyes looking the other way. The direction is the wind's direction on the wind rose when its name is one of the eight Catalan winds (Tramuntana N, Gregal NE, Llevant E, Xaloc SE, Migjorn S, Garbí SW, Ponent W, Mestral NW), and otherwise a fixed one of the eight taken from its id. A node test checks the direction for a named wind and that the same id always gets the same direction.
- [ ] Motion follows the current rule: idle movement only with expressive motion on and not under `prefers-reduced-motion`; while the wind works, the tails move faster, because that shows real work. A node test checks the idle animation is absent when motion is off.

## Out of scope

- Android: task 046.
- The other packs: 047 (Cap de vent, Rosa de rellotge), 049 (Mapa de vent), 051 (faces that blow while working).
- A combined avatar for rotllanes: they keep the current stack of faces.

## Notes

- Server: `internal/companion/winds.go` (`Wind` gets `Avatar`). Store the theme in the account's state folder next to `winds.json`, like `collective.json`. Handler in `internal/web/winds_web.go` (`handleWinds`). Update `docs/api-contract.md` (`/api/winds` lines ~174–175 and the new route).
- Web: `face()` in `internal/web/app/winds.js` dispatches by pack. The wind direction function lives there too, and 046 mirrors it with the same table and the same hash.
- The large view (`large === true`) of Cares keeps the trade's kit. The new packs don't draw kits; the trade is in the pill (043).
- Drawing reference: `esperit()` in `mockups/vents-avatars.html` (SVG paths, gradients, SMIL animation). Port it to the `svgNode` style of `winds.js`. CSS animations are fine instead of SMIL if they respect the motion rule.
- English is the default language, Catalan is also supported: «Avatars», «Free»/«Lliure» and the pack names go in both.
