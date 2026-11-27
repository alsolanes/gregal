---
title: Vents: paquet Pelut, vents peluts en 3D fets amb Blender (web i servidor)
status: done
depends_on: []
---

## Why

The person asked for a more 3D, furry avatar style in the avatar editor. Today the packs are Cares, Esperit, Cap, Rosa, Mapa and Minairons; only Cares' large faces and the minairons are 3D clay (Blender, tools/characters/build.py, tasks 112, 119, 142). A new pack «Pelut» («Furry» in English) gives every wind a soft furry 3D body, its own colour in the fur, rendered in Blender with real hair, and the avatar pickers show the packs as the 3D figures they are.

## Acceptance criteria

- [x] build.py renders a `pelut` silhouette for every mood (the props of 142 included): a round body covered in short fur (Blender hair curves or particle hair, Cycles), big eyes, small feet; its sheets and mask are in internal/web/app/characters/pelut/ and in manifest.json, and `go test ./internal/web -run TestCharactersManifestMatchesSheets` passes. The new sheets add at most about 400 KB.
- [x] `pelut` is a pack like `minairo`: the account theme (POST /api/winds/avatar-theme) and a wind's own `avatar` accept it, the server keeps it, and docs/api-contract.md lists it. A Go test.
- [x] On the web every wind in the Pelut pack is drawn with the pelut clay sheets in its colour, at every size (the small faces use the sheet's still frame or a matching furry SVG fallback while it loads); the avatar pickers (Preferences › Avatars and the wind editor in Free mode) list «Pelut» / «Furry» with ca and en strings. Node tests.
- [x] The avatar pickers show each pack's sample larger (about 64 px) and, for the clay packs (Cares, Minairons, Pelut), as the 3D clay figure, so the person sees the 3D style before choosing. A Node test and a Playwright screenshot in light and dark.

## Out of scope

- Android (task 156).
- Re-rendering the other families.

## Notes

- Follow task 119 (the minairó clay family) for the whole shape of the change: build.py SILHOUETTES and CAPS, the manifest, characters.js `play`, winds.js `packs`/`themeChoices`/`packFace`, winds_web.go and winds_names.go for the pack names.
- Render only the new family: `blender -b -P tools/characters/build.py -- --only pelut`, and look at the contact sheets (`--preview`) before committing; fur must read as fur at 128 px and as a fuzzy outline at 40 px.
- Blender 5.3 is installed; no new dependency.
- Change of plan from the coordinator during the task: the pelut renders at 256 px frames with a 128 px mask, so it looks sharp at a phone's 72-96 dp, and its budget became about 1.2 MB instead of 400 KB. The manifest has a per-silhouette `"sizes": {"pelut": [256, 128]}`, which characters.js reads; the other families stay at the global `size`/`maskSize` and were not re-rendered. `TestCharactersMinairoWithinBudget` keeps its 2 MB for the other families (pelut excluded), and `TestCharactersPelutAt256WithinBudget` holds pelut to 1.2 MB. The pelut sheets are 847,022 bytes (120,072 of them masks).
- Fur: particle hair on the body (2,500 parents × 12 interpolated children, `fur()`), Principled Hair BSDF in neutral grey, in the mask so it takes the wind's colour, short round the face so the eyes and mouth stay clear. The pelut's sheets keep 8 alpha levels and `FUR_QUALITY` 75 (the fringe is noisy anyway): with the defaults it cost almost twice as much.
- Web: winds.js draws an SVG pelut (swept tufts round a round body, eyes from `eyePair`) and hands it to `gregalCharacters.play`: large faces play the sheet, smaller ones hold the still frame. A scene figure (`face(w, 'figure')`, used by the pati) keeps the drawing because the pati changes moods on the SVG it holds. `play()` leaves alone a face already handed to it or already clay. The pickers show 64 px samples; Cares, Minairons and Pelut go through the clay player as a still figure without the wind's trade kit.
- Memory (from the review): a tinted 256 px sheet is megabytes, so a still face tints only its still frame, is drawn once outside the animation loop, and the tinted-sheet cache keeps the last 24 (`kept` in characters.js); the editor's colour picker no longer piles up sheets.
- Screenshots of the pickers (Preferences › Avatars and the Free-mode editor field, light and dark) were taken with Playwright against a sandbox server and looked at; they are not committed.
- Follow-up: Android (156) must read `sizes` from the manifest for the pelut; WindClay.kt today reads only the global `size`/`maskSize`.
- Follow-up: the pati figures (winds-pati.js) of every clay pack stay drawn; making them clay needs the pati to change moods through `gregalCharacters.mood`.
- Follow-up: `TestAvuiPrepCardForTheNextMeeting` (winds_avui_prep_test.go:74) fails when run around 22:00 local time: the prep card comes out `Held`, probably the night's quiet hours. It does not touch anything this task changed, but it turned `uv run scripts/check.py` red for this branch tonight.
