---
title: "Web Vents: l'engranatge diu Configuració i els grups s'ordenen per data"
status: todo
depends_on: []
---

## Why

Dos caps de les tasques 180 i 181: l'engranatge del peu de la barra obre Configuració però l'etiqueta encara diu «Preferències», i el codi d'actualitzacions de l'escriptori la torna a posar a cada canvi d'estat. I sota «Converses» els grups no tenen data, així que surten després de les converses a soles en lloc de barrejar-s'hi per data.

## Acceptance criteria

- [ ] A Vents, l'engranatge del peu té de nom accessible i `title` «Configuració» / «Settings», també després que el codi d'actualitzacions de l'escriptori canviï d'estat. A Codi es queda com està. Test node.
- [ ] Sota «Converses», les converses a soles i els grups s'ordenen junts de més nova a més vella, fent servir la data de l'última activitat que el servidor ja dona (per exemple `ts` de `/api/winds/summaries`, o el que tingui cada grup). Un grup sense data va al final. Test node.
- [ ] Comprovat al navegador amb converses a soles i grups barrejats.

## Out of scope

Android (ja ordena per data al calaix).

## Notes

- `internal/web/app/winds-workspace.js` (`renderThreads`, la fila «Mostra'n més» de la 180, l'engranatge del peu que va afegir la 181), el codi d'actualitzacions que reescriu l'etiqueta (cerca `prefsBtn`).
- Treball lleuger: `uv run scripts/check.py --fast` i els tests node; la comprovació sencera i la revisió es fan un sol cop per a tot el lot.
