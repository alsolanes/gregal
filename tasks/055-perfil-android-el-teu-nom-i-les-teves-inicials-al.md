---
title: "Perfil (Android): el teu nom i les teves inicials al calaix"
status: done
depends_on: ["053"]
---

## Why

Android parity with 053: the person's avatar and name in the drawer, and the same name and colour as on the web.

## Acceptance criteria

- [x] At the bottom of the drawer (`Drawer.kt`, both `GregalDrawerContent` and `SimpleVentsDrawer`) a row with the round initials avatar on the chosen colour and the name; tapping it opens Preferences at the name. A Robolectric test.
- [x] Preferences has «Com et dius» and the avatar colour, saved to `/api/profile` (`gregal_user_name`, `gregal_user_color`) so the web shows the same. Default name as on the web: the account name from `/api/me`, or «Tu» / «You» locally. A test that the POST carries both keys and that the drawer shows the account name when none is saved.
- [x] `ANDROID_HOME=... ./gradlew.bat testDebugUnitTest -Pgregal.skipBackend -Pgregal.plotsPreview` passes.

## Out of scope

- The photo (056).

## Notes

- `Drawer.kt` (the Preferences row near line 148), `PrefsScreen.kt` (add the field to the General or Appearance page), `GregalApi.kt` (`/api/me`, `MeDto`), `AppStrings.kt` (edit by hand, both languages). Android does not read `/api/profile` yet.
- Done: `UserAvatar.kt` holds the row (30 dp circle, ink from `windInk`, name in `Tinta`, one line), «Com et dius» + the eight wind colours (`WindSuggestions.all`, same as `presetDefaults` on the web) and the name rules of `user-avatar.js`. The row is the last line of both drawers; tapping it opens Preferences › General, where the fields are first. Name and colour are saved together with one «Desa» button (one POST with both keys, `t` = now); an empty name sends `v: null`, like the web removing the key. `MeInfo`/`MeDto` gained `local`; `connectOnce` keeps the whole `/api/me` answer instead of only `admin`, and `syncProfile()` reads `/api/profile` after connecting.
- Not verified on a device or emulator: only Robolectric. The field is not focused when opened from the avatar (the web does not focus it on touch screens either).
- Follow-up: Android reads only the two profile keys; the rest of `/api/profile` (theme, language…) is still not synced on Android.
- Follow-up: the name and colour are not cached offline; with no server answer the drawer shows «Tu»/the account name until the next connection.
