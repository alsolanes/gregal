---
title: "Després d'un tall, el model ho torna a provar pensant menys"
status: todo
depends_on: []
---

## Why

Caps de la 185: quan una resposta es talla perquè el model s'ha gastat el pressupost pensant, el reintent li diu que no redacti el fitxer pensant, però el deixa pensar igual de fort. I un tall que és només raonament (sense text ni crida) no arriba al camí de represa: es reintenta amb més pressupost i la mateixa consigna, i pot acabar el torn en error.

## Acceptance criteria

- [ ] Quan una resposta tallada era sobretot raonament (`raonatSobretot`), el reintent es fa amb un esforç de raonament més baix que el del rol (per exemple `low`, o sense raonament si el proveïdor no en té de més baix), amb el mecanisme que ja hi ha (`llm.WithThink` / `reasoningEffort`). Test Go amb un servidor fals que comprova el `reasoning_effort` del reintent.
- [ ] Un tall que només porta raonament (cap text, cap crida) també rep la consigna d'escriure ja i per parts, no només més pressupost. Test Go.
- [ ] El torn de prova d'extrem a extrem de la 185 continua passant, i n'hi ha un altre on el proveïdor no tanca la crida (cap crida, només raonament fins al límit) que també acaba amb el fitxer sencer al disc. Test Go.

## Out of scope

Canviar el pressupost per defecte o el model.

## Notes

- `internal/llm/continua.go` (`continuaTallat` té complexitat 25: parteix-la abans d'afegir-hi res perquè el lint no hi vegi cap problema nou), `internal/llm/length.go` (`escalaLength`, `raonatSobretot`), `internal/llm/compat.go` (`reasoningEffort`), `internal/agent/escriure_per_parts_test.go`.
- Treball lleuger: `check.py --fast` i `go test ./internal/llm ./internal/agent`; la comprovació sencera i la revisió es fan un sol cop per a tot el lot.
