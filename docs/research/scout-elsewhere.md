# Com fan l'exploració prèvia els altres harnesses

Nota de recerca, 2026-10-29. Motivació: el scout de Gregal corria **sempre** abans
del primer pas del coordinador i s'enduia 45-57 s per torn (71-87% del total, mesurat
en aquest repo). Volíem saber si això és el que fan els altres o si som un cas rar.

**Conclusió curta:** cap harness mainstream no fa un investigador **obligatori i
programat pel harness** abans del primer pas del model. L'exploració és una
delegació que decideix el model dins del bucle, un índex precalculat sense cap
crida al model, o una fase de pla que demana l'usuari. Gregal era l'excepció.

| Harness | Abans / paral·lel / dins del bucle | Com arriba el context al bucle | Font |
|---|---|---|---|
| Claude Code (Explore, Plan) | Dins del bucle, ho decideix el model. Explore és un subagent de només lectura; Plan recull context abans de proposar el pla. | Finestra de context pròpia; només en torna un resum. Els subagents **arrenquen en segon pla per defecte**; el resultat "arriba a Claude com una notificació de finalització en un torn posterior". En primer pla, bloqueja. | https://docs.claude.com/en/docs/claude-code/sub-agents |
| Codex CLI | `/plan` canvia el xat actiu a mode pla (no hi ha planificador a part). Els subagents van en paral·lel però s'han de demanar. | "El fil principal recull els resultats dels subagents a la seva resposta final"; els fils són inspeccionables. | https://learn.chatgpt.com/docs/developer-commands · https://learn.chatgpt.com/docs/agent-configuration/subagents |
| Aider | Cap pas d'exploració. Mapa del repo estàtic amb tree-sitter, ordenat pel graf de dependències. | S'injecta a **cada** prompt, ~1k tokens per defecte (`--map-tokens`), sense cap crida extra. Encara "demana a l'usuari que digui quins fitxers cal tocar". | https://aider.chat/docs/repomap.html · https://aider.chat/2023/10/22/repomap.html |
| Cursor | Subagent Explore, llançat "quan convé". Tria explícita **primer pla** (bloqueja) vs **segon pla** (torna de seguida). | Els subagents comencen amb context net, així que el pare passa la informació al prompt; només torna el resum final. | https://cursor.com/docs/subagents.md · https://cursor.com/help/customization/context.md |
| OpenHands | Els subagents de TaskToolSet van **síncrons**: "el pare bloqueja fins que el subagent acaba". Les microagents/skills s'injecten, no es busquen. | Les skills sense trigger van al `<REPO_CONTEXT>` del **prompt de sistema inicial** (es cobren cada torn); les regles per ruta s'injecten al **resultat de l'eina** quan es toca el fitxer. | https://docs.openhands.dev/sdk/guides/task-tool-set · https://docs.openhands.dev/sdk/guides/skill |
| SWE-agent | Sense fase prèvia. `search_dir`/`search_file`/`find_file` són eines ACI dins del bucle únic `Agent.forward()`. | `HistoryProcessor` comprimeix l'historial abans de cada prompt. | https://github.com/SWE-agent/SWE-agent/blob/main/docs/background/architecture.md |
| Gemini CLI | `codebase_investigator` és una **eina** que crida l'agent principal; no hi ha execució en paral·lel documentada. | Els resultats tornen com a resultat d'eina. Els subagents no poden cridar subagents. | https://github.com/google-gemini/gemini-cli/blob/main/docs/core/subagents.md |
| Cline | El mode pla és el **mateix** agent amb eines de només lectura; `/deep-planning` és una versió més llarga. | L'historial sencer passa al mode Act; no es reinjecta res. | https://docs.cline.bot/core-workflows/plan-and-act |
| Roo Code | El mode orquestrador delega subtasques; "la tasca pare es pausa". | El pare reprèn "només amb el resum"; també porta un índex d'embeddings a Qdrant exposat com a `codebase_search`. | https://roocodeinc.github.io/Roo-Code/features/boomerang-tasks · https://roocodeinc.github.io/Roo-Code/features/codebase-indexing |
| Devin | El coordinador llança Devins gestionats en paral·lel, cadascun a la seva VM. | Els fills escriuen al coordinador, que es "desperta sol" en acabar. | https://docs.devin.ai/work-with-devin/advanced-capabilities |

