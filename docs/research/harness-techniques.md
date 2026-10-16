# Tècniques dels harnesses de qualitat i què en pot aprendre Gregal

*Recerca del 2026-10-16, sobre Gregal 1.9.4 (branca `graceful-step-limit`). Document de recerca: no s'hi implementa res.*

## 0. Resum executiu

Els harnesses que surten bé a les comparatives no deuen la qualitat a cap prompt màgic. Hi ha tres coses que es repeteixen:

1. **Eines a prova d'errors.** Les edicions toleren diferències d'espais i Unicode però tenen guardes. Quan una edició falla, l'error diu exactament què cal fer. Després d'editar hi ha diagnòstic immediat: lint o LSP.
2. **Context net i estable.** El prefix no canvia, de manera que la caché de KV o de prompt s'aprofita. Les observacions velles s'emmascaren en lloc de resumir-ho tot. Les sortides llargues es retallen amb cap i cua.
3. **Adaptació al model.** Cada família té el seu prompt, el seu format d'edició, les seves eines i la seva configuració de mostreig.

Gregal ja fa bé moltes coses: diagnòstic de sintaxi després de cada edició, best-of-N, porta de verificació, guarda contra bucles (DoomTracker), continuació de respostes tallades, mapa del projecte fora del system prompt i data sense hora. On queda enrere és en tres punts:

- **Estabilitat del prefix.** Reescriu l'historial a cada torn.
- **Suport real dels servidors locals.** No envia res específic per a Ollama ni per a llama.cpp.
- **Tolerància de l'edició.** Només normalitza CRLF.
- També falta un **registre de perfils per model**.

**Passos següents recomanats**, detallats a la secció 4:

| # | Proposta | Esforç | Impacte |
|---|---|---|---|
| ★1 | Mesurar primer: un bench reproduïble amb mètriques de caché, edició i eines (P1) | M | Habilita la resta |
| ★2 | Prefix estable i historial només d'afegir (P2) | S–M | Molt alt en local, alt en remot |
| ★3 | Proveïdors locals natius: Ollama `/api/chat` amb `num_ctx` i `keep_alive` fixos, `id_slot` a llama.cpp, sondeig de capacitats (P3) | M | Molt alt en local |
| ★4 | Cascada d'edició tolerant amb guardes, i error amb la millor coincidència (P4) | S–M | Alt |
| ★5 | Emmascarar observacions per blocs i truncar amb cap i cua, en lloc de retallar a 600 caràcters i resumir aviat (P5) | M | Alt (cost i qualitat) |

---

## 1. Com funciona Gregal avui

Aquest mapa s'ha fet llegint el codi. Serveix per no proposar coses que Gregal ja fa.

