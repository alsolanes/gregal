---
title: Vents web: crear un vent des d'una descripció
status: todo
depends_on: []
---

## Why

To create a wind today you fill in name, role, persona, colour and face by hand, or pick one of the fixed suggestions. The web creator never calls `/api/winds/suggest`; only Android does, and only from a name and role. OpenBot creates an agent from one sentence and lets you review it before saving. We want that: one sentence fills the whole wind, and you see it before creating it.

## Acceptance criteria

- [ ] `POST /api/winds/suggest` with `{description}` (and no name) returns `{name, role, persona, style, color}`. The name is not one of the user's existing winds, the colour is `#rrggbb`, and the style letters are valid. A bad model reply falls back to defaults, as today. `{name, role}` keeps working as before. Go test in `winds_rotllanes_test.go` or `winds_names_test.go`.
- [ ] The role the reply gives maps to a trade through the existing role→kit rule (`winds_kit.go`) when one fits. For example, «un vent que m'ajudi amb receptes de temporada» gives a role whose kit is `cooking`. Go test.
- [ ] The web creator has a «Descriu el teu vent» field with a button. On success it fills name, role, persona, colour and the eyes, mouth and shape controls, and the preview face (clay, with the trade badge if it has a trade) updates. Nothing is created until «Crea el vent». `winds-start.test.cjs` (or the creator's test file) with a stubbed `/api/winds/suggest`.
- [ ] While the suggestion runs the button is disabled. If it fails, an error line shows and the fields keep what they had. Node test.
- [ ] `docs/api-contract.md` documents the new body and reply. The strings exist in Catalan and English.

## Out of scope

Android (task 166). New silhouettes (170). Generating images. Changing the fixed suggestion list.

## Notes

- Backend: `internal/web/winds_suggest.go` (`windSuggestSystem`, `parseWindSuggest`, `normalizeWindStyle`), `winds_kit.go`. Web: the creator dialog in `internal/web/app/winds-workspace.js` (around `style=()=>eyes.value+mouth.value+shape.value`). The preview goes through `clay(...)` so it matches the result.
- The collective name (fullets, pirates…) must still shape the voice and the name, as `windSuggestSystem` does today.
