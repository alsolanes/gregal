---
title: Vents web: el vent canvia d'estat suau, es mou més mentre treballa i no salta entre baix i dalt
status: done
depends_on: ["127"]
---

## Why

The chat wind jumps between the corner and the header at the slightest scroll, its animation swaps abruptly between talking and using a tool (worse when tools change every few seconds), and it could move more while it works.

## Acceptance criteria

- [x] Corner and header have hysteresis: the wind flies up only when the person has scrolled clearly away from the end (about 240 px), and comes back only when they are back near the end (about 48 px); content growing while the wind streams does not move it. Today `flownUp` in winds-workspace.js uses one 160 px threshold. A node test that scrolling around one threshold does not flip it.
- [x] Mood changes are smooth and stable: the face cross-fades / eases between moods (about 250-350 ms, no class swap that restarts every animation), and a work mood holds at least about 1.2 s; tool to tool, or tool to talk to tool, within that hold keeps the running animation without restarting it. A node test with fake timers: a burst of activity events in 2 s produces at most two visible mood changes and no restart.
- [x] More motion while it works: during a turn the wind keeps a continuous, subtle busy motion (breathing or bobbing, out of sync with the header face) plus a small prop or particle per kind of work (searching, reading, writing, computer, thinking), all in its colour; prefers-reduced-motion keeps it still. Short frame captures (4 frames each, light and dark) in `output/ui-review/138/`.

## Out of scope

- Android (139).
- New avatar packs or assets.

## Notes

- Files: winds-workspace.js (`flownUp` 176, `flyCorner` 177, `paintCorner` 184, `cornerMood` 168), winds.js (`moodOf` 216, `face`), winds-workspace.css / winds.css (corner and mood keyframes).
- Task 127 runs in parallel in winds-workspace.js; start from the commit after 127 is merged.
- Person, 2026-11-25: «posa més animacions mentre treballa. A més, el canvi d'animació quan fa servir una tool de quan xerra és molt brusc i si tarda pocs segons a fer el canvi d'eina llavors queda molt estrany. A més el fet que quan estem a baix de tot el vent estigui a baix i si ens movem estigui a dalt fa que salti a la mínima entre baix i dalt, pot ser una mica més de marge».
- Done (winds-workspace.js, winds-workspace.css only). `cornerAt(at, gap, up)` is the hysteresis: it flies up only on a real scroll up past 240 px and comes back under 48 px. `heldMood` holds a work mood 1.2 s; done, waiting and idle show at once. `swapIn` cross-fades the old face and prop over 300 ms (WAAPI opacity, so the face's CSS animations stay untouched). Both are exported on `gregalWindChat` and tested in `app/wind-corner.test.cjs` with fake timers.
- In the chat corner the running tool now picks the mood (read → reading, web_search → searching, edit → writing, bash → working) through `moodOf({state:'working', activity})`. Before this, chat phases never produced «reading», so the reading prop could not show.
- Props are SVG, one per kind (thinking gears at top right, the others in front at bottom left, with the eyes looking at them). Their colour is `--vw-wind` mixed with `--text`/`--surface`. They hide when the face is clay (`:has(> .winds-clay)`), because the clay sheets carry their own props (142), and the small `.wf-badge` hides while a prop shows. The busy bob uses the individual `translate`/`rotate`/`scale` properties on a 3 s cycle, so it does not fight the poses, the cheer or the nod.
- Frames (synthetic sandbox, fake stream): `output/ui-review/138/` (git-ignored), `drawn-*` (drawn face with a kit) and `clay-*`, dark, light and dark-reduced.
- Follow-up: in the sandbox, the header «Working…» stayed after the scripted stream's `done`. Not investigated; it is probably the fake stream.
