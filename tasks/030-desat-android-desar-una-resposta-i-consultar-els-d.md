---
title: "Desat (Android): desar una resposta i consultar els desats"
status: todo
depends_on: [028]
---

## Why

Same as task 029 on the phone, where the person mostly cooks and plans: keep a reply with one tap and find it again from «Desat» in the drawer.

## Acceptance criteria

- [ ] A wind reply's actions (where copy and share are) include «Desa»; it calls `/api/notes` with the reply text and `kind:"auto"`, then shows «Desat ✓» with an action to open it. A ViewModel test with a fake server checks the request.
- [ ] `DesatScreen` starts with the saved items: search field, chips per kind present (from `kinds`, localised), pinned first, rows with title, kind and date; `WindSavedGallery` stays below. A Compose test renders it with fake data (screenshot in light and dark).
- [ ] The note view renders cards (`Rich.Card`, `Rich.Result`, `Rich.Chart`) instead of showing their JSON as code, so a saved recipe looks like in the chat. A Compose test with a recipe note checks it.
- [ ] New strings in Catalan and English (`AppStrings.kt` by hand).

## Out of scope

- The wind's proposal (task 032).
- Editing the card inside a note (the note editor stays plain markdown).

## Notes

- `VentsActivity.kt`: `DesatScreen` (line ~439), `WindSavedGallery`. `Notes.kt`: note view uses `MdPageBody` (`WikiScreen.kt:170`), which handles only `Rich.Inline`/`Rich.Code`; `CompanyScreen.kt:2096-2124` shows how the chat draws the other blocks.
- Reply actions: copy at `CompanyScreen.kt:1302` and `:1610`.
- Tasks 025 and 027 are touching chat files on other branches: keep the change to the reply actions small and rebase on what has landed.
- Run tests with `ANDROID_HOME` set (memory: project-android-tests-sdk).
