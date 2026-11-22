---
title: "Vents (web): la pastilla de l'ofici al costat del nom de cada vent"
status: todo
depends_on: []
---

## Why

In Vents you can't see what each wind is good at without opening its profile. The user liked the mockup where every wind's name has a small pill with its trade («Compara», «Webs», «Notícies»), tinted with the wind's colour (decided 2026-11-22 on `mockups/vents-layout.html`). That is the quality level they want.

## Acceptance criteria

- [ ] Wherever the web client shows a wind's name next to its face, there's a pill with the wind's `role` in the wind's colour: the sidebar wind rows, the start page faces (under the name), the chat header and the «All winds» directory. A node test with the fake DOM checks the pill text and colour in each of the four places.
- [ ] A wind with a `kit` shows that kit's icon in the pill. A wind without a kit shows the pill with no icon, and a wind with an empty `role` shows no pill. A node test covers all three cases.
- [ ] The pill text and its contrast are readable in both the light and dark themes: the text colour mixes the wind's colour with the theme's text colour, as in the mockup. A screenshot in each theme is enough.

## Out of scope

- Android: task 044.
- A new short-label field on the wind. `role` is already the short UI subtitle («música», «notícies»). If a role turns out too long, the pill truncates it with an ellipsis.
- Filters by trade in the conversation list (a follow-up once the list exists).

## Notes

- Places: `renderNav` channel rows and `home()` faces in `internal/web/app/winds-workspace.js`, the chat header (`owner()` in the same file), and the directory (`route('crew')`).
- Kit names: `kits` in `internal/web/app/winds.js` (24 kits: shopping, books, music, finance, cooking…). The icon map kit → icon goes next to the existing nav icons, with no new dependency.
- Mockup: `mockups/vents-layout.html`, the `.tag` style.
