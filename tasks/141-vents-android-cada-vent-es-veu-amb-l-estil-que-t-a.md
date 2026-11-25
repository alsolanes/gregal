---
title: Vents Android: cada vent es veu amb l'estil que té al servidor
status: todo
depends_on: []
---

## Why

The person says the different wind styles do not show on Android through the server: winds set up with their own style/pack (and the account avatar theme) on desktop look default on the phone.

## Acceptance criteria

- [ ] Reproduce first against a sandbox server built from this branch: set on desktop/web the account avatar theme and, in Lliure mode, different packs, shapes, eyes and barretina colours for several winds; open Android connected to that server and compare. Find where Android loses it (parsing of `style`/`avatar`/`avatarTheme` from GET /api/winds, the cached wind list in VentsOffline, refresh after a change, or the server not sending it). A failing test that shows it.
- [ ] After the fix every wind on Android looks as on desktop (pack, shape, eyes, mouth, kit, barretina) in the list, chat header, corner, Avui and notifications, and a change made on desktop shows on Android after a refresh. Tests, and side-by-side screenshots in `output/ui-review/141/`.

## Out of scope

- New styles.

## Notes

- Files: GregalViewModel.kt (`avatarThemeOf`, wind parsing), WindFace.kt (`faceStyle`, `windPack`), VentsOffline, internal/web winds GET handler.
- Task 124 found an old server answering 404 for avatar-theme; this is about a current server.
- Person, 2026-11-25: «fes que a les animacions, segons què faci vagi interactuant amb l'objecte, crea noves animacions blender si cal, ha de ser molt xulo. A més busca vies de proactivitat, arregla que es vegin els diferents estils de vent a android via servidor, i fes que funcioni la pestanya "avui" perquè no funcionava el temps ni els equips a seguir. Escrivia el que volia seguir però no sortia res ni canviava».
