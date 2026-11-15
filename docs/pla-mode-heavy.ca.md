# Pla: molts vents treballant de debò (mode «heavy», com Grok Heavy)

*Recerca del 15/11/2026. Encara no hi ha res fet.*

## Què fa Grok Heavy (i Claude Research)

- **Grok 4 Heavy**: llança diversos agents en paral·lel sobre **la mateixa
  pregunta**, cadascun treballa sol, i després «comparen la feina com un grup
  d'estudi». No és una votació: sovint només un troba el truc, i el comparteix.
  Multiplica per ~10 el càlcul en temps de resposta; és més lent i car, però
  encerta més en problemes difícils.
- **Claude Research** (Anthropic): un agent líder planifica, llança 3–5
  subagents en paral·lel amb **encàrrecs clars** (objectiu, format, eines,
  límits), cadascun amb context propi, i el líder ho uneix. Millora un 90 % en
  recerca, però gasta ~15× tokens. L'esforç s'ha d'ajustar a la pregunta
  (simple: 1 agent; comparació: 2–4; recerca complexa: més).
- Contrapunt (Cognition): agents en paral·lel que decideixen sols es
  contradiuen. Per això cal una **síntesi** que reconciliï, no enganxar textos.

## Què fa Gregal avui (i per què sembla inútil)

Hi ha sis camins amb més d'un agent (`delegate`, scouts, equip, la colla,
rotllana, l'eina `wind` i best-of), però **cap fa el que fa Grok Heavy**:

1. **Cap sintetitza.** Els resultats s'enganxen (`delegate`, `wind all`,
   scouts: `agent/delegate.go:615`, `web/winds_specialists.go:192`), es tria un
   guanyador (best-of, només per a codi) o es redueixen a 900 caràcters (la
   colla). Ningú compara ni reconcilia.
2. **Cap diversitat.** Tots els agents fan servir el mateix model, la mateixa
   temperatura i gairebé el mateix prompt: en paral·lel, sobretot es repeteixen.
3. **La rotllana és seqüencial i ancorada.** Les veus parlen una darrere
   l'altra (`web/winds_stream.go:622`); la primera proposa i les altres només
   poden afegir «un risc» o dir «-». La síntesi (`rotllanaSummary`,
   `web/winds_debat.go:26`) només surt si el missatge diu «debateu».
4. **Pressupostos petits.** Scouts de 4 passos, fills de 12, resums tallats a
   6000 caràcters: moltes respostes queden `[PARTIAL]`.
5. **Context perdut.** Els subagents i els vents delegats no veuen la
   conversa; a l'equip, el constructor no veu la recerca (van alhora).
6. **Errors empassats.** Una veu que falla se salta sense dir res; no hi ha
   reintent amb un altre model.
7. **Feina doble.** Amb `scout_parallel`, el coordinador explora mentre el
   scout també ho fa (`web/motor_web.go:48`).

## La proposta: «Tempesta» (mode heavy)

Un mode que es pot demanar («pensa-ho a fons», un botó al compositor) o que
Gregal tria sol quan la pregunta és difícil (comparacions, recerca,
decisions, problemes amb truc). Mai per a xerrades o coses simples.

### El flux

1. **Encàrrec (1 crida curta).** El model fort reformula la pregunta, decideix
   quants agents calen (2–6) i dona a cadascun un **angle** diferent:
   «busca dades oficials», «fes d'advocat del diable», «mira-ho des del cost»,
   «resol-ho pel camí directe». Cada encàrrec diu objectiu, format i límits.
2. **Treball en paral·lel.** N agents alhora, amb **la conversa resumida** i
   eines (web, fitxers en lectura). Diversitat de debò: models diferents
   (p. ex. glm-5.3-flash, deepseek-v4.1-flash, el de codi), temperatures
   diferents i angles diferents. Pressupost de 15–25 passos i un temps màxim;
   si un falla, es reintenta amb un altre model.
3. **Grup d'estudi (opcional, per a les difícils).** Cada agent veu un resum de
   les respostes dels altres i pot corregir la seva: és el «qui ha trobat el
   truc el comparteix» de Grok.
4. **Síntesi (model fort, rol `think`).** Compara: en què coincideixen, en què
   no, qui té proves. Quan hi ha conflicte en una dada, un **verificador**
   amb eines la comprova. Escriu **una sola resposta**, amb targetes (pla,
   comparació…) i fonts, i diu fins a quin punt n'està segur.
5. **A la UI.** Mentre treballen: «4 vents hi estan treballant», amb què fa
   cadascun en directe (escriptori i Android). Al final, la resposta i, plegat,
   «Com hi hem arribat»: angles, desacords i qui ho va resoldre.

### Cost i límits

- Gasta 3–10× més tokens que una resposta normal: ha de tenir un sostre per
  torn (configurable) i dir-ho abans si és car.
- Amb models locals (sense paral·lel real), N = 1–2.
- Config nova: `heavy: {agents, models, synth_role, steps, budget}` al costat
  de `agent.parallel_subagents`.

### On encaixa al codi

- **`agent.RunHeavy(ctx, cfg, client, task, history)`**, nou, a partir de
  `RunBestOf` (`agent/bestof.go:197`) però sense worktrees: fa l'abanic amb
  `buildDelegateRunner` (`agent/delegate.go:149`), reutilitza el semàfor i els
  `DelegateEvent` perquè la UI vegi cada agent, i afegeix la síntesi.
- **Entrada**: una acció `heavy` a l'eina `delegate` o un mode del torn a
  `conduirTorn` (`web/motor_web.go`).
- **Arreglar el que ja hi ha amb el mateix motor**:
  - rotllana: veus en paral·lel i síntesi **sempre**;
  - `wind all`: torna una resposta sintetitzada, no textos enganxats;
  - `delegate`: passa un resum de la conversa als fills;
  - scouts: no explorar dues vegades.

## Fases

1. **Motor + escriptori**: `RunHeavy`, síntesi, verificador, botó i panell en
   directe. Bateria d'avaluació (`gqual/eval/run.py`): mateixes preguntes,
   resposta normal contra «Tempesta», per comprovar que millora de debò.
2. **Android**: el mateix botó i el panell dels agents.
3. **Rotllana i colla** sobre el motor nou (paral·lel + síntesi obligatòria).
4. **Automàtic**: el router decideix quan cal Tempesta i amb quants agents.

## Fonts

- [Elon Musk explica Grok 4 Heavy (OfficeChai)](https://officechai.com/miscellaneous/elon-musk-explains-how-grok4-heavy-xais-multi-agent-reasoning-model-works/)
- [Grok 4 (DataCamp)](https://www.datacamp.com/blog/grok-4)
- [Com Anthropic va fer el sistema de recerca multiagent (ByteByteGo)](https://blog.bytebytego.com/p/how-anthropic-built-a-multi-agent)
- [Quan fer servir sistemes multiagent (Claude)](https://claude.com/blog/building-multi-agent-systems-when-and-how-to-use-them)
