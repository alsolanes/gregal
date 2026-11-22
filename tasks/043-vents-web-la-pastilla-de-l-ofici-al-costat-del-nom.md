---
title: "Vents (web): la pastilla de l'ofici al costat del nom de cada vent"
status: done
depends_on: []
---

## Why

In Vents you can't see what each wind is good at without opening its profile. The user liked the mockup where every wind's name has a small pill with its trade («Compara», «Webs», «Notícies»), tinted with the wind's colour (decided 2026-11-22 on `mockups/vents-layout.html`). That is the quality level they want.

## Acceptance criteria

- [x] Wherever the web client shows a wind's name next to its face, there's a pill with the wind's `role` in the wind's colour: the sidebar wind rows, the start page faces (under the name), the chat header and the «All winds» directory. A node test with the fake DOM checks the pill text and colour in each of the four places.
- [x] A wind with a `kit` shows that kit's icon in the pill. A wind without a kit shows the pill with no icon, and a wind with an empty `role` shows no pill. A node test covers all three cases.
- [x] The pill text and its contrast are readable in both the light and dark themes: the text colour mixes the wind's colour with the theme's text colour, as in the mockup. A screenshot in each theme is enough.

## Out of scope

- Android: task 044.
- A new short-label field on the wind. `role` is already the short UI subtitle («música», «notícies»). If a role turns out too long, the pill truncates it with an ellipsis.
- Filters by trade in the conversation list (a follow-up once the list exists).

## Notes

- Places: `renderNav` channel rows and `home()` faces in `internal/web/app/winds-workspace.js`, the chat header (`owner()` in the same file), and the directory (`route('crew')`).
- Kit names: `kits` in `internal/web/app/winds.js` (24 kits: shopping, books, music, finance, cooking…). The icon map kit → icon goes next to the existing nav icons, with no new dependency.
- Mockup: `mockups/vents-layout.html`, the `.tag` style.
- Built: `tag(w)` in `winds-workspace.js` makes the pill (`.vw-tag`, `--c` = wind colour) and is used in the sidebar wind rows, the start page faces, the chat header (`owner()`) and both lists of the «All winds» directory (`team()`). The icon comes from `kitIcon(w.kit)`, 24 stroke icons next to `navIcon`, which now shares `strokeIcon()` with it. `/api/winds` already sends `kit` (`kitOf`), so no backend change.
- The pill text is `roleName(w)`, the same text the client already showed as the role: the wind's `role`, or the squad specialist's name in the UI language. No new strings.
- The pill replaces the old role subtitle in the chat header (`<small>`) and in the directory (`<p>`), so the role is not shown twice; their CSS went away. Below 600 px the chat header hides the pill, as it hid the old subtitle.
- Start page faces are now as wide as their pill (68–148 px) instead of a fixed 68 px, otherwise every role was cut to three letters; longer roles still end in an ellipsis.
- Tests: `internal/web/app/winds-start.test.cjs` («every wind name next to its face carries a pill…», «the pill shows the kit’s icon…»). The fake DOM's `style.setProperty` now keeps the value so the colour can be checked.
- Verified by screenshots (Playwright, page served from disk with API fixtures, not kept in the repo) in the light and dark themes: home, chat header and directory. Measured light-theme text contrast is about 4.1–4.2:1 for the orange/beige winds, a bit under WCAG AA 4.5:1 for small text; dark theme is well above.
- Not verified: the real server and a real wind list; Electron desktop; the mobile layout.
- Follow-up: in the light theme the mockup's 50 % mix gives about 4.1:1 contrast for pastel winds; a 60 % mix with `--ink` would reach AA if that matters.
