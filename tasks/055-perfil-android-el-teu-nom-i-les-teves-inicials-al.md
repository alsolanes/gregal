---
title: "Perfil (Android): el teu nom i les teves inicials al calaix"
status: todo
depends_on: ["053"]
---

## Why

Android parity with 053: the person's avatar and name in the drawer, and the same name and colour as on the web.

## Acceptance criteria

- [ ] At the bottom of the drawer (`Drawer.kt`, both `GregalDrawerContent` and `SimpleVentsDrawer`) a row with the round initials avatar on the chosen colour and the name; tapping it opens Preferences at the name. A Robolectric test.
- [ ] Preferences has «Com et dius» and the avatar colour, saved to `/api/profile` (`gregal_user_name`, `gregal_user_color`) so the web shows the same. Default name as on the web: the account name from `/api/me`, or «Tu» / «You» locally. A test that the POST carries both keys and that the drawer shows the account name when none is saved.
- [ ] `ANDROID_HOME=... ./gradlew.bat testDebugUnitTest -Pgregal.skipBackend -Pgregal.plotsPreview` passes.

## Out of scope

- The photo (056).

## Notes

- `Drawer.kt` (the Preferences row near line 148), `PrefsScreen.kt` (add the field to the General or Appearance page), `GregalApi.kt` (`/api/me`, `MeDto`), `AppStrings.kt` (edit by hand, both languages). Android does not read `/api/profile` yet.
