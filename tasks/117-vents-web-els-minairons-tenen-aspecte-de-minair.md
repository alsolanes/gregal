---
title: Vents web: els minairons tenen aspecte de minairó
status: done
depends_on: []
---

## Why

When the person calls their helpers «minairons», the helpers should look like Catalan minairons, not generic blobs: it makes the app feel ours and fun.

## Acceptance criteria

- [x] A new avatar pack «minairó»: a small round body in the wind's colour, a long soft white beard covering the lower face, a red barretina (Catalan cap: red tube folded to one side with a darker band), the same big expressive eyes and lids as the 110 redesign peeking above the beard, tiny feet; per-wind variety from the style/id (beard length, barretina fold side, a small accessory: a boxwood needle case, a sprig of yellow St John's wort, a tiny pebble). Readable at 24–28 px (barretina + beard silhouette), clay-like volume like 110. Web SVG in `winds.js` as a pack; before/after screenshots small and large, dark and light, in `output/ui-review/117/`.
- [x] Every mood from 110 works on the minairó (idle, thinking, searching, reading, writing, working, discussing, waiting, done, failed, news); extra character: the barretina tip bobs and the beard sways with the motion; done = a little jump with the barretina flying; working = restless «què farem?» energy. A node test that each mood renders and differs, and reduced motion keeps it still.
- [x] The pack is offered in Preferences › Avatars (with its sample face, 096) and is the default when the collective name is minairons (`collective.js`), unless the person picked another pack. A node test.
- [x] Strings (pack name «Minairons»/«Minairons») in Catalan and English.

## Out of scope

- Android (118).
- Clay render (119).

## Notes

- Files: `winds.js` (packs, `face`, `avatarPrefs`, `themeChoices`), `winds.css`, `collective.js`.
- References (2026-11-24): minairons (also manairons, menairons; diablorins in the Alta Ribagorça; femilians further south) are tiny Pyrenean helpers from Catalan, Aragonese and Andorran folklore, «diminuts com l'ull d'una agulla», thousands kept in a boxwood or reed needle case (canut d'agulles); when it opens they pour out like a swarm shouting «Què farem? Què direm?», tireless workers who must always be given work; born from the herba menaironera (St John's wort, yellow flowers). Modern illustrations (e.g. Apel·les Mestres, Pep Montserrat's «El sabater i els minairons») draw them as tiny bearded folk with a long white/grey beard and a red barretina, often next to mushrooms that look huge beside them. Sources: rodamots.cat/minairons, catalunyamagrada.cat (Els Minairons), muntanyesmagiques.blogspot.com (Llegenda dels minairons), en.wikipedia.org/wiki/Minairó. Our own design, no copied illustration.
- Design (2026-11-24), for 118 (Android) and 119 (clay) to match. All in `winds.js` `minairo()` and `winds.css`; drawn in the 80 box, the whole figure `translate(4 3) scale(.9)` like Cares.
  - Body: an ellipse in the wind colour by `style[2]`: `o` round 24×23.5 at cy 49, `n` plump 26.5×22 at cy 50.5, `s` slim 22×25 at cy 48. Same clay as Cares: radial light→fill→deep (`mix(fill,#1B1530,.4)`), plus the `relief()` shade and rim only (no sheen/spec: the cap is on top), and a soft brow shadow under the cap. Feet: two 5.6×3.4 ellipses in the deep tone at ±.42 rx under the body.
  - Eyes: the 110 eyes and lids (`eyePair`, now shared with Cares) at x 40 ± .34 rx, y cy − 10, scaled ×.84. Poses, lids, smiles and springs are the Cares ones (`poses`, `wf-pose/wf-aim/wf-lid/wf-smile`).
  - Beard: white (`#FFFDF7` → `#E2DACD` downwards), from under the eyes to a scalloped point at y L (mouth style `n` 63.5, `s` 67.5, `o` 71, `t` 74.5; 67 under 28 px). Its widest point is .94 rx, so the wind colour shows on both sides. Three soft strands, a two-lobed moustache with a pale crease, a small mouth in a dark warm red (by mouth style; failed shows the worry wave), and a round rosy nose (`mix(fill,#E4577A,.5)`) on top.
  - Barretina `#C9342C`: a low crown, a dark band `mix(red,#1B1530,.55)` just above the eyes, and the tube rising off the crown, folding over and hanging by the head down to eye level. The hanging part is shaded darker and lies on the crown. It falls downwind: winds from the east half (0 < bearing < 180) to the left, the west half to the right, north and south by the seed. Authored for the round body and scaled to rx/24 wide.
  - Accessory by seed % 3, on the side away from the tip, only at 28 px and up: 0 a boxwood needle case (`#D9B26A`) standing by its feet, 1 a sprig of St John's wort (`#F4C431` flower, `#6C9A3E` stem) tucked in the band, 2 a grey pebble (`#A8A39B`) by its feet. No trade kits on the minairó.
  - Motion (winds.css): `.wf-tip` (the tube) bobs −3°/+5° around its fold (4 s; .8 s while working, writing or searching); `.wf-beard` sways ±2° from its top half a second behind (4 s; .4 s working). Working is `wf-restless`, a .8 s side-to-side hop with tilt («Què farem?»). Done plays the Cares hop with `wf-fly` (the tip flies up −40° and flops back) and `wf-flutter` (the beard lifts), once per transition. Reduced motion stills everything through the existing `.winds-face *` rule.
- Default pack: decided on the server, not in `collective.js`, because only the server knows whether the person ever picked a pack (`avatarTheme` was `cares` both when never chosen and when Cares was picked). `GET /api/winds` now answers `minairo` when no theme was saved and the collective is minairons or one of its aliases in `presetAliases` (which include «minairó» and English «helpers»); picking any pack, Cares included, saves it and wins. Tested by Go `TestMinairoIsThePackOfMinairons`. `companion.ValidAvatar` and `docs/api-contract.md` know `minairo`.
- The minairó is also a pack a wind can pick in Free mode; the exact pack lists in `winds.test.js` and `winds-avatar-picker.test.cjs` gained it.
- Android is left behind until 118: it can now receive `minairo` as theme or avatar and must draw it (or fall back to Cares).
- characters.js needed no change: it already keeps the SVG for any face with `data-avatar`; a test now pins that for the minairó until 119.
- Screenshots (24/28/48/120 px and a 240 px close-up, dark and light, every mood, motion frames; rounds r1–r6, r5/r6 final) in `output/ui-review/117/` (ignored by git). Not checked in the real app, only in a static page with the real winds.js/winds.css.
- Follow-up: at 28 px the accessory is a speck; hiding it under ~40 px may read cleaner.
- Follow-up: the mood badge in the top-right corner overlaps the barretina tip when it falls to the right.
