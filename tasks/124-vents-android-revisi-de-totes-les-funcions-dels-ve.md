---
title: Vents Android: revisió de totes les funcions dels vents i l'error 404 en canviar els avatars
status: todo
depends_on: []
---

## Why

The person reports that some wind features on Android do not work; changing the avatar icons fails with a 404 (2026-11-24). Every route Android calls exists on the server (checked), so the 404 comes from a handler, a wrong method/query, or a path built at runtime.

## Acceptance criteria

- [ ] Changing avatars on Android works: the account avatar theme (Preferences › Avatars), a wind's own pack in Lliure mode, and the minairó barretina colour all save and show, with no 404, against a server built from this branch. A test that reproduces the 404 first (Robolectric against a fake server, or a Go handler test) and passes after the fix.
- [ ] Every wind feature on Android is exercised once on the emulator against a sandbox server and listed in the task notes as works / fixed / still broken (with why): wind list, create/edit wind, chat and stream, approvals, Feina/activity, shelf (calaix), Avui (weather, follows, calendar), routines, wiki, coneixements, memòria, docs (open, edit), shared chats/rotllanes, crew, notifications, comparisons pager, avatars/clay/minairons, settings.
- [ ] Each bug found that is fixable in under ~80 lines is fixed with a test; bigger ones become notes with a proposed task.

## Out of scope

- Desktop/web (125).
- New features.

## Notes

- Emulator: AVD Medium_Phone_API_36; helpers in %TEMP%/g090/adb.sh (shot/ui/tapt/tap/typ; MSYS_NO_PATHCONV=1; screenshot coords ×1.2).
- Sandbox server: isolated GREGAL_DATA_DIR, GREGAL_AUTH, GREGAL_MEMORY, HOME, USERPROFILE, --config copy, GREGAL_API_TOKEN test token (see memory: project-gregal-sandbox-testing). Never touch the person's real config or sessions. g200 (%TEMP%/g200, port 18800, real models, lang ca) exists; build a fresh binary from the branch for it.
- Robolectric tests never hit 127.0.0.1:8097; use Conn("127.0.0.1","9","t") or a fake server. No captureToImage (times out).
- Android strings: edit AppStrings.kt by hand (ca+en); never run regen-strings.py.
