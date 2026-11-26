---
title: Vents Android: crear un vent des d'una descripció
status: todo
depends_on: [165]
---

## Why

Android parity with 165. The Android creator already calls `/api/winds/suggest`, but only to fill the persona and face from a name and role you have already typed.

## Acceptance criteria

- [ ] `WindCreatorScreen` has a «Descriu el teu vent» field and button. With a stubbed API reply `{name, role, persona, style, color}` it fills name, role, persona, colour and face, and the preview updates. Nothing is saved until you tap create. Robolectric test.
- [ ] While the suggestion runs the button is disabled and shows progress. If it fails, `app.vents.suggestError` shows and the fields stay as they were. Test.
- [ ] Editing an existing wind does not show the description field. The current per-field suggest button (name and role → persona and face) keeps working. Test.
- [ ] The new strings exist in Catalan and English in `Strings.kt`.

## Out of scope

The backend and the web (165). New silhouettes (170).

## Notes

- `android/.../WindCreatorScreen.kt` (`onSuggest`, `suggesting`), `GregalViewModel.suggestWind`, `GregalApi.kt:650` (`SuggestDto` gains name, role, color).
- Edit `Strings.kt` by hand; do not run `regen-strings.py`.
