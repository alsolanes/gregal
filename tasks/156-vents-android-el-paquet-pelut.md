---
title: Vents Android: el paquet Pelut
status: todo
depends_on: ["155"]
---

## Why

Android parity for 155: the Pelut pack on Android.

## Acceptance criteria

- [ ] Android accepts `pelut` as the account theme and a wind's avatar (windPacks in WindFace.kt), draws every wind in that pack with the pelut clay sheets in its colour (WindClayFace, family `pelut`) at large and small sizes, and falls back to a Canvas face when the sheet is missing. Robolectric tests.
- [ ] The Android avatar pickers (Preferences avatars and WindCreatorScreen in Free mode) list «Pelut» / «Furry» with ca and en strings in AppStrings.kt (by hand), and show the clay packs as their 3D figure. A test.

## Out of scope

- Web and server (155).

## Notes

- The sheets reach the APK through the copyClayCharacters gradle task; nothing to copy by hand.
- Edit Strings/AppStrings by hand; do not run regen-strings.py.
- The pelut sheets are 256 px with a 128 px mask (155): WindClay must read the frame and mask size per silhouette as the manifest gives it, not one global size.
