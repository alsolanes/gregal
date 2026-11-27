---
title: "Mentre el model escriu una eina, la UI diu quina i quin fitxer"
status: todo
depends_on: []
---

## Why

Un torn de Codi a la web va estar 4 min 42 s mostrant només «Pensant · 67k · 4m 42s» després de respondre una pregunta («Pixel art 2D retro»), sense dir què feia. El servidor ja compta els caràcters que arriben (`internal/web/progres.go`: `reasoning`, `text`, `tool`, i la cua del raonament a `tail`), però la UI ho resumeix tot com «Pensant». Quan els caràcters són arguments d'una eina (per exemple un `write` d'un joc sencer), l'usuari hauria de veure «Escrivint index.html · 67k» i no un «Pensant» mut.

## Acceptance criteria

- [ ] L'event `progress` de la fase del model porta el nom de l'eina que s'està escrivint (`tool_name`) en quant se sap, i el fitxer objectiu (`tool_target`, el `path` o `file` dels arguments parcials) en quant apareix al JSON parcial. Test Go amb deltes d'arguments parcials.
- [ ] La web (el xat de Codi i el de Vents, on es pinti aquest estat) diu «Escrivint <fitxer> · <mida>» / «Writing <file> · <size>» quan hi ha `tool` > 0 amb un `tool_target`, «Preparant <eina> · <mida>» sense fitxer, i ensenya la cua del raonament (`tail`) quan només raona. Test node.
- [ ] El recompte diu clarament què compta: caràcters escrits, no tokens de context. Test node del format.
- [ ] `docs/api-contract.md` documenta els camps nous. Si Android pinta aquest estat, digues a Notes si queda enrere (seria una tasca a part).

## Out of scope

Canviar quan s'emet `progress` o l'interval.

## Notes

- `internal/web/progres.go` (`delta`, `emetLocked`), `internal/llm/progress.go` (`StreamProgress`; potser cal passar el nom de l'eina o el tros d'arguments a més de la mida), el JS que pinta «Pensant · 67k · 4m 42s» (cerca les claus `act.now.*` a `internal/web/app/i18n.js` i qui les fa servir).
