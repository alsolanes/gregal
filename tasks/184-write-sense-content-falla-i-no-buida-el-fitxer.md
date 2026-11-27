---
title: "write sense content falla i no buida el fitxer"
status: todo
depends_on: []
---

## Why

Un torn de Codi (DeepSeek V4.1 Flash, «Alt», autonomia total) va estar 37 minuts per acabar dient «no he pogut escriure la web». El registre (events.jsonl, run 364 del 2026-11-27) mostra quatre crides `write` a `index.html` de 57.344 i 24.576 tokens de sortida cadascuna (5-11 minuts cada una), que van arribar amb arguments `{"path": "..."}` i prou: el contingut s'havia perdut pel límit de sortida. `write` va escriure 0 bytes i va contestar «escrit … (0 bytes)» com si anés bé, així que el model ho tornava a provar igual fins que el detector de crides repetides va tancar el torn. A més, un `write` sense `content` buida qualsevol fitxer que ja existís.

## Acceptance criteria

- [ ] Un `write` amb el camp `content` absent (no un `""` explícit) no toca el fitxer i torna un error que diu que el contingut no ha arribat, segurament perquè la resposta s'ha tallat pel límit de sortida, i que escrigui el fitxer per parts (un esquelet amb `write` i la resta amb `edit`). Test Go: un fitxer existent conserva el contingut.
- [ ] Un `write` amb `content: ""` explícit continua creant un fitxer buit. Test Go.
- [ ] Si una resposta del model fa servir tot el pressupost de sortida (tokens de sortida de l'usage >= max_tokens demanat) però el proveïdor no diu `finish_reason: length`, es tracta com un tall: les crides d'eina d'aquella resposta no s'executen i passa pel camí de `continuaTallat` (`internal/llm/continua.go`). Test Go amb un servidor fals compatible OpenAI que torna `finish_reason: tool_calls` amb `completion_tokens` = max_tokens.

## Out of scope

El temps que el model passa raonant («Alt»). El detector de crides repetides.

## Notes

- `internal/agent/loop.go` ~1131 (`case "write"`), `internal/llm/client.go` (~1190-1203, on es mira `finish == "length"`), `internal/llm/length.go`, `internal/llm/continua.go` (`argsComplets`, `promptCridaTallada`).
- Mira també si el camí de `RecoverCalls` (DSML dins del text) pot deixar una crida amb només `path` quan el text es talla, i digues-ho a Notes.
