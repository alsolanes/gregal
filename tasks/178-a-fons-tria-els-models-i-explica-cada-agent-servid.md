---
title: "«A fons»: tria els models i explica cada agent (servidor i web)"
status: done
depends_on: []
---

## Why

A «A fons» el servidor tria sol els models dels tres agents (Directe, Fonts, Advocat del diable) i de la síntesi (`tempestaModels`, `tempestaSynthRole` a `internal/web/winds_tempesta.go`), i l'app només ensenya el nom de l'angle i el model. Vols poder triar cada model i entendre què fa cada agent.

## Acceptance criteria

- [x] Hi ha una preferència desada al config amb el model (`provider/model`) de cada angle (`direct`, `sources`, `skeptic`) i de la síntesi. Buida o amb un model que ja no existeix = la tria automàtica d'ara. Un endpoint (o el de preferències de vents que ja hi hagi) la llegeix i la desa, i rebutja un model desconegut amb 400. Test Go.
- [x] `windTempesta` fa servir els models triats. Test Go amb proveïdor fals que comprova quin model rep cada agent i la síntesi.
- [x] Cada agent de l'esdeveniment `tempesta` porta una descripció curta del que fa, en català i en anglès (camps nous al costat de `ca`/`en`). Test Go.
- [x] Web: a Preferències de Vents, una secció «A fons» amb un selector de model per a cada agent i la síntesi (amb «Automàtic»), i a la vista de la Tempesta del xat, cada agent ensenya la seva descripció (title o text petit). Test node.
- [x] `docs/api-contract.md` documenta el camp nou i l'endpoint. Android ve a la 179.

## Out of scope

Canviar el nombre d'agents o els angles. Android (179).

## Notes

- Fes servir el catàleg de models que ja hi ha (`/api/models`) per al selector, i les preferències de vents existents si n'hi ha. Strings d'interfície en ca i en.
- Endpoint: `GET/POST /api/winds/tempesta/models`. GET → `{models:{direct,sources,skeptic,synth}}` (`"provider/model"` o `""` = automàtic). POST `{slot, model}`; `model:""` torna a automàtic. Fitxer: `internal/web/winds_tempesta_models.go`.
- On es desa: amb les altres preferències de model de l'usuari (`session.SaveModelDefault`, el mateix lloc que el model dels vents, scope `vents`), claus `~tempesta.<slot>`; no al `config.yaml`, que és compartit entre comptes i porta claus. «Automàtic» es desa com a `auto` perquè el fitxer no admet valors buits.
- «Un model que ja no existeix»: proveïdor fora del config, o model que el proveïdor ja no llista (es comprova amb `refreshModelAvailability`, amb la seva memòria cau d'un minut, a l'inici del torn «A fons»). En tots dos casos mana la tria automàtica; el GET segueix tornant el que hi ha desat.
- Camps nous de cada agent a l'esdeveniment `tempesta`: `desc_ca`, `desc_en` (`omitempty`: les veus d'una rotllana no en porten). Les descripcions són a `tempestaDescs`, fora de `tempestaAngles`, per no tocar les línies dels briefs que canvia la 177.
- Web: la secció «A fons» és a Preferències › Models (`internal/web/app/tempesta-prefs.js`), que és on porta el botó Preferències de Vents; no hi ha cap pàgina pròpia de Vents a Preferències.
- Android (179) llegeix i desa amb el mateix endpoint i ensenya `desc_ca`/`desc_en`.
- L'esdeveniment `tempesta` es documenta a la fila POST de `/api/winds/tempesta/models` i no a la de `/api/winds/chat/stream` (línia ~198) perquè la 177 edita aquella fila; en integrar-les es pot afegir allà una referència.
- Un model triat fa servir la temperatura del rol dels vents, no les temperatures diferents (0.4/0.8/1.0) que la tria automàtica dona quan repeteix model.
- Follow-up: Preferències repeteix en curt les descripcions dels agents (`tempesta-prefs.js`) que el servidor ja envia a `tempestaDescs`; si canvien, cal canviar-les als dos llocs (o que el GET les enviï).
- Follow-up: les tries de `modelOverride` (també `vents` i `~think`) es restauren amb la sessió quan reprens una conversa, de manera que en una conversa vella poden ser les d'abans.
