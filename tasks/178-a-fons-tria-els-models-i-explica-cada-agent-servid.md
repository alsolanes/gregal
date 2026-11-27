---
title: "«A fons»: tria els models i explica cada agent (servidor i web)"
status: todo
depends_on: []
---

## Why

A «A fons» el servidor tria sol els models dels tres agents (Directe, Fonts, Advocat del diable) i de la síntesi (`tempestaModels`, `tempestaSynthRole` a `internal/web/winds_tempesta.go`), i l'app només ensenya el nom de l'angle i el model. Vols poder triar cada model i entendre què fa cada agent.

## Acceptance criteria

- [ ] Hi ha una preferència desada al config amb el model (`provider/model`) de cada angle (`direct`, `sources`, `skeptic`) i de la síntesi. Buida o amb un model que ja no existeix = la tria automàtica d'ara. Un endpoint (o el de preferències de vents que ja hi hagi) la llegeix i la desa, i rebutja un model desconegut amb 400. Test Go.
- [ ] `windTempesta` fa servir els models triats. Test Go amb proveïdor fals que comprova quin model rep cada agent i la síntesi.
- [ ] Cada agent de l'esdeveniment `tempesta` porta una descripció curta del que fa, en català i en anglès (camps nous al costat de `ca`/`en`). Test Go.
- [ ] Web: a Preferències de Vents, una secció «A fons» amb un selector de model per a cada agent i la síntesi (amb «Automàtic»), i a la vista de la Tempesta del xat, cada agent ensenya la seva descripció (title o text petit). Test node.
- [ ] `docs/api-contract.md` documenta el camp nou i l'endpoint. Android ve a la 179.

## Out of scope

Canviar el nombre d'agents o els angles. Android (179).

## Notes

- Fes servir el catàleg de models que ja hi ha (`/api/models`) per al selector, i les preferències de vents existents si n'hi ha. Strings d'interfície en ca i en.
