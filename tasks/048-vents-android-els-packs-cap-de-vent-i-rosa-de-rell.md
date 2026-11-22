---
title: "Vents (Android): els packs Cap de vent i Rosa de rellotge"
status: done
depends_on: ["046", "047"]
---

## Why

Android parity with task 047: the Cap de vent and Rosa de rellotge packs.

## Acceptance criteria

- [x] `cap` and `rosa` appear in Preferences → Avatars and in the editor's picker (Free mode), and `WindFace` draws them when chosen. A Robolectric test.
- [x] Cap de vent blows toward the opposite of the wind's direction and Rosa points to it, with the same direction as the web. Below 28 dp they drop the details as on the web. A unit test for the angle and a Compose test for the small variant.
- [x] Motion follows `LocalWindMotion`: still when off, unless the wind is working. A Compose test.

## Out of scope

- Mapa de vent (050) and the blowing faces (051).

## Notes

- `WindFace.kt`. Reference: `bufador()` and `rosa()` in `mockups/vents-avatars.html`.
- Strings by hand in `Strings.kt`/`AppStrings.kt`. Set `ANDROID_HOME` for the tests.

### What was built (2026-11-22)

- `windPacks` lists `cap` and `rosa`, so Preferences › Avatars (`ChoicePills`), the editor's Free picker (`FaceOptions`), `avatarThemeOf` and `windPack` take them with no other change. Strings `winds.avatarCap` («Cap de vent» / "Wind head") and `winds.avatarRosa` («Rosa de rellotge» / "Compass dial") in `AppStrings.kt`, same texts as the web's `i18n.js`.
- `WindFace` dispatches to `CapFace` and `RosaFace`, ports of `cap()` and `rosa()` in winds.js in the same 100 box (as `EsperitFace`), with the mood badge on top. Cap: head 11 toward the bearing (R 25, 28 small), breath group at the rim turned `bearing + 90` so it blows to `bearing + 180`, dashes 12/6; clipped to the face box like the web's SVG. Rosa: dial, ring, eight cream points, the wind's point turned by `bearing`.
- Small variant under 28 dp: Cap keeps the mouth and two solid thick lines; Rosa keeps only the wider wind point and a bigger centre.
- Rosa's 32 minute ticks: the web draws them only on the large figure (`large === true`); Android draws them from 80 dp, the same size from which Cares shows the kit.
- Motion as Esperit: the idle loop (breath flow 1.8 s, point sway ±5° over 4 s) runs only when `motion` (`LocalWindMotion`, animator scale, not a small non-tappable face); while the wind works (thinking … discussing) the breath flows at 0.7 s and the point swings ±10° at 1.4 s even with motion off.

### Tests

- `WindCapRosaTest`: Preferences offers both and posts `{theme:"rosa"}`; the editor in Free mode saves `cap`; `WindFace` draws three different faces for Cares, Cap and Rosa; Cap direction by pixels (Tramuntana blows south, Migjorn north, Ponent east, nothing behind the head); Rosa direction by the wind colour on the pointing side for N, E, S, W and not on the opposite side; 24 dp vs 28 dp (Cap's middle breath line, Rosa's east cardinal point); motion off idle still, working moves; motion on idle moves.
- The angle is tested through the rendering, not through a separate angle function.

### Not verified

- Not run on a device; rendered in Robolectric for the eight directions at 96, 40 and 24 dp and compared by eye with the web code.
- Follow-up: colours use Compose `lerp` (Oklab) where the web mixes in sRGB, as Esperit does, so Rosa's dark dial reads a little lighter than on the web.
- Follow-up: `rosePoint` fades each half separately; the web fades the point as a group, so the seam between halves may show a faint line on the faded points.
- Follow-up: the `image()` bitmap helper is now copied in `WindEsperitTest`, `WindBreathTest` and `WindCapRosaTest`.
- After rebasing on the 051 fix (list faces keep the breath still), the full Android suite passes.
