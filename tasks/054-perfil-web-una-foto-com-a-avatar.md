---
title: "Perfil (web): una foto com a avatar"
status: done
depends_on: ["053"]
---

## Why

Initials are the fallback; most people want their own photo as their avatar.

## Acceptance criteria

- [x] `POST /api/profile/photo` takes a JPEG, PNG or WebP image (up to 5 MB), stores a square copy of at most 256×256 in the account's state folder and answers its URL; `GET` serves it only to that account; `DELETE` removes it. A Go test: upload, fetch, another account gets 404, an oversized or non-image file gets 400, delete then 404.
- [x] In Preferences, next to the colour, «Puja una foto» / «Upload a photo» and «Treu la foto» / «Remove photo». With a photo, the sidebar foot draws it round instead of the initials; without one, or if it fails to load, the initials of 053. A node test of the fallback.
- [x] The photo travels with the account: another browser signed in as the same person shows it. Covered by the Go test above (stored per account, not in the browser).

## Out of scope

- Cropping in the browser (the server centres and crops to a square). Android (056).

## Notes

- Image decoding with `image/jpeg`, `image/png` and `golang.org/x/image/webp` (already in `go.mod`); resizing with `golang.org/x/image/draw`. No new dependency.
- Follow the per-account storage of `profile.go` (`profilePath`) and the size limits and checks of `winds_image.go`.
- Update `docs/api-contract.md`.
- Done: the photo is `profile-photo.png` next to `profile.json` (always PNG, so transparency survives). POST takes the raw image as the body (no multipart). Another browser finds it through `photo` in `/api/me` (`/api/profile/photo?v=<mtime ms>` or `""`); the same field comes back from POST/DELETE. Images over 50 MP are refused (400) before decoding, so a tiny PNG claiming a huge size cannot exhaust memory.
- Web: `user-avatar.js` fetches the photo with `api()` (it needs the token) into a blob URL; `face()` is the pure fallback rule. Prefs: a «Foto de l’avatar» / «Avatar photo» row after the colour, with upload and remove buttons; a 400 shows a localised message.
- Tests: `internal/web/profile_test.go::TestProfilePhotoIsPerAccountSquareAndRemovable`, `internal/web/app/user-avatar.test.cjs` («the avatar draws the photo…», «the status dot stays on top…»). The WebP decoder is registered but no test uploads a WebP (the standard library cannot encode one).
- Follow-up: Android (056) and the other clients (desktop uses the web; VS Code, Chrome, Telegram do not show the avatar) do not read `photo` from `/api/me` yet.