| Àrea | Estat actual | On és |
|---|---|---|
| System prompt | Ordre: identitat (~300 caràcters), memòria d'usuari, fets de l'entorn, regles del mode (~3 KB en mode code), data (sense hora), AGENTS.md/memòria/skills/subagents/knowledge, resum de compactació, idioma. Varia segons el mode i l'idioma (ca/en), **però no segons el model**. | `internal/config/config.go` (`SystemPrompt`), `internal/agent/mode.go` (`PromptFor`, `AmbContext`), `internal/agent/entorn.go` (`FetsEntorn`, `DataAra`), `internal/agent/skills.go` |
| Mapa del projecte | Va dins del missatge d'usuari, no al system prompt (pensat per la caché de KV). **Però `SenseMapesAntics` esborra els mapes vells de tots els missatges d'usuari a cada torn nou**, i això reescriu l'historial. | `internal/agent/mapa_projecte.go`, `internal/agent/historial.go` |
| Eines | 31 eines natives, més `delegate` i les de MCP. Sempre es fa servir function calling natiu, sense `tool_choice`. En TUI, web i Telegram s'envia **sempre `SpecsAll()`**, també en els modes chat i inspect (allà les restriccions s'apliquen amb la política de permisos). | `internal/agent/loop.go` (`Specs`, `SpecsAll`) |
| Reparació de crides | Interpreta el DSML de DeepSeek. Les crides en text de Hermes o Qwen (`<tool_call>`) **només s'eliminen i es demana al model que les repeteixi** (fins a 2 vegades). Els arguments JSON invàlids es converteixen en `{}`. | `internal/llm/dsml.go`, `internal/agent/einatext.go`, `internal/llm/client.go` (`safeToolCalls`) |
| Edició | Tres eines: `write` (fitxer sencer), `edit` (substitució exacta i única) i `patch` (diverses operacions atòmiques). Només normalitza CRLF. L'error explica el motiu: espais diferents, context que no hi és, cap coincidència, o ambigüitat. | `internal/tools/tools.go` (`Edit`, `perQueNoHiEs`), `internal/tools/patch.go` |
| Després d'editar | Comprovació de sintaxi del fitxer editat (go/parser, go vet, py_compile, node --check, bash -n, tsc), més el hook `post_edit`. **No hi ha LSP.** | `internal/agent/hook.go` (`apendixEdicio`), `internal/agent/diag.go`, `diag_lang.go`, `diag_exec.go` |
| Context | Calcula els tokens com a caràcters/4. Compacta en arribar al 75% del pressupost. `MakeRoom` fa: (1) retalla a 600 caràcters les sortides d'eina anteriors a les 6 últimes, (2) resumeix amb el mateix model (map-reduce, 30 línies) i (3) fa un retall dur. Les sortides es tallen a 8000 caràcters i només se'n queda el cap. **No hi ha deduplicació de lectures.** | `internal/agent/compact.go`, `internal/agent/histcompact.go`, `internal/tools/tools.go` (`MaxOutputChars`) |
| Finestra de context | Ordre de prioritat: el rol; després un sondeig de `/models` (context_length, max_model_len, n_ctx); després una taula de famílies; si no, el valor per defecte (32k, o 8k en local). | `internal/agent/window.go`, `internal/llm/probe.go` |
| Repo map | Go amb go/parser i la resta de llenguatges amb regex. Rànquing lèxic (identificadors de la tasca, git i recència) amb un pressupost de 1200 tokens. **No fa servir tree-sitter, ni graf de referències, ni PageRank.** | `internal/repomap/*`, `internal/repomap/rank.go` |
| Modes i pla | Modes: code, chat, inspect, goal, autonomous i debug. També hi ha `/plan` amb crític de pla, todos, i un scout de només lectura abans del torn. **No hi ha separació arquitecte/editor.** | `internal/agent/mode.go`, `plansys.go`, `plancritic.go`, `todos.go`, `scout.go` |
| Verificació | Porta `--verify` (fins a 3 rondes), revisor amb veredicte, best-of-N en worktrees amb puntuació (l'usuari tria), `RatxaVermella` i calibratge de les apostes. | `internal/agent/verify_gate.go`, `internal/verify/*`, `internal/agent/bestof.go`, `internal/agent/motor.go` |
| Reintents | Backoff (6 intents), failover, escalat de `max_tokens` quan la resposta surt buida per longitud, continuació de respostes tallades (fins a 3 cops), negociació de camps opcionals a partir dels errors 400, pressupost de passos amb avís i síntesi final. | `internal/llm/retry.go`, `failover.go`, `length.go`, `continua.go`, `compat.go`, `internal/agent/pressupost.go` |
| Caché de prompt | Anthropic: `cache_control` al system, a l'última eina i a l'últim bloc. OpenAI: `prompt_cache_key` opcional. **En proveïdors locals no s'envia res** (ni `cache_prompt`, ni `id_slot`, ni `keep_alive`, ni `num_ctx`). Ollama, LM Studio i llama.cpp són simples presets compatibles amb OpenAI. | `internal/llm/anthropic.go`, `internal/llm/client.go`, `internal/config/cataleg.go` |
| Delegació | `delegate` llança d'1 a 4 subagents de només lectura. Hi ha subagents d'usuari, team/flows, i un router que decideix per paraules clau. El rol `delegate` pot apuntar a un model més barat, però no hi ha cap encaminament automàtic per a tasques laterals. | `internal/agent/delegate.go`, `internal/team/*`, `internal/config/router.go` |
| Perfils per model | **No n'hi ha**, fora de la taula de finestres i de `glmForcesReasoning`. Les peculiaritats de cada model s'aprenen dels errors 400. | `internal/agent/window.go`, `internal/llm/compat.go` |
| Avaluació | `--eval` amb 6 tasques petites (veredicte, passos, tokens, temps, línia base). `bench/` només conté fixtures, sense runner. El JSON headless inclou els tokens de la caché i hi ha `GREGAL_PERFIL=1`. | `internal/evals/evals.go`, `evals/`, `bench/`, `internal/agent/run.go` |

---

## 2. Què fa destacar cada harness

### opencode (anomalyco/opencode, abans sst/opencode)
- **Un system prompt per família de model**: `anthropic.txt`, `beast.txt` (GPT-4/o1/o3, molt insistent en l'autonomia), `gemini.txt`, `kimi.txt`, `codex.txt`, `default.txt`… A tots s'hi afegeix un bloc `<env>`. [system.ts v1.19.2](https://github.com/anomalyco/opencode/blob/v1.19.2/packages/opencode/src/session/system.ts), [prompts](https://github.com/anomalyco/opencode/tree/v1.19.2/packages/opencode/src/session/prompt)
- **L'eina d'edició també depèn del model**: els GPT reben `apply_patch`, el format amb què es van entrenar. [registry.ts](https://github.com/anomalyco/opencode/blob/v1.19.2/packages/opencode/src/tool/registry.ts)
- **Cadena de 9 *replacers*** a `edit`: exacte, LineTrimmed, BlockAnchor (Levenshtein amb llindar 0,65), espais normalitzats, indentació flexible, escapes, vores retallades, ContextAware i MultiOccurrence. Una guarda (`isDisproportionateMatch`) rebutja les coincidències massa grans. A la versió 2.0 la cadena es redueix a exacte, Unicode normalitzat i espais finals. [edit.ts v1.19.2](https://github.com/anomalyco/opencode/blob/v1.19.2/packages/opencode/src/tool/edit.ts), [edit.ts 2.0](https://github.com/anomalyco/opencode/blob/dev/packages/core/src/tool/plugin/edit.ts)
- **Diagnòstics LSP afegits al resultat de cada edició**: «LSP errors detected in this file, please fix:». (Mateix edit.ts.)
- **Poda abans de compactar**: protegeix els 40k tokens de sortides d'eina més recents i substitueix les més velles per «[Old tool result content cleared]», però només si així s'alliberen més de 20k tokens. Les sortides es tallen a 2000 línies o 50 KB i la sortida completa es desa en un fitxer. [compaction.ts](https://github.com/anomalyco/opencode/blob/v1.19.2/packages/opencode/src/session/compaction.ts), [truncate.ts](https://github.com/anomalyco/opencode/blob/v1.19.2/packages/opencode/src/tool/truncate.ts)
- **Reparació dels arguments de les eines** quan l'esquema deixa clara la intenció: JSON dins d'una cadena, `"20"`→20, `"false"`→false. És un error típic dels models oberts. [tool-input-repair.ts](https://github.com/anomalyco/opencode/blob/dev/packages/core/src/plugin/tool-input-repair.ts)
- **Agents plan i build**, amb un recordatori injectat en canviar de mode i `MAX_STEPS_PROMPT` en esgotar els passos (Gregal ja fa una cosa equivalent). [agent.ts](https://github.com/anomalyco/opencode/blob/v1.19.2/packages/opencode/src/agent/agent.ts)

### DeepSeek Harness (`dsh`, deepseek-ai/deepseek-harness, agost del 2026)
- **Presets**. *minimal* té una sola línia de persona, un bash persistent, lectura d'imatges i compactació: el bash ho fa tot. *standard* hi afegeix eines de fitxers, plan, subagents i todo. Algunes fonts de tercers diuen que *minimal* és el que DeepSeek va fer servir als benchmarks de V4, però **no ho he pogut confirmar** a la model card. [minimal.patch.yml](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/bundle/web-app/presets/minimal.patch.yml), [standard.patch.yml](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/bundle/web-app/presets/standard.patch.yml)
- **Llegir abans d'escriure, amb control de versió**: errors `FS_NOT_OBSERVED` i `FS_STALE_VERSION`. [filesystem.md](https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/subsystems/filesystem.md)
- **Compactació**:
  - S'activa a `min(W·0,8, W−O−65536)`, on W és la finestra de context i O el sostre de sortida.
  - Es conserva literalment el 16% més recent.
  - Mai no separa una crida del seu resultat, ni resumeix el system prompt.
  - La crida de resum aprofita el prefix que ja hi ha a la caché.
  - **Primer poda gratis**: cada resultat de més de 8192 caràcters es redueix a cap + «[... middle pruned ...]» + cua.
  - [compaction.md](https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/subsystems/compaction.md), [tool-result-pruner](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/compaction/compaction-tool-result-pruner/README.md)
- **Ordre del prompt pensat per la caché**. El mode pla només canvia una secció del prompt: `exit_plan_mode` és sempre registrada, i així el catàleg d'eines no canvia mai. [system-prompt.md](https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/subsystems/system-prompt.md), [plan.md](https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/subsystems/plan.md)
- **API amb thinking**: si la petició porta eines, cal tornar a enviar el `reasoning_content` de tots els torns. Gregal ja ho fa. [thinking_mode](https://api-docs.deepseek.com/guides/thinking_mode)
- **El thinking pesa molt en les tasques agèntiques**: amb V4-Pro, SWE Verified puja de 73,6 (sense thinking) a 80,6 (Max). [model card](https://huggingface.co/deepseek-ai/DeepSeek-V4-Pro)

### Aider
- **Repo map**: tree-sitter extreu definicions i referències, i un PageRank personalitzat ordena els fitxers. Puja el pes dels fitxers del xat (×50) i dels identificadors mencionats (×10). Té un pressupost d'1k tokens, que es multiplica per 8 si no hi ha fitxers al xat. [repomap.py](https://github.com/Aider-AI/aider/blob/main/aider/repomap.py), [blog](https://aider.chat/2023/10/22/repomap.html)
- **Format d'edició per model**: whole, diff, diff-fenced, udiff i editor-*. Cada model té la seva configuració a `model-settings.yml`, amb ablacions documentades. [edit-formats](https://aider.chat/docs/more/edit-formats.html), [model-settings.yml](https://github.com/Aider-AI/aider/blob/main/aider/resources/model-settings.yml)
- **udiff contra la mandra**: GPT-4 Turbo passa del 20% al 61%. Sense aplicació flexible dels pedaços hi ha **9 vegades més errors**, i sense hunks «d'alt nivell», entre un 30% i un 50% més. [unified-diffs](https://aider.chat/docs/unified-diffs.html)
- **Codi dins de JSON = pitjor codi**, perquè cal escapar-lo. [code-in-json](https://aider.chat/2024/08/14/code-in-json.html)
- **Arquitecte/editor**: o1-preview com a arquitecte i DeepSeek com a editor van treure un 85%. Amb els models de raonament nous el guany desapareix. [architect](https://aider.chat/2024/09/26/architect.html), [r1-sonnet](https://aider.chat/2025/01/24/r1-sonnet.html)
- **Reflexió**: fins a 3 rondes. Si un bloc SEARCH falla, l'error diu «Did you mean to match some of these actual lines», i també quins blocs sí que s'han aplicat. [editblock_coder.py](https://github.com/Aider-AI/aider/blob/main/aider/coders/editblock_coder.py)

### Claude Code
- **Edit estricte**: coincidència exacta i única, i lectura obligatòria abans d'editar (amb algunes excepcions en models nous). Rutes absolutes. [tools-reference](https://code.claude.com/docs/en/tools-reference), [swe-bench-sonnet](https://www.anthropic.com/engineering/swe-bench-sonnet)
- **Ordre fix de la petició pensat per la caché**: system i eines, després context, després conversa. Canviar les eines invalida la caché. `/cost` mostra el percentatge d'encerts. [prompt-caching](https://code.claude.com/docs/en/prompt-caching)
- **Subagents Explore i Plan de només lectura**, sense CLAUDE.md, que només tornen un resum. [sub-agents](https://code.claude.com/docs/en/sub-agents)
- **Compactació**: es pot dirigir («Compact Instructions»), té una guarda contra el *thrashing* i neteja els resultats d'eina vells. [how-claude-code-works](https://code.claude.com/docs/en/how-claude-code-works)
- **Llista de tasques**: només s'activa per defecte en els models més antics. Els nous ja no la necessiten, i les definicions ocupen context. [task-tool-availability](https://code.claude.com/docs/en/tools-reference#task-tool-availability)
- **CLAUDE.md de menys de 200 línies**: els fitxers llargs redueixen l'adherència. [memory](https://code.claude.com/docs/en/memory)

### OpenAI Codex CLI
- **`apply_patch`, el format V4A en què el model està entrenat**: no porta números de línia i s'ancora amb `@@ class/def`. En models GPT-5 és una eina *freeform* restringida per una gramàtica Lark, i així el codi no s'ha d'escapar dins de JSON. [cookbook GPT-4.1](https://github.com/openai/openai-cookbook/blob/main/examples/gpt4-1_prompting_guide.ipynb), [apply_patch_spec.rs](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/handlers/apply_patch_spec.rs)
- **El mateix cookbook dona xifres sobre el prompt**:
  - Tres recordatoris al system prompt (persistència, fer servir eines, planificar) donen aproximadament +20% a SWE-bench Verified.
  - Induir una planificació explícita dona +4%.
  - Passar els esquemes d'eines per l'API en lloc d'enganxar-los al prompt dona +2%.
- **Cerca tolerant del context**: exacte, després sense espais finals, després retallat, i finalment amb puntuació Unicode convertida a ASCII. [seek_sequence.rs](https://github.com/openai/codex/blob/main/codex-rs/apply-patch/src/seek_sequence.rs)
- **Regles de prompt útils**: «no rellegeixis el fitxer després de l'apply_patch»; validar d'allò específic a allò general; el pla se salta en el 25% de tasques més fàcils. [default.md](https://github.com/openai/codex/blob/main/codex-rs/protocol/src/prompts/base_instructions/default.md)
- **Compactació com a traspàs**: el resum es planteja com «handoff summary for another LLM», i el model que continua rep «Another language model started…». La sortida de shell es talla amb cap i cua. [compact prompt](https://github.com/openai/codex/blob/main/codex-rs/prompts/templates/compact/prompt.md), [output-truncation](https://github.com/openai/codex/blob/main/codex-rs/utils/output-truncation/src/lib.rs)

### Gemini CLI i Qwen Code
- **Detecció de bucles en tres capes**: crida repetida 5 vegades; trossos de 50 caràcters repetits 10 vegades dins del *stream*; i, a partir del torn 30, un jutge LLM que mira els últims 20 torns. [loopDetectionService.ts](https://github.com/google-gemini/gemini-cli/blob/main/packages/core/src/services/loopDetectionService.ts)
- **Edició que es corregeix sola**: prova exacte, flexible, regex i fuzzy. Si tot falla, un LLM proposa un parell search/replace corregit. [llm-edit-fixer.ts](https://github.com/google-gemini/gemini-cli/blob/main/packages/core/src/utils/llm-edit-fixer.ts)
- **Compressió**: els resultats d'eina vells es redueixen a les 30 últimes línies i la sortida completa va a un fitxer temporal. El resum es fa en un `<state_snapshot>` XML, i una passada de **sondeig** comprova que no s'hi hagin perdut rutes ni detalls. [chatCompressionService.ts](https://github.com/google-gemini/gemini-cli/blob/main/packages/core/src/context/chatCompressionService.ts)
- **Qwen Code: exemples few-shot de crides d'eina segons el model** (XML per a qwen-coder, JSON per a la resta), que ensenyen el cicle sencer llegir → editar → `ruff && pytest`. També detecta bucles d'altres tipus: lectures que no avancen, cicles A-B, ordres de shell repetides. [prompts.ts](https://github.com/QwenLM/qwen-code/blob/main/packages/core/src/core/prompts.ts), [loopDetectionService.ts](https://github.com/QwenLM/qwen-code/blob/main/packages/core/src/services/loopDetectionService.ts)

### Kimi CLI i Crush
- **Kimi**:
  - Compacta al 85%, o quan falten menys de 50k tokens.
  - El prompt diu «Make MINIMAL changes» i acaba amb «test what you build».
  - Té **D-Mail**, una eina perquè el model replegui un camí mort tornant a un *checkpoint*. Experimental, fora del perfil per defecte.
  - [compaction.py](https://github.com/MoonshotAI/kimi-cli/blob/main/src/kimi_cli/soul/compaction.py), [system.md](https://github.com/MoonshotAI/kimi-cli/blob/main/src/kimi_cli/agents/default/system.md), [dmail.md](https://github.com/MoonshotAI/kimi-cli/blob/main/src/kimi_cli/tools/dmail/dmail.md)
- **Crush**:
  - LSP natiu: diagnòstics després de cada edició, eines `lsp_definition`, `references` i `lsp_rename`, i **`lsp_replace_symbol`**, que edita una funció sencera pel nom i així evita tots els problemes d'espais.
  - Exigeix llegir abans d'editar i comprova el mtime.
  - Detecta bucles amb un hash de (crides + resultats): salta si es repeteix més de 5 vegades en els últims 10 passos.
  - [edit.go](https://github.com/charmbracelet/crush/blob/main/internal/agent/tools/edit.go), [lsp_replace_symbol.md](https://github.com/charmbracelet/crush/blob/main/internal/agent/tools/lsp_replace_symbol.md), [loop_detection.go](https://github.com/charmbracelet/crush/blob/main/internal/agent/loop_detection.go)

### SWE-agent i mini-swe-agent (ACI)
- **Ablacions del paper amb GPT-4 Turbo, SWE-bench Lite** (amb l'ACI sencera, 18,0%):
  - Visor de 100 línies amb números de línia. Si es mostra el fitxer sencer, el resultat baixa a 12,7%.
  - Edició que **es desfà si el linter detecta errors**. Sense lint, 15,0%.
  - Cerca resumida. Paginar els resultats d'un en un dona 12,0%, menys que no tenir cap eina de cerca.
  - Missatge explícit quan una ordre no produeix sortida.
  - Les observacions de més de 5 passos enrere es redueixen a una sola línia.
  - [arXiv 2405.15793](https://arxiv.org/abs/2405.15793)
- **mini-swe-agent**: només bash, unes 100 línies, historial lineal. Diu passar del 74% a SWE-bench Verified. Amb els models forts, una ACI pesada aporta poc. [repo](https://github.com/SWE-agent/mini-swe-agent)

### OpenHands, Agentless i companyia
- **OpenHands**:
  - CodeAct: les accions són codi Python. Fins a +20% en 17 LLM. [arXiv 2402.01030](https://arxiv.org/abs/2402.01030)
  - El condensador amb resum LLM fa el cost per torn fins a 2 vegades més baix amb la mateixa taxa de resolució (54% contra 53%). L'historial és un registre on només s'afegeix, i les condensacions hi queden com a «tombstones». [blog](https://www.openhands.dev/blog/openhands-context-condensensation-for-more-efficient-ai-agents), [README del condensador](https://raw.githubusercontent.com/OpenHands/software-agent-sdk/main/openhands-sdk/openhands/sdk/context/condenser/README.md)
  - Hi ha un crític entrenat per al best-of-N: 60,6% amb un intent i 66,4% amb el millor de 5. *Aquestes xifres vénen de fragments de cerca; no he llegit la pàgina.* [blog](https://www.openhands.dev/blog/sota-on-swe-bench-verified-with-inference-time-scaling-and-critic-model)
- **Agentless** (32% a SWE-bench Lite amb 0,70 $ per tasca) és un procés fix:
  - Localitza per fitxer, després per esquelet de classes i funcions, i finalment la línia exacta.
  - Genera diverses candidates en format SEARCH/REPLACE i les normalitza.
  - Escriu un **test de reproducció**, que només es conserva si falla abans del canvi.
  - Filtra amb els tests de regressió i tria per **vot de majoria**.
  - [arXiv 2407.01489](https://arxiv.org/abs/2407.01489)
- **CodeMonkeys**: cada trajectòria escriu a la vegada el test i l'edició, i se'n tria una per vot amb els tests generats. Arriba al 57,4% a Verified, i al 66,2% si aquesta selecció s'aplica a un conjunt d'altres sistemes. [arXiv 2501.14723](https://arxiv.org/abs/2501.14723)
- **R2E-Gym**: els verificadors per execució i els que no executen són complementaris. Cadascun sol arriba al 42–43%; junts, al 51%. [arXiv 2504.07164](https://arxiv.org/abs/2504.07164)
- **AutoCodeRover** cerca sobre l'AST i localitza errors per l'espectre dels tests ([arXiv 2404.05427](https://arxiv.org/abs/2404.05427)). **Moatless** fa servir eines estructurades com `find_class`, `find_function` i `view_code` ([repo](https://github.com/aorwall/moatless-tools)).

### Cline, Roo, Goose, Zed, Cursor i Continue
- **Cline**:
  - SEARCH/REPLACE amb tres nivells: exacte, línies retallades, i àncora a la primera i l'última línia. [diff.ts](https://github.com/cline/cline/blob/v3.30.0/src/core/assistant-message/diff.ts)
  - La **Focus Chain** recorda el todo cada 6 missatges. [focus-chain](https://github.com/cline/cline/blob/v3.30.0/docs/features/focus-chain.mdx)
  - Hi ha una **variant de prompt `xs`** per a models petits o locals, amb menys seccions i menys eines, i la documentació demana activar «Use Compact Prompt» amb Ollama i LM Studio (un prompt aproximadament un 90% més curt). [variants](https://github.com/cline/cline/tree/v3.30.0/src/core/prompts/system-prompt/variants), [running-models-locally](https://github.com/cline/cline/blob/main/docs/running-models-locally/overview.mdx)
- **Roo Code**:
  - `apply_diff` amb una pista `:start_line:`, cerca fuzzy de Levenshtein que comença pel mig, i error amb el percentatge de semblança i **la millor coincidència amb números de línia**. [multi-search-replace.ts](https://github.com/RooCodeInc/Roo-Code/blob/main/src/core/diff/strategies/multi-search-replace.ts)
  - Després de condensar, torna a injectar les definicions tree-sitter dels fitxers que ha llegit («folded file context»). [condense](https://github.com/RooCodeInc/Roo-Code/blob/main/src/core/condense/index.ts)
- **Goose**:
  - *Toolshim*: un model intèrpret converteix en crides el text dels models que no saben cridar eines. Amb mistral-nemo d'intèrpret, phi4 i gemma3 s'acosten a llama3.3 70B ([blog](https://github.com/block/goose/blob/main/documentation/blog/2025-04-11-finetuning-toolshim/index.md)). Però en el seu benchmark, els models amb shim «consistently score lower» ([goose-benchmark](https://goose-docs.ai/blog/2025/03/31/goose-benchmark/)).
  - El mode Lead/Worker s'ha retirat.
- **Zed**:
  - Edicions en *streaming* amb un *matcher* fuzzy de programació dinàmica. Esborrar costa 10 i inserir, 3. [streaming_fuzzy_matcher.rs](https://github.com/zed-industries/zed/blob/main/crates/agent/src/tools/edit_session/streaming_fuzzy_matcher.rs)
  - Al prompt, un exemple aporta aproximadament un 10% de pass rate amb Gemini 2.5. [edit_file_prompt_xml.hbs](https://github.com/zed-industries/zed/blob/main/crates/agent/src/templates/edit_file_prompt_xml.hbs)
  - Fa tests estocàstics de 100 a 200 execucions per cas. [zenml](https://www.zenml.io/llmops-database/testing-and-evaluation-strategies-for-ai-powered-code-editor-with-agentic-editing)
- **Cursor**:
  - Un model d'*apply* reescriu el fitxer sencer amb **speculative edits**: el codi existent fa d'esborrany, i això dona unes 13 vegades més velocitat ([instant-apply](https://cursor.com/blog/instant-apply)).
  - La cerca semàntica, combinada amb grep, dona un +12,5% de precisió ([semsearch](https://cursor.com/blog/semsearch)).
- **Continue Instinct**: un model local de *next edit*, Qwen2.5-Coder-7B ajustat, que funciona a Ollama. [blog](https://blog.continue.dev/instinct/)

### Recerca sobre context
- **The Complexity Trap** (JetBrains):
  - **Emmascarar les observacions velles**, amb una finestra de 10, **redueix el cost a la meitat** i iguala o supera el resum amb LLM.
  - El resum allarga les trajectòries un 15%, perquè amaga els senyals que indiquen que ja es pot parar.
  - L'opció híbrida (emmascarar sempre i resumir poc, en blocs grans) surt entre un 7% i un 11% més barata.
  - [arXiv 2508.21433](https://arxiv.org/abs/2508.21433)
- **Context Rot** (Chroma): amb un sol distractor la precisió ja baixa. Un prompt enfocat de ~300 tokens guanya al prompt sencer de 113k. [research.trychroma.com](https://research.trychroma.com/context-rot)
- **Lost in the Middle**: la informació del mig del context es fa servir pitjor. [arXiv 2307.03172](https://arxiv.org/abs/2307.03172)
- **Anthropic**:
  - Recuperar la informació en el moment que cal. Netejar els resultats d'eina és la forma de compactació «més segura i lleugera». Notes estructurades. [context engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)
  - Missatges d'error que diuen com corregir la crida, i una opció `response_format` concisa (fa servir aproximadament ⅓ dels tokens). [writing tools](https://www.anthropic.com/engineering/writing-tools-for-agents)
- **Manus**:
  - **La taxa d'encerts de la caché de KV és la mètrica clau**: en un agent, l'entrada és ~100 vegades la sortida.
  - Historial on només s'afegeix i serialització determinista.
  - **Emmascarar eines en lloc de treure-les.**
  - Compressió que es pot desfer: es guarda la ruta, no el contingut.
  - Recitar el todo al final del context.
  - Deixar els errors dins del context.
  - [blog](https://manus.im/blog/Context-Engineering-for-AI-Agents-Lessons-from-Building-Manus)

---

## 3. Llista de mancances respecte de Gregal

| Tècnica | Qui la fa servir | Gregal avui |
|---|---|---|
| Historial on només s'afegeix i prefix estable per byte | Manus, Claude Code, DeepSeek, OpenHands | ✗ `SenseMapesAntics` reescriu els missatges d'usuari vells a cada torn |
| Mesura de la caché (encerts) | Claude Code `/cost`, Manus | ~ Llegeix `cached_tokens` en OpenAI i Anthropic, però no llegeix `timings.cache_n` de llama.cpp ni en mostra la taxa |
| `num_ctx` i `keep_alive` d'Ollama, API nativa | Aider (avisa), documentació d'Ollama | ✗ |
| `id_slot`, `cache_prompt` i `timings` de llama.cpp | documentació de llama.cpp | ✗ |
| Sondeig de capacitats (`/props`, `/api/show`, `/api/ps`, LM Studio `/api/v0/models`) | — | ~ Només `/models` |
| Cascada d'edició tolerant amb guardes | opencode, Codex, Cline, Roo, Gemini, Zed | ✗ Només CRLF |
| Error amb la millor coincidència i números de línia | Roo, Aider | ~ Diu el motiu, però no ensenya la coincidència |
| Llegir abans d'editar i detectar fitxers desfasats | Claude Code, DeepSeek, Crush, Qwen | ✗ |
| Diagnòstics LSP i edició per símbol | opencode, Crush, Qwen Code | ~ Compiladors per línia d'ordres, sense LSP |
| Emmascarar observacions per blocs | JetBrains, SWE-agent, opencode | ~ Retalla a 600 caràcters en arribar al llindar, i després resumeix |
| Truncar amb cap i cua i desar la sortida completa | opencode, Codex, Gemini, DeepSeek | ✗ Només el cap, i 8000 caràcters |
| Evitar lectures repetides | Codex («no rellegeixis») | ✗ |
| Prompt i eines segons el model, i variant compacta per a models locals | opencode, Cline `xs`, Aider, Qwen Code | ✗ |
| Configuració de mostreig per família | model cards | ✗ Només la temperatura del rol |
| Executar les crides d'eina escrites en text | Goose toolshim, Cline (XML) | ✗ Només les elimina i torna a demanar |
| Reparar els arguments JSON | opencode 2.0 | ✗ Es converteixen en `{}` |
| Repo map amb graf de referències i PageRank | Aider | ✗ Rànquing lèxic |
| Detectar repeticions dins del *stream* i cicles A-B | Gemini, Qwen Code | ~ DoomTracker mira crides i sortides, però no el text |
| Model petit per a les tasques laterals | Aider (weak model), Goose | ✗ Les tasques laterals fan servir el rol chat, i la compactació fa servir el mateix model |
| Arquitecte/editor | Aider, Cursor (apply) | ✗ |
| Selecció automàtica del best-of-N amb tests o vot | Agentless, CodeMonkeys, OpenHands | ~ Hi ha puntuació, però tria l'usuari |
| Recitar el todo cada N passos | Cline Focus Chain, Manus | ~ Avisa si no hi ha todo, però no el recita |
| Mesurar pass@k i variància, i errors d'edició i d'eines | Zed (estocàstic), Aider (well-formed) | ✗ |

---

## 4. Propostes per prioritat

Ordenades per impacte respecte de l'esforç. Esforç: S (≤2 dies), M (≤1 setmana), L (més d'1 setmana). ★ = pas següent recomanat.

### ★P1. Mesurar abans de canviar: bench reproduïble amb mètriques noves
- **Què és**:
  - Un runner per a `bench/` (t1, t3–t9 i la prova amagada) dins de `--eval`, amb **`--repeat N`**, que doni la taxa d'èxit, la mediana i el rang interquartílic.
  - Mètriques noves al `summary.json`:
    - `cache_hit_rate`: `cached_tokens` dividit pels tokens de prompt; en llama.cpp, `timings.cache_n/(cache_n+prompt_n)`.
    - `prefill_tokens` de veritat.
    - `edit_fail_rate`: edicions rebutjades respecte del total.
    - `tool_call_invalid`: JSON invàlid, crides en text, eines inexistents.
    - `doom_hits`.
    - `compactions`.
- **Per què**: Zed fa servir tests estocàstics de 100 a 200 execucions, i Aider mesura el *well-formed rate*. Sense mesures, la resta de propostes no es pot validar ni comparar entre models locals.
- **Fitxers**: `internal/evals/evals.go`, `internal/agent/run.go` (`RunResult`), `internal/llm/usage.go`, `internal/llm/client.go` (llegir `timings`), `bench/` (afegir-hi `.prompt` i `.assert`).
- **Esforç**: M. **Impacte**: habilita totes les altres propostes.
- **Com es mesura**: un `results/summary.json` per cada perfil de model, comparat amb `baseline.json`.

### ★P2. Prefix estable i historial on només s'afegeix
- **Què és**:
  1. Deixar de reescriure els missatges d'usuari antics:
     - Els mapes vells s'han de deixar tal com són, i el mapa nou ha de ser un **delta**: només el que ha canviat a git status i als fitxers.
     - Si cal netejar, que sigui **per blocs**: en compactar o en passar un llindar, no a cada torn.
  2. Treure el resum de compactació del system prompt i posar-lo en un missatge `user` marcat (`<compacted-summary>`, com DeepSeek).
  3. Verificar que el system prompt no canvia durant la sessió:
     - Si `.gregal/memory.md`, l'índex de skills o el de knowledge canvien a mitja sessió, l'actualització s'ha d'injectar com a missatge al final, no al system prompt.
     - L'ordre de les eines i el de les claus JSON han de ser deterministes.
  4. Test de regressió: dos torns consecutius han de compartir el prefix byte a byte.
- **Per què**:
  - Manus considera la taxa d'encerts de la caché de KV la mètrica principal: un sol token diferent invalida tot el que ve després.
  - En local, **cada torn nou de l'usuari torna a processar tota la conversa**: un *prefill* de 30k tokens triga 10–35 s en un Mac amb un model de 7B ([discussió llama.cpp #4167](https://github.com/ggml-org/llama.cpp/discussions/4167)).
  - En remot, l'entrada amb caché és ~10 vegades més barata.
  - Claude Code i DeepSeek ordenen la petició precisament per això.
- **Fitxers**: `internal/agent/historial.go`, `internal/agent/mapa_projecte.go`, `internal/tui/app.go` (`sysPrompt`, ~l. 2135/2363), `internal/web/serve.go` (~l. 2059/2483), `internal/telegram/turn.go`, `internal/agent/mode.go` (`AmbContext`), `internal/agent/histcompact.go`.
- **Esforç**: S–M. **Impacte**: molt alt en local (latència del primer token), alt en remot (cost).
- **Com es mesura**: `cache_hit_rate` de P1 en una sessió de 5 torns. Amb llama.cpp, mirar `timings.prompt_n` a partir del segon torn.

### ★P3. Proveïdors locals de primera: Ollama natiu, slots de llama.cpp i sondeig de capacitats
- **Què és**:
  - **Ollama**:
    - Un *provider type* `ollama` que faci servir `/api/chat`, amb `options.num_ctx` fix (≥32k, o el que ja estigui carregat segons `/api/ps`) i `keep_alive` llarg (o `-1`).
    - Mai no s'ha de variar `num_ctx` d'una petició a l'altra, perquè això torna a carregar el model.
  - **llama.cpp**:
    - `cache_prompt: true` explícit.
    - `id_slot` fix per sessió, i un altre slot per a les tasques laterals (títols, memòria, resums), perquè no expulsin el prefix principal.
    - Llegir `timings` (`cache_n`, `prompt_n`, `prompt_per_second`).
  - **Sondeig a l'arrencada**:
    - llama.cpp `/props`: `n_ctx`, `chat_template`, `total_slots`.
    - Ollama `/api/show` (si `capabilities` inclou `tools`, i `context_length`) i `/api/ps`.
    - LM Studio `/api/v0/models` (`max_context_length`, `state`).
    - vLLM `max_model_len`, que ja es llegeix.
  - **`gregal doctor`**:
    - Avisar si Ollama té `num_ctx` < 16k.
    - Avisar si llama.cpp té `--context-shift` actiu, o si fa servir la plantilla «Generic».
- **Per què**:
  - El `num_ctx` per defecte d'Ollama és de 2048 o 4096, segons la documentació, i Ollama **descarta en silenci** el principi del context, és a dir, el system prompt i les eines ([context-length](https://github.com/ollama/ollama/blob/main/docs/context-length.mdx), [aider ollama](https://aider.chat/docs/llms/ollama.html)). La documentació d'Ollama recomana ≥64k per a agents.
  - Si canvia el `num_ctx`, el model es torna a carregar ([issue #9749](https://github.com/ollama/ollama/issues/9749)).
  - Amb `keep_alive` per defecte (5 min), el model es descarrega i es perd la caché ([faq](https://github.com/ollama/ollama/blob/main/docs/faq.mdx)).
  - Els slots i els `timings` de llama.cpp estan descrits al [README del servidor](https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md).
- **Fitxers**: `internal/config/cataleg.go`, un nou `internal/llm/ollama.go` (o una variant de `client.go`), `internal/llm/probe.go`, `internal/agent/window.go`, `internal/llm/compat.go`, `doctor.go`, `docs/provider-presets.md`.
- **Esforç**: M. **Impacte**: molt alt per als usuaris locals, i elimina una classe sencera d'errors silenciosos.
- **Com es mesura**: P1 amb Ollama i amb llama.cpp. Temps fins al primer token del torn 2 o més; `load_duration` d'Ollama (ha de ser ~0 després del primer torn); taxa d'èxit amb el `num_ctx` per defecte comparada amb el `num_ctx` gestionat.

### ★P4. Cascada d'edició tolerant amb guardes, i error que ensenya la millor coincidència
- **Què és**:
  - A `edit` i `patch`, provar en ordre:
    1. exacte;
    2. línies sense espais finals;
    3. línies retallades;
    4. Unicode normalitzat (cometes, guions, espais rars), com fa Codex;
    5. indentació relativa, conservant la indentació original;
    6. àncores a la primera i l'última línia, amb semblança ≥0,65 al mig, per a blocs de 3 línies o més.
  - **Guardes**:
    - La coincidència ha de ser única.
    - S'ha de rebutjar una coincidència desproporcionada (la de opencode).
    - En una coincidència no exacta, el resultat ha d'avisar «s'ha aplicat amb coincidència aproximada (nivell X)».
  - Si tot falla, l'error ha d'ensenyar **el fragment més semblant amb els números de línia** i el percentatge de semblança, com fa Roo.
- **Per què**:
  - A Aider, sense aplicació flexible hi ha 9 vegades més errors d'edició ([unified-diffs](https://aider.chat/docs/unified-diffs.html)).
  - Tots els harnesses principals fan alguna cascada: [opencode](https://github.com/anomalyco/opencode/blob/v1.19.2/packages/opencode/src/tool/edit.ts), [Codex](https://github.com/openai/codex/blob/main/codex-rs/apply-patch/src/seek_sequence.rs), [Cline](https://github.com/cline/cline/blob/v3.30.0/src/core/assistant-message/diff.ts), [Roo](https://github.com/RooCodeInc/Roo-Code/blob/main/src/core/diff/strategies/multi-search-replace.ts).
  - Els models locals s'equivoquen sobretot amb els espais i la indentació.
- **Fitxers**: `internal/tools/tools.go` (`Edit`, `perQueNoHiEs`), `internal/tools/patch.go`, i un nou `internal/tools/fuzzy.go` amb tests.
- **Esforç**: S–M. **Impacte**: alt, i més en models petits.
- **Com es mesura**: `edit_fail_rate` i passos per tasca (P1). Un test unitari amb un corpus d'edicions fallides reals, extretes dels logs.

### ★P5. Emmascarar observacions per blocs i truncar amb cap i cua
- **Què és**:
  1. Truncar en el moment d'afegir la sortida, no més tard. Es guarda el **cap i la cua** (per exemple, 60% i 40%), la sortida completa es desa a `.gregal/out/<id>.txt` i el model rep una pista («llegeix-ne el rang amb read»). Avui només es guarda el cap, i els errors de compilació i de test solen ser al final.
  2. Substituir les observacions velles per un text que permet recuperar-les: «[sortida de `read foo.go` esborrada; torna-la a llegir si cal]».
     - S'ha de fer **per blocs**: quan es passa un llindar, s'emmascara tot fins al pas N−10 d'un sol cop, amb histèresi. Així la caché només es trenca de tant en tant.
  3. El resum LLM ha de quedar com a darrer recurs, poc freqüent i en blocs grans.
- **Per què**:
  - JetBrains: emmascarar redueix el cost a la meitat i iguala o supera el resum, que a més allarga les trajectòries un 15% ([arXiv 2508.21433](https://arxiv.org/abs/2508.21433)).
  - DeepSeek poda gratis abans de resumir.
  - opencode i Gemini desen la sortida completa en un fitxer.
  - El retall actual a 600 caràcters es fa sobre totes les sortides menys les 6 últimes, i de cop. Si es fa per blocs, la caché es preserva.
- **Fitxers**: `internal/agent/histcompact.go` (`PruneToolOutputs`, `MakeRoom`), `internal/agent/compact.go`, `internal/tools/tools.go` (`truncate`, `MaxOutputChars`).
- **Esforç**: M. **Impacte**: alt (cost, latència en local, menys «context rot»).
- **Com es mesura**: P1 amb tasques llargues (t9). Tokens d'entrada, compactacions, taxa d'èxit i `cache_hit_rate`.

### P6. Registre de perfils per família de model
- **Què és**: una taula declarativa, `internal/llm/perfils.go` (o en YAML), indexada per patró de model. Per a cada família guarda:
  - la variant de prompt: `full` o `compact`;
  - el conjunt d'eines: `full` o `core`;
  - el format d'edició preferit;
  - el mostreig recomanat: temp, top_p, top_k, min_p, presence_penalty;
  - com es comporta amb el thinking: com s'activa i es desactiva, i què cal fer amb el raonament a l'historial;
  - la finestra;
  - si se sap que les crides d'eina natives fallen.

  Aquesta taula hauria de substituir `finestresConegudes` i `glmForcesReasoning`, que avui són casos aïllats.
- **Per què**:
  - opencode té un prompt per família; Aider té `model-settings.yml` amb ablacions; Qwen Code té exemples few-shot segons el model.
  - Les model cards demanen configuracions de mostreig concretes, per exemple:
    - Qwen3 no s'ha de fer servir mai amb *greedy decoding*, perquè entra en bucle ([Qwen3-8B](https://huggingface.co/Qwen/Qwen3-8B)).
    - Qwen3-Coder: temp 0,7, top_p 0,8, rep 1,05 ([card](https://huggingface.co/Qwen/Qwen3-Coder-30B-A3B-Instruct)).
    - Devstral: temp 0,15 ([card](https://huggingface.co/mistralai/Devstral-Small-2507)).
    - gpt-oss: temp 1,0, i el CoT es passa durant les crides d'eina ([harmony](https://github.com/openai/openai-cookbook/blob/main/articles/openai-harmony.md)).
    - DeepSeek-R1: sense system prompt ([card](https://huggingface.co/deepseek-ai/DeepSeek-R1)).
- **Fitxers**: un fitxer nou a `internal/llm/`, `internal/agent/window.go`, `internal/llm/compat.go`, `internal/config/config.go` (el rol pot sobreescriure el perfil), `internal/agent/mode.go`.
- **Esforç**: M. **Impacte**: alt en local, mitjà en remot.
- **Com es mesura**: P1 per model, comparant el perfil per defecte amb el perfil propi de la família.

### P7. Conjunt d'eines reduït segons el mode i el model, decidit a l'inici de la sessió
- **Què és**:
  - Els modes chat i inspect no han d'enviar `write`, `edit`, `patch`, `office_*`, `gh_pr_create` ni `git_commit`.
  - El perfil `core` (P6) es queda amb unes 10–12 eines: read, edit, patch, write, bash, grep, glob, symbols, todowrite, question i delegate.
  - Les eines MCP i office s'afegeixen només si el projecte o l'usuari les fa servir.
  - **El conjunt es fixa a l'inici** i no canvia durant la sessió, per no trencar la caché (consell de Manus).
- **Per què**:
  - «Less is More»: un model de 8B quantitzat falla amb 46 eines i funciona amb 19 ([arXiv 2411.15399](https://arxiv.org/abs/2411.15399)).
  - Els esquemes de les eines són uns 16 KB que es tornen a processar cada vegada que la caché falla.
  - Cline `xs` també retalla les eines.
- **Fitxers**: `internal/agent/loop.go` (`Specs`, `SpecsAll`), `internal/tui/app.go` (~1153), `internal/web/motor_web.go` (~321), `internal/telegram/turn.go` (~440).
- **Esforç**: S. **Impacte**: mitjà–alt en local.
- **Com es mesura**: tokens de prompt del primer torn i `tool_call_invalid` (P1).

### P8. Executar les crides d'eina escrites en text, i reparar els arguments
- **Què és**:
  - Ara, quan el model escriu una crida en text, Gregal l'elimina i la torna a demanar. En lloc d'això, s'han d'**interpretar i executar** els formats següents, de la mateixa manera que ja es fa amb el DSML:
    - Hermes: `<tool_call>{json}</tool_call>`;
    - Qwen3-Coder: `<function=..><parameter=..>`;
    - JSON pelat: `{"name":..,"arguments":..}`.
  - Afegir-hi la reparació d'arguments d'opencode 2.0: JSON dins d'una cadena, números en cadena, booleans en cadena.
  - Si tot falla, l'error ha de dir quina part no s'ha pogut interpretar. Mai no s'ha de convertir en `{}` en silenci.
- **Per què**:
  - És exactament el problema de Cline a l'[issue #10843](https://github.com/cline/cline/issues/10843): Qwen2.5-Coder a Ollama escriu JSON pelat i l'agent entra en bucle.
  - Repetir la petició costa un torn sencer, que en local vol dir un *prefill* més.
  - Reparar arguments és barat i elimina molts fracassos ([tool-input-repair.ts](https://github.com/anomalyco/opencode/blob/dev/packages/core/src/plugin/tool-input-repair.ts)).
- **Fitxers**: `internal/agent/einatext.go`, `internal/llm/dsml.go` (generalitzar-lo en un `toolmarkup` amb diversos formats), `internal/llm/client.go` (`safeToolCalls`), `internal/agent/motor.go` (`textRetries`).
- **Esforç**: S–M. **Impacte**: alt en models locals petits.
- **Com es mesura**: `tool_call_invalid` i torns perduts (P1) amb Qwen2.5-Coder o Qwen3 a Ollama.

### P9. Llegir abans d'editar i detectar fitxers desfasats
- **Què és**:
  - Recordar quins fitxers s'han llegit, amb el hash o el mtime de cada lectura.
  - Una edició sobre un fitxer que no s'ha llegit, o que ha canviat des de la lectura, s'ha de rebutjar amb «llegeix-lo i torna-ho a provar».
  - Excepció: si `old` coincideix exactament i de manera única amb la versió actual, es pot aplicar. És la política nova de Claude Code.
- **Per què**: Claude Code, DeepSeek (`FS_NOT_OBSERVED` / `FS_STALE_VERSION`) i Crush ho fan. Evita edicions fetes a cegues i les curses amb altres agents o amb l'usuari, que Gregal pot tenir en team/flows.
- **Fitxers**: `internal/tools/tools.go`, `internal/tools/patch.go`, `internal/agent/loop.go`, i l'estat de la sessió.
- **Esforç**: S. **Impacte**: mitjà.
- **Com es mesura**: `edit_fail_rate`, i quants fitxers es fan malbé a les proves de team.

### P10. Evitar lectures repetides
- **Què és**:
  - Si `read` demana un fitxer (i un rang) que no ha canviat des de l'última lectura, encara visible al context, s'ha de respondre «sense canvis des del pas N».
  - Després d'una edició, n'hi ha prou de tornar el diff i el context proper, sense rellegir el fitxer.
  - Afegir al prompt la regla de Codex: «no rellegeixis després d'editar».
- **Per què**: rellegir els mateixos fitxers és dels malbarataments més grans en un agent. En local, cada lectura repetida són milers de tokens de *prefill*. [Codex default.md](https://github.com/openai/codex/blob/main/codex-rs/protocol/src/prompts/base_instructions/default.md)
- **Fitxers**: `internal/tools/tools.go` (`Read`), `internal/agent/hook.go`, `internal/agent/mode.go` (`codeRules`).
- **Esforç**: S. **Impacte**: mitjà–alt en tokens.
- **Com es mesura**: tokens d'entrada i nombre de crides a `read` per tasca (P1).

### P11. Prompt compacte per a models locals o petits
- **Què és**: una variant del system prompt i de `codeRules` de menys d'1k tokens, amb només les regles crítiques, juntament amb el conjunt d'eines `core`. S'activa amb el perfil (P6), amb `prompt: compact` o automàticament quan la finestra és menor de 32k.
- **Per què**:
  - Cline `xs` i «Use Compact Prompt» redueixen el prompt aproximadament un 90% ([docs](https://github.com/cline/cline/blob/main/docs/running-models-locally/overview.mdx)).
  - Chroma: els prompts enfocats rendeixen més que els llargs ([context-rot](https://research.trychroma.com/context-rot)).
  - El preset *minimal* de DeepSeek té una persona d'una sola línia.
- **Fitxers**: `internal/agent/mode.go`, `internal/config/config.go`.
- **Esforç**: S. **Impacte**: mitjà–alt en local.
- **Com es mesura**: P1 amb un model de 7–14B, prompt complet contra prompt compacte (èxit i temps fins al primer token).

### P12. Model petit i amb restriccions per a les tasques laterals
- **Què és**:
  - Un rol `small` o `fast` per a la compactació, els títols, `automem`, les decisions del router, el scout i la selecció de fitxers.
  - Quan el servidor ho permet, fer servir sortida estructurada: `response_format` amb `json_schema`, `grammar` a llama.cpp, `format` a Ollama i `structured_outputs` a vLLM.
  - En local, aquestes crides han d'anar en un altre `id_slot` (P3).
- **Per què**:
  - Aider fa servir un *weak model* per als missatges de commit i els resums.
  - Les gramàtiques garanteixen JSON vàlid ([grammars](https://github.com/ggml-org/llama.cpp/blob/master/grammars/README.md), [vLLM structured outputs](https://github.com/vllm-project/vllm/blob/main/docs/features/structured_outputs.md)).
  - Avui la compactació fa servir el mateix model, que pot ser el més lent.
- **Fitxers**: `internal/agent/compact.go`, `internal/agent/aside.go`, `internal/agent/automem.go`, `internal/config/router.go`, `internal/agent/scout.go`, `internal/config/config.go`.
- **Esforç**: S–M. **Impacte**: mitjà (latència i cost).
- **Com es mesura**: temps de compactació i qualitat del resum (taxa d'èxit després de compactar, a t9).

### P13. Detectar repeticions dins del *stream* i bucles més rics
- **Què és**: estendre DoomTracker amb tres coses:
  - **trossos de 50 caràcters repetits** dins del *stream*: s'avorta la generació i es reintenta amb `presence_penalty`, o amb un avís;
  - cicles A-B;
  - moltes lectures sense cap acció.
- **Per què**:
  - Els models locals, sobretot Qwen3 i R1 a temperatura baixa, entren en repeticions sense fi i cremen tota la sortida. En local, això són minuts.
  - Gemini CLI i Qwen Code ho detecten ([gemini](https://github.com/google-gemini/gemini-cli/blob/main/packages/core/src/services/loopDetectionService.ts), [qwen](https://github.com/QwenLM/qwen-code/blob/main/packages/core/src/services/loopDetectionService.ts)).
- **Fitxers**: `internal/agent/finalize.go` (DoomTracker), `internal/llm/client.go` (stream), `internal/llm/continua.go`.
- **Esforç**: S. **Impacte**: mitjà, alt amb models locals.
- **Com es mesura**: tokens de sortida per tasca i casos de `finish_reason=length` (P1).

### P14. Calibrar el comptatge de tokens
- **Què és**:
  - Aprendre per cada model la ràtio entre caràcters i tokens a partir del `prompt_tokens` que retorna el proveïdor, i aplicar-la a `EstimateTokens`.
  - Opcionalment, fer servir `/tokenize` de llama.cpp.
  - La compactació s'ha de disparar amb l'estimació calibrada.
- **Per què**:
  - El càlcul de caràcters/4 té un error d'un ±30%. Amb finestres petites, si s'estima de menys, Ollama trunca en silenci; si s'estima de més, es compacta massa aviat.
  - DeepSeek calcula el llindar amb la reserva de sortida.
- **Fitxers**: `internal/llm/client.go` (`EstimateTokens`), `internal/llm/usage.go`, `internal/agent/compact.go`.
- **Esforç**: S. **Impacte**: mitjà en local.
- **Com es mesura**: error de l'estimació respecte de l'ús real, registrat a P1.

### P15. Format d'edició segons la mida del model, i sense codi dins de JSON
- **Què és**:
  - Per a models de menys de ~30B, o quan el perfil ho indiqui, preferir `write` (fitxer sencer) per als fitxers petits (menys de ~300 línies), i deixar `edit` per als grans.
  - Estudiar una eina d'edició amb el cos com a text lliure, fora de JSON:
    - un bloc SEARCH/REPLACE dins del contingut de l'assistent; o
    - l'eina *freeform* amb gramàtica on el servidor ho suporti (com Codex amb Lark).
- **Per què**:
  - Leaderboard d'Aider: Qwen2.5-Coder-32B treu un **8,0% amb diff (71,6% ben formats) i un 16,4% amb whole (99,6% ben formats)** ([polyglot_leaderboard.yml](https://github.com/Aider-AI/aider/blob/main/aider/website/_data/polyglot_leaderboard.yml)).
  - El codi dins de JSON empitjora la qualitat ([code-in-json](https://aider.chat/2024/08/14/code-in-json.html)).
  - El fitxer sencer s'aprofita molt bé de la descodificació especulativa per n-grames (P20).
- **Fitxers**: `internal/agent/mode.go` (regles), el perfil de P6, `internal/tools/tools.go`.
- **Esforç**: M. **Impacte**: alt amb models petits, baix amb models grans.
- **Com es mesura**: P1 amb Qwen2.5-Coder o Qwen3 de 8–32B, en mode edit contra mode whole.

### P16. Repo map amb graf de referències
- **Què és**:
  - Afegir referències (identificadors que es fan servir) a l'extractor de regex, i també a go/parser.
  - Construir un graf de fitxers i ordenar-lo amb PageRank personalitzat cap als fitxers de la tasca, com fa Aider.
  - Si cal precisió, tree-sitter per CGO o WASM, tot i que això complica la compilació.
  - Pressupost dinàmic: més tokens quan la tasca no menciona cap fitxer.
- **Per què**: el rànquing lèxic no troba el codi relacionat que no comparteix noms amb la tasca. El repo map d'Aider és un dels motius de la seva eficàcia ([repomap](https://aider.chat/docs/repomap.html)).
- **Fitxers**: `internal/repomap/rank.go`, `extract_regex.go`, `extract_go.go`.
- **Esforç**: M (amb regex) o L (amb tree-sitter). **Impacte**: mitjà.
- **Com es mesura**: passos de localització (crides `grep`/`read` abans de la primera edició) i èxit en tasques de diversos fitxers (t3, t9).

### P17. Diagnòstics LSP i edició per símbol (opcional)
- **Què és**: un client LSP mínim (gopls, typescript-language-server, pyright) que, després de cada edició, afegeix els diagnòstics del fitxer editat, com fan opencode i Crush. Més endavant, una eina `replace_symbol` que reemplaça una funció sencera pel nom.
- **Per què**:
  - Gregal ja fa diagnòstics de sintaxi amb els compiladors per línia d'ordres. L'LSP hi afegeix errors de tipus entre fitxers sense compilar.
  - `lsp_replace_symbol` evita tots els problemes d'espais ([Crush](https://github.com/charmbracelet/crush/blob/main/internal/agent/tools/lsp_replace_symbol.md)).
- **Fitxers**: un nou `internal/lsp/`, i `internal/agent/hook.go` (`apendixEdicio`).
- **Esforç**: L. **Impacte**: mitjà, i més gran en TS i Python, on els compiladors per línia d'ordres són lents o febles.
- **Com es mesura**: rondes de `--verify` que fallen, i errors de tipus que sobreviuen al final del torn.

### P18. Recitar el todo i fer el resum en format de traspàs
- **Què és**:
  - Cada ~6 passos (o després de compactar), afegir **al final** un recordatori curt amb el todo actual, com la Focus Chain de Cline i la recitació de Manus.
  - El prompt de compactació s'ha de reescriure com un «handoff» (Codex):
    - objectiu, fet, pendent, decisions, fitxers tocats, errors oberts;
    - no s'han de separar mai una crida i el seu resultat;
    - en acabar, s'han de reinjectar les signatures dels fitxers tocats recentment (Roo).
- **Per què**: lost-in-the-middle; manté els objectius a la part del context que el model atén més ([Manus](https://manus.im/blog/Context-Engineering-for-AI-Agents-Lessons-from-Building-Manus), [Cline](https://github.com/cline/cline/blob/v3.30.0/docs/features/focus-chain.mdx), [Codex](https://github.com/openai/codex/blob/main/codex-rs/prompts/templates/compact/prompt.md)).
- **Fitxers**: `internal/agent/todos.go`, `internal/agent/motor.go`, `internal/agent/compact.go`.
- **Esforç**: S. **Impacte**: mitjà en tasques llargues (autonomous, goal).
- **Com es mesura**: èxit a t9 i en tasques autònomes llargues, i requisits oblidats segons el revisor.

### P19. Best-of-N amb selecció automàtica per tests
- **Què és**:
  - En el mode headless de best-of (i, opcionalment, en l'autònom), primer s'ha de generar un **test de reproducció** i comprovar que falla abans del canvi.
  - Després es generen N candidates, es filtren amb aquest test i amb els tests de regressió, i es tria per vot o amb el revisor (`verify/score.go`) sense haver-ho de demanar a l'usuari.
- **Per què**:
  - Agentless i CodeMonkeys guanyen molt amb aquest esquema ([Agentless](https://arxiv.org/abs/2407.01489), [CodeMonkeys](https://arxiv.org/abs/2501.14723)).
  - Combinar verificadors que executen i que no executen és millor que cadascun sol ([R2E-Gym](https://arxiv.org/abs/2504.07164)).
  - Gregal ja té els worktrees i la puntuació.
- **Fitxers**: `internal/agent/bestof.go`, `cli_bestof.go`, `internal/verify/score.go`.
- **Esforç**: M. **Impacte**: alt en CI i headless, baix en ús interactiu.
- **Com es mesura**: pass@1 de best-of-3 contra un sol intent a P1.

### P20. Guia i comprovacions per al servidor local (speculative decoding, KV, flags)
- **Què és**: un document `docs/local-models.md` (ca/en) i comprovacions a `gregal doctor` sobre:
  - `--cache-reuse 256`;
  - `-ctk q8_0 -ctv q8_0`, i no `q4_0` a K;
  - `-ub` 1024–2048 per accelerar el *prefill*;
  - `--spec-type ngram-mod` per a les edicions, una mena de *prompt lookup*;
  - un model esborrany (`-md`);
  - desactivar `--context-shift`;
  - `OLLAMA_CONTEXT_LENGTH` i `OLLAMA_KEEP_ALIVE`;
  - els problemes de caché amb models SWA o híbrids (Gemma, Qwen3-Next): [issue #19794](https://github.com/ggml-org/llama.cpp/issues/19794).
- **Per què**:
  - El *prompt lookup* dona entre 2 i 4 vegades més velocitat en tasques que copien l'entrada, sense perdre qualitat ([prompt-lookup-decoding](https://github.com/apoorvumang/prompt-lookup-decoding)).
  - Quantitzar la K a q8_0 fa variar la perplexitat un ~0,03%; a q4_0, un ~3,3% ([PR #7412](https://github.com/ggml-org/llama.cpp/pull/7412)).
  - La quantització extrema de la KV empitjora les crides d'eina ([function-calling.md](https://github.com/ggml-org/llama.cpp/blob/master/docs/function-calling.md)).
- **Fitxers**: `docs/`, `doctor.go`.
- **Esforç**: S. **Impacte**: mitjà, per a usuaris que no coneixen aquests flags.
- **Com es mesura**: `prompt_per_second` i els tokens per segon que retorna llama.cpp, abans i després.

### P21. Arquitecte/editor (remot fort + editor local)
- **Què és**: un mode on el rol `architect` (remot o gran) escriu el pla i els canvis en prosa o pseudo-diff, i el rol `editor` (local i ràpid) els aplica amb el format més adequat per a ell, el fitxer sencer. És útil quan la planificació és difícil i l'edició és mecànica.
- **Per què**:
  - Aider va treure un 85% amb o1 i DeepSeek com a editor.
  - R1 amb Sonnet com a editor va igualar o1 per una catorzena part del cost.
  - **Però amb els models de raonament nous el guany desapareix** ([architect](https://aider.chat/2024/09/26/architect.html), [r1-sonnet](https://aider.chat/2025/01/24/r1-sonnet.html)).
  - Cursor separa el pla de l'aplicació ([instant-apply](https://cursor.com/blog/instant-apply)).
- **Fitxers**: `internal/agent/` (un mode o flow nou), `internal/team/flow.go`, `internal/config/router.go`.
- **Esforç**: M–L. **Impacte**: incert. Només s'hauria de fer després de P1, amb dades.
- **Com es mesura**: P1 amb una parella de models, comparada amb el model gran sol i amb el petit sol (èxit i cost).

### P22. Prompt: reproduir el problema primer i fer canvis mínims
- **Què és**: revisar `codeRules` i afegir-hi, si encara no hi són:
  - «abans de corregir un bug, reprodueix-lo amb un test o un script»;
  - «canvis mínims; no toquis els tests existents en un refactor» (Kimi);
  - «valida d'allò específic a allò general» (Codex);
  - al final, «rellegeix la petició i comprova cada requisit» (Crush).

  També s'hauria de tenir en compte el +20% que el cookbook atribueix als recordatoris de persistència, eines i planificació.
- **Per què**: [Anthropic SWE-bench](https://www.anthropic.com/engineering/swe-bench-sonnet), [Kimi system.md](https://github.com/MoonshotAI/kimi-cli/blob/main/src/kimi_cli/agents/default/system.md), [Crush coder.md.tpl](https://github.com/charmbracelet/crush/blob/main/internal/agent/templates/coder.md.tpl), [cookbook GPT-4.1](https://github.com/openai/openai-cookbook/blob/main/examples/gpt4-1_prompting_guide.ipynb).
- **Fitxers**: `internal/agent/mode.go`.
- **Esforç**: S. **Impacte**: baix–mitjà. Cal mesurar-lo, perquè pot allargar les tasques simples.
- **Com es mesura**: P1, tota la suite (canvis innecessaris i èxit).

### Taula de prioritats

| Rang | Proposta | Esforç | Impacte (local / remot) |
|---|---|---|---|
| ★1 | P1 Bench i mètriques | M | habilitador |
| ★2 | P2 Prefix estable | S–M | ●●●●● / ●●●● |
| ★3 | P3 Proveïdors locals natius | M | ●●●●● / – |
| ★4 | P4 Cascada d'edició | S–M | ●●●● / ●●● |
| ★5 | P5 Emmascarar per blocs, cap i cua | M | ●●●● / ●●●● |
| 6 | P8 Crides en text i reparació d'arguments | S–M | ●●●● / ●● |
| 7 | P7 Eines reduïdes | S | ●●●● / ●● |
| 8 | P10 Evitar lectures repetides | S | ●●● / ●●● |
| 9 | P13 Repeticions al *stream* | S | ●●●● / ●● |
| 10 | P11 Prompt compacte | S | ●●●● / ● |
| 11 | P6 Perfils per model | M | ●●●● / ●●● |
| 12 | P9 Llegir abans d'editar | S | ●●● / ●●● |
| 13 | P14 Comptatge de tokens calibrat | S | ●●● / ● |
| 14 | P18 Recitar el todo i resum de traspàs | S | ●●● / ●●● |
| 15 | P12 Model petit amb gramàtica per a les tasques laterals | S–M | ●●● / ●● |
| 16 | P15 Format d'edició segons la mida | M | ●●●● / ● |
| 17 | P20 Guia i doctor per al servidor local | S | ●●● / – |
| 18 | P19 Best-of amb tests i vot | M | ●● / ●●● |
| 19 | P16 Repo map amb graf | M–L | ●● / ●● |
| 20 | P22 Regles de prompt | S | ●● / ●● |
| 21 | P17 LSP | L | ●● / ●●● |
| 22 | P21 Arquitecte/editor | M–L | incert |

P6 surt a la posició 11, però **serveix de base** per a P7, P11 i P15: convé fer-la poc després de les cinc primeres.

---

## 5. Secció específica: models locals

### 5.1 Què limita i per què
- **El *prefill* domina el temps.** Amb Llama-2-7B Q4_0, una RTX 4090 processa ~14.770 tokens/s de prompt i en genera ~189. Un M4 Max en processa ~886 i en genera ~83 ([discussió #15013](https://github.com/ggml-org/llama.cpp/discussions/15013), [discussió #4167](https://github.com/ggml-org/llama.cpp/discussions/4167)). Si la caché falla en una conversa de 30k tokens, el Mac triga desenes de segons abans de començar a respondre. En aquests usuaris, **la taxa d'encerts de la caché pesa més que qualsevol altra optimització.**
- **El servidor pot truncar el context en silenci**: Ollama descarta el principi, i llama.cpp ho fa si té `--context-shift` actiu.
- **Les crides d'eina depenen de la plantilla de xat.** Amb llama.cpp cal `--jinja`, que ara ja és el valor per defecte. Si la plantilla cau al format «Generic», les crides són menys fiables i gasten més tokens ([function-calling.md](https://github.com/ggml-org/llama.cpp/blob/master/docs/function-calling.md)). vLLM necessita `--enable-auto-tool-choice --tool-call-parser <família>` ([tool_calling.md](https://github.com/vllm-project/vllm/blob/main/docs/features/tool_calling.md)).

### 5.2 Regles de disseny per a Gregal (què trenca la caché de KV)
1. No hi ha d'haver res variable al principi del prompt: hora, cwd canviant, git status. Gregal ja treu l'hora; falta el cwd a la web.
2. Historial on només s'afegeix: res de `SenseMapesAntics` a cada torn (P2).
3. Eines i ordre de les claus JSON deterministes, sense canviar-les durant la sessió (P7).
4. Truncar en el moment d'afegir, mai més tard. Emmascarar per blocs (P5).
5. Respectar el que fa la plantilla amb el raonament (Qwen3 i gpt-oss l'eliminen de l'historial). Si Gregal torna a enviar `reasoning_content` a un servidor local que el descarta, no passa res, però s'ha de comprovar que no reescriu els torns vells.
6. En llama.cpp, un slot per a la sessió i un altre per a les tasques laterals (P3).
7. Amb `--cache-reuse N`, llama.cpp pot recuperar el prefix després d'una secció emmascarada movent els trossos ([PR #9866](https://github.com/ggml-org/llama.cpp/pull/9866)). És un bon complement de P5.

### 5.3 Paràmetres concrets per proveïdor

| Servidor | A la petició | Al servidor (doctor / docs) | Sondeig |
|---|---|---|---|
| llama.cpp | `cache_prompt:true`, `id_slot`, `parallel_tool_calls:true` (cal activar-ho explícitament), `chat_template_kwargs`, `json_schema`/`grammar` per a les tasques laterals | `--cache-reuse 256`, `-ctk q8_0 -ctv q8_0`, `-fa on`, `-ub 1024+`, sense `--context-shift`, `--slot-save-path` (desar i restaurar el prefix escalfat), `--spec-type ngram-mod` | `GET /props` (`n_ctx`, `chat_template`, `total_slots`), `timings` a cada resposta |
| Ollama | `/api/chat` natiu, `options.num_ctx` fix, `keep_alive:"30m"` o `-1`, `format` (JSON schema) per a les tasques laterals | `OLLAMA_CONTEXT_LENGTH≥32k–64k`, `OLLAMA_KEEP_ALIVE`, `OLLAMA_FLASH_ATTENTION=1`, `OLLAMA_KV_CACHE_TYPE=q8_0` | `/api/show` (`capabilities`, `context_length`), `/api/ps` (model carregat) |
| LM Studio | API compatible amb OpenAI | TTL d'inactivitat prou llarg | `/api/v0/models` (`max_context_length`, `state`), `/api/v1/models` (`trained_for_tool_use`) |
| vLLM | `structured_outputs`, `strict:true` a les eines | `--enable-auto-tool-choice --tool-call-parser {hermes,qwen3_xml,llama3_json,mistral,deepseek_v31,openai,glm45…}`, la caché de prefix ja és activa per defecte, `speculative_config` ngram | `/v1/models` (`max_model_len`) |
| SGLang | — | RadixAttention activa per defecte | `/get_model_info` (no verificat) |

Fonts: [llama.cpp server README](https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md), [Ollama api.md](https://github.com/ollama/ollama/blob/main/docs/api.md), [Ollama faq](https://github.com/ollama/ollama/blob/main/docs/faq.mdx), [LM Studio REST](https://lmstudio.ai/docs/developer/rest/endpoints), [vLLM prefix caching](https://github.com/vllm-project/vllm/blob/main/docs/features/automatic_prefix_caching.md), [SGLang](https://arxiv.org/abs/2312.07104).

### 5.4 Crides d'eina poc fiables: ordre de les solucions
1. Natiu, amb la plantilla correcta. És el que la sonda de Gregal va trobar amb els models locals ([docs/tool-call-probe.md](../tool-call-probe.md)).
2. Gramàtica *lazy* al servidor. llama.cpp restringeix la sortida només a partir de `<tool_call>`, de manera que el JSON i els noms d'eina són vàlids sense limitar el text lliure ([PR #9639](https://github.com/ggml-org/llama.cpp/pull/9639)).
3. Interpretació tolerant al client de Hermes, Qwen-XML i JSON pelat, més la reparació d'arguments (P8).
4. *Few-shot* de crida en el format de la família (Qwen Code).
5. Com a darrer recurs, un model intèrpret (el toolshim de Goose). El mateix Goose diu que dona pitjor resultat que les crides natives.

### 5.5 Mostreig recomanat (perfils de P6)

| Família | Mostreig | Notes |
|---|---|---|
| Qwen3 | thinking: T 0,6 / top_p 0,95 / top_k 20; sense thinking: T 0,7 / 0,8 / 20; `presence_penalty` 0–2 | Mai *greedy*. Per al thinking, `enable_thinking` via `chat_template_kwargs` ([card](https://huggingface.co/Qwen/Qwen3-8B)) |
| Qwen3-Coder | T 0,7, top_p 0,8, top_k 20, rep 1,05 | Format de crida XML ([card](https://huggingface.co/Qwen/Qwen3-Coder-30B-A3B-Instruct)) |
| DeepSeek-R1 | T 0,6 | Sense system prompt, i crides d'eina poc fiables a llama.cpp ([card](https://huggingface.co/deepseek-ai/DeepSeek-R1)) |
| DeepSeek V3 en local | T ~0,3 | ([card](https://huggingface.co/deepseek-ai/DeepSeek-V3-0324)) |
| gpt-oss | T 1,0, top_p 1,0 | Harmony; `Reasoning: low/medium/high` al system prompt; el CoT es passa durant les crides d'eina ([harmony](https://github.com/openai/openai-cookbook/blob/main/articles/openai-harmony.md)) |
| GLM-4.6 | T 1,0; per a codi, top_p 0,95, top_k 40 | ([card](https://huggingface.co/zai-org/GLM-4.6)) |
| Devstral Small | T 0,15 | Entrenat amb l'scaffold d'OpenHands ([card](https://huggingface.co/mistralai/Devstral-Small-2507)) |
| Gemma 3/4 | T 1,0, top_k 64, top_p 0,95 | Problemes de caché amb SWA ([model card](https://ai.google.dev/gemma/docs/core/model_card_4)) |

### 5.6 Descodificació especulativa
- **N-grames sense model esborrany**: a llama.cpp, `--spec-type ngram-mod`; a vLLM, `method: ngram`. Va molt bé per a les edicions, que copien gran part de l'entrada: entre 2 i 4 vegades més velocitat ([prompt-lookup](https://github.com/apoorvumang/prompt-lookup-decoding), [vLLM n_gram](https://github.com/vllm-project/vllm/blob/main/docs/features/speculative_decoding/README.md)).
- **Model esborrany**: `-md`, `--spec-draft-n-max`. Els flags antics `--draft-max` i `--draft-min` ja no existeixen.
- **A Gregal**: el servidor ho ha de configurar, així que n'hi ha prou de documentar-ho (P20). Però és un argument a favor de P15: el fitxer sencer encaixa bé amb l'especulació per n-grames.

### 5.7 Compactació pensada per al *prefill*
- Emmascarar abans que resumir, i fer-ho per blocs (P5).
- El resum l'ha de fer el model petit, en un altre slot (P12).
- El llindar s'ha de calcular amb tokens calibrats (P14), i cal compactar abans d'arribar al ~85% de `num_ctx`, perquè Ollama no trunqui per sota.
- Mai no s'han de tornar a enviar fitxers sencers: diff o rang de línies, o «sense canvis» (P10).
- Es pot mesurar la velocitat de *prefill* real (`timings.prompt_per_second`) i decidir quan compactar segons els **segons** estimats de *prefill*, no només segons el percentatge de context. És una idea pròpia: no l'he trobat a cap harness.

---

## 6. Caveats

- WebFetch no funcionava. Les fonts s'han llegit en text pla (curl i l'API de GitHub), a més de WebSearch.
- Les xifres de la secció del crític d'OpenHands vénen de fragments de cerca.
- No he pogut confirmar que DeepSeek fes servir el preset *minimal* als benchmarks de V4.
- Els flags de llama.cpp són els del README actual de master. Alguns s'han canviat de nom fa poc (`--spec-*`), i convé comprovar-los contra la versió que tingui l'usuari.
- opencode té dues bases de codi: la v1.19.2, la «clàssica», i la reescriptura 2.0. Es citen totes dues.
- Els números de benchmark de tercers no són comparables entre ells. Les propostes s'han de validar amb P1 sobre els models que fan servir els usuaris de Gregal.
