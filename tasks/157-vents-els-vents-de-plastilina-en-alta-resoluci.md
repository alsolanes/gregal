---
title: Vents: els vents de plastilina en alta resolució
status: blocked
depends_on: ["155"]
---

## Why

On phones the clay winds look low resolution: the sheets are 128 px frames with a 64 px colour mask, while the chat wind is 72 dp and the profile 96 dp, 190 to 340 px on current screens, and the colour mask is stretched 3 to 5 times. Task 155 renders the new pelut family at 256 px with a 128 px mask; this does the same for pebble, cloud, curl and minairo.

## Acceptance criteria

- [x] Every clay family is rendered at 256 px frames with a 128 px mask (and cap), the manifest says so, and `go test ./internal/web -run TestCharactersManifestMatchesSheets` passes; any per-silhouette size override added by 155 is gone if no longer needed.
- [x] The web and Android players draw the 256 px sheets at every size with no other change of look (existing clay tests pass), and the Android caches hold a few winds in a few moods at the new size without decoding on every frame. Robolectric test for the frame size.
- [ ] On the emulator (420 dpi) the chat wind and the profile wind are sharp: a screenshot before and after in the task notes.

## Out of scope

- New poses or families.

## Notes

- Depends on 155 (it changes build.py and the manifest). Assets grow from about 1.8 MB to about 5-6 MB; say the final number in the notes.
- Render: `blender -b -P tools/characters/build.py -- --only pebble,cloud,curl,minairo` (about 25 min).
- Rendered with `--only pebble,cloud,curl,minairo` (about 55 min, 1,764 Cycles renders) after build.py went to a global `SIZE` of 256 (mask 128) and lost `SIZES`; the pelut was not re-rendered (its sheets are byte for byte those of 155). The contact sheets (`--preview`, under %TEMP%/g157/preview) were looked at at 256 and 40 px, light and dark: same figures, sharper.
- Asset sizes: pebble 837,210, cloud 786,062, curl 924,112, minairo 1,000,860, pelut 847,022 bytes; 4,395,266 bytes in all (plus the 0.9 KB manifest), the four re-rendered families went from 1,598,317 to 3,548,244 bytes (manifest included before, not after).
- The manifest has no `sizes` any more: every family is the global `size` 256 / `maskSize` 128. characters.js and its test read only the global size again. Go tests: `TestCharactersManifestMatchesSheets` now also requires 256/128; `TestCharactersMinairoWithinBudget` (all families but the pelut) went from 2 MB to 3.7 MB for the real 3.55 MB; `TestCharactersPelutAt256WithinBudget` keeps 1.2 MB and checks the global size instead of the override.
- Android (WindClay.kt): the decoded-sheet cache went from 24 to 48 MB and counts a cap mask only when there is one (a 24-frame 256 px sheet and its mask are about 8 MB); the tinted-sheet cache went from 16 to 32 MB, and a still face (motion off, or any face under 64 dp) now tints only its still frame (256 KB), as 155 did on the web. Bitmap pixels live in native memory since Android 8, not in the Java heap. Robolectric: `WindClayTest.everyFamilyHas256PxFramesAndAStillFaceTintsOneFrame`.
- Emulator (emulator-5554, 420 dpi), PNGs in %TEMP%/g157/shots: the wind profile before (installed 1.101.0 APK, `before-profile.png`) and after (this branch, `after-profile.png`), side by side at 3x in `compare-profile.png`: the eyes, mouth and edges are sharp where they were soft. The home strip winds (clay, about 64 dp) in `compare-strip.png` (`before-chat.png` / `after-home.png`). Not done: the chat's corner wind (72 dp) after. The emulator was in a bad state that evening (2 GB of RAM, 870 MB swapped, frames of 1-2 s, ANRs in the app and then "Process system isn't responding"); an APK built from df6af48a with the 128 px assets hung the same way (`before-chatwind.png` shows it under the ANR dialog), and the dropbox ANR trace is the same as one from 20:46, before this work, the main thread waiting on the RenderThread in `HardwareRenderer.setStopped`. So no frame-time comparison either: on that emulator both builds were equally stuck. The criterion stays unticked and the task `blocked` until someone takes the chat-wind screenshot (and a frame-time check) on a healthy emulator or a phone; everything else is done.
