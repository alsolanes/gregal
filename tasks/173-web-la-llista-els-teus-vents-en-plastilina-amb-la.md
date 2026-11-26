---
title: "Web: la llista Els teus vents en plastilina amb la seva silueta"
status: done
depends_on: []
---

## Why

A la pàgina «Els teus vents» (`team()` a winds-workspace.js) cada fila dibuixa el vent amb `face(w,'figure',...)`, que és el dibuix vectorial. El dibuix només té les siluetes còdol, núvol i rínxol, així que un vent gota, sol o onada hi surt com un còdol, mentre que a l'inici i al xat surt amb la seva forma de plastilina. El mateix vent sembla dos personatges, a la pàgina on més es miren.

## Acceptance criteria

- [x] Les files de «Els teus vents» (tant la llista personal com la d'un equip) mostren el vent del paquet Cares en plastilina amb la seva silueta (o, n, s, g, u, w), amb la insígnia de l'ofici i l'estat d'espera (`waitMood`). Test node.
- [x] Les escenes que fan servir `'figure'` per a dibuixar el seu terra (el pati, `winds-pati.js`) segueixen igual. Test node existent o nou.
- [x] La mida de la cara a la fila es manté (no salta el disseny en carregar el full).
- [x] Comprovat al navegador amb un vent de cada silueta.

## Out of scope

Dibuixar les tres siluetes noves en vectorial. Android (la seva llista ja és plastilina des de la 167; si no ho és, apunta-ho a Notes).

## Notes

- `internal/web/app/winds-workspace.js:779` i `:786` (`face(w,'figure',waitMood(w))`), `internal/web/app/winds.js:948` (Cares: `large || !gregalCharacters ? drawn : play(drawn,w,true)`), `characters.js` `play`.
- Web: the two crew rows in `team()` ask `face(w, false, waitMood(w))` instead of `'figure'`, so `face()` in `winds.js` hands Cares (and the pelut) to the clay player as a still, the path of 167; no new code in the player. The drawn fallback (no sheets) is now the list face without the trade tool, as on the start page; a minairó row changes from the drawn full figure to the small drawn minairó of the start page (still not clay in lists, as before).
- Tests: `winds-start.test.cjs` "the rows of your winds and of a squad are list faces with their waiting mood, not the scene figure" (workspace layer, failed before the change); `characters.test.cjs` "a list face of a gota, sol or onada wind holds the still of its silhouette; a scene figure stays drawn" runs the real `winds.js` and covers the pati's `'figure'` (it already passed: it pins the `face()` behaviour the rows now rely on).
- Browser (sandbox on :8811, app served from disk): every row is a 58x58 `canvas.winds-face.winds-clay` (58x58 `svg` before), Garbí gota, Xaloc sol, Tramuntana onada, Mestral núvol with badge, Llevant rínxol. The waiting mood in the browser was not checked (no pending ask on that server); the node tests cover it.
- Android: the directory row (`DirectoryRow` in `VentsHomeParts.kt`, `WindHalo` 44 -> `WindFace` 52 dp, compact, not `clay`) already takes the small Cares clay path of 167 with `style.shape.key`, so gota, sol and onada show their own silhouette, with the badge.
- Follow-up: Android's `DirectoryRow` always draws its wind idle; the web row shows `waitMood` (waiting for a pending ask, news), Android does not.
