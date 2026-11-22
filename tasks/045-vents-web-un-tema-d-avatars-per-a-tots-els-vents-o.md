---
title: "Vents (web): un tema d'avatars per a tots els vents, o lliure per a cadascun, amb l'Esperit de vent"
status: done
depends_on: []
---

## Why

Wind avatars look like everyone else's (a ball with a face). The user wants avatar packs (Cares, Esperit de vent, Cap de vent, Mapa de vent, Rosa de rellotge), seen in `mockups/vents-avatars.html` (2026-11-22). They want to choose one theme for every wind, or a free mode where each wind uses any pack. This task builds the choice and the first new pack, Esperit de vent; the other packs come in 047 and 049.

## Acceptance criteria

- [x] A wind has an `avatar` field (`cares`, `esperit`; empty means `cares`) and the account has an avatar theme (`cares`, `esperit` or `free`; `cares` by default). `GET /api/winds` returns `avatarTheme` and each wind's `avatar`. `POST /api/winds` saves `avatar` and keeps it when omitted. `POST /api/winds/avatar-theme` `{theme}` saves the theme, and an unknown pack is 400. Go tests for each case, including two accounts not seeing each other's theme.
- [x] In the Vents preferences, «Avatars» offers Cares, Esperit de vent and Free. With a pack chosen, every face in Vents is drawn with that pack. With Free, each wind uses its own `avatar`. The wind editor shows the pack picker only in Free mode; with a theme set, it says which theme is in use. Node tests with the fake DOM for both modes.
- [x] Changing the theme keeps each wind's own `avatar`, so going back to Free brings back every wind's choice. A Go test.
- [x] Esperit de vent: the face with two wind tails pointing to where the wind comes from, and eyes looking the other way. The direction is the wind's direction on the wind rose when its name is one of the eight Catalan winds (Tramuntana N, Gregal NE, Llevant E, Xaloc SE, Migjorn S, Garbí SW, Ponent W, Mestral NW), and otherwise a fixed one of the eight taken from its id. A node test checks the direction for a named wind and that the same id always gets the same direction.
- [x] Motion follows the current rule: idle movement only with expressive motion on and not under `prefers-reduced-motion`; while the wind works, the tails move faster, because that shows real work. A node test checks the idle animation is absent when motion is off.

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

### What was built (2026-11-22)

- Server: `Wind.Avatar` (`avatar,omitempty`) and `companion.ValidAvatar` (`cares`, `esperit`; 047 adds its packs there). Saving an unknown pack is 400; an empty `avatar` keeps the stored one. The theme lives in `avatar-theme.json` in `windStateDir()` (next to `winds.json`, per account). `GET /api/winds` adds `avatarTheme` and each wind's `avatar` (raw, empty = Cares). New `POST /api/winds/avatar-theme` in `winds_web.go`. Contract updated.
- Web: `winds.js` keeps the theme (`setAvatarTheme`, set from every `/api/winds` load, also by `winds-workspace.js`), and `face()` dispatches to `esperit()` before the Cares drawing. Esperit draws no kit. `avatarField()` is the editor picker (Free mode only; otherwise a line naming the theme) used by the old editor (`windForm`), the live create dialog and the live profile dialog in `winds-workspace.js` (saves at once). `avatarPrefs()` fills the «Avatars» row in Preferences › Appearance › Personalization (`prefs.js`, `#prefsAvatars`).
- Motion: the web has no «Moviment expressiu» preference (only Android does), so the idle gate is `prefers-reduced-motion` alone: `face()` adds `wf-idle` to the svg only when motion is allowed, and the idle sway/drift in `winds.css` hangs on it. While the wind works (thinking, searching, reading, writing, working, discussing) the tails and drift run at 0.75 s instead of 3 s with `!important`, so they move even under reduced motion. Still lists (`.vw-message:not(.vw-pending)`) stay still as before.
- face() has no pixel size, so the mockup's compact variant under 28 px is not drawn: every Esperit face has the streak, blush and smile.

### Direction (task 046 mirrors this exactly)

- Compass degrees, 0 = N, the bearing the wind comes from; the tails point there, the eyes look the other way. Name match is case- and accent-insensitive (NFD, combining marks removed, trimmed, lowercase): tramuntana 0, gregal 45, llevant 90, xaloc 135, migjorn 180, garbi 225, ponent 270, mestral 315.
- Any other wind: `h = 0; for each UTF-16 unit c of the id: h = (h * 31 + c) mod 2^32`; direction = `(h % 8) * 45`. These are the low bits of Java's `String.hashCode()`, so Kotlin can write `(id.hashCode() and 7) * 45`. Pinned in `wind-face.test.cjs`: `bit` → 225, `teca` → 315, `llevant-2` → 135, `espurna` → 90.
- Web only: a wind with no id yet (the editor preview) hashes the slug of its name, so the preview points where the saved wind will.

### Tests

- Go: `internal/web/winds_avatar_test.go` (default and unknown theme, avatar saved/kept/unknown 400, theme keeps each wind's avatar, per account).
- Node: `internal/web/app/wind-face.test.cjs` (direction, dispatch by theme and Free, tails and eyes, motion gate), `internal/web/winds.test.js` (Preferences row, every face redrawn with the pack, old editor in both modes), `internal/web/app/winds-start.test.cjs` (workspace passes the theme; profile and create dialog picker in Free mode, none with a theme).

### Not verified

- Not seen in a browser: the Esperit drawing, its CSS animations (transform-origin of the tails group, `var()` inside `@keyframes`) and the look of the picker inside the workspace dialogs (it uses `winds-field`/`winds-muted`, not `vw-field`) were not rendered or compared with the mockup.
- Follow-up: faces drawn before any `/api/winds` load (e.g. `vents-activity.js` on the Automations page) use Cares whatever the account theme is.
- Follow-up: the Android client is behind until 046: it ignores `avatarTheme` and `avatar` and keeps drawing Cares.
