---
title: Vents web: treure la pàgina de vents antiga
status: done
depends_on: ["125", "126", "127", "128", "130", "132", "134", "136"]
---

## Why

The legacy wind page (`#ventsPage .winds-page`, the «Fem lloc…» desk) is hidden with `display:none!important` but its markup and code still ship and run.

## Acceptance criteria

- [x] The legacy markup is removed from index.html and the winds.js code used only by it is deleted; everything still used by winds-workspace.js (faces, packs, editor) stays. All node and Go tests pass; a test that `.winds-page` is gone.
- [x] The app loads with no console errors on every Vents page in light and dark (Playwright run noted in the task).

## Out of scope

- Any visual change.

## Notes

- Blocked on the diff guard (`uv run scripts/check.py --fast --base 23dfef8e`): «tests-removed: 20 more tests removed than added» and «size: 480 changed lines (limit 400)». The removed tests only exercised the deleted page; a human has to approve the removal or ask for a split (e.g. winds.js code in one task, markup + tests in another).
- Files: index.html, winds.js, winds-workspace.css:8.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
- Removed: the `.winds-page` markup in `#ventsPage` (the three scripts stay there: wind-i18n.js, wind-desk.js, winds.js), the `display:none!important` rule in winds-workspace.css, `$('windsListCard').hidden=true` in winds-workspace.js (the node is gone), and from winds.js the list, chat, profile, gallery, editor (`windForm`), conversation form, `loadWindsPage`, `gregalWindSetup`, `gregalWinds.create`/`messageFromHash` and the `#vent=` hash handler. Faces, packs, `avatarField`, `avatarPrefs`, news, `moodOf` and `palette` stay.
- Tests removed with the UI they tested (winds.test.js): the legacy list/chat/profile/gallery/editor/rotllana tests, including the barretina picker of the old editor. Rewritten against the API that stays: hostile faces, «Avatars» in Preferences, Cap/Rosa packs, watchNews. New: `the legacy wind page is gone`.
- Playwright run 2026-11-25, sandbox on port 18950 (%TEMP%/g137, `pages.cjs`): Inici, Avui, Feina, Desat, El pati, Vents, the editor, a profile, a wind chat and Preferences › Avatars, in light and dark: no console errors. The only network message is the 404 of `/api/winds/shared`, which winds-shared.js reads as «sharing off» on a server without shared chats (not touched here).
- Follow-up: the workspace editor (winds-workspace.js `createWind`) has no barretina picker; the old editor's was the only one on the web.
- Follow-up: wind-desk.js still carries the old desk (render, update, submit, routines dialog); only `flowFor` and `resolveTask` are used now. team.js still calls `gregalWindDesk.update` and looks for `#windsWorkApprovals`; attach.js still binds `#windWorkAttach`.
- Follow-up: winds.css and the `winds.*` keys in i18n.js keep the styles and strings only the old page used.
- Follow-up: onboarding.js focuses `#windChatInput` in Vents, which no longer exists; it should focus the workspace composer.
- Approved by the person on 2026-11-25 («Yes, merge 137»): removing the 20 legacy-page tests listed above, whose UI is gone. On merge, the small `select` helper came back in winds.js because task 153's barretina picker uses it.
