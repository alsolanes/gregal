---
title: "Vents (Android): el tema d'avatars i l'Esperit de vent"
status: todo
depends_on: ["045"]
---

## Why

Android is the main client and has to allow the same as the web: choosing an avatar theme for every wind, or a pack per wind in free mode, with the Esperit de vent pack (task 045 builds the server side and the web).

## Acceptance criteria

- [ ] Android reads `avatarTheme` and each wind's `avatar` from `GET /api/winds`, and draws every wind face with the theme's pack, or with each wind's pack in Free mode. A Robolectric test with a fake response checks which pack `WindFace` uses in each mode.
- [ ] In Preferences, «Avatars» offers Cares, Esperit de vent and Free, and saves with `POST /api/winds/avatar-theme`. The wind creator/editor shows the pack picker only in Free mode and saves `avatar` with the wind. A Compose test for each.
- [ ] Esperit de vent is drawn as on the web (tails toward the wind's direction, eyes looking the other way), with the same direction for the same wind. A unit test with the same table and hash as the web test.
- [ ] Motion follows `LocalWindMotion` and «Moviment expressiu»: still when off, unless the wind is working. A Compose test with motion off.

## Out of scope

- The other packs: 048, 050, 051.
- Rotllanes keep the current stack of faces.

## Notes

- `WindFace.kt` (`WindStyle`, `drawWindBody`) and `Winds.kt` (parsing `/api/winds`). Preferences in `PrefsScreen.kt`, editor in `WindCreatorScreen.kt`.
- The direction function mirrors the one in `winds.js` (045): same names, same degrees, same hash of the id.
- Strings: edit `Strings.kt`/`AppStrings.kt` by hand, in Catalan and English. Set `ANDROID_HOME` for the gradle tests.
