---
title: "Web: la llista Els teus vents en plastilina amb la seva silueta"
status: todo
depends_on: []
---

## Why

A la pàgina «Els teus vents» (`team()` a winds-workspace.js) cada fila dibuixa el vent amb `face(w,'figure',...)`, que és el dibuix vectorial. El dibuix només té les siluetes còdol, núvol i rínxol, així que un vent gota, sol o onada hi surt com un còdol, mentre que a l'inici i al xat surt amb la seva forma de plastilina. El mateix vent sembla dos personatges, a la pàgina on més es miren.

## Acceptance criteria

- [ ] Les files de «Els teus vents» (tant la llista personal com la d'un equip) mostren el vent del paquet Cares en plastilina amb la seva silueta (o, n, s, g, u, w), amb la insígnia de l'ofici i l'estat d'espera (`waitMood`). Test node.
- [ ] Les escenes que fan servir `'figure'` per a dibuixar el seu terra (el pati, `winds-pati.js`) segueixen igual. Test node existent o nou.
- [ ] La mida de la cara a la fila es manté (no salta el disseny en carregar el full).
- [ ] Comprovat al navegador amb un vent de cada silueta.

## Out of scope

Dibuixar les tres siluetes noves en vectorial. Android (la seva llista ja és plastilina des de la 167; si no ho és, apunta-ho a Notes).

## Notes

- `internal/web/app/winds-workspace.js:779` i `:786` (`face(w,'figure',waitMood(w))`), `internal/web/app/winds.js:948` (Cares: `large || !gregalCharacters ? drawn : play(drawn,w,true)`), `characters.js` `play`.
