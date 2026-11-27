---
title: "Web Vents: l'engranatge diu Configuració i els grups s'ordenen per data"
status: done
depends_on: []
---

## Why

Dos caps de les tasques 180 i 181: l'engranatge del peu de la barra obre Configuració però l'etiqueta encara diu «Preferències», i el codi d'actualitzacions de l'escriptori la torna a posar a cada canvi d'estat. I sota «Converses» els grups no tenen data, així que surten després de les converses a soles en lloc de barrejar-s'hi per data.

## Acceptance criteria

- [x] A Vents, l'engranatge del peu té de nom accessible i `title` «Configuració» / «Settings», també després que el codi d'actualitzacions de l'escriptori canviï d'estat. A Codi es queda com està. Test node.
- [x] Sota «Converses», les converses a soles i els grups s'ordenen junts de més nova a més vella, fent servir la data de l'última activitat que el servidor ja dona (per exemple `ts` de `/api/winds/summaries`, o el que tingui cada grup). Un grup sense data va al final. Test node.
- [x] Comprovat al navegador amb converses a soles i grups barrejats.

## Out of scope

Android (ja ordena per data al calaix).

## Notes

- `internal/web/app/winds-workspace.js` (`renderThreads`, la fila «Mostra'n més» de la 180, l'engranatge del peu que va afegir la 181), el codi d'actualitzacions que reescriu l'etiqueta (cerca `prefsBtn`).
- Treball lleuger: `uv run scripts/check.py --fast` i els tests node; la comprovació sencera i la revisió es fan un sol cop per a tot el lot.
- Fet: a Vents (`gregalProduct==='vents'`), `winds-workspace.js` posa a l'engranatge la clau `nav.settings` a `data-i18n-title`/`data-i18n-aria`; `i18n.js` en canviar d'idioma i `desktop-updates.js` (que ara llegeix la clau del botó) hi tornen a posar «Configuració»/«Settings». A Codi el botó no es toca.
- Fet: `refreshSolo` també llegeix `/api/winds/summaries`; `renderThreads` ordena converses a soles (`at`) i grups (`ts` de `rot:<id>`) junts; sense data, al final.
- Comprovat amb Playwright al sandbox (%TEMP%/gvisual-186, port 8820) amb l'API de converses i grups simulada: ordre barrejat correcte i l'engranatge diu «Settings».
- Follow-up: després d'escriure en un grup no es torna a llegir `/api/winds/summaries` (només es refresca en carregar i en enviar a un vent a soles), així que el grup no puja fins a la següent càrrega.
- Follow-up: si la pàgina de Vents es munta dins de Codi, el clic de l'engranatge ja hi anava a la Configuració de Vents (181) tot i que l'etiqueta diu «Preferències».