## Mecanismes que val la pena copiar

1. **Segon pla per defecte i lliurament tard (Claude Code).** Els subagents de la
   conversa principal arrenquen en segon pla; el resultat arriba com una notificació
   en un torn posterior, i a Claude encara se li pot dir que esperi. Els subagents de
   fons tenen un **joc d'eines més petit**.
   https://docs.claude.com/en/docs/claude-code/sub-agents
2. **Interruptor sync/async per crida (Cursor).** Primer pla bloqueja i retorna en
   línia; segon pla torna de seguida. El seu truc de cost: Explore "usa un model més
   ràpid per defecte… cosa que permet 10 cerques en paral·lel en el temps d'una sola
   cerca de l'agent principal". https://cursor.com/docs/subagents.md
3. **Context previ estàtic quan es pot precalcular (Aider).** AST → graf de
   dependències → ordenar → 1k tokens, injectat a cada torn. Zero latència, cap crida;
   el preu és que no decideix *què* cal llegir. https://aider.chat/docs/repomap.html
4. **Injecció determinista a mig torn (OpenHands).** Les regles per ruta cauen al
   resultat de l'eina quan es toca un fitxer: lliurament tard sense volta de model.
   https://docs.openhands.dev/sdk/guides/skill
5. **L'steering no és gratis (OpenAI).** `response.steer` és l'equivalent d'API de
   "lliurar el brief tard", però el servidor "acaba l'element de sortida actual i la
   feina d'eines allotjades que ja corre" abans de continuar, i la resposta original
   pot acabar `incomplete` (`reason: steered`). Només GPT-6/WebSocket.
   https://developers.openai.com/api/docs/guides/steering

## El que ningú no fa (espai negatiu)

- **Incondicional vs decidit pel model.** Tothom delega per judici ("quan Claude
  troba una tasca que hi encaixa", "quan convé") o per acció de l'usuari (Shift+Tab,
  `/plan`, `@investigator`). Cap harness documenta executar *sempre* un investigador
  de només lectura en torns de codi o de depuració.
- **Ningú no publica comptabilitat de latència.** Anthropic dona múltiples de tokens
  (≈4× amb agents, ≈15× multi-agent) i avisa que "la majoria de tasques de codi tenen
  menys parts realment paral·lelizables que la recerca, i els agents encara no són
  bons coordinant i delegant en temps real". Ningú no publica una quota de temps com
  el 71-87% de Gregal. https://www.anthropic.com/engineering/multi-agent-research-system
- **El lliurament tardà dins del MATEIX torn** només existeix com a notificació de
  finalització en segon pla ("un torn posterior") o com a steering de l'usuari. Un
  brief obligatori que corri contra el primer pas del coordinador no està documentat
  enlloc: el que fa Gregal amb `scout_parallel` és terreny nou.

## Sense evidència trobada

- **Claude Code:** si una notificació de segon pla pot caure dins del mateix torn amb
  `claude -p`; cap xifra de latència.
- **Codex CLI:** cap senyal que `/plan` engegui un planificador a part (les docs
  descriuen un canvi de mode al xat); cap dada de temps.
- **Cursor:** si Explore corre abans de la primera edició a *totes* les tasques;
  cap número de latència.
- **Gemini CLI:** execució de subagents en paral·lel/segon pla — absent a les docs
  llegides (pot existir, no verificat).
- **Devin:** cap pas intern de recerca obligatòria abans de planificar; els
  managed-Devins són a nivell de sessió.
- **OpenHands, SWE-agent, Cline, Roo Code:** cap comparació de cost/latència
  publicada dels seus patrons d'exploració.
- **Tots:** cap A/B públic d'explorar-abans vs explorar-dins sobre latència.

## Què en vam fer, a Gregal

- Es va implementar el lliurament tardà (`scout_parallel`, brief per la bústia del
  steering) i es va **restringir als proveïdors que serveixen crides concurrents**:
  amb el model local d'una sola ranura el paral·lel empitjora el torn (58 s → 2m18s).
- Les tasques que ja diuen el fitxer (o un identificador entre cometes) no porten
  scout: és el guany gran (torn de ~65 s a 4-8 s).
- El mapa del projecte que Gregal ja injecta sense cap crida al model és exactament
  el mecanisme 3 (Aider), i és l'argument perquè el scout sigui l'excepció i no la norma.
