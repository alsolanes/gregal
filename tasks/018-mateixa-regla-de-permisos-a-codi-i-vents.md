---
title: "Mateixa regla de permisos a Codi i a Vents"
status: todo
depends_on: []
---

## Why

Avui els Vents escriuen al projecte i fan commits locals sense preguntar encara que hagis triat «Demana abans de canviar», i Codi segueix el nivell d'accés. La regla ha de ser la mateixa: el nivell d'accés que tries val per als dos modes; Codi és l'agent de codi i Vents l'enfocament multiagent, però amb els mateixos permisos.

## Acceptance criteria

- [ ] Amb l'accés «Demana abans de canviar» (`AskEdits`), un vent que vol escriure, editar o fer un commit dins del projecte demana aprovació, igual que Codi.
- [ ] Amb l'accés «edicions» (el per defecte), Codi deixa passar sol el mateix que els vents avui amb `WindAutoAllow` dins del projecte (escriure, editar, executar i provar, commit local), i continua preguntant per push, PR, desplegar, instal·lar, esborrar o escriure fora del projecte.
- [ ] En tots dos modes, les portes dures (instal·lar, esborrar fora, serveis del sistema, primera publicació pública) pregunten sempre, i el que la política denega continua denegat.
- [ ] Els casos propis dels vents es mantenen: a la rotllana només escriu el primer que parla, una petició de revisió és de només lectura, els xats compartits només cerquen, i l'estudi només escriu a la seva carpeta.
- [ ] Una taula de tests (acció × mode × nivell d'accés) comprova la mateixa decisió a Codi i a Vents.
- [ ] `docs/api-contract.md` i `docs/modes.md` descriuen la regla comuna. La pàgina de Permisos d'Android es veu en tots dos modes amb la tasca 017; si la web necessita un canvi per mostrar el mateix nivell d'accés a Vents, es fa aquí o s'apunta a Notes.

## Out of scope

- Unificar els dos canals d'aprovació (`/api/approve` i `/api/winds/pending` + `/api/winds/answer`) en un de sol: es manté la targeta de cada xat. Si cal, tasca a part.
- El selector d'Android (tasca 017).

## Notes

- Vents: `internal/web/plan.go` ~470-560 (`Decide` → carpeta de projectes → navegador local → `WindAutoAllow` → regla «sol» → portes dures `windRisky` → primera publicació); `windTurnMode` a `winds_permissions.go` ~94.
- Codi: `internal/agent/loop.go` ~760-800 (`AskEdits`, `writablePath`, bash al projecte), `access.go`, `autonomous_policy.go` (`WindAutoAllow` ~135).
- Preferència de l'usuari: a Act no s'ha d'aturar per feina normal dins del projecte; només pregunta el que és destructiu o cap enfora, o si ha triat «pregunta».
- `docs/api-contract.md` ~274 diu que la rotllana és de només lectura, però el primer que parla pot escriure: corregir-ho de passada.
