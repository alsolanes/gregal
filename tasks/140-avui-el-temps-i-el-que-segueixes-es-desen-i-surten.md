---
title: Avui: el temps i el que segueixes es desen i surten (web i Android)
status: done
depends_on: ["135"]
---

## Why

In Avui the person types a team or thing to follow and nothing appears or changes, and the weather does not show. Their v1.94.0 server has `place: ""` and `follows: []` in /api/winds/avui/settings after trying, so the saves never landed.

## Acceptance criteria

- [x] Reproduce first, against a sandbox server built from this branch, the exact flow on both clients: web/desktop Avui glance (vents-avui-glance.js) and Android Avui (VentsAvuiGlance.kt): type a team (e.g. «Barça») and a place (e.g. «Girona») and confirm. Find why nothing is saved or shown (request shape, missing confirm step, place search needing a pick, error swallowed). Write a failing test per client and one Go test if the server is at fault.
- [x] After the fix, on both clients: adding a follow saves it (settings show it), the next matches appear in the glance (or a clear «no he trobat partits per X» if the search finds none), removing works, and setting a place shows its weather; any failure shows a message instead of nothing. Tests for each.
- [x] Screenshots of web and Android Avui with a follow and the weather in `output/ui-review/140/`.

## Out of scope

- The Avui daily-brief redesign (134/135).

## Notes

- Files: internal/web/winds_avui.go (settings, follows, weather), vents-avui-glance.js, android VentsAvuiGlance.kt / VentsAvui.kt, docs/api-contract.md.
- Task 135 (Android Avui brief) is being merged first; start from after it.
- Never touch the person's real server data; use a sandbox (memory note project-gregal-sandbox-testing).
- Person, 2026-11-25: «fes que a les animacions, segons què faci vagi interactuant amb l'objecte, crea noves animacions blender si cal, ha de ser molt xulo. A més busca vies de proactivitat, arregla que es vegin els diferents estils de vent a android via servidor, i fes que funcioni la pestanya "avui" perquè no funcionava el temps ni els equips a seguir. Escrivia el que volia seguir però no sortia res ni canviava».
- Reproduction (sandbox 127.0.0.1:18870 built from this branch, web driven with Playwright, Android VM with Robolectric against the same server): the request shapes were right and the saves landed on both clients. What made it look like "nothing happens": (1) Android's follow/place field did nothing on the keyboard's Enter/Done key, only the small «Segueix»/«Desa» button saved; (2) a place Open-Meteo cannot find (or slow/failing) left no weather and no reason (the web showed the generic «Digues on ets» again); (3) a follow whose search failed or found nothing stayed «buscant» forever; (4) Android swallowed save errors (reverted silently); (5) neither client looked again when the routine's answer arrived (30 s to a few minutes later), so a reload was needed.
- Fix: glance `weather_error`, follow `none`/`error` (server + contract), both clients show them, Android IME Done saves and shows the server's message on a failed save, and both clients look again every 15 s while something is pending (web waits while the form is open; Android stops after 20 tries).
- Not reproduced: why the person's real v1.94.0 server has `place:""` and `follows:[]`. The server code for settings is unchanged since v1.94.0 and saves fine in the sandbox; the Android Enter key (1) is the most likely cause. If it still happens after this, check which account/data dir the client writes to.
- Screenshots in output/ui-review/140/ are gitignored (output/): web from the live sandbox (real Open-Meteo and model answers), Android from Robolectric (`AvuiGlanceUiTest.fotogramaSeguimentsITemps*`, sample data).
- Follow-up: the scheduler runs one due job per 30 s tick, so with several new follows the last one can take minutes to get its first answer; a follow could run its first look right after being added.
- Follow-up: the Barça slug from a non-UTF-8 client is `bar-a` (seen with curl on Windows); harmless but the follow id is ugly.
