---
title: Vents Android: el vent canvia d'estat suau, es mou més mentre treballa i no salta entre baix i dalt
status: todo
depends_on: ["133"]
---

## Why

Same as 138 on Android: the chat wind flips between corner and header at the slightest scroll (`up = follow.behind`, which is `list.canScrollForward`), mood changes are abrupt, and it could move more while it works.

## Acceptance criteria

- [ ] Corner and header have hysteresis: the wind goes up only when the person has scrolled clearly away from the end (about 200 dp) and comes back only near the end (about 40 dp); content growing while the wind streams does not move it. A unit test of the up/down decision over a scroll sequence.
- [ ] Mood changes are smooth and stable: the face animates between moods (about 250-350 ms) instead of switching, and a work mood holds at least about 1.2 s; quick tool switches keep the running animation without restarting. A unit test with a virtual clock.
- [ ] More motion while it works: a continuous subtle busy motion plus a small prop or particle per kind of work, in the wind's colour; LocalWindMotion off keeps it still. Robolectric frames in `output/ui-review/139/`.

## Out of scope

- Web (138).

## Notes

- Files: CompanyScreen.kt (`WindPresence` call 1443, `follow.behind` 2461), VentsChatChrome.kt (`WindPresence`, `windPresenceMood`), WindFace.kt (moods, `windWorkMoods`).
- Task 133 runs in parallel and touches CompanyScreen.kt and VentsChatChrome.kt; start after 133 is merged.
- Person, 2026-11-25: «posa més animacions mentre treballa. A més, el canvi d'animació quan fa servir una tool de quan xerra és molt brusc i si tarda pocs segons a fer el canvi d'eina llavors queda molt estrany. A més el fet que quan estem a baix de tot el vent estigui a baix i si ens movem estigui a dalt fa que salti a la mínima entre baix i dalt, pot ser una mica més de marge».
