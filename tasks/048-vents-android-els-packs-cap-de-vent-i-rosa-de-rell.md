---
title: "Vents (Android): els packs Cap de vent i Rosa de rellotge"
status: todo
depends_on: ["046", "047"]
---

## Why

Android parity with task 047: the Cap de vent and Rosa de rellotge packs.

## Acceptance criteria

- [ ] `cap` and `rosa` appear in Preferences → Avatars and in the editor's picker (Free mode), and `WindFace` draws them when chosen. A Robolectric test.
- [ ] Cap de vent blows toward the opposite of the wind's direction and Rosa points to it, with the same direction as the web. Below 28 dp they drop the details as on the web. A unit test for the angle and a Compose test for the small variant.
- [ ] Motion follows `LocalWindMotion`: still when off, unless the wind is working. A Compose test.

## Out of scope

- Mapa de vent (050) and the blowing faces (051).

## Notes

- `WindFace.kt`. Reference: `bufador()` and `rosa()` in `mockups/vents-avatars.html`.
- Strings by hand in `Strings.kt`/`AppStrings.kt`. Set `ANDROID_HOME` for the tests.
