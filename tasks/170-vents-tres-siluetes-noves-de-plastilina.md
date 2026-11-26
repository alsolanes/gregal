---
title: Vents: tres siluetes noves de plastilina
status: done
depends_on: []
---

## Why

Every Cares wind is one of three shapes (bola `o`, núvol `n`, espiral `s`), so a crew of eight winds repeats. OpenBot offers about two dozen shapes. We want more variety in the same clay style, with soft bodies and no corners.

## Acceptance criteria

- [x] `tools/characters/build.py` has three new silhouettes, each a metaball body with no corner, on the same ground line and scale. Every mood is rendered with its masks, and `manifest.json` maps each one to a new shape letter. `TestCharactersManifestMatchesSheets` passes.
- [x] `normalizeWindStyle` and the suggest prompt (`windSuggestSystem`) accept the three new shape letters. `POST /api/winds` keeps a style with them. Go test.
- [x] The web creator's shape control and Android's `WindCreatorScreen` offer the three new shapes, each with a live sample. A wind with a new shape plays its clay sheet on the web (`characters.js` maps the letter) and on Android (`WindClayFace`). Node and Robolectric tests.
- [x] Without the sheets, or in a code-drawn spot, a new-shape wind draws as the nearest existing drawn shape and never as an empty or broken face. Web and Android tests.
- [x] The shape names exist in Catalan and English.

## Out of scope

New moods, props or packs. Adding the code-drawn version of the new shapes beyond the fallback.

## Notes

- Before rendering every mood, show the user a contact sheet (`blender -b -P tools/characters/build.py -- --moods "" --preview output/ui-review/170`, after rendering `idle` only) with a few candidates, and let them pick the three. Wind themes such as gota, estel or fulla are only ideas.
- The sheets today are 4.5 MB for five silhouettes. Three more add about 2.7 MB to the web and the APK. Say the figure in the final message.
- Files: `tools/characters/build.py` (`SILHOUETTES`), `internal/web/winds_suggest.go`, `internal/web/app/winds.js` (shape letter), `characters.js` (`'ons'.includes(...)`), `winds-workspace.js` (creator), `android/.../WindFace.kt`, `WindStyle`, `WindCreatorScreen.kt`.
- If 169 is merged first, render `look` for the new silhouettes too.
- The user picked gota (`g`), sol (`u`) and onada (`w`) from six candidates (gota, estel, sol, bufet, flama, onada). `--only` now also limits the contact sheets, so candidates can be previewed with only `idle` rendered.
- Requirement changed, approved by the user: the three shapes add 2,800,652 bytes (gota 888,832, sol 890,240, onada 1,021,580), so the clay budget in `TestCharactersMinairoWithinBudget` goes from 3.7 MB to 6.6 MB (6,497,002 bytes now, without the pelut). The pelut's own budget is unchanged.
- Requirement changed: `MascotMinimalTest` counted 48 style codes (4 eyes x 4 mouths x 3 shapes); with six shapes it is 96.
- Drawn fallback: gota draws as the ball, sol as the cloud (both bumpy), onada as the curl. On the web the creator's shape control is a select, so the live sample is the preview face, which plays the clay sheet of the shape picked; Android's chips each show their clay sample.
- Follow-up: the curl is «Rínxol» on the web and «Espiral» on Android; not changed here.
