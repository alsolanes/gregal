---
title: Vents: personatges de plastilina fets amb Blender, renderitzats en fotogrames
status: done
depends_on: ["110"]
---

## Why

For a Dots-level plush/clay look, render our characters in Blender (free) ahead of time and play them as sprite sheets, with the wind's colour applied at runtime.

## Acceptance criteria

- [x] A Blender Python pipeline in `tools/characters/` (run with `blender -b -P build.py`) models the 110 silhouettes as soft clay/plush bodies with eyes, lights them like a studio, animates every mood from 110 (same timing/signatures) and renders each mood as a short loop of frames (≤ 128 px, WebP with alpha) in a neutral body tone plus a colour mask, so one render serves every wind colour.
- [x] Output: `internal/web/app/characters/<silhouette>/<mood>.webp` sprite sheets plus a `manifest.json` (frames, fps, loop, size, mask). Total size budget stated in the notes and kept (aim < 1.5 MB for all).
- [x] Rendering is reproducible: same script → same frames (seeded); a README in `tools/characters/` says how to re-render and change a design. A test (Python or Go) checks the manifest matches the files.
- [x] Licence: the renders and scripts are the project's (Blender GPL does not apply to outputs); note it.

## Out of scope

- Players (113, 114).

## Notes

- Needs Blender installed on this machine (user installs it, e.g. `winget install BlenderFoundation.Blender`).
- Research on 2026-11-24 (no budget, open source only): Grok Bot's avatars are SVG paths moved by JavaScript (same tech as ours); what makes them good is design: one strong silhouette, expressive eyes carrying the expression, a distinct motion signature per state (idle calm and curious, thinking, working kicks into gear, waiting, blocked, done settles), springs and anticipation instead of constant loops, state detail on hover. Dots are 3D plush/clay characters with soft studio light. Our designs stay our own (no copies of their characters).
- User: «l'estètica ha de ser millor del que tenim, dots és molt millor, grok bot també» and chose both steps: redesign in code first, then a Blender-rendered clay version (free, GPL; renders are ours to redistribute).
- Done 2026-11-24. `tools/characters/build.py` (+ README) renders `internal/web/app/characters/`: 3 silhouettes x 11 moods, 66 WebP files plus `manifest.json`. Test: `internal/web/characters_test.go` (`TestCharactersManifestMatchesSheets`) checks every sheet and mask the manifest names exists with the right size, and that nothing else is in the folder.
- Size budget: 1.5 MB for everything. Now 1,232,366 bytes (pebble 400 KB, cloud 373 KB, curl 459 KB; masks 174 KB of that). Sheets are 128 px frames side by side, lossy WebP q82 with alpha; masks are half size (64 px), white with alpha, lossless. A new mood costs about 35-45 KB per silhouette.
- Render: Blender 5.3, Cycles on the CPU, 160 samples, no denoiser, dithering off; about 6 min for everything on this laptop (two passes per frame: colour, then mask). Re-render with `blender -b -P tools/characters/build.py`; contact sheets with `-- --moods "" --preview DIR`.
- Reproducible, with one limit: same script, same seed, same frames to the eye, but two runs can differ by one level in 255 at a few pixels (Blender-internal, also on one thread and with PYTHONHASHSEED fixed), so 3 to 6 of the 66 WebP files change bytes between runs. The denoiser and dithering made it much worse and are off. EEVEE on the GPU had the same 1-level noise; Cycles CPU was kept because it is as fast here and its bounce light is softer.
- Design, aligned with 110 (741d5ea7): silhouettes from the third style letter, `pebble` (o: broad shoulders, flat settled base), `curl` (s: the pebble, slightly smaller, with a gust crest breaking to one side) and `cloud` (n), all with two clay feet and one shared framing (same ground line and scale; room above for hops and the crest). Clay: neutral grey Principled with subsurface, sheen and a seeded noise bump (felt/plasticine grain), warm key, dim fill, cool rim, soft contact shadow. Eyes: big dark glossy ellipses with a white catchlight, clay upper lids hinged on the eye (close and slant: determined in working/writing, sad in failed), happy arcs in done; pink cheek blush the tint does not touch; mouth smile, small o (thinking, searching, discussing talk) and worried wave (failed).
- Moods at 12 fps (stop-motion pace): idle 24 (breathe, glance, blink), thinking 16 (slow tilt, eyes up), searching 12 (peers forward, fast scan), reading 16 (two lines and a return), writing 12 (head down, bob), working 8 (hard squash-bob, stepping feet, determined squint), discussing 16 (turns and talks), waiting 24 (leans toward the person, wide eyes looking out and down, slow blink), failed 20 once (flinch, droop, sigh, holds), done 16 once (anticipation squash, stretched hop, happy arcs, landing, wobble to rest), news 16 (hello hops). Moods are the winds.js list plus `waiting`.
- Tint: `multiply` the sheet by the wind colour where the (upscaled) mask is; the mask fades under the blush. Eyes, catchlights, mouth and shadow are never tinted. Described in the README and the manifest (`tint`).
- Licence: the script and renders are the project's (MIT); Blender's GPL does not cover its output. In the README.
- Review sheets (ignored by git): `output/ui-review/112/final/` (`contact-*`, `motion-<silhouette>-*`, `moods-128|40-dark|light.png`), earlier rounds in `output/ui-review/112/r1..r11`.
- Not verified: how the sheets look tinted inside the real app (players are 113/114); only the review sheets composite the tint.
- Follow-up: the 110 eye styles (`style[0]`: p, b, e star, m calm) and mouth styles are not rendered; every clay wind has the same eyes and mouth. Adding them multiplies the sheets (budget).
- Follow-up: byte-exact renders would need a different renderer setup or a post-pass; the 1-level differences only matter for diffs, not for the look.
