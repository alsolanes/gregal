---
title: "Android: Webs i Carpeta de treball endreçades"
status: done
depends_on: []
---

## Why

In Webs every card spends two lines on a raw share link (`http://127.0.0.1:8197/share/27d5b9b8558c…/`) that nobody reads, while the title is cut at one line. In the Working folder the paths are cut at both ends («…\Users\…\AppData\L…»), so the part that tells folders apart (the end) is lost, and the folder's avatar is two letters of its name («PR»).

## Acceptance criteria

- [x] Webs: a card no longer shows the full link; at most one short line (scheme dropped, long middle shortened, e.g. `127.0.0.1:8197/share/…2b86c61/`). «Open» and «Copy link» stay. The title may take two lines. A unit test of the shortening.
- [x] Working folder: paths keep their end and lose their start when they don't fit («…\projectes\flor-de-lluna»), in the header card, the project rows and the disk browser; the header avatar is a folder icon, not initials. A unit or Compose test of the path shortening.
- [x] Screenshots of both screens, dark and light, with synthetic paths (no real user names).

## Notes

- `Webs.kt` `WebRow` (line ~300) and `NamedWebRow`; `CarpetaScreen.kt` lines ~80–165 and `FilaCarpeta` (~289). Tests: `VentsWebsTest.kt`, `CarpetaTest.kt`.
- Webs: `shortLink` (Webs.kt) drops the scheme and, past 36 characters, keeps host + first segment + `…` + the last 8 characters. A tunnel card hides the link line when it says the same as the title. Card titles (`WebRow`, `NamedWebRow`) take up to 2 lines.
- Working folder: `camiCurt` now cuts at a separator (`…\projectes\flor-de-lluna`, not `…ectes\flor-de-lluna`). `CamiEnUnaLinia` (CarpetaScreen.kt) measures the monospace character width and asks `camiCurt` for as many as fit the line, with `TextOverflow.StartEllipsis` as a fallback. Header avatar is `Icons.Rounded.Folder`; `inicialsProjecte` and its test went away with it.
- Screenshots: `WebsCarpetaLookTest` serves synthetic JSON from a tiny local socket server and writes `android/app/build/webs-carpeta-look/{webs,carpeta}-{dark,light}.png`.
- Follow-up: `camiCurt` defaults `home` to a hard-coded personal home path; it should come from the `home` field of `/api/dirs`.
- Follow-up: CarpetaScreen derives a missing project name with `substringAfterLast('/')`, which returns the whole path for Windows paths.
- Follow-up: in Webs, a running app's action row (Open, Copy link, Restart, Stop, Remove) overflows 360 dp and «Remove» is only reachable by scrolling sideways.
