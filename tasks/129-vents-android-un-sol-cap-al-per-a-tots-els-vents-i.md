---
title: Vents Android: un sol capçal per a tots els vents i el vent reacciona amb el cos
status: done
depends_on: ["124", "128"]
---

## Why

Same as 128 on Android: one header for every wind, and the wind reacts with its body.

## Acceptance criteria

- [x] Every wind (Gregal included) has the same chat header: big face with mood, name, role chip in its colour, one overflow menu (Perfil, Fils, Desa a la wiki, Ordinador); tapping the face opens the profile. A Compose test.
- [x] The wind nods on send, leans in with a small burst in its colour when a result arrives and shakes on error. Motion respects reduced motion (web: prefers-reduced-motion; Android: LocalWindMotion). A unit test of the event to reaction mapping.
- [x] A failed turn shows as a wind bubble in the thread with a retry, ca+en. A Compose test. Robolectric frames or emulator screenshots (light, dark) in `output/ui-review/129/`.

## Out of scope

- Web (128).

## Notes

- Files: CompanyScreen.kt, VentsChatChrome.kt, WindFace.kt; strings in AppStrings.kt by hand.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
- From 124's audit: the doc editor toolbar stays Catalan when the app is in English (doc-page.js + Android bridge); fix here or note.
- Done (CompanyScreen.kt `WindConversation`/`WindBubble`, VentsChatChrome.kt, AppStrings.kt; WindFace.kt did not change). Tests in `WindHeaderReactTest`.
- Header: every wind chat on Android was already `WindConversation` + `VentsChatHeader` (Gregal included), so the structure was there. What changed: the «⋯» menu is Perfil, Fils, Desa a la wiki, L’ordinador de Gregal (new keys `app.vents.menu.profile|threads|wiki`, ca+en; the computer reuses `app.computer.name`), each with an icon, plus «Continua al servidor» only when a transfer is possible. «Vigila» (responsabilitats) left the chat menu to match the web; it is still in the drawer (`watch` panel), which opens the same sheet. While the wind sits in the corner the header's face slot folds to 0 dp (springs to 44 dp with the flight, snaps without motion), so the name does not leave a hole, as on the web. The header and tap-to-profile tests passed against the old code as well (that behaviour already existed); the menu ones failed on the labels.
- Reactions: `windReaction(WindEvent)` (SENT/APPROVED → NOD, REPLY → LEAN, FAILED/DECLINED → SHAKE) and `windTurnEvent(msgs)` (what the last message says when a turn ends; null when you stopped it before a reply). `WindPerch.react` replaces `nod`/`yes`; the gesture runs in its own `graphicsLayer` on the corner wind, only with `windMotionEnabled()`. Nod and shake keep 103's swings (vertical / horizontal, `WindFlightTest` pins them); lean tilts 8° towards the thread, moves 7 dp right and 4 dp up and grows 6 %. Every send in the chat (composer, chips, starters, card buttons, voice, continue, retry) goes through `sent()`, which nods. The burst is the face's existing DONE confetti (`windTurnCue`, in the wind's colour), which fires at the same moment the turn ends well; no second burst was added. Frames `lean-*` in `output/ui-review/129/` show the tilt and then the burst.
- Failed turn: `WindBubble`'s error card (sand warning) is now the wind speaking: the wind's byline above it, «No he pogut arribar-hi» / «I couldn’t get to it», the reason in small, and a «Torna-ho a provar» pill in the wind's colour (plus «Continua al servidor» when available). It is in every chat that shows error rows, groups included (no byline there).
- Frames (git-ignored) in `output/ui-review/129/`: `failed-{light,dark}`, `menu-{light,dark}` (popup drawn on top), `lean-{light,dark}` and `-2` (250 ms and 450 ms after a real busy → done turn). Not checked on a device or emulator; in the dark Robolectric lean frame a faint rectangle shows around the tilted clay face, which may be a Robolectric rasterising artefact.
- Follow-up: on an error the corner wind both shakes (this task) and shrugs (`windTurnCue` FAILED, 111); the web replaced its shrug with the shake. Decide whether the shrug should stay on the corner wind.
- Follow-up: the doc editor toolbar is still Catalan when the app is in English: doc-page.js has no language handling and the Android `DocPageBridge` passes no language; it needs doc-page.js i18n plus the app's language in the bridge or URL (Docs.kt/DocScreen.kt), outside this task's files.
- Follow-up: Gregal's web «⋯» menu adds «Què pot fer Gregal» (the guide); Android reaches the guide from the drawer and does not add it to the chat menu.
