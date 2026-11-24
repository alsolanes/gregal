---
title: Vents Android: els minairons tenen aspecte de minairó
status: done
depends_on: []
---

## Why

When the person calls their helpers «minairons», the helpers should look like Catalan minairons, not generic blobs: it makes the app feel ours and fun.

## Acceptance criteria

- [x] A new avatar pack «minairó»: a small round body in the wind's colour, a long soft white beard covering the lower face, a red barretina (Catalan cap: red tube folded to one side with a darker band), the same big expressive eyes and lids as the 110 redesign peeking above the beard, tiny feet; per-wind variety from the style/id (beard length, barretina fold side, a small accessory: a boxwood needle case, a sprig of yellow St John's wort, a tiny pebble). Readable at 24–28 px (barretina + beard silhouette), clay-like volume like 110. Android Canvas pack in `WindFace.kt` matching 117 (read its notes if it lands first); Robolectric frames small and large, dark and light, in `output/ui-review/118/`.
- [x] Every mood works with the same extra character (barretina bobs, beard sways, jump on done). A unit test that each mood draws differently and motion off keeps it still.
- [x] Offered in the avatar picker (PrefsScreen, 096) and the default when the collective is minairons (`VentsCollective.kt`) unless another pack was chosen. A test.
- [x] Strings in Catalan and English.

## Out of scope

- Web (117).
- Clay render (119).

## Notes

- Files: `WindFace.kt`, `WindVolume.kt`, `PrefsScreen.kt`, `VentsCollective.kt`.
- References (2026-11-24): minairons (also manairons, menairons; diablorins in the Alta Ribagorça; femilians further south) are tiny Pyrenean helpers from Catalan, Aragonese and Andorran folklore, «diminuts com l'ull d'una agulla», thousands kept in a boxwood or reed needle case (canut d'agulles); when it opens they pour out like a swarm shouting «Què farem? Què direm?», tireless workers who must always be given work; born from the herba menaironera (St John's wort, yellow flowers). Modern illustrations (e.g. Apel·les Mestres, Pep Montserrat's «El sabater i els minairons») draw them as tiny bearded folk with a long white/grey beard and a red barretina, often next to mushrooms that look huge beside them. Sources: rodamots.cat/minairons, catalunyamagrada.cat (Els Minairons), muntanyesmagiques.blogspot.com (Llegenda dels minairons), en.wikipedia.org/wiki/Minairó. Our own design, no copied illustration.
- Blocked on the default (criterion 3): the backend rejects the pack and cannot say «never chosen». `companion.ValidAvatar` (internal/companion/winds.go) only accepts cares/esperit/cap/rosa/mapa, so POST /api/winds/avatar-theme {theme:"minairo"} (and a wind's own avatar "minairo") is a 400 today; and GET /api/winds returns `avatarTheme: "cares"` both when never chosen and when Cares was picked, so «minairons by default unless another pack was chosen» cannot tell an explicit Cares apart. A client-only rule (server "cares" + collective minairons → minairó) would make Cares impossible to pick while the helpers are called minairons, so it is not in this diff. Unblock with a backend task (shared with 117): accept "minairo" in ValidAvatar, report never-chosen (e.g. `avatarTheme: ""`) and update docs/api-contract.md; then the Android default is one line where MainActivity provides LocalAvatarTheme (and the same value as the picker's selection in PrefsScreen), using VentNom.plural.
- Done in this branch: "minairo" is in `windPacks`, so the Preferences picker offers «Minairons» with its sample face and posts it (tested against FakeGregal), and the editor offers it per wind in Free mode.
- Design (Android Canvas, `drawMinairo` in WindFace.kt, 80 box): a Cares clay body of R 25 at (40, 49) with its feet; red barretina = a soft tube leaning to the fold side whose top falls over and hangs by the ear, with a fold crease and a dark wine band; long lumpy white beard to a rounded point with a moustache; round rosy nose; the 110/111 eyes and lids (`drawWindEye`) and white brows that follow the lid and brow angle. The mood poses, springs, done hop, aura and badges are the Cares ones (the minairó goes through the Cares path of WindFace, not a separate composable). Secondary motion: the barretina tip bobs on its own clock per mood (fast while working), droops when failed, lifts when done/happy, trails the figure's vertical motion and the done hop (clamped to ±18° so it stays attached); the beard sways and stretches with the motion; the moustache curls up with the smile and droops on fail; a mouth peeks under it when talking or done. Per wind from the id (`minairoLook`): fold side, beard length (66/72/78; faces under 40 dp use at least 72 so the beard still reads), and from 56 dp an accessory: a boxwood needle case held at its side, a St John's wort sprig in the band, or a pebble at its feet. The status badge moves to the top-left corner when the barretina folds right.
- Clay: WindFace skips the clay player for the minairó pack, so it stays drawn until 119 adds sheets (tested).
- Frames: GREGAL_UI_REVIEW=output/ui-review/118/rN with WindMinairoTest; r1 (cap looked like a baseball cap, beard strands looked like teeth), r2 (tube + hanging tip, wool beard; tip flew off while working and on the hop), r3 (tapered crown, crease, clamped tip, longer small beards).
- 117 had no commits when this was done; match the web's look (colours, fold rule, `minairoLook` hash) when it lands.
- Follow-up: comparing two faces drawn at different positions is unreliable on Robolectric NATIVE graphics (gradient dithering depends on device position); WindMinairoTest compares at one spot.
- Unblocked on 2026-11-24 by task 117's server change: `minairo` is a valid pack and GET /api/winds returns it as the default for minairons when no theme was saved, so Android draws it by applying the server's theme. Follow-up: the Android design was made before 117 landed; align its fold rule (web: by wind direction) and proportions with the web.
