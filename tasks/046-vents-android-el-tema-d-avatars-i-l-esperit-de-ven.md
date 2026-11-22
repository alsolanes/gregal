---
title: "Vents (Android): el tema d'avatars i l'Esperit de vent"
status: done
depends_on: ["045"]
---

## Why

Android is the main client and has to allow the same as the web: choosing an avatar theme for every wind, or a pack per wind in free mode, with the Esperit de vent pack (task 045 builds the server side and the web).

## Acceptance criteria

- [x] Android reads `avatarTheme` and each wind's `avatar` from `GET /api/winds`, and draws every wind face with the theme's pack, or with each wind's pack in Free mode. A Robolectric test with a fake response checks which pack `WindFace` uses in each mode.
- [x] In Preferences, «Avatars» offers Cares, Esperit de vent and Free, and saves with `POST /api/winds/avatar-theme`. The wind creator/editor shows the pack picker only in Free mode and saves `avatar` with the wind. A Compose test for each.
- [x] Esperit de vent is drawn as on the web (tails toward the wind's direction, eyes looking the other way), with the same direction for the same wind. A unit test with the same table and hash as the web test.
- [x] Motion follows `LocalWindMotion` and «Moviment expressiu»: still when off, unless the wind is working. A Compose test with motion off.

## Out of scope

- The other packs: 048, 050, 051.
- Rotllanes keep the current stack of faces.

## Notes

- `WindFace.kt` (`WindStyle`, `drawWindBody`) and `Winds.kt` (parsing `/api/winds`). Preferences in `PrefsScreen.kt`, editor in `WindCreatorScreen.kt`.
- The direction function mirrors the one in `winds.js` (045): same names, same degrees, same hash of the id.
- Strings: edit `Strings.kt`/`AppStrings.kt` by hand, in Catalan and English. Set `ANDROID_HOME` for the gradle tests.

### What was built (2026-11-22)

- `Wind.avatar` is parsed from `GET /api/winds`; `GregalViewModel.avatarTheme` comes from `avatarTheme` (`avatarThemeOf`, unknown = `cares`) on every catalog load and on the offline copy, and resets to `cares` when the account changes. `MainActivity` provides it as `LocalAvatarTheme`.
- `WindStyle` carries `pack` (the wind's own avatar) and `bearing` (`windDirection`), filled by `Wind.faceStyle`, so no `WindFace` caller changed. `WindFace` asks `windPack(theme, style)` and draws `EsperitFace` for `esperit`.
- `windDirection` mirrors `direction()` in winds.js: the rose by name (NFD, marks `̀-ͯ` removed, trimmed, lowercase), otherwise `(id.hashCode() and 7) * 45`; an empty id hashes the slug of the name, so the editor preview points where the saved wind will.
- Esperit is the web's drawing in the same 100 box (tails path, gradients, blush, eyes offset away from the tails, mouth) plus the mood badge in its corner. Motion: the idle sway and drift (3 s) run only when the Cares face would move (`LocalWindMotion`, the system animator scale, not a small non-tappable face); while the wind works (thinking … discussing) the tails run at 0.75 s even with motion off.
- Preferences › Appearance: «Avatars» with `ChoicePills` (Cares, Esperit de vent, Free) → `vm.setAvatarTheme` → `POST /api/winds/avatar-theme`; the selection changes only once the server saved it, errors go through `companyError`.
- Editor: in Free mode a pack picker (`FaceOptions` with the two packs) and the draft carries `avatar`; with a theme a line names it and the draft's `avatar` is empty, so `WindRequests.save` omits it and the server keeps the wind's own.
- Strings `winds.avatars`, `winds.avatarsHelp`, `winds.avatar`, `winds.avatarCares`, `winds.avatarEsperit`, `winds.avatarFree`, `winds.avatarThemeInUse` in `AppStrings.kt` (ca and en, same keys and texts as the web). `Strings.kt` is generated and was not touched.

### Tests

- `WindEsperitTest`: the rose by name, the pinned ids (bit 225, teca 315, llevant-2 135, espurna 90) and the slug fallback; a fake server answering `GET /api/winds` in theme and Free modes, checking the theme, each wind's avatar, `windPack`, and that `WindFace` draws Cares in Free for a wind without avatar and something else under the Esperit theme; motion off: idle Esperit still, working Esperit's tails move.
- `WindAvatarThemeTest`: Preferences «Avatars» posts `{theme:"esperit"}` to a fake server and selects it; the editor in Free mode saves `avatar`, and with a theme names it, offers no pack and saves an empty `avatar` (omitted from the request).

### Not verified

- Not run on a device: only rendered in Robolectric (four winds pointing N, NE, E, SE looked like the mockup).
- Follow-up: Preferences shows the theme from the last `/api/winds` load; if the winds have not loaded in this session it shows Cares until they do (the web reloads `/api/winds` when the row opens).
- Follow-up: with a theme set, faces built from a style code alone (template gallery, eyes/mouth/shape options in the editor) also draw that pack, pointing north, so the Cares-specific choices preview as Esperit.
- `CrewProposalTest` compared `faceStyle` exactly; its expectation now includes the bearing `WindStyle` carries (from the name while the wind has no id). Stricter, not weaker.
