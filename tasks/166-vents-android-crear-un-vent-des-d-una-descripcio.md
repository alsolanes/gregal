---
title: Vents Android: crear un vent des d'una descripció
status: done
depends_on: [165]
---

## Why

Android parity with 165. The Android creator already calls `/api/winds/suggest`, but only to fill the persona and face from a name and role you have already typed.

## Acceptance criteria

- [x] `WindCreatorScreen` has a «Descriu el teu vent» field and button. With a stubbed API reply `{name, role, persona, style, color}` it fills name, role, persona, colour and face, and the preview updates. Nothing is saved until you tap create. Robolectric test.
- [x] While the suggestion runs the button is disabled and shows progress. If it fails, `app.vents.suggestError` shows and the fields stay as they were. Test.
- [x] Editing an existing wind does not show the description field. The current per-field suggest button (name and role → persona and face) keeps working. Test.
- [x] The new strings exist in Catalan and English in `Strings.kt`.

## Out of scope

The backend and the web (165). New silhouettes (170).

## Notes

- `android/.../WindCreatorScreen.kt` (`onSuggest`, `suggesting`), `GregalViewModel.suggestWind`, `GregalApi.kt:650` (`SuggestDto` gains name, role, color).
- Edit `Strings.kt` by hand; do not run `regen-strings.py`.
- Done. The tests are in `android/app/src/test/java/xyz/solanes/gregal/WindDescribeTest.kt`. The new strings (`app.vents.describe`, `describeHint`, `describeGo`) went into `AppStrings.kt`, next to the other `app.vents.*` keys. `Strings.kt` is generated from the web's `i18n.js` and says not to edit it by hand.
- The proposed colour is uppercased and added to the palette, so it stays selectable after you pick another colour. The preview wears the proposed `kit` while the role is still the proposed one, as on the web. No test checks the badge: `WindFace` only exposes its accent to semantics.
- Not checked on a device or emulator.
