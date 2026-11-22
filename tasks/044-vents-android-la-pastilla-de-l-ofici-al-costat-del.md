---
title: "Vents (Android): la pastilla de l'ofici al costat del nom de cada vent"
status: done
depends_on: []
---

## Why

The same as task 043, on Android, which is the main client: every wind's name gets a small pill with its trade, in the wind's colour, so you know who to ask without opening profiles.

## Acceptance criteria

- [x] On the Vents start page (the faces row), in the chat header and in the winds directory, each wind's name has a pill with its `role` in the wind's colour. A Compose test (Robolectric) finds the pill text next to the name in each of the three screens.
- [x] A wind with a `kit` shows that kit's icon in the pill. A wind without a kit shows the pill with no icon, and a wind with an empty `role` shows no pill. A Compose test covers the three cases.
- [x] The pill is readable in light and dark themes and with large text: it truncates with an ellipsis instead of pushing the name off the row. A Compose test at font scale 1.5 checks that the name is still shown.

## Out of scope

- Web: task 043.
- A new field on the wind (see 043).

## Notes

- Places: `VentsHomeParts.kt` (start page faces), the chat header in `VentetChat.kt`/`WindMessageUi.kt`, `VentsCrew` in `CompanyScreen.kt`.
- Kit icons: map the `windKits` keys in `WindFace.kt` to Material icons that are already in the project, with no new dependency.
- Strings: edit `Strings.kt` by hand if a string is added (do not run `regen-strings.py`). Set `ANDROID_HOME` for the gradle tests.

### What was built (2026-11-22)

- `WindRolePill(wind)` and `windKitIcon(kit)` in `VentsHomeParts.kt`. The pill copies `.tag` from the mockup: background is the wind colour at 18% alpha, text is the wind colour mixed 50% with the theme's `Ink` (Compose `lerp`, which mixes in Oklab, not sRGB like the web's `color-mix`), 11.5sp, 6dp corners, at least 20dp high, a 12dp icon. `windKitIcon` maps all 24 `windKits` keys to Material icons from `material-icons-extended`, which was already a dependency. No strings were added.
- The notes above pointed at code the app no longer renders. `HomeFacesRow` (`VentsHomeParts.kt`) and `VentsCrew` (`CompanyScreen.kt`) are only used by tests. The pill went into the screens the app shows today:
  - Start page faces row: `StartFaces` in `VentsStart.kt`. The pill sits under the name. The face column now grows from 60dp up to 96dp, because at 60dp only two letters fitted.
  - Chat header: `VentsChatHeader` in `VentsChatChrome.kt` takes a new `wind` parameter, and the pill goes next to the name. `WindConversation` (`CompanyScreen.kt`) passes the wind and no longer repeats the role as a subtitle.
  - Winds directory: `DirectoryRow` (manage mode), with the pill next to the name. It replaces the old muted role line.
- The name is always measured first in its row, so the pill only gets the space that is left and truncates with "…".
- "Kit" means `Wind.faceStyle.kit`: the explicit kit, otherwise the specialty, otherwise the kit inferred from the role. This is the kit the avatar draws. A wind with no explicit kit whose role matches `WindKit` (for example "Música…") still shows an icon. Task 043 (web) should follow the same rule.

### Tests

`android/app/src/test/java/xyz/solanes/gregal/WindRolePillTest.kt` (Robolectric, screenshots in `android/app/build/role-pill/`). The full `./gradlew testDebugUnitTest` run passed.

### Not verified

- Not checked on a device or emulator, only in Robolectric screenshots, which looked fine in both themes.
- Contrast was not measured. Pastel winds in the light theme (for example "calma" on #EFC69A) look a bit faint and are probably under 4.5:1 for 11.5sp text.
- The start page has no font-scale-1.5 test. The name has its own line there, so the pill cannot push it out.

Follow-up: `HomeFacesRow` (`VentsHomeParts.kt`) and `VentsCrew` (`CompanyScreen.kt`) are no longer rendered anywhere except in tests; remove them and their tests.
Follow-up: the `DirectoryRow` contentDescription `"${w.name}: ${w.role}"` reads "Ponent: " when the role is empty (this was already the case before this task).
Follow-up: check the pill text contrast for pastel winds in the light theme, here and on the web (043).
