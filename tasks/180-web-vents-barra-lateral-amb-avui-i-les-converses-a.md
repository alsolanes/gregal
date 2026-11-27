---
title: "Web Vents: barra lateral amb Avui i les converses a dalt"
status: done
depends_on: []
---

## Why

A la barra lateral de Vents les converses queden a sota de Avui, Feina, Desat, El pati i tota la llista de vents, i per arribar-hi s'ha de baixar. «Configuració i més» és un desplegable que obre set opcions dins la barra, cosa que confon.

## Acceptance criteria

- [x] L'ordre de la barra (`renderNav` a `internal/web/app/winds-workspace.js`) és: Nova conversa, Cerca, Avui; després la secció Converses; després Vents (Gregal, els recents i Tots els vents); després Feina, Desat i El pati; després Projectes; i a baix de tot una sola fila «Configuració» / «Settings». Test node que llegeix l'ordre del DOM.
- [x] La fila «Configuració» és un botó normal, no un `details`: no desplega res a la barra i porta a la vista `settings` (la pàgina és la tasca 181; fins llavors pot obrir Preferències com ara). Queda marcada (`aria-current`) quan ets en qualsevol de les vistes de configuració (`apres`, `rules`, `library`, `wiki`, `memoria`, `publicacions`, `settings`). Test node.
- [x] La clau `gregal_vw_more` de localStorage deixa d'existir al codi. Test node o grep al test.
- [x] Comprovat al navegador a 1280 px: amb 6 converses, totes es veuen sense fer scroll.

## Out of scope

La pàgina de configuració (181). Android (182). La barra de baix del mòbil.

## Notes

- `internal/web/app/winds-workspace.js` ~120-170 (`renderNav`, el `details.vw-more`), `winds-workspace.css` (`.vw-more`). Els tests de la barra que ja hi ha (cerca `vw-nav` als `*.test.cjs`) poden esperar l'ordre vell: si canvia el requisit, digues-ho a les notes.
- Fet: la fila «Configuració» porta a la vista `settings`, que de moment és només el títol i una llista (`settingsView` a `winds-workspace.js`, classe `.vw-settings-list`) amb les set entrades que hi havia al desplegable: Preferències, Què sap de tu, Regles, Memòria del projecte, l'ordinador, Biblioteca i Wiki. La 181 ha d'omplir-la: icona, nom i línia de descripció per entrada, el fil «Vents / Configuració / …» o botó enrere dins les subseccions, i l'engranatge del peu de la barra cap a `settings`. La capçalera ja diu «Vents / Configuració».
- La fila queda marcada amb `SETTINGS_VIEWS`; per a Memòria i Comparticions (pàgines de l'app, no vistes dels vents) la barra es torna a pintar quan `memoriaPage` o `publicacionsPage` canvien `hidden`, com ja feia amb el Racó.
- Requisit canviat: els tests de `winds-start.test.cjs` que esperaven el desplegable (`details`, la fila de l'ordinador a la barra, `gregal_vw_more`) ara passen per la pàgina de Configuració; el del desplaçament en obrir el desplegable s'ha substituït pels dos tests nous.
- `gregal_vw_more` també s'ha tret de `profileKeys` (`internal/web/profile.go`).
- Comprovat al navegador a 1280×800 amb 6 converses: abans les tres últimes quedaven sota la vora (fins a 874 px), ara l'última acaba a 540 px.
- Follow-up: l'Android encara té «Configuració i més» (`Drawer.kt:249`, `VentsWebParityTest.kt`); és la 182.
