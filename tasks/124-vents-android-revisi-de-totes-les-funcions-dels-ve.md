---
title: Vents Android: revisió de totes les funcions dels vents i l'error 404 en canviar els avatars
status: done
depends_on: []
---

## Why

The person reports that some wind features on Android do not work; changing the avatar icons fails with a 404 (2026-11-24). Every route Android calls exists on the server (checked), so the 404 comes from a handler, a wrong method/query, or a path built at runtime.

## Acceptance criteria

- [x] Changing avatars on Android works: the account avatar theme (Preferences › Avatars), a wind's own pack in Lliure mode, and the minairó barretina colour all save and show, with no 404, against a server built from this branch. A test that reproduces the 404 first (Robolectric against a fake server, or a Go handler test) and passes after the fix.
- [x] Every wind feature on Android is exercised once on the emulator against a sandbox server and listed in the task notes as works / fixed / still broken (with why): wind list, create/edit wind, chat and stream, approvals, Feina/activity, shelf (calaix), Avui (weather, follows, calendar), routines, wiki, coneixements, memòria, docs (open, edit), shared chats/rotllanes, crew, notifications, comparisons pager, avatars/clay/minairons, settings.
- [x] Each bug found that is fixable in under ~80 lines is fixed with a test; bigger ones become notes with a proposed task.

## Out of scope

- Desktop/web (125).
- New features.

## Notes

- Emulator: AVD Medium_Phone_API_36; helpers in %TEMP%/g090/adb.sh (shot/ui/tapt/tap/typ; MSYS_NO_PATHCONV=1; screenshot coords ×1.2).
- Sandbox server: isolated GREGAL_DATA_DIR, GREGAL_AUTH, GREGAL_MEMORY, HOME, USERPROFILE, --config copy, GREGAL_API_TOKEN test token (see memory: project-gregal-sandbox-testing). Never touch the person's real config or sessions. g200 (%TEMP%/g200, port 18800, real models, lang ca) exists; build a fresh binary from the branch for it.
- Robolectric tests never hit 127.0.0.1:8097; use Conn("127.0.0.1","9","t") or a fake server. No captureToImage (times out).
- Android strings: edit AppStrings.kt by hand (ca+en); never run regen-strings.py.

### Results (2026-11-24, emulator against a sandbox server built from this branch, Vents debug build)

**The avatar 404.** Against a server built from this branch all three save and show with no 404: the account theme (Preferences › Appearance › Avatars: Wind spirit, then Free), a wind's own pack in Free mode (Minairons) and the barretina colour (purple, kept in `style`). The 404 comes from a server older than the app: `POST /api/winds/avatar-theme` was added on 2026-11-22 (task 045). A server built from `01f64755^` answers `404 {"error":"not found"}` to it, and `400 invalid request` to saving a wind with `avatar`; the phone showed «Companion: HTTP 404: {"error":"not found"}». Fixed: both answers now show `winds.avatarServerOld` («update Gregal on the server»), as the 400s from v1.91–1.92 already did. Test: `WindAvatarThemeTest` › `a server from before avatar themes says to update it instead of a raw 404`. The person's server still has to be updated for the avatars to save.

| Feature | Result |
|---|---|
| Wind list | works |
| Create / edit wind | works (Xaloc created; Gregal edited: Minairons + purple barretina) |
| Chat and stream | works (`/api/winds/chat/stream`) |
| Approvals | not triggered: in-project writes and even `rm` passed with no approval card (wind auto-allow); see follow-up |
| Feina / Activity / Automations / Watching | works |
| Shelf, Drawer (calaix) | works; the calaix section titles come from the server in Catalan only (follow-up) |
| Avui: weather, follows | works (Girona; following «Girona») |
| Avui: calendar | not tested: needs a connected Google account |
| Routines | works (create, turn on a suggested watch, run now) |
| Wiki | works (summary page written) |
| Coneixements | **fixed**: «+ New knowledge» sent POST and the server answered 405; it now sends PUT, as the contract says. Test: `WindConeixementsTest` › `saving a wind's knowledge is a PUT, the server answers 405 to a POST` |
| Memòria («What they remember about you») | works |
| «Become an expert» card | **fixed**: its title showed the raw key `app.vents.expertTitle` (the key had become `app.{vents}.expertTitle` in ca and en). Test: `AppStringsFormatTest` › `capClauPortaElNomDelsVents` |
| Docs (open, edit) | works (new document, typed text saved through `/blocks`); the editor toolbar is Catalan when the app is English (follow-up) |
| Rotllanes | works (a group of two winds answered) |
| Shared chats | 404 by contract: the sandbox has a shared token and no accounts; the app shows none |
| Crew | works (propose, start, the group's question arrives) |
| Notifications | works (routine news rings with the app in the background); during quiet hours it waits in Avui, as designed |
| Comparisons pager | works (two product cards, swipe to the second) |
| Avatars / clay / minairons | works (minairó with purple barretina in the chat header) |
| Settings | works (General, Appearance, Models, Permissions, Connections, Server) |

Every `/api/winds…` path Android calls was also probed with its method against the branch server: only coneixements had a method mismatch; the other 404s are the documented no-accounts ones (`shared`, `send`) or need a query id.

- Follow-up: the doc editor page (`doc-page.js` in the Android WebView) has no saved `gregal_idioma`, so it is always Catalan; the bridge could pass the app language (`DocPageBridge.lang()`) and the page store it before loading `docs-panel.js`. It touches the web asset (125), so it is left out here.
- Follow-up: a wind turn ran `rm` on a project file with no approval; check whether destructive commands should still ask under wind auto-allow.
- Follow-up: a routine that answers the silence marker `(res)` is still stored as a novetat (`/api/winds/novetats` lists `text: "(res)"`).
- Follow-up: server-side Catalan-only texts shown in English: calaix section titles (`winds_calaix.go`: «Classificació», «Equips que segueixes») and the built-in knowledge «Lliçons de feina».
- Follow-up: the memory learner wrote Spanish memories («Prefiere saludos en español») from an English chat on a `lang: ca` server.
- Follow-up: without accounts the app polls `GET /api/winds/shared` every ~20 s and gets a 404 each time; the web stops after the first 404.
- Follow-up: `VentsStartPageTest` › `la portada ensenya els vents i les converses recents, no un fil` fails between 00:00 and 01:00: its thread is `now.minusHours(1)`, which is yesterday then, so «Xaloc · Avui» is not shown. Seen in the full run at 00:00 on 2026-11-25; not touched by this task.
- Follow-up: tapping a wind on the home opens a new empty session each time («Company — Gregal», 0 messages), so empty sessions pile up in the wind's Sessions list.
