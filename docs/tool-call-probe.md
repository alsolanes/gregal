# Sonda tool-calling local (2026-09-12)

Objectiu M2b: saber si els models locals suporten function calling natiu
abans de dissenyar l'agent loop. Resultat: **SÍ**.

## Topologia trobada (verificada amb `ss` + `/proc`)

- `llama-swap :8087` — router, grup `default` **exclusive**, `healthCheckTimeout: 300`:
  `Ornith-1.5-35B` (àlies: ornith, agentic, coder-agent, tool-agent…),
  `Ornith-1.5-35B-Fast`, `Ornith-1.5-35B-Tools` (reasoning on),
  `Qwen38-Flash-FAST`, `Nex-2.5-mini`.
- `llama-server directe :5800` — Nex-2.5-mini ROCmFP4 (`--alias Nex-2.5-mini`).
- `:8089` = gateway `tools-server/prompt_injector.py`, exigeix Bearer (claus a la seva BD).
- `:8091` = dashboard, `:8094` = estàtics marea, `:8092` = mort. Ja no són models.

## Prova 1: el model demana eina

`POST :5800/v1/chat/completions` amb `tools:[{read}]`, `tool_choice:auto`:

```json
{ "role": "user", "content": "Llegeix el fitxer /tmp/prova.txt fent servir l'eina read." }
```

Resposta: `message` amb claus `role, content, reasoning_content, tool_calls`;
`content` buit i:

```json
"tool_calls": [{ "type": "function", "id": "Y1MZ…",
  "function": { "name": "read", "arguments": "{\"path\":\"/tmp/prova.txt\"}" } }]
```

→ tool_call natiu, arguments JSON vàlid. (1.4 s incloent arrencada freda del probe.)

## Prova 2: volta sencera

Segon `POST` amb historial + `{"role":"assistant","tool_calls":[…]}` +
`{"role":"tool","tool_call_id":…, "content":"1|hola sóc el contingut de prova"}`:

- `finish_reason: stop`, `tool_calls: null`
- `content`: cita el contingut de l'eina en bloc ```text.

→ El loop OpenAI-style funciona de punta a punta contra llama.cpp local.

## Conseqüències per gregal (M2b)

- Es pot implementar l'agent loop amb el format `tools` estàndard, sense
  protocol propi ni parse de `<tool_call>`.
- Rol `code` per defecte: `Nex-2.5-mini` via `:5800` directe (ràpid, ja carregat);
  `Ornith-1.5-35B-Tools` via swap com a alternativa pesada (compte: el swap és
  exclusive, carregar Ornith desallotja el Nex — avisar a la UI).
- `verify` amb diff ja fet (M2). El loop ha de reutilitzar `tools.Classify` i
  `pendingOp` per cada crida del model (allow directa, ask amb s/n, deny dura).
- Límit de passos del loop al config (p. ex. 10), comptador visible a la statusbar.
