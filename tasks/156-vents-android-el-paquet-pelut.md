---
title: Vents Android: el paquet Pelut
status: done
depends_on: ["155"]
---

## Why

Android parity for 155: the Pelut pack on Android.

## Acceptance criteria

- [x] Android accepts `pelut` as the account theme and a wind's avatar (windPacks in WindFace.kt), draws every wind in that pack with the pelut clay sheets in its colour (WindClayFace, family `pelut`) at large and small sizes, and falls back to a Canvas face when the sheet is missing. Robolectric tests.
- [x] The Android avatar pickers (Preferences avatars and WindCreatorScreen in Free mode) list «Pelut» / «Furry» with ca and en strings in AppStrings.kt (by hand), and show the clay packs as their 3D figure. A test.

## Out of scope

- Web and server (155).

## Notes

- The sheets reach the APK through the copyClayCharacters gradle task; nothing to copy by hand.
- Edit Strings/AppStrings by hand; do not run regen-strings.py.
- The pelut sheets are 256 px with a 128 px mask (155): WindClay must read the frame and mask size per silhouette as the manifest gives it, not one global size.
- Done on one branch with 157, which re-rendered every family at 256 px and dropped the manifest's per-silhouette `sizes`: WindClay reads the frame and mask size from the manifest's global `size`/`maskSize`, which is now every silhouette's size, so there is no per-silhouette lookup to add.
- WindFace: a wind in the `pelut` pack is WindClayFace family `pelut` at every size, with or without `clay`, and never wears a trade kit (as on the web). Without its sheets it falls through to the Cares Canvas face of its shape (the web has a furry SVG fallback; Android has no furry Canvas drawing). Small faces hold the still frame, which 157 tints alone.
- Pickers: Preferences › Avatars samples and the editor's Free-mode avatar options pass `clay = true`, so Cares, Minairons and Pelut show their 3D figure; FaceOptions got a `clay` flag that only the avatar row sets (the eye, mouth and shape options stay drawn, where the choice shows). Sample sizes stay 40 and 42 dp.
- Tests: WindPelutTest (pack and labels; clay pelut at 96, 40 and 24 dp in two moods; red and blue at 96 and 40 dp; Canvas fallback; Preferences samples; editor in Free mode saves `pelut`). Picker renders from a throwaway Robolectric test (not committed): %TEMP%/g157/shots/picker-prefs-light.png and picker-editor-dark.png.
- Follow-up: windFaceBitmap (notification large icon) draws a pelut as the Cares figure of its shape; it could use the pelut's still frame.
- Follow-up: the picker samples are still 40-42 dp; the web made them 64 px in 155.
- Follow-up: in the editor, the shape options (Bola, Núvol, Espiral) are shown for a pelut wind although its figure ignores the shape.
