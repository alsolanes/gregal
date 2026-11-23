---
title: "Perfil (web): el teu nom i les teves inicials al peu de la barra lateral"
status: todo
depends_on: []
---

## Why

The bottom left of the sidebar shows the model name, which the composer already shows. That place belongs to the person: their avatar and their name, as in any app with accounts. This task does the name and an avatar of initials on a colour; the photo is 054.

## Acceptance criteria

- [ ] The account profile (`/api/profile`) keeps the person's name and avatar colour as two new keys in `profileKeys` (`gregal_user_name`, `gregal_user_color`), per account like the other preferences. A Go test: POST them for one account, GET returns them, another account does not see them, and a name over the length limit is rejected.
- [ ] Preferences has a «Com et dius» / «What's your name» field and a choice of avatar colours (the wind palette). Empty name: the account name from `/api/me`, or «Tu» / «You» in local mode with no accounts. A node test of the default name in the three cases (saved name, account name, local).
- [ ] `.side-foot` shows a round avatar with up to two initials of the name on the chosen colour (the text colour picked for contrast) and the name next to it, instead of `#endpoint`; clicking it opens Preferences at the name. The connection dot (`#statusDot`) stays, as a small dot on the avatar. In the compact sidebar only the avatar shows. A node test checks the initials («Aleix Solanes» → «AS», «anna» → «A», «Tu» → «T») and that the foot no longer holds the model name.
- [ ] Desktop and web look right in dark and light themes; a before/after screenshot in the task notes.

## Out of scope

- The photo (054). Android (055).
- The model name stays in the composer pill; nothing else about the composer changes.

## Notes

- `internal/web/index.html:1646` (`.side-foot`, `#endpoint`, `#statusDot`), `internal/web/app/shell-layout.js` (moves `keysBtn` and `prefsBtn` into the foot), `serve.go:3403` (fills `#endpoint`), `internal/web/profile.go` (`profileKeys`, `profileMaxValue`), `internal/web/app/prefs.js`, `/api/me` (`handleMe`, `serve.go:957`).
- Every user-facing string in Catalan and English.
- Update `docs/api-contract.md` with the two profile keys.
