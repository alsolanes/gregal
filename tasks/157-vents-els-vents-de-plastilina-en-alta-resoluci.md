---
title: Vents: els vents de plastilina en alta resolució
status: todo
depends_on: ["155"]
---

## Why

On phones the clay winds look low resolution: the sheets are 128 px frames with a 64 px colour mask, while the chat wind is 72 dp and the profile 96 dp, 190 to 340 px on current screens, and the colour mask is stretched 3 to 5 times. Task 155 renders the new pelut family at 256 px with a 128 px mask; this does the same for pebble, cloud, curl and minairo.

## Acceptance criteria

- [ ] Every clay family is rendered at 256 px frames with a 128 px mask (and cap), the manifest says so, and `go test ./internal/web -run TestCharactersManifestMatchesSheets` passes; any per-silhouette size override added by 155 is gone if no longer needed.
- [ ] The web and Android players draw the 256 px sheets at every size with no other change of look (existing clay tests pass), and the Android caches hold a few winds in a few moods at the new size without decoding on every frame. Robolectric test for the frame size.
- [ ] On the emulator (420 dpi) the chat wind and the profile wind are sharp: a screenshot before and after in the task notes.

## Out of scope

- New poses or families.

## Notes

- Depends on 155 (it changes build.py and the manifest). Assets grow from about 1.8 MB to about 5-6 MB; say the final number in the notes.
- Render: `blender -b -P tools/characters/build.py -- --only pebble,cloud,curl,minairo` (about 25 min).
