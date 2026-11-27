---
title: Vents: animacions de plastilina on el vent fa servir objectes segons la feina
status: done
depends_on: []
---

## Why

While it works, the big wind should do something with an object that matches the work, like the reference companions: it is the most visible place for fun.

## Acceptance criteria

- [x] New clay animations rendered with Blender (`tools/characters/build.py`) for the work moods, each with a prop the wind handles: searching (a magnifier it sweeps), reading (a book or page it turns), writing (a pencil it scribbles with), computer (a small laptop it taps), thinking (a thought bubble or spinning gear), done (a tiny confetti pop). Each silhouette gets them (pebble, curl, cloud, minairo), with the body and barretina masks so colours still apply; total size budget noted and kept reasonable (target under +1.5 MB). Contact sheets in `output/ui-review/142/`.
- [x] The web player (characters.js) and Android player (WindClay.kt) play the matching animation from the work kind of the live turn (the same mapping as `moodOf` / `windWorkMoods`), looping while the kind lasts and blending back to talk/idle. Tests on both for the mapping and that the prop sheet is chosen.
- [x] Reduced motion shows a still frame with the prop. A test on both.

## Out of scope

- The Canvas/SVG faces (138/139 handle those).

## Notes

- Read tools/characters/README.md and tasks 114, 119 and 122 notes first.
- Blender 5.3 is at C:\Program Files\Blender Foundation\Blender 5.3\blender.exe; free only, no paid assets.
- Person, 2026-11-25: «fes que a les animacions, segons què faci vagi interactuant amb l'objecte, crea noves animacions blender si cal, ha de ser molt xulo. A més busca vies de proactivitat, arregla que es vegin els diferents estils de vent a android via servidor, i fes que funcioni la pestanya "avui" perquè no funcionava el temps ni els equips a seguir. Escrivia el que volia seguir però no sortia res ni canviava».

### Done (142)

- `build.py`: in the six work moods the wind grows two clay hands (body mask, so they take its colour) and handles a prop that keeps its own colours (outside the mask): `searching` sweeps a brass magnifier, lifts it to the right eye and peeks through the see-through lens with that eye magnified (`zoom`), then dips it low; `reading` holds an open red book and turns a page with its right hand (wind-up squash, stretch on the lift, a happy settle); `writing` scribbles on a pad with a chunky yellow pencil, bobbing with each stroke; `working` (the "computer" kind: bash, scheduling, any other tool) types on a dark laptop on the ground, hands popping over the lid, then winds up and hits enter with a squash and a screen flash that lights its face; `thinking` keeps a hand on its chin and pushes two meshed cogs round in ratchet steps (wind-back, snap, overshoot); `done` winds back a party popper, pops it at the top of the hop and the confetti flutters down, four pieces landing on its head. `PROPS` names the prop and the still frame of each; `props()` builds them, `handle()` moves them from the same curves `pose()` uses for body and eyes (`lens`, `leaf`, `scribble`, `ratchet`, `TYPING`, `popper`). The minairó's cogs go beside its head (its barretina fills the top). Frames: searching 12→16, reading 16→20, working 8→12, done 16→20.
- Manifest: the work moods carry `prop` and `still`. Players: with reduced motion they hold `still` (web `frameOf`, Android `WindClaySheet.still` as the first frame when motion is off) instead of frame 0; `done` stills on its last frame (confetti landed). The mapping from work kind to sheet was already `moodOf` / `windWorkMoods` → `<silhouette>/<mood>`; the tests now pin it to the prop each kind shows, and that the face loops it and crossfades back to discussing/idle.
- Size: pebble 373,948, cloud 354,468, curl 412,838, minairo 456,090, total with the manifest 1,598,268 bytes (was 1,370,480: +227,788, well under the +1.5 MB target). `TestCharactersMinairoWithinBudget` now allows 2 MB (it allowed 1.5 MB: the task sets the budget at +1.5 MB, so the limit was raised on purpose, not to get green). The APK grows by about the same (WebP is stored).
- Tests: node `characters.test.cjs` «every work mood has a sheet with its prop…», «the clay wind plays the prop sheet of the work kind, loops it…», «reduced motion holds each sheet on its still frame…» (replaces «reduced motion shows the first frame…»: the requirement changed); `wind-face.test.cjs` «each work kind of a live turn plays the clay sheet with its prop»; Go `TestCharactersManifestMatchesSheets` checks `still` is a frame of the sheet; Android `WindClayTest.eachWorkKindPlaysTheSheetWithItsProp` and `motionOffHoldsTheStillFrameWithTheProp`.
- Review sheets (not committed): `output/ui-review/142/final/` (`contact-128|40-light|dark`, `motion-<silhouette>-*`, `minairons-*`), earlier rounds r1–r6 with big single frames. At 40 px the magnifier, book, laptop, cogs and popper read; the pencil and the confetti are small but visible.
- Not verified: in the real web app or on a device (only the review sheets, which composite the tint the same way the players do, and the player tests).
- Follow-up: hands are the body colour, so they read by shading only; a slightly darker hand tint would read better at 48 px.
- Follow-up: the laptop's lid hides the hands between taps (by design, they pop over it); a 3/4 view would show the keys but needs another layout.
- Follow-up: the props always sit on the same side (popper and pencil on the right), like the barretina (119 follow-up).
