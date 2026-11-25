---
title: Vents: animacions de plastilina on el vent fa servir objectes segons la feina
status: todo
depends_on: []
---

## Why

While it works, the big wind should do something with an object that matches the work, like the reference companions: it is the most visible place for fun.

## Acceptance criteria

- [ ] New clay animations rendered with Blender (`tools/characters/build.py`) for the work moods, each with a prop the wind handles: searching (a magnifier it sweeps), reading (a book or page it turns), writing (a pencil it scribbles with), computer (a small laptop it taps), thinking (a thought bubble or spinning gear), done (a tiny confetti pop). Each silhouette gets them (pebble, curl, cloud, minairo), with the body and barretina masks so colours still apply; total size budget noted and kept reasonable (target under +1.5 MB). Contact sheets in `output/ui-review/142/`.
- [ ] The web player (characters.js) and Android player (WindClay.kt) play the matching animation from the work kind of the live turn (the same mapping as `moodOf` / `windWorkMoods`), looping while the kind lasts and blending back to talk/idle. Tests on both for the mapping and that the prop sheet is chosen.
- [ ] Reduced motion shows a still frame with the prop. A test on both.

## Out of scope

- The Canvas/SVG faces (138/139 handle those).

## Notes

- Read tools/characters/README.md and tasks 114, 119 and 122 notes first.
- Blender 5.3 is at C:\Program Files\Blender Foundation\Blender 5.3\blender.exe; free only, no paid assets.
- Person, 2026-11-25: «fes que a les animacions, segons què faci vagi interactuant amb l'objecte, crea noves animacions blender si cal, ha de ser molt xulo. A més busca vies de proactivitat, arregla que es vegin els diferents estils de vent a android via servidor, i fes que funcioni la pestanya "avui" perquè no funcionava el temps ni els equips a seguir. Escrivia el que volia seguir però no sortia res ni canviava».
