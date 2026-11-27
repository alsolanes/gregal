---
title: Vents web: crear un vent des d'una descripció
status: done
depends_on: []
---

## Why

To create a wind today you fill in name, role, persona, colour and face by hand, or pick one of the fixed suggestions. The web creator never calls `/api/winds/suggest`; only Android does, and only from a name and role. OpenBot creates an agent from one sentence and lets you review it before saving. We want that: one sentence fills the whole wind, and you see it before creating it.

## Acceptance criteria

- [x] `POST /api/winds/suggest` with `{description}` (and no name) returns `{name, role, persona, style, color}`. The name is not one of the user's existing winds, the colour is `#rrggbb`, and the style letters are valid. A bad model reply falls back to defaults, as today. `{name, role}` keeps working as before. Go test in `winds_rotllanes_test.go` or `winds_names_test.go`.
- [x] The role the reply gives maps to a trade through the existing role→kit rule (`winds_kit.go`) when one fits. For example, «un vent que m'ajudi amb receptes de temporada» gives a role whose kit is `cooking`. Go test.
- [x] The web creator has a «Descriu el teu vent» field with a button. On success it fills name, role, persona, colour and the eyes, mouth and shape controls, and the preview face (clay, with the trade badge if it has a trade) updates. Nothing is created until «Crea el vent». `winds-start.test.cjs` (or the creator's test file) with a stubbed `/api/winds/suggest`.
- [x] While the suggestion runs the button is disabled. If it fails, an error line shows and the fields keep what they had. Node test.
- [x] `docs/api-contract.md` documents the new body and reply. The strings exist in Catalan and English.

## Out of scope

Android (task 166). New silhouettes (170). Generating images. Changing the fixed suggestion list.

## Notes

- Backend: `internal/web/winds_suggest.go` (`windSuggestSystem`, `parseWindSuggest`, `normalizeWindStyle`), `winds_kit.go`. Web: the creator dialog in `internal/web/app/winds-workspace.js` (around `style=()=>eyes.value+mouth.value+shape.value`). The preview goes through `clay(...)` so it matches the result.
- The collective name (fullets, pirates…) must still shape the voice and the name, as `windSuggestSystem` does today.
- Done: the reply also carries `kit` (the role→kit rule on the proposed role), so the web preview can show the trade badge before the wind exists; it is the same kit the wind gets once created. If the model gives no role, the description itself becomes the role. Model errors stay 502; an unparsable reply gives defaults (a free collective name, a palette colour, `pso`, the default persona).
- Done: the web preview already goes through `gregalWinds.face(w, true)`, which draws it in clay via `gregalCharacters.play`; the badge shows only while the role is still the suggested one.
- Follow-up: the proposed name and persona are always Catalan (as in the `{name, role}` mode); an English user gets a Catalan persona.
