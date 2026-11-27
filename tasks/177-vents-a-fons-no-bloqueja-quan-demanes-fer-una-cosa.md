---
title: "Vents: «A fons» no bloqueja quan demanes fer una cosa"
status: done
depends_on: []
---

## Why

Amb «A fons» (Tempesta, `internal/web/winds_tempesta.go`) els tres agents i la síntesi treballen en només lectura. Si demanes «fes-me una web», cap no pot escriure ni publicar, i la resposta diu «el servidor em té en mode inspecció». És fals i despista: en els torns següents del mateix fil, ja sense «A fons», el vent ho repeteix i ni ho prova.

## Acceptance criteria

- [x] Un missatge a un vent amb `tempesta:true` que demana fer una cosa (la mateixa decisió que `crewChatIntent(msg) == "work"`) es treballa com un torn normal, amb les eines d'escriure, i no com a Tempesta. Test Go amb proveïdor fals: el `write` queda fet i no s'emet cap esdeveniment `tempesta`.
- [x] La resposta d'aquest torn porta una línia curta, en la llengua del servidor (ca i en), que diu que «A fons» és per investigar i aquesta l'ha feta en mode normal. Test Go.
- [x] Les consignes dels agents i de la síntesi de la Tempesta diuen que «A fons» només investiga i que, si el que es demana és fer una cosa, cal dir-ho així (desactiva «A fons»), mai que el servidor els ha bloquejat. Test del brief.
- [x] `docs/api-contract.md` (fila de `/api/winds/chat` o la de la Tempesta) ho explica. Web i Android no canvien: el servidor decideix.

## Out of scope

Triar models (tasques 178, 179). Les colles (176).

## Notes

- `internal/web/winds_stream.go` ~288 (`Tempesta bool`) i ~430 (`case req.Tempesta`), `internal/web/winds_crew_chat.go` (`crewChatIntent`).
- Fet: `handleWindTurn` (`winds_stream.go`) calcula `crewChatIntent` un sol cop; amb `tempesta:true` i intenció `work` fa un torn normal i afegeix al final de la resposta una línia en cursiva (ca/en). Les consignes dels agents i de la síntesi porten `tempestaReadOnlyNote` (`winds_tempesta.go`).
- Tests: `TestTempestaWorkRequestRunsAsNormalTurn`, `TestTempestaBriefsSayDeepOnlyResearches` (`internal/web/winds_tempesta_test.go`).
- Web i Android envien `tempesta` igual que abans; la línia arriba dins del text de `done`.
