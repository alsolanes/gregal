---
title: Vents web: els minairons tenen aspecte de minairó
status: todo
depends_on: []
---

## Why

When the person calls their helpers «minairons», the helpers should look like Catalan minairons, not generic blobs: it makes the app feel ours and fun.

## Acceptance criteria

- [ ] A new avatar pack «minairó»: a small round body in the wind's colour, a long soft white beard covering the lower face, a red barretina (Catalan cap: red tube folded to one side with a darker band), the same big expressive eyes and lids as the 110 redesign peeking above the beard, tiny feet; per-wind variety from the style/id (beard length, barretina fold side, a small accessory: a boxwood needle case, a sprig of yellow St John's wort, a tiny pebble). Readable at 24–28 px (barretina + beard silhouette), clay-like volume like 110. Web SVG in `winds.js` as a pack; before/after screenshots small and large, dark and light, in `output/ui-review/117/`.
- [ ] Every mood from 110 works on the minairó (idle, thinking, searching, reading, writing, working, discussing, waiting, done, failed, news); extra character: the barretina tip bobs and the beard sways with the motion; done = a little jump with the barretina flying; working = restless «què farem?» energy. A node test that each mood renders and differs, and reduced motion keeps it still.
- [ ] The pack is offered in Preferences › Avatars (with its sample face, 096) and is the default when the collective name is minairons (`collective.js`), unless the person picked another pack. A node test.
- [ ] Strings (pack name «Minairons»/«Minairons») in Catalan and English.

## Out of scope

- Android (118).
- Clay render (119).

## Notes

- Files: `winds.js` (packs, `face`, `avatarPrefs`, `themeChoices`), `winds.css`, `collective.js`.
- References (2026-11-24): minairons (also manairons, menairons; diablorins in the Alta Ribagorça; femilians further south) are tiny Pyrenean helpers from Catalan, Aragonese and Andorran folklore, «diminuts com l'ull d'una agulla», thousands kept in a boxwood or reed needle case (canut d'agulles); when it opens they pour out like a swarm shouting «Què farem? Què direm?», tireless workers who must always be given work; born from the herba menaironera (St John's wort, yellow flowers). Modern illustrations (e.g. Apel·les Mestres, Pep Montserrat's «El sabater i els minairons») draw them as tiny bearded folk with a long white/grey beard and a red barretina, often next to mushrooms that look huge beside them. Sources: rodamots.cat/minairons, catalunyamagrada.cat (Els Minairons), muntanyesmagiques.blogspot.com (Llegenda dels minairons), en.wikipedia.org/wiki/Minairó. Our own design, no copied illustration.
