---
title: Vents: minairons de plastilina fets amb Blender
status: todo
depends_on: ["117"]
---

## Why

When the person calls their helpers «minairons», the helpers should look like Catalan minairons, not generic blobs: it makes the app feel ours and fun.

## Acceptance criteria

- [ ] `tools/characters/build.py` gains a «minairo» silhouette: clay body in the neutral tone + colour mask (tinted per wind), a white felt-like beard and a red barretina NOT in the mask (always red/white), the 110 eyes and lids, every mood with barretina/beard secondary motion; sheets in `internal/web/app/characters/minairo/` with the manifest updated, within the size budget (note the new total).
- [ ] The web player (113) and Android player (114) use the minairó sheets for large faces when the wind's pack is minairó (from 117/118), with the code-drawn minairó as fallback. Tests in both clients.
- [ ] Contact sheets of the minairó at 128 and 40 px, dark and light, in `output/ui-review/119/`, compared with the code minairó; keep the version that looks clearly better.

## Out of scope

- Other collectives' looks.

## Notes

- Blender 5.3 at `C:\Program Files\Blender Foundation\Blender 5.3lender.exe`; see `tools/characters/README.md`.
- References (2026-11-24): minairons (also manairons, menairons; diablorins in the Alta Ribagorça; femilians further south) are tiny Pyrenean helpers from Catalan, Aragonese and Andorran folklore, «diminuts com l'ull d'una agulla», thousands kept in a boxwood or reed needle case (canut d'agulles); when it opens they pour out like a swarm shouting «Què farem? Què direm?», tireless workers who must always be given work; born from the herba menaironera (St John's wort, yellow flowers). Modern illustrations (e.g. Apel·les Mestres, Pep Montserrat's «El sabater i els minairons») draw them as tiny bearded folk with a long white/grey beard and a red barretina, often next to mushrooms that look huge beside them. Sources: rodamots.cat/minairons, catalunyamagrada.cat (Els Minairons), muntanyesmagiques.blogspot.com (Llegenda dels minairons), en.wikipedia.org/wiki/Minairó. Our own design, no copied illustration.
