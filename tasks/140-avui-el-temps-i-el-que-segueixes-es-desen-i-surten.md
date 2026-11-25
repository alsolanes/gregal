---
title: Avui: el temps i el que segueixes es desen i surten (web i Android)
status: todo
depends_on: ["135"]
---

## Why

In Avui the person types a team or thing to follow and nothing appears or changes, and the weather does not show. Their v1.94.0 server has `place: ""` and `follows: []` in /api/winds/avui/settings after trying, so the saves never landed.

## Acceptance criteria

- [ ] Reproduce first, against a sandbox server built from this branch, the exact flow on both clients: web/desktop Avui glance (vents-avui-glance.js) and Android Avui (VentsAvuiGlance.kt): type a team (e.g. «Barça») and a place (e.g. «Girona») and confirm. Find why nothing is saved or shown (request shape, missing confirm step, place search needing a pick, error swallowed). Write a failing test per client and one Go test if the server is at fault.
- [ ] After the fix, on both clients: adding a follow saves it (settings show it), the next matches appear in the glance (or a clear «no he trobat partits per X» if the search finds none), removing works, and setting a place shows its weather; any failure shows a message instead of nothing. Tests for each.
- [ ] Screenshots of web and Android Avui with a follow and the weather in `output/ui-review/140/`.

## Out of scope

- The Avui daily-brief redesign (134/135).

## Notes

- Files: internal/web/winds_avui.go (settings, follows, weather), vents-avui-glance.js, android VentsAvuiGlance.kt / VentsAvui.kt, docs/api-contract.md.
- Task 135 (Android Avui brief) is being merged first; start from after it.
- Never touch the person's real server data; use a sandbox (memory note project-gregal-sandbox-testing).
- Person, 2026-11-25: «fes que a les animacions, segons què faci vagi interactuant amb l'objecte, crea noves animacions blender si cal, ha de ser molt xulo. A més busca vies de proactivitat, arregla que es vegin els diferents estils de vent a android via servidor, i fes que funcioni la pestanya "avui" perquè no funcionava el temps ni els equips a seguir. Escrivia el que volia seguir però no sortia res ni canviava».
