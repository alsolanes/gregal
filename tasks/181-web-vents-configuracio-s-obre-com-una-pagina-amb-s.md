---
title: "Web Vents: Configuració s'obre com una pàgina amb subseccions"
status: todo
depends_on: [180]
---

## Why

A les altres aplicacions, Configuració és una pàgina amb les seves seccions a dins. A Vents, les opcions eren un desplegable a la barra lateral, i Preferències era un diàleg a part.

## Acceptance criteria

- [ ] La vista `settings` és una pàgina amb títol «Configuració» / «Settings» i una llista de subseccions, cadascuna amb icona, nom i una línia que diu què hi ha: Preferències, Què sap de tu, Regles, Memòria del projecte, l'ordinador (`computerName()`), Biblioteca i Wiki. Cada entrada obre el que obre ara (la seva vista o Preferències). Test node.
- [ ] El botó d'engranatge del peu de la barra (al costat de «Tu») i la fila «Configuració» de la 180 porten a aquesta pàgina. Test node.
- [ ] Dins d'una subsecció hi ha una manera clara de tornar a Configuració (fil d'Ariadna «Vents / Configuració / …» a la capçalera o botó enrere). Test node.
- [ ] Funciona a 390 px: la pàgina és una llista d'una columna. Comprovat al navegador a 390 i 1280 px. Textos en ca i en.

## Out of scope

Moure el contingut de Preferències dins la pàgina (segueix sent el seu diàleg). Android (182).

## Notes

- `internal/web/app/winds-workspace.js` (`route`, la capçalera `vw-top` ~269 amb els noms de les vistes, `renderNav`).
