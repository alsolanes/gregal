---
title: "Web Vents: barra lateral amb Avui i les converses a dalt"
status: todo
depends_on: []
---

## Why

A la barra lateral de Vents les converses queden a sota de Avui, Feina, Desat, El pati i tota la llista de vents, i per arribar-hi s'ha de baixar. «Configuració i més» és un desplegable que obre set opcions dins la barra, cosa que confon.

## Acceptance criteria

- [ ] L'ordre de la barra (`renderNav` a `internal/web/app/winds-workspace.js`) és: Nova conversa, Cerca, Avui; després la secció Converses; després Vents (Gregal, els recents i Tots els vents); després Feina, Desat i El pati; després Projectes; i a baix de tot una sola fila «Configuració» / «Settings». Test node que llegeix l'ordre del DOM.
- [ ] La fila «Configuració» és un botó normal, no un `details`: no desplega res a la barra i porta a la vista `settings` (la pàgina és la tasca 181; fins llavors pot obrir Preferències com ara). Queda marcada (`aria-current`) quan ets en qualsevol de les vistes de configuració (`apres`, `rules`, `library`, `wiki`, `memoria`, `publicacions`, `settings`). Test node.
- [ ] La clau `gregal_vw_more` de localStorage deixa d'existir al codi. Test node o grep al test.
- [ ] Comprovat al navegador a 1280 px: amb 6 converses, totes es veuen sense fer scroll.

## Out of scope

La pàgina de configuració (181). Android (182). La barra de baix del mòbil.

## Notes

- `internal/web/app/winds-workspace.js` ~120-170 (`renderNav`, el `details.vw-more`), `winds-workspace.css` (`.vw-more`). Els tests de la barra que ja hi ha (cerca `vw-nav` als `*.test.cjs`) poden esperar l'ordre vell: si canvia el requisit, digues-ho a les notes.
