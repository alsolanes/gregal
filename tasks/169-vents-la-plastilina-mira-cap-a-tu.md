---
title: Vents: la plastilina mira cap a tu
status: todo
depends_on: []
---

## Why

OpenBot's figures turn their eyes toward the pointer, and that makes them feel alive. Our clay winds' eyes are baked into the sprite sheets, so they cannot follow anything. Only the code-drawn faces glance (`--wf-look`). We want a large clay wind in repose to look toward you.

## Acceptance criteria

- [ ] `tools/characters/build.py` renders a `look` sheet per silhouette: 9 still frames with the eyes (and a slight head turn) toward centre, the four sides and the four diagonals. The masks are rendered too, and `manifest.json` lists the sheet. `go test ./internal/web -run TestCharactersManifestMatchesSheets` passes.
- [ ] Web: a large clay face in `idle` shows the `look` frame nearest the pointer's direction from its centre. After 2 s without the pointer moving, or with the pointer outside the window, it goes back to its idle sheet. With reduced motion it does not follow. `characters.test.cjs` with a stubbed pointer.
- [ ] Web: one pointer listener for all faces, and no drawing while the pointer is still. Node test.
- [ ] Android: a large clay face in `idle` looks toward where you touch the screen while your finger is down, then goes back to idle. With reduced motion it does not follow. `WindClayTest`.

## Out of scope

Following the pointer in moods other than `idle`. Small faces. New silhouettes (170 renders `look` for its own).

## Notes

- Ask the user before rendering if a contact sheet of the 9 frames (`--preview`) does not look right.
- The sheets add about 9 frames per silhouette to `internal/web/app/characters/` and the APK. Say the size added in the final message.
- Files: `tools/characters/build.py`, `internal/web/app/characters.js`, `android/.../WindClay.kt`.
