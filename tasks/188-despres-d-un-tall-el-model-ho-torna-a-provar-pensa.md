---
title: "Després d'un tall, el model ho torna a provar pensant menys"
status: done
depends_on: []
---

## Why

Caps de la 185: quan una resposta es talla perquè el model s'ha gastat el pressupost pensant, el reintent li diu que no redacti el fitxer pensant, però el deixa pensar igual de fort. I un tall que és només raonament (sense text ni crida) no arriba al camí de represa: es reintenta amb més pressupost i la mateixa consigna, i pot acabar el torn en error.

## Acceptance criteria

- [x] Quan una resposta tallada era sobretot raonament (`raonatSobretot`), el reintent es fa amb un esforç de raonament més baix que el del rol (per exemple `low`, o sense raonament si el proveïdor no en té de més baix), amb el mecanisme que ja hi ha (`llm.WithThink` / `reasoningEffort`). Test Go amb un servidor fals que comprova el `reasoning_effort` del reintent.
- [x] Un tall que només porta raonament (cap text, cap crida) també rep la consigna d'escriure ja i per parts, no només més pressupost. Test Go.
- [x] El torn de prova d'extrem a extrem de la 185 continua passant, i n'hi ha un altre on el proveïdor no tanca la crida (cap crida, només raonament fins al límit) que també acaba amb el fitxer sencer al disc. Test Go.

## Out of scope

Canviar el pressupost per defecte o el model.

## Notes

- `internal/llm/continua.go` (`continuaTallat` té complexitat 25: parteix-la abans d'afegir-hi res perquè el lint no hi vegi cap problema nou), `internal/llm/length.go` (`escalaLength`, `raonatSobretot`), `internal/llm/compat.go` (`reasoningEffort`), `internal/agent/escriure_per_parts_test.go`.
- Treball lleuger: `check.py --fast` i `go test ./internal/llm ./internal/agent`; la comprovació sencera i la revisió es fan un sol cop per a tot el lot.
- `continuaTallat` s'ha partit (`tallContinuable`, `cridaTrencada`, `reintent`, `lliuraFallada`, `alSostre`, `conversaReintent`) i la `do` ara rep el context de cada intent (tipus `cridaModel`). Les quatre vies de `client.go` el fan servir a dins, i així el reintent porta el raonament rebaixat.
- `menysRaonament` baixa un graó respecte del rol: `high`/`medium`/`max`/buit → `low`, `low` → `no` (que surt com `reasoning_effort: none` i `enable_thinking: false`), `no` es queda igual. Es calcula sempre des del rol, no des de l'últim reintent, i un cop baixat no es torna a pujar dins de la mateixa crida. Només es baixa si el tall era `raonatSobretot`; un tall als arguments manté l'esforç del rol.
- Tall només de pensament: `continuaTallat` el continua només si la crida porta eines (paràmetre `eines`, `len(specs) > 0`). Sense eines (`Chat`, `ChatStream`) puja tal qual com a error, com abans (`TestRetryLengthEscalaTresCopsIDesisteix`). Abans de la consigna nova (`promptRaonamentTallat`) encara passa per l'escalada de pressupost d'`escalaLength` (fins a 3 intents més): la consigna és a més, no en comptes del pressupost. Si al sostre de continuacions no s'ha escrit res, puja l'error original en comptes d'una nota buida.
- Que l'esforç rebaixat no enverini les peticions següents (revisió del lot): a les vies compatibles amb OpenAI/Azure, la correcció d'esforç recordada (`effortFixes`) ara té la clau `base|model|valor demanat` (`clauEsforc`) i només s'aplica a aquell mateix valor; abans un `none` del reintent rebutjat deixava `minimal` per a totes les peticions del model. A Responses (`reasoning`) i Anthropic (`output_config`), `menysRaonament` marca el context (`esforcRebaixat`) i `campsPerEsforc` fa efímer el rebuig d'aquell camp: val només per al reintent. Tests: `TestReasoningEffortFixOnlyForTheRequestedValue`, `TestResponsesNonStreamingAndErrors/rejected_retry_effort_does_not_turn_reasoning_off`, `TestAnthropicRejectedRetryEffortIsDroppedOnce`.
- El test d'extrem a extrem nou (`TestFitxerGranTallatNomesPensantAcabaEscritPerParts`) fa servir el mateix model fals que el de la 185 amb `tancaCrida` fals: tallat dins del pensament, el proveïdor envia només raonament i `finish_reason: length`, sense cap crida.
- Follow-up: el tall només de pensament crema 4 intents d'`escalaLength` abans de la consigna quan el límit del proveïdor és fix. Es podria saltar l'escalada si el pensament ja passa del pressupost, però canvia `escalaLength` i els seus tests.
