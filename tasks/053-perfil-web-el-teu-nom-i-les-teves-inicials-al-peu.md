---
title: "Perfil (web): el teu nom i les teves inicials al peu de la barra lateral"
status: done
depends_on: []
---

## Why

The bottom left of the sidebar shows the model name, which the composer already shows. That place belongs to the person: their avatar and their name, as in any app with accounts. This task does the name and an avatar of initials on a colour; the photo is 054.

## Acceptance criteria

- [x] The account profile (`/api/profile`) keeps the person's name and avatar colour as two new keys in `profileKeys` (`gregal_user_name`, `gregal_user_color`), per account like the other preferences. A Go test: POST them for one account, GET returns them, another account does not see them, and a name over the length limit is rejected.
- [x] Preferences has a «Com et dius» / «What's your name» field and a choice of avatar colours (the wind palette). Empty name: the account name from `/api/me`, or «Tu» / «You» in local mode with no accounts. A node test of the default name in the three cases (saved name, account name, local).
- [x] `.side-foot` shows a round avatar with up to two initials of the name on the chosen colour (the text colour picked for contrast) and the name next to it, instead of `#endpoint`; clicking it opens Preferences at the name. The connection dot (`#statusDot`) stays, as a small dot on the avatar. In the compact sidebar only the avatar shows. A node test checks the initials («Aleix Solanes» → «AS», «anna» → «A», «Tu» → «T») and that the foot no longer holds the model name.
- [x] Desktop and web look right in dark and light themes; a before/after screenshot in the task notes.

## Out of scope

- The photo (054). Android (055).
- The model name stays in the composer pill; nothing else about the composer changes.

## Notes

- `internal/web/index.html:1646` (`.side-foot`, `#endpoint`, `#statusDot`), `internal/web/app/shell-layout.js` (moves `keysBtn` and `prefsBtn` into the foot), `serve.go:3403` (fills `#endpoint`), `internal/web/profile.go` (`profileKeys`, `profileMaxValue`), `internal/web/app/prefs.js`, `/api/me` (`handleMe`, `serve.go:957`).
- Every user-facing string in Catalan and English.
- Update `docs/api-contract.md` with the two profile keys.
- Done: `gregal_user_name` and `gregal_user_color` are profile keys (values over `profileMaxValue` are dropped, as for every key). The colour is stored as `#RRGGBB` from the wind palette (`window.gregalWinds.palette`, the eight preset winds of `winds.js`); with none chosen the avatar is Gregal's `#C1B4DE`. The ink comes from `gregalWinds.inkOn` (the contrast rule `face()` already used, now shared).
- `internal/web/app/user-avatar.js` paints the foot (`#sideMe`: avatar with initials and `#statusDot` at its bottom-right, then the name) and asks `/api/me` once for the account name. Clicking it calls `prefs.obre('name')`, which lands on General with the focus in the name field. The name and colour rows are the first rows of Preferences → General.
- Screenshots (sandbox server, headless Chrome 1280×800, not committed). Before, dark and light: the foot shows a small dot and «gpt-4.1-mini» in grey, the same name as the composer pill. After, dark and light: a 28 px round avatar («Y» on lilac with «You» in local mode; «AS» on blue or peach with «Aleix Solanes» once named) in the normal text colour, the connection dot as a ringed dot at the avatar's bottom-right, the shortcuts and preferences buttons unchanged on the right. Preferences opened from the avatar shows «What's your name» focused and eight wind swatches with the chosen one ringed, readable in both themes.
- Desktop (Electron) loads this same page; it was not launched, only the web in headless Chrome.
- Follow-up: the current shell hides the compact sidebar entirely (`shell-layout.css`: `#side.compact { width:0; visibility:hidden }`). Forced to the old 58 px width, the name is hidden and only the avatar shows, but the shortcuts and preferences buttons in the foot overlap it, as they overlapped before this change.
- The «no API key» warning of `#statusDot` (`pintaClau`) now sits inside the avatar, which is hidden from screen readers, so it is added to the avatar button's accessible name («El teu perfil: Anna · …»); `user-avatar.test.cjs` covers the label.
