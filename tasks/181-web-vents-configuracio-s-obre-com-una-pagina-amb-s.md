---
title: "Web Vents: Configuració s'obre com una pàgina amb subseccions"
status: done
depends_on: [180]
---

## Why

A les altres aplicacions, Configuració és una pàgina amb les seves seccions a dins. A Vents, les opcions eren un desplegable a la barra lateral, i Preferències era un diàleg a part.

## Acceptance criteria

- [x] La vista `settings` és una pàgina amb títol «Configuració» / «Settings» i una llista de subseccions, cadascuna amb icona, nom i una línia que diu què hi ha: Preferències, Què sap de tu, Regles, Memòria del projecte, l'ordinador (`computerName()`), Biblioteca i Wiki. Cada entrada obre el que obre ara (la seva vista o Preferències). Test node.
- [x] El botó d'engranatge del peu de la barra (al costat de «Tu») i la fila «Configuració» de la 180 porten a aquesta pàgina. Test node.
- [x] Dins d'una subsecció hi ha una manera clara de tornar a Configuració (fil d'Ariadna «Vents / Configuració / …» a la capçalera o botó enrere). Test node.
- [x] Funciona a 390 px: la pàgina és una llista d'una columna. Comprovat al navegador a 390 i 1280 px. Textos en ca i en.

## Out of scope

Moure el contingut de Preferències dins la pàgina (segueix sent el seu diàleg). Android (182).

## Notes

- `internal/web/app/winds-workspace.js` (`route`, la capçalera `vw-top` ~269 amb els noms de les vistes, `renderNav`).
- Fet: cada entrada de `settingsView` és un `button.vw-settings-row` amb icona, `strong` (nom) i `small` (què hi ha). Les línies de Preferències i de l'ordinador són les de l'Android (182); les altres són noves. Memòria del projecte porta ara la icona `notes` (abans repetia la de la Wiki).
- Preferències obre el diàleg amb `gregalPanels.prefs.obre()` (com la paleta), no fent clic a `#prefsBtn`: aquest botó, a Vents, ara porta a Configuració. S'intercepta amb un `click` en captura al document, perquè guanyi l'`onclick` de prefs.js sigui quin sigui l'ordre d'arrencada.
- Fil: a Què sap de tu, Regles, Biblioteca i Wiki la capçalera diu «Vents / Configuració / …» i «Configuració» és un botó (`.vw-crumb`) que hi torna. Memòria del projecte i Comparticions són pàgines de l'app: ja tenien el seu «Torna» (`projectMemoryBack`), que torna a Configuració perquè la vista dels vents no ha canviat.
- Requisit canviat: dos tests de l'ordinador a `winds-start.test.cjs` comparaven tot el text del botó amb el nom; ara la fila porta també la descripció i comparen el `strong`.
- Comprovat al navegador (sandbox, 1280×800 i 390×844): l'engranatge obre Configuració (no el diàleg), les set files en una columna (390: 350 px d'ample, sense desplaçament lateral), «Winds / Settings / Rules» i el botó hi torna, el «Back» de la memòria hi torna i la fila Preferències obre el diàleg.
- Follow-up: l'engranatge encara es diu «Preferències» / «Preferences» (`prefs.title`, i `desktop-updates.js` el torna a posar en cada estat d'actualització); a Vents hauria de dir «Configuració».
- Follow-up: amb una actualització a punt (`has-update`), l'engranatge ara porta a Configuració i cal un clic més a Preferències per arribar a Actualitzacions (el `#desktopUpdateNotice` continua instal·lant directament).
- Follow-up: si es clica l'engranatge mentre una conversa es prepara (`route()` torna d'hora), el clic no fa res.
