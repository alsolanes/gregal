---
title: Vents Android: els minairons tenen aspecte de minairó
status: todo
depends_on: []
---

## Why

When the person calls their helpers «minairons», the helpers should look like Catalan minairons, not generic blobs: it makes the app feel ours and fun.

## Acceptance criteria

- [ ] A new avatar pack «minairó»: a small round body in the wind's colour, a long soft white beard covering the lower face, a red barretina (Catalan cap: red tube folded to one side with a darker band), the same big expressive eyes and lids as the 110 redesign peeking above the beard, tiny feet; per-wind variety from the style/id (beard length, barretina fold side, a small accessory: a boxwood needle case, a sprig of yellow St John's wort, a tiny pebble). Readable at 24–28 px (barretina + beard silhouette), clay-like volume like 110. Android Canvas pack in `WindFace.kt` matching 117 (read its notes if it lands first); Robolectric frames small and large, dark and light, in `output/ui-review/118/`.
- [ ] Every mood works with the same extra character (barretina bobs, beard sways, jump on done). A unit test that each mood draws differently and motion off keeps it still.
- [ ] Offered in the avatar picker (PrefsScreen, 096) and the default when the collective is minairons (`VentsCollective.kt`) unless another pack was chosen. A test.
- [ ] Strings in Catalan and English.

## Out of scope

- Web (117).
- Clay render (119).

## Notes

- Files: `WindFace.kt`, `WindVolume.kt`, `PrefsScreen.kt`, `VentsCollective.kt`.
- References (2026-11-24): minairons (also manairons, menairons; diablorins in the Alta Ribagorça; femilians further south) are tiny Pyrenean helpers from Catalan, Aragonese and Andorran folklore, «diminuts com l'ull d'una agulla», thousands kept in a boxwood or reed needle case (canut d'agulles); when it opens they pour out like a swarm shouting «Què farem? Què direm?», tireless workers who must always be given work; born from the herba menaironera (St John's wort, yellow flowers). Modern illustrations (e.g. Apel·les Mestres, Pep Montserrat's «El sabater i els minairons») draw them as tiny bearded folk with a long white/grey beard and a red barretina, often next to mushrooms that look huge beside them. Sources: rodamots.cat/minairons, catalunyamagrada.cat (Els Minairons), muntanyesmagiques.blogspot.com (Llegenda dels minairons), en.wikipedia.org/wiki/Minairó. Our own design, no copied illustration.
