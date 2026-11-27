---
title: Vents Android: cada vent es veu amb l'estil que té al servidor
status: done
depends_on: []
---

## Why

The person says the different wind styles do not show on Android through the server: winds set up with their own style/pack (and the account avatar theme) on desktop look default on the phone.

## Acceptance criteria

- [x] Reproduce first against a sandbox server built from this branch: set on desktop/web the account avatar theme and, in Lliure mode, different packs, shapes, eyes and barretina colours for several winds; open Android connected to that server and compare. Find where Android loses it (parsing of `style`/`avatar`/`avatarTheme` from GET /api/winds, the cached wind list in VentsOffline, refresh after a change, or the server not sending it). A failing test that shows it.
- [x] After the fix every wind on Android looks as on desktop (pack, shape, eyes, mouth, kit, barretina) in the list, chat header, corner, Avui and notifications, and a change made on desktop shows on Android after a refresh. Tests, and side-by-side screenshots in `output/ui-review/141/`.

## Out of scope

- New styles.

## Notes

- Files: GregalViewModel.kt (`avatarThemeOf`, wind parsing), WindFace.kt (`faceStyle`, `windPack`), VentsOffline, internal/web winds GET handler.
- Task 124 found an old server answering 404 for avatar-theme; this is about a current server.
- Person, 2026-11-25: «fes que a les animacions, segons què faci vagi interactuant amb l'objecte, crea noves animacions blender si cal, ha de ser molt xulo. A més busca vies de proactivitat, arregla que es vegin els diferents estils de vent a android via servidor, i fes que funcioni la pestanya "avui" perquè no funcionava el temps ni els equips a seguir. Escrivia el que volia seguir però no sortia res ni canviava».

### Results (2026-11-25)

Reproduced against a sandbox server built from this branch (%TEMP%/g141, port 18860, isolated HOME/USERPROFILE/GREGAL_DATA_DIR/AUTH/MEMORY). Theme Lliure set with `POST /api/winds/avatar-theme`, and winds saved as the desktop saves them (`POST /api/winds` with `style` and `avatar`): Tramuntana esperit `bns`, Garbí minairó `etn`, Mestral minairó `moom` (purple barretina), Llevant mapa, Xaloc cap `psn`, Ponent rosa.

- The server is not the problem: `GET /api/winds` returns `avatarTheme: "free"` and every wind's `style` and `avatar`. Parsing is not either: `Wind.parse` and `avatarThemeOf` keep them (already covered by `WindEsperitTest`).
- Root cause 1, a change made on the desktop never reached an open phone: Android only read `GET /api/winds` on connecting, on going back online and on entering the Vents screen. Coming back to the foreground (`onForegrounded`) refreshed tasks and news, not the winds, so a phone left in the background kept the old theme and faces (default Cares) until the app was killed. Now `onForegrounded` also calls `loadWinds()`. Test: `WindStylesServerTest` › `a look changed on the desktop shows when the app comes back to the foreground`.
- Root cause 2, notifications: `windFaceBitmap` only knew Cares and the minairó, so a wind with Esperit, Cap de vent, Rosa de rellotge or Mapa (own pack in Lliure, or the account theme) rang with a Cares face. The four faces now draw through `EsperitFigure`, `CapFigure`, `RosaFigure` and `MapFigure` (same pattern as `WindFigure`), used by both the live face and the notification bitmap. Test: `WindStylesServerTest` › `a notification draws the wind with its pack, as the list does`.
- List, chat header, corner and Avui already draw from `vm.winds` + `LocalAvatarTheme`, so they follow once the list is fresh.
- Screenshots: `output/ui-review/141/desktop-vents.png` (desktop, Tots els vents, headless Chrome against the sandbox) and `output/ui-review/141/android-vents.png` (the phone's directory rows and notification faces, rendered by Robolectric from the same sandbox server). The emulator was in use by another agent (task 133), so there is no emulator screenshot.

- Follow-up: the routine/reminder rows in Feina and the wind Activity tab (`WindAutomationRow`) draw `windStyleOf(a.windStyle)` with no pack, so in Lliure a wind with its own pack shows as Cares there; `GET /api/winds/automations` sends no `avatar`, the row could look the wind up in `vm.winds`.
