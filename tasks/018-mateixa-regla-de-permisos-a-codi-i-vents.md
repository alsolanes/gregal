---
title: "Mateixa regla de permisos a Codi i a Vents"
status: done
depends_on: []
---

## Why

Avui els Vents escriuen al projecte i fan commits locals sense preguntar encara que hagis triat «Demana abans de canviar», i Codi segueix el nivell d'accés. La regla ha de ser la mateixa: el nivell d'accés que tries val per als dos modes; Codi és l'agent de codi i Vents l'enfocament multiagent, però amb els mateixos permisos.

## Acceptance criteria

- [x] Amb l'accés «Demana abans de canviar» (`AskEdits`), un vent que vol escriure, editar o fer un commit dins del projecte demana aprovació, igual que Codi.
- [x] Amb l'accés «edicions» (el per defecte), Codi deixa passar sol el mateix que els vents avui amb `WindAutoAllow` dins del projecte (escriure, editar, executar i provar, commit local), i continua preguntant per push, PR, desplegar, instal·lar, esborrar o escriure fora del projecte.
- [x] En tots dos modes, les portes dures (instal·lar, esborrar fora, serveis del sistema, primera publicació pública) pregunten sempre, i el que la política denega continua denegat.
- [x] Els casos propis dels vents es mantenen: a la rotllana només escriu el primer que parla, una petició de revisió és de només lectura, els xats compartits només cerquen, i l'estudi només escriu a la seva carpeta.
- [x] Una taula de tests (acció × mode × nivell d'accés) comprova la mateixa decisió a Codi i a Vents.
- [x] `docs/api-contract.md` i `docs/modes.md` descriuen la regla comuna. La pàgina de Permisos d'Android es veu en tots dos modes amb la tasca 017; si la web necessita un canvi per mostrar el mateix nivell d'accés a Vents, es fa aquí o s'apunta a Notes.

## Out of scope

- Unificar els dos canals d'aprovació (`/api/approve` i `/api/winds/pending` + `/api/winds/answer`) en un de sol: es manté la targeta de cada xat. Si cal, tasca a part.
- El selector d'Android (tasca 017).

## Notes

- Vents: `internal/web/plan.go` ~470-560 (`Decide` → carpeta de projectes → navegador local → `WindAutoAllow` → regla «sol» → portes dures `windRisky` → primera publicació); `windTurnMode` a `winds_permissions.go` ~94.
- Codi: `internal/agent/loop.go` ~760-800 (`AskEdits`, `writablePath`, bash al projecte), `access.go`, `autonomous_policy.go` (`WindAutoAllow` ~135).
- Preferència de l'usuari: a Act no s'ha d'aturar per feina normal dins del projecte; només pregunta el que és destructiu o cap enfora, o si ha triat «pregunta».
- `docs/api-contract.md` ~274 diu que la rotllana és de només lectura, però el primer que parla pot escriure: corregir-ho de passada.
- Fet: la regla és `actRule` a `internal/agent/loop.go` (dins `Policy.Decide`, modes code i debug): amb «edicions» i un projecte, el que `WindAutoAllow` aprova passa sol; amb `AskEdits`, no. Després, `tools.Risky` torna «ask» per a qualsevol ordre de shell arriscada, també amb un permís del config. `plan.go` ja no fa l'autoaprovat propi del vent: fa servir el `Decide` comú i, si l'eina va a la carpeta de projectes dels vents, el torna a cridar amb aquella carpeta.
- Canvia per a Codi amb «edicions»: el commit local, les notes de coneixement de projecte, engegar una app del projecte i publicar una carpeta del projecte passen sols; `go install`, `systemctl restart …` i les altres portes dures pregunten encara que el config les permeti. Tests ajustats perquè el requisit ha canviat: `dev_permissions_test.go` (git_commit), `knowledge_policy_test.go` (nota de projecte), `selfstop_test.go` (systemctl restart amb permís del config).
- Primera publicació pública a Codi: la decisió és «allow», però l'eina `publish` la deixa privada fins que es confirma (`webActivate`); no surt cap targeta d'aprovació com als vents.
- L'estudi (`windStudyCtx`) continua forçant `AskEdits=false` dins la seva carpeta de pràctiques: és un cas propi dels vents.
- Web: no ha calgut cap canvi; el nivell d'accés es llegeix de `/api/state` (`access`) al cadenat del compositor i a Preferències. No he comprovat a mà que el cadenat es vegi a la vista de Vents.
- Follow-up: `WindAutoAllow` ara és la regla comuna de Codi i Vents; caldria rebatejar-la (p. ex. `ActAutoAllow`), però en canviar-li el nom el golangci-lint la marca com a nova per complexitat ciclomàtica (56): el canvi de nom va amb dividir-la, tasca a part.
- Follow-up: amb «Autonomia total» Codi és autònom però els vents continuen en `code` amb «edicions» (`windTurnMode`); si han de ser iguals també aquí, tasca a part.
- Follow-up: a Codi, una primera publicació pública podria demanar aprovació amb targeta com als vents en comptes de quedar privada en silenci.
