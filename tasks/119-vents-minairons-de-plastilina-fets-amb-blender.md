---
title: Vents: minairons de plastilina fets amb Blender
status: done
depends_on: ["117"]
---

## Why

When the person calls their helpers «minairons», the helpers should look like Catalan minairons, not generic blobs: it makes the app feel ours and fun.

## Acceptance criteria

- [x] `tools/characters/build.py` gains a «minairo» silhouette: clay body in the neutral tone + colour mask (tinted per wind), a white felt-like beard and a red barretina NOT in the mask (always red/white), the 110 eyes and lids, every mood with barretina/beard secondary motion; sheets in `internal/web/app/characters/minairo/` with the manifest updated, within the size budget (note the new total).
- [x] The web player (113) and Android player (114) use the minairó sheets for large faces when the wind's pack is minairó (from 117/118), with the code-drawn minairó as fallback. Tests in both clients.
- [x] Contact sheets of the minairó at 128 and 40 px, dark and light, in `output/ui-review/119/`, compared with the code minairó; keep the version that looks clearly better.

## Out of scope

- Other collectives' looks.

## Notes

- Blender 5.3 at `C:\Program Files\Blender Foundation\Blender 5.3lender.exe`; see `tools/characters/README.md`.
- References (2026-11-24): minairons (also manairons, menairons; diablorins in the Alta Ribagorça; femilians further south) are tiny Pyrenean helpers from Catalan, Aragonese and Andorran folklore, «diminuts com l'ull d'una agulla», thousands kept in a boxwood or reed needle case (canut d'agulles); when it opens they pour out like a swarm shouting «Què farem? Què direm?», tireless workers who must always be given work; born from the herba menaironera (St John's wort, yellow flowers). Modern illustrations (e.g. Apel·les Mestres, Pep Montserrat's «El sabater i els minairons») draw them as tiny bearded folk with a long white/grey beard and a red barretina, often next to mushrooms that look huge beside them. Sources: rodamots.cat/minairons, catalunyamagrada.cat (Els Minairons), muntanyesmagiques.blogspot.com (Llegenda dels minairons), en.wikipedia.org/wiki/Minairó. Our own design, no copied illustration.
- Done 2026-11-24. `tools/characters/build.py` adds the `minairo` silhouette (manifest family `minairo`, not a style letter: it stands for the pack whatever the wind's style). Body in the neutral clay under the mask; white felt beard with a two-lobed moustache, a rosy nose, and a red barretina (#C9342C, with the traditional wide black matte ribbon just above the eyes, tube folding over to the right and hanging down to the eyes), all outside the mask. 110 eyes and lids at ×0.84, mouth under the moustache. `follow()` gives the beard sway and the cap's sway/flop as two damped springs driven by the body's tilt, turn, hop and squash (pure per frame: loops are run twice first), so the tip flies on the done hop and bobs in working.
- Size budget: WebP keeps alpha lossless, which was about half of every sheet. The sheets' alpha is now rounded to 32 levels (no visible step), and everything was re-rendered with it: pebble 317,382, cloud 300,120, curl 354,220, minairo 431,470, manifest 680; total 1,403,872 bytes (was 1,232,366 for three silhouettes). `TestCharactersMinairoWithinBudget` keeps it under 1.5 MB.
- Players: `characters.js` plays the `minairo` sheets for a large face with `data-avatar="minairo"` (other packs and kits stay drawn); Android `WindFace` passes `"minairo"` as the family to `WindClayFace` when the pack is the minairó (it wears no kit). The drawn minairó stays until the sheet loads and when it is missing. Tests: node `a minairó plays the minairó clay sheets…`; Android `WindMinairoTest` `the clay player plays the minairo sheets` (which replaced «keeps the drawn minairo» from 117/118, the requirement changed) and `WindClayTest.aMinairoWithoutSheetsKeepsItsDrawing`.
- Review sheets (ignored by git): rounds `output/ui-review/119/r1..r5`, final in `output/ui-review/119/final/` (`contact-128|40-dark|light.png`, `motion-minairo-*`, and `compare-code-vs-clay-*` with the drawn minairó from 117 r6 on top and the clay one below, at full size and 40 px). Judgement: the clay one has real volume and reads as barretina + beard at 40 px; the drawn one is crisper and has the accessories. Kept the clay one for large faces, the drawn one everywhere else and as fallback.
- Not verified: the minairó in the real app on web or on a device; only the review sheets composite the tint, and the Android tests check which sheet plays.
- Follow-up: the clay barretina always falls to the right; the drawn one falls downwind (east winds left). Mirroring the canvas would flip the lighting; a mirrored render doubles the sheets.
- Follow-up: no accessories (needle case, St John's wort, pebble) in clay; each would need its own sheets or an overlay.
- Follow-up: the half-size mask shows small steps along the beard's edge when the sheet is scaled up past 128 px.
- Follow-up: `node --test internal/web/app/*.test.cjs` fails 16 tests in convs/export/plugins (`vm.SourceTextModule is not a constructor`, they need `--experimental-vm-modules`); unrelated to this task.
