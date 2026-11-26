---
title: Vents: la plastilina mira cap a tu
status: done
depends_on: []
---

## Why

OpenBot's figures turn their eyes toward the pointer, and that makes them feel alive. Our clay winds' eyes are baked into the sprite sheets, so they cannot follow anything. Only the code-drawn faces glance (`--wf-look`). We want a large clay wind in repose to look toward you.

## Acceptance criteria

- [x] `tools/characters/build.py` renders a `look` sheet per silhouette: 9 still frames with the eyes (and a slight head turn) toward centre, the four sides and the four diagonals. The masks are rendered too, and `manifest.json` lists the sheet. `go test ./internal/web -run TestCharactersManifestMatchesSheets` passes.
- [x] Web: a large clay face in `idle` shows the `look` frame nearest the pointer's direction from its centre. After 2 s without the pointer moving, or with the pointer outside the window, it goes back to its idle sheet. With reduced motion it does not follow. `characters.test.cjs` with a stubbed pointer.
- [x] Web: one pointer listener for all faces, and no drawing while the pointer is still. Node test.
- [x] Android: a large clay face in `idle` looks toward where you touch the screen while your finger is down, then goes back to idle. With reduced motion it does not follow. `WindClayTest`.

## Out of scope

Following the pointer in moods other than `idle`. Small faces. New silhouettes (170 renders `look` for its own).

## Notes

- Ask the user before rendering if a contact sheet of the 9 frames (`--preview`) does not look right.
- The sheets add about 9 frames per silhouette to `internal/web/app/characters/` and the APK. Say the size added in the final message.
- Files: `tools/characters/build.py`, `internal/web/app/characters.js`, `android/.../WindClay.kt`.
- Done: the `look` sheet is frame `3 * row + column`, row 0 up and column 0 left; frame 4 looks straight ahead (pointer or finger within a quarter of the face's width of its centre). `follow()` returns no swing for `look`, so each frame stands on its own.
- Budget: with `look` at QUALITY 82 the non-pelut sheets came to 3,720,745 bytes, over the 3.7 MB of `TestCharactersMinairoWithinBudget`. `look` is saved at `LOOK_QUALITY = 65` (pelut keeps `FUR_QUALITY`); compared at 256 px it looks the same as 82. Added to `internal/web/app/characters/` (and the APK, which copies the folder): 191,824 bytes across the 11 `look` files.
- Android: `Modifier.windTouch()` on MainActivity's root Box feeds `windTouch` (root coordinates) without consuming; touches in dialogs or popups (their own windows) are not followed. While looking, the face's `WindClayKey` is `<silhouette>/look/<frame>`. `WindClay.sheet` now takes the mood's sheet name (`windClayMood(mood)` or `look`).
- Contact sheet of the 9 frames for the five silhouettes: `output/ui-review/169/look-contact-256-light.png` (cut from the `motion-*` sheets of `--preview`, not committed).
- Follow-up: the non-pelut sheets are now 3,695,330 bytes, under 5 KB from their 3.7 MB budget; task 170's new silhouettes will need a budget of their own or a decision on this one.
