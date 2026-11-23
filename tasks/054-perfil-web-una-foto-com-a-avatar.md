---
title: "Perfil (web): una foto com a avatar"
status: todo
depends_on: ["053"]
---

## Why

Initials are the fallback; most people want their own photo as their avatar.

## Acceptance criteria

- [ ] `POST /api/profile/photo` takes a JPEG, PNG or WebP image (up to 5 MB), stores a square copy of at most 256×256 in the account's state folder and answers its URL; `GET` serves it only to that account; `DELETE` removes it. A Go test: upload, fetch, another account gets 404, an oversized or non-image file gets 400, delete then 404.
- [ ] In Preferences, next to the colour, «Puja una foto» / «Upload a photo» and «Treu la foto» / «Remove photo». With a photo, the sidebar foot draws it round instead of the initials; without one, or if it fails to load, the initials of 053. A node test of the fallback.
- [ ] The photo travels with the account: another browser signed in as the same person shows it. Covered by the Go test above (stored per account, not in the browser).

## Out of scope

- Cropping in the browser (the server centres and crops to a square). Android (056).

## Notes

- Image decoding with `image/jpeg`, `image/png` and `golang.org/x/image/webp` (already in `go.mod`); resizing with `golang.org/x/image/draw`. No new dependency.
- Follow the per-account storage of `profile.go` (`profilePath`) and the size limits and checks of `winds_image.go`.
- Update `docs/api-contract.md`.
