---
title: "Mentre el model escriu una eina, la UI diu quina i quin fitxer"
status: done
depends_on: []
---

## Why

Un torn de Codi a la web va estar 4 min 42 s mostrant només «Pensant · 67k · 4m 42s» després de respondre una pregunta («Pixel art 2D retro»), sense dir què feia. El servidor ja compta els caràcters que arriben (`internal/web/progres.go`: `reasoning`, `text`, `tool`, i la cua del raonament a `tail`), però la UI ho resumeix tot com «Pensant». Quan els caràcters són arguments d'una eina (per exemple un `write` d'un joc sencer), l'usuari hauria de veure «Escrivint index.html · 67k» i no un «Pensant» mut.

## Acceptance criteria

- [x] L'event `progress` de la fase del model porta el nom de l'eina que s'està escrivint (`tool_name`) en quant se sap, i el fitxer objectiu (`tool_target`, el `path` o `file` dels arguments parcials) en quant apareix al JSON parcial. Test Go amb deltes d'arguments parcials.
- [x] La web (el xat de Codi i el de Vents, on es pinti aquest estat) diu «Escrivint <fitxer> · <mida>» / «Writing <file> · <size>» quan hi ha `tool` > 0 amb un `tool_target`, «Preparant <eina> · <mida>» sense fitxer, i ensenya la cua del raonament (`tail`) quan només raona. Test node.
- [x] El recompte diu clarament què compta: caràcters escrits, no tokens de context. Test node del format.
- [x] `docs/api-contract.md` documenta els camps nous. Si Android pinta aquest estat, digues a Notes si queda enrere (seria una tasca a part).

## Out of scope

Canviar quan s'emet `progress` o l'interval.

## Notes

- `internal/web/progres.go` (`delta`, `emetLocked`), `internal/llm/progress.go` (`StreamProgress`; potser cal passar el nom de l'eina o el tros d'arguments a més de la mida), el JS que pinta «Pensant · 67k · 4m 42s» (cerca les claus `act.now.*` a `internal/web/app/i18n.js` i qui les fa servir).
- Què era el «67k» de la captura: el recompte de caràcters de **raonament** de la crida en curs, no arguments d'una eina. A `index.html` el «Pensant · N» només sortia amb `tool` = 0 i `text` = 0; amb arguments d'eina hauria dit «Preparant una eina» (sense xifra). Per tant, durant aquells 4m42s el proveïdor no va enviar cap delta d'arguments: o el model escrivia el joc dins el raonament, o el proveïdor retenia la crida d'eina fins al final. Ara el recompte diu «67k caràcters» (no tokens), i quan sí que arriben arguments en directe, «Escrivint index.html · 67k caràcters» o «Preparant bash · 40 caràcters».
- Nou `llm.WithToolText(ctx, fn(nom, args))`: els parsers de streaming i de JSON sencer (OpenAI-compatible, Anthropic, Responses, Ollama) passen el nom de l'eina amb cada tros d'arguments. `progresTorn.eina` busca `"path"`/`"file"` amb el valor tancat en una finestra de 512 bytes del final dels arguments, perquè un `write` de 67k no es recorri sencer a cada delta.
- L'etiqueta es construeix a `internal/web/app/model-progress.js` (pur, amb test node); `fmtChars` hi ha passat des d'`index.html`.
- El xat de Vents (`winds-workspace.js`) no rep events `progress`, així que no hi ha res a pintar allà.
- Android queda enrere: `GregalViewModel.kt` (`progressLine`) pinta `progress` però no fa servir `tool_name` ni `tool_target`.
- Follow-up: Android, mostrar `tool_name`/`tool_target` a `progressLine` (tasca a part).
- Follow-up: si el model fa dues crides seguides a la mateixa eina (dos `write`), `tool_target` manté el primer fitxer fins que arriba el `path` del segon.
