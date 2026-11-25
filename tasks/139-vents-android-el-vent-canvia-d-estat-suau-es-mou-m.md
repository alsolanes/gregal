---
title: Vents Android: el vent canvia d'estat suau, es mou més mentre treballa i no salta entre baix i dalt
status: done
depends_on: ["133"]
---

## Why

Same as 138 on Android: the chat wind flips between corner and header at the slightest scroll (`up = follow.behind`, which is `list.canScrollForward`), mood changes are abrupt, and it could move more while it works.

## Acceptance criteria

- [x] Corner and header have hysteresis: the wind goes up only when the person has scrolled clearly away from the end (about 200 dp) and comes back only near the end (about 40 dp); content growing while the wind streams does not move it. A unit test of the up/down decision over a scroll sequence.
- [x] Mood changes are smooth and stable: the face animates between moods (about 250-350 ms) instead of switching, and a work mood holds at least about 1.2 s; quick tool switches keep the running animation without restarting. A unit test with a virtual clock.
- [x] More motion while it works: a continuous subtle busy motion plus a small prop or particle per kind of work, in the wind's colour; LocalWindMotion off keeps it still. Robolectric frames in `output/ui-review/139/`.

## Out of scope

- Web (138).

## Notes

- Files: CompanyScreen.kt (`WindPresence` call 1443, `follow.behind` 2461), VentsChatChrome.kt (`WindPresence`, `windPresenceMood`), WindFace.kt (moods, `windWorkMoods`).
- Task 133 runs in parallel and touches CompanyScreen.kt and VentsChatChrome.kt; start after 133 is merged.
- Person, 2026-11-25: «posa més animacions mentre treballa. A més, el canvi d'animació quan fa servir una tool de quan xerra és molt brusc i si tarda pocs segons a fer el canvi d'eina llavors queda molt estrany. A més el fet que quan estem a baix de tot el vent estigui a baix i si ens movem estigui a dalt fa que salti a la mínima entre baix i dalt, pot ser una mica més de marge».
- Hysteresis: `windFliesUp` + `rememberWindUp` (VentsChatChrome.kt) replace `follow.behind` for the chat wind only; the «Últims» pill still uses `follow.behind`. It only flies up when the list itself scrolled (first visible index/offset changed), so a growing reply never lifts it. `pxFromEnd` counts unseen items below as the smallest visible one (they have no size yet), so it errs on staying down.
- Hold: `WindMoodHold` (1.2 s for work moods, also before DONE/FAILED/WAITING) is applied inside `WindPresence`, so the clay face changes less often too. Time is the frame clock (`withFrameMillis`), virtual in tests.
- Blend: `WindMoodBlend` replaces the two-mood spring; a 300 ms FastOutSlowIn tween blends pose, badge, breath and the work trail by weight, and a change halfway starts from the current weights (no jump).
- Busy motion: work moods add a slow float (2.3 s) and sway (3.7 s) to the pose; `drawWorkTrail` draws one accent-coloured particle set per work kind on Cares and minairó Canvas faces of 64 dp or more with motion on.
- WindBreathTest waited 100 ms for the breath to vanish after going idle; it now waits 400 ms because the breath fades out over 300 ms (the requirement of this task).
- Tests: WindSmoothTest (hysteresis sequence, virtual-clock hold, burst of tools, blend continuity), WindBusyMotionTest (real LazyColumn scroll sequence, frames, still without motion).
- Follow-up: the other avatar packs (esperit, cap, rosa, mapa) and the clay sheets get the hold and the hysteresis but not the work trail; the header face (`VentsChatHeader`) is not held.
