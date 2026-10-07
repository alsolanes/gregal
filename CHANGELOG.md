# Changelog

## Unreleased

## v1.8.1 - 2026-10-07 - Chat plots and project dashboard

- Render inline line, bar and scatter charts from `gregal-plot` data blocks.
- Save plot snapshots per project and user with their source session.
- Organize dashboard cards with notes, ordering and compact or wide layouts.
- Edit titles and data, import JSON and export PNG/JSON.
- Validate chart data and prevent concurrent edits from silently overwriting changes.
- Saved plots do not refresh automatically.
- Create, rename and delete named dashboards; move plots between them without
  losing their library data. Search plots and export the collection as JSON.

## v1.7.10 - 2026-10-06 - Interrupted agent stream recovery

- Recover interrupted buffered agent streams with up to three attempts at the
  same model step; discard partial text and tool calls without replaying prior
  tools. Live chat output is not automatically replayed. Cancellation stops
  recovery immediately.

## v1.7.9 - 2026-10-05 - Clearer configuration, agent previews and safer execution

- Simplify provider setup with labeled fields and separate advanced settings.
- Make working Team agents easier to identify, improve role animations, and stop
  activity animations when a run stops.
- Let agents request isolated HTML, Markdown and localhost previews; keep the
  newest artifact visible and respect the user's dismissal of automatic previews.
- Share shell permission restrictions between foreground and background tools;
  hard-denied commands cannot be enabled by a broad allow override.
- Allow internal task-list bookkeeping without unnecessary approval prompts.
- Propagate cancellation into headless tool execution and delegated runs; clean
  up foreground shell descendants on cancellation, timeout and normal exit.
- Require HTTPS for remote desktop backends and tighten Unix password-file access.
- Add a functional checklist, coding/budget/cancellation regressions and real-model
  acceptance evidence. Benchmark observations are task-specific, not a general
  guarantee of higher speed or equivalent quality than another harness.


## v1.7.5 — 2026-10-05 — Modes simplificats i continuïtat de sessions

- El desplegable de modes avançats conserva l’obertura escollida per l’usuari
  durant els refrescos d’estat.
- Entrada simplificada amb «Fer una tasca» i «Conversar», modes especials
  desplegables i selecció de rols dins de les opcions avançades.
- Detecció de models limitada al rol actiu per evitar esperes de proveïdors aliens.
- Control explícit de l'esforç de raonament i compatibilitat amb els passos
  mecànics de GLM-5.3, que no permet desactivar el raonament.
- Taller 2D de sessions amb cua d'atenció per a aprovacions i preguntes.
- Cerca de converses pel títol, el nom, el projecte i la carpeta de treball.
- Estat visible dels torns mentre treballen, esperen una resposta o acaben.
- Resum de fitxers modificats i comprovacions observades, amb checkpoints
  recuperables després d'una reconnexió.

## v1.7.4 — 2026-10-04 — Recuperació dels límits de resposta

- Conserva les opcions de raonament i de model alternatiu en canviar de model.
- Corregeix el tractament dels torns truncats i la duplicació d'errors de l'agent.

## v1.7.3 — 2026-10-04 — Taller mediterrani i actualitzacions

- Oficina 2D mediterrània integrada a l'Equip, amb panell d'agents sense
  solapaments, contrast millorat i focus del teclat conservat.
- Actualitzacions de l'app accessibles des de Preferències, amb comprovació,
  descàrrega, progrés i instal·lació en reiniciar per decisió de l'usuari.
- Accés a les descàrregues oficials per als executables portables.


## v1.6.1 — 2026-09-30 — Windows, continuïtat i acabats visuals

- App portable de Windows x64 amb el backend local integrat.
- Model seleccionat persistent per usuari i rol; cada conversa conserva
  la seva selecció i les converses noves hereten les últimes tries.
  Si un model ja no està disponible, s'utilitza el configurat i se n'avisa.
- Preferències d'aparença i models recents independents del port local.
- Esborranys separats per pestanya i errors visibles en gestionar sessions.
- Capçalera més neta, icones coherents, compositor i blocs de codi més cuidats,
  amb controls accessibles també en finestres estretes.
- Respostes completes al transcript durable i estat de permisos coherent
  quan una actualització falla.


## v1.6.0 — 2026-09-29 — bucles llargs, un sol motor i banc contra opencode

Una tirada real va cremar 146 passos i 50 minuts en una tasca trivial:
el model tornava a córrer una suite que no podia quedar verda (un test
viu contra un model que l'endpoint ja no servia) i cada corrida comptava
com a progrés. D'aquí surt aquesta tanda.

- **Ratxa de comprovacions vermelles**: tres test/build/vet/lint en
  vermell seguits guien el model (llegeix l'error sencer, comprova amb
  `git stash` si és preexistent) i bloquegen l'ampliació (sis en
  autònom); nou seguides en interactiu tanquen el torn amb síntesi encara
  que quedin passos. Només compten les ordres que són comprovacions: un
  `grep` buit no és un vermell ni un `sed` verd un verd.
- **Bash sense `cd`**: el prompt diu que cada ordre comença al directori
  del projecte; el model posava `cd <camí> &&` a totes.
- **Mapa del projecte en començar el torn** (`agent.MapaProjecte`):
  llistat de fitxers (o resum per carpetes), estat de git i el contingut
  dels fitxers petits que la tasca anomena. Va amb el missatge de la
  tasca, no al system (no trenca el KV cache), no es repeteix si no ha
  canviat i no surt a la bombolla. Les primeres voltes del model eren
  `pwd`/`ls`/`find`/llegir `TASK.md`: t1 passa de 5 passos a 3.
- **`think: auto`**: el model raona per planificar, després de llegir i
  davant d'un vermell, i no als passos mecànics (després d'una edició
  aplicada o d'un verd), l'ampliació ni la síntesi.
- Prompt: el resum final diu què ha canviat, com s'ha verificat i què
  queda, sense tornar a escriure el codi; `write` és per a fitxers nous.
- **Topall interactiu de 3 ampliacions** i síntesi llarga (canvis, què
  passa, què queda en vermell i si és preexistent, pas següent).
- **La web i l'escriptori condueixen el motor compartit** (`agent.Torn`,
  `internal/web/motor_web.go`) en comptes d'un bucle propi de 400 línies:
  cap de les proteccions anteriors hi arribava. El motor guanya el que
  la web tenia i ell no: checklist per sessió, síntesi davant d'una
  resposta buida o del topall autònom, reintent de la síntesi per
  context i topall de recuperacions de context.
- **Web: la conversa es pinta en ordre**. Les targetes d'eina arribaven
  pel registre durable com a «activity» genèric i no es pintaven; la
  resposta final quedava a dalt, per sobre de les aprovacions; una eina
  denegada deixava l'última targeta girant per sempre. Aprovacions amb
  diff llegible, sense «Permès» doble.
- **Aprovacions web sense el tall de 120 s**: `agent.approval_timeout_s`
  (defecte 30 minuts, negatiu = sense límit) i aturar el torn talla
  l'espera.
- **`think: no` arriba al proveïdor**: el wire no copiava
  `enable_thinking` ni `chat_template_kwargs`, i només el headless posava
  el mode al context. Ara el respecten tots els clients, i també s'envia
  `reasoning_effort: none` (el que entén opencode zen).
- **Camps opcionals negociats** (`internal/llm/compat.go`): zen rebutja
  amb 400 `enable_thinking` i `chat_template_kwargs`. Si un proveïdor
  rebutja un camp opcional (`stream_options`, els de pensar), es torna a
  provar sense i se'n recorda per a aquell endpoint: un camp opcional mai
  no trenca una crida.
- **Tokens reals**: el headless suma l'`usage` de cada crida (streaming
  amb `include_usage`, amb reintent si el proveïdor no l'accepta) i el
  topall de cost autònom el fa servir. `tokens_source` diu d'on surten.
- **Diagnòstics després d'escriure** per a Python (`py_compile`, i
  `ruff`/`pyflakes` si hi són), JS (`node --check`), TypeScript (`tsc`
  si hi ha `tsconfig`) i shell, a més de Go, JSON i YAML.
- **`--config` inexistent és un error**: abans s'hi escrivia la plantilla
  per defecte i el torn anava a `localhost:5800` sense dir res.
- **Banc A/B contra opencode** (`bench/ab.py`): mateix model, mateix
  prompt, còpia neta per tasca, fitxers que no es poden tocar i tests
  ocults. Dues tasques noves: t8 (trampa amb un test extern en vermell) i
  t9 (bug entre mòduls amb test ocult).
- TUI: la carpeta de treball a la barra d'estat s'adapta a l'espai i no
  talla mai l'estat (a 80 columnes l'aprovació sortia «◌ espera la t»).
- Els tests del TUI ja no desen converses a les dades de debò.

## v1.5.4 — 2026-09-25

- **Streams truncats**: si el model talla la resposta a mig stream (MTP,
  cursa del mòbil, tall de xarxa), el client el recupera en lloc de
  quedar-se amb una resposta incompleta.
- **Desbordament final de context**: l'error d'entrada massa llarga a
  l'últim pas es recupera en lloc de tomba el torn sencer.
- Correccions menors de failover i proves (emptyreply, stream_tools).

## v1.5.3 — 2026-09-23

- **`gregal advise`**: detecta millores (dependències, toolchain, higiene del
  repositori i backend) sense necessitat de xarxa.
- **TUI i escriptori**: estètica i solidesa (sis pantalles, cockpit en columna,
  diàlegs superposats, composer de dues files), un sol tema amb markdown i codi
  pintats als dos fronts, i selector de models amb pestanyes.
- **`:root` regenerat** des d'`internal/tema` (prova `TestCSSAlDia` en verd).
- **Clients alineats** amb la cua d'execucions i els esdeveniments v2.
- **Telegram**: la prova de permisos 0600 només corre a Unix (a Windows no
  existeixen aquests permisos) i el bot fa servir el motor de torn compartit.
- **Skills**: el gregal sap què és i què pot fer.

## sense publicar — TUI al nivell d'opencode

Tres coses separaven el TUI d'opencode: dos bucles d'agent que anaven
divergint, una configuració pensada per a qui edita YAML, i una pantalla
que acumulava cromat. Aquesta tanda va per aquí.

### Un sol tema, i el codi pintat als dos fronts

La paleta vivia copiada a tres llocs (`internal/tui/theme.go`, el `:root`
d'`index.html` i `desktop/main.js`) amb tests que comparaven les còpies amb
`docs/identitat.md`. Els tests feien la seva feina, però vigilar tres còpies
no és tenir-ne una, i cap de les tres sabia res dels colors del codi.

- **`internal/tema`** és ara la font: els tokens, l'estil de glamour (el
  markdown del TUI), l'estil de chroma (el codi als dos fronts) i el bloc
  `:root` del CSS en surten. El full d'`index.html` es genera i un test
  (`TestCSSAlDia`) avisa si es queda enrere.
- **Vuit colors de sintaxi** documentats a `docs/identitat.md`, derivats de
  la paleta. El TUI feia servir l'estil per defecte de glamour, d'una altra
  marca, i en tema clar ni canviava.
- **Contrast**: el gris més apagat del tema clar puja de `#7C8B95` a
  `#5F6E78`. El de abans donava 3,2:1 sobre el panell i no passava AA.

### El markdown de les respostes

- **Al web, el renderitza el servidor** (`POST /api/md`) amb goldmark,
  chroma i bluemonday. El renderitzador propi del client (`app/md.js`, 157
  línies) es queda per al text que arriba en directe: cobria el que els
  models solen escriure, però una taula en sortia com un paràgraf massís en
  negreta i els blocs de codi mai no havien tingut color. Vuit tests, quatre
  d'ells d'injecció (`<script>`, `onerror`, `javascript:`, `position:fixed`).
- **Al TUI, el text es pinta com a markdown mentre arriba**, refet cada
  150 ms. Abans era una cua de 160 caràcters retallada per l'esquerra: amb
  paràgrafs la línia ballava i no es podia llegir res. El bloc de codi que
  queda obert a mig arribar es tanca per pintar-lo.
- **Els diffs del TUI porten sintaxi**: chroma sobre el fitxer sencer (així
  veu el context de les cadenes i els comentaris) i els `+` i `−` al marge.

### La pantalla del TUI, de dalt a baix

El pla és a `docs/plans/2026-09-21-tui-pla-de-millora.md`: cinc tandes,
cadascuna amb el seu commit i la suite verda.

- **La conversa**: el diff d'un edit es pinta dins del rail de feina, sota
  la fila de l'eina, amb sintaxi i els `+`/`−` al marge. La sortida d'una
  eina s'ensenya fins a sis files i diu quantes en queden (`Ctrl+L` les té
  totes). El resum del torn és un peu apagat amb plurals bons. Fora el rail
  de scroll: el percentatge ja és a la barra quan no s'és al final.
- **Capçalera, composer i barra**: la capçalera diu `≋ GREGAL │ projecte
  │ branca` i és quieta. El composer té dues files: l'entrada a dalt i un
  peu amb el mode, el proveïdor/model i `↵ envia · ? ajuda`; amb feina, el
  peu porta el spinner, què fa, quant fa i com aturar-ho. La barra d'estat
  passa de nou senyals i tres jocs de pistes a quatre segments: mode,
  `ctx n%`, estat i permisos. Els comptadors ↑↓ i el cost són a `/stats`.
- **`animacions: off` per defecte**: la mar, la mascota i la marea del
  mesurador només amb `on`. Un test comprova que dos `View()` seguits en
  repòs són idèntics byte a byte.
- **Diàlegs superposats**: el selector, l'aprovació (amb el diff a dins),
  la pregunta del model, la cerca a l'historial (ara amb la llista de
  coincidències) i l'ajuda es pinten centrats sobre la conversa, amb el fons
  atenuat i amplada màxima de 72. Abans s'encabien entre la conversa i el
  composer i obrir-los feia saltar tot el text. `?` amb el composer buit
  obre l'ajuda de tecles i modes; qualsevol tecla la tanca.
- **Benvinguda de sis files** sense caixa: marca amb `[45° NE]`, una frase,
  tres exemples i les tecles. Els modes s'expliquen al selector de mode i a
  `?`. Abans era una targeta de tretze files que ocupava mitja pantalla.
- **Cockpit en columna**: a 120 columnes o més, la conversa cedeix 34
  columnes i a la dreta hi va la tasca, els canvis, la cua, el context (amb
  mesurador) i la validació. `Ctrl+T` la commuta i ho desa (`cockpit:
  on|off`). Per sota de 120, la caixa de sempre. El revisor automàtic també
  hi viu: a la barra ja no hi cabia.
- **Fitxers dorats**: sis pantalles × dos temes × tres amplades (80, 120,
  160) desades tal com es pinten a `internal/tui/testdata/golden`. Un canvi
  d'estil que no es volia surt al `go test` abans que a la pantalla de
  ningú; els volguts es regeneren amb `-update` i es revisen al diff.
- **Desviació**: la barra `┃` a l'esquerra dels blocs de codi del markdown
  no és possible amb glamour v1 (pinta l'`Indent` com a espais i ignora
  l'`IndentToken` als blocs de codi). El bloc queda sense fons i amb la
  sintaxi pintada.

### Tipografia i línies

- **Escala de cinc mides** al web (`--t1`..`--t5`), amb res per sota d'11,5
  px: 120 declaracions hi passen, i les 47 que estaven a 9,5-11 px pugen.
  La barra lateral era el pitjor cas.
- **Una sola línia vertical** al TUI: el missatge teu perd la barra `▌` i es
  distingeix pel fons i per la marca. Amb el rail de feina i el de scroll
  n'hi havia tres competint.
- **Els torns se separen** amb una línia en blanc, i un pas que només crida
  eines ja no deixa un «◌ escrivint…» penjat.

### Aturar de debò

- **La cursa d'Atura**: la cua marca el torn com a «running» abans que
  l'executor n'hagi registrat l'id, i entremig hi cap una premuda. Qui
  aturava just quan el torn arrencava no aturava res: el botó deia que sí i
  el torn seguia, amb la mar de la capçalera movent-se. Ara `stopRun` busca
  el torn viu de la sessió a la cua quan encara no en sap l'id.
- El proveïdor fals dels tests s'allibera sempre en acabar: una fallada de
  mig segon es convertia en quatre minuts de timeout que amagaven quin test
  havia petat.

### Un sol motor de torn

Hi havia dues màquines d'estats fent la mateixa feina. La del headless
era un bucle `for` a `run.go`; la del TUI vivia dins d'`Update`, repartida
entre sis casos de missatge i 1.100 línies, amb tretze comptadors al
`Model` que `startAgent` havia de posar a zero un per un. Cada arranjament
s'havia de fer dues vegades i sempre se n'oblidava una.

Ara la màquina és a `internal/agent/motor.go` i no fa entrada ni sortida:
rep el que ha passat (`RepPas`, `RepExecucions`, `RepAprovacio`,
`RepPregunta`, `RepAmpliacio`, `RepCompactacio`, `RepCheckpoint`,
`RepSintesi`) i diu què cal fer després (`Seguent()` torna una ordre:
demanar un pas, compactar, executar eines, demanar permís, preguntar,
ampliar el pressupost, sintetitzar, acabar). Els clients hi posen com
criden el model, com executen les eines i com demanen permisos.

- El TUI passa de 4.226 a 3.737 línies: `Update` només alimenta el motor
  i executa l'ordre que en surt. Els tretze comptadors han desaparegut.
- **El bot de Telegram també hi passa.** Portava una tercera màquina
  d'estats pròpia a `turn.go`: sense compactació (una conversa llarga
  acabava desbordant la finestra), sense síntesi quan s'esgotaven els
  passos, amb un topall fix de 180 s per pas que tallava els models lents,
  i `/stop` no aturava l'eina en marxa (només la crida següent). Ara
  alimenta el mateix motor, executa les eines amb el context del torn i
  publica els avisos del motor (reintents, compactació, checklist) a
  l'activitat del xat. Els textos per defecte dels events viuen a
  `agent.Event.Missatge()`, amb un test que vigila que cap clau nova es
  quedi sense text.
- El headless condueix el mateix motor amb un bucle de trenta línies.
- La màquina d'estats té quinze tests propis que corren en un segon,
  sense terminal, sense xarxa i sense fitxers. Abans no era provable:
  només es podia tocar a través de la interfície.
- El motor parla amb claus de traducció, no amb text: no depèn de la i18n
  de cap client.
- Les diferències que arrossegaven les dues còpies queden resoltes per
  construcció, no per haver-les tornat a sincronitzar a mà.

### Fiabilitat del bucle

- **Un sol registre d'eines** (`tools.Noms()`). N'hi havia tres i no
  coincidien: 24 noms a l'agent, 14 a la validació del config i 13 al
  menú de permisos. `permissions.tools.office_edit: deny` feia fallar la
  càrrega d'un config que l'agent executava sense problema, i el menú no
  deixava tocar ni office ni el navegador.
- **Ctrl+C durant una eina ja no tanca el programa.** El camí que executa
  eines no registrava cap cancel·lació, o sigui que Ctrl+C enmig d'un
  `go test` de dos minuts queia al «surt» i s'enduia la sessió. Ara el
  primer atura (i mata de debò la comanda: `tools.BashCtx` i el navegador
  reben el context del torn) i el segon surt.
- **Esc atura també el que ja s'ha aprovat**: `pendingOp.run` rep context.
- **El mode autònom del TUI respecta els tres topalls.** Només mirava el
  cost; `max_tool_steps` i `max_minutes` no existien fora del headless.
  Ara tots dos bucles criden `agent.TopallAutonom`.
- **La decisió d'eines és la mateixa a tot arreu** (`agent.PlanCalls`).
  El TUI en tenia una còpia amb l'ordre de comprovacions canviat.
- **Els reintents d'error de servidor són dos**, com al headless (n'era 1).
- **L'aprovació ja no es denega sola als dos minuts.**
- **Quan el proveïdor no envia ni un byte, el bloc de feina ho diu**: «el
  proveïdor no ha respost en 23 s · Esc atura», a partir dels 8 s. Abans
  deia «escrivint…» durant minuts amb el motor d'inferència caigut darrere
  d'un gateway (504/524 o cap resposta) i semblava que el TUI s'havia penjat.
  Test amb un proveïdor que no respon mai: Esc talla la petició en menys d'un
  segon i la capçalera queda quieta.
- Les files de les eines segueixen sent les seves després del retall de
  línies velles (l'índex del timeline es desplaça amb la resta).
- Al web, el test de cancel·lació comprova també que `agent_busy` cau abans
  de 2 s: és el que mou la mar de la capçalera (`body.working`). El compte enrere
  venia de la web; en un terminal, aixecar-se de la cadira volia dir
  tornar amb l'edició rebutjada i l'agent continuant amb «EINA REBUTJADA».

### Aprovació

- **El diff es veu ABANS de decidir.** Fins ara el que es llegia era el
  JSON dels arguments retallat a 300 caràcters i el diff arribava després
  d'aprovar: un `write` d'un fitxer sencer s'aprovava a cegues.
- **Tres botons**: Permet · Sempre en aquesta sessió · Denega. «Sempre»
  existia (la tecla `r`) però només com a lletra dins d'una pista, i
  semblava que l'agent demanava permís per la mateixa cosa a cada pas.
- La descripció és llegible (`edit internal/config/config.go`) i la barra
  d'estat ja no en repeteix una còpia tallada a mig JSON.

### Proveïdors, claus i menús

- **Catàleg de proveïdors** (`config.Cataleg()`): Ollama, LM Studio,
  llama.cpp, OpenRouter, OpenAI, Groq, DeepSeek, Mistral, Together,
  Cerebras i OpenCode Zen, amb la URL bona i la variable d'entorn
  habitual. Es dona d'alta des de providers → «+ afegeix un proveïdor»;
  si la variable ja existeix, la clau s'agafa sola.
- **Les claus es desen** a `~/.config/gregal/auth.yaml` (0600), fora del
  config. Abans una clau literal només vivia en memòria i, sense cap
  `${VAR}` definida, cada arrencada te la tornava a demanar.
- **La clau no s'escriu mai a la pantalla**: les tecles no passen pel
  composer, es pinten com a rodonetes i no toquen l'historial.
- **Els menús filtren escrivint.** Amb un proveïdor que llista seixanta
  models, triar-ne un era baixar cinquanta cops amb la fletxa.
- **Triar model desa sempre.** El menú ho feia només per a la sessió i
  `/provider rol` ho desava: dos camins que feien coses diferents sense
  dir-ho.

### Pantalla

- **Una fila per eina.** La crida i el resultat compartien dues files
  (`▸ bash …` i després `✓ bash`); ara el resultat reescriu la fila de la
  crida: `✓ read internal/config/config.go · 9 línies`. Les escriptures
  tampoc repeteixen «edit aplicat a x.go» just sobre el diff.
- **La benvinguda marxa amb el primer missatge.** Ocupava tretze de les
  trenta-quatre files d'una pantalla normal i no se n'anava mai.
- La mar de la capçalera ja no surt enganxada a la branca.
- La barra lateral de scroll només es pinta quan hi ha res a desplaçar.
- Fora emojis de la conversa i de la barra: la identitat va de glifs
  monoespaiats, i els cadenats feien una cel·la o dues segons el terminal.

### `gregal advise`: el gregal proposa millores

- Sis detectors de només lectura (toolchain Go, lockfile npm, backend
  empaquetat desfasat, higiene del repo, mòduls Go i paquets npm
  actualitzables) que tornen propostes amb severitat, esforç i caducitat.
- Robusta per disseny: cada detector té timeout i recuperació de pànic;
  sense xarxa, sense git o sense npm, ho diu (`no-disponible`) i continua.
  `--offline` omet la xarxa del tot; el codi de sortida sempre és 0.
- Les propostes es desen a `propostes.json` (atòmic, 0600, sense duplicats,
  caduquen a 30 dies); un fitxer corrupte es conserva com a `.corrupte-*`.


## v1.5.2 — 2026-09-20 — mode autònom, cua d'execucions i servei compartit

- **Mode `autonomous`**: tasques llargues per fites. `internal/agent/autonomous.go`
  i `autonomousRules` (les regles de `code` més un contracte de llarg abast).
  Cada `checkpoint_every` eines (10 per defecte) fa un checkpoint i cada
  `review_every` (2) demana revisió al rol `reviewer`. Límits de seguretat:
  `max_minutes` (120), `max_tool_steps` (500), `max_cost_usd` i les comandes de
  `verify` que corren a cada checkpoint. Vàlid a `--mode`, a `/mode` i a la webapp.
- **Cua d'execucions compartida**: un torn per sessió, serialització per
  workspace, prioritat i cancel·lació per id; API v2 de runs amb idempotència i
  esdeveniments de cicle de vida (`/api/v2/events`). El TUI i l'escriptori
  deleguen els torns a la cua quan hi ha servei connectat.
- **Fixats (pins) persistents** al model i al llistat de converses.
- **Servei compartit**: descoberta local amb descriptor, lock recuperable i
  identitat del servei a `/api/health`.
- **Workspace explícit** a les execucions no interactives i als fluxos;
  verificació amb el shell del sistema.

## v1.5.1 — 2026-09-20 — el mode xat pot mirar el sistema, i el rol nou surt al teclat

- **La data del sistema entra al prompt** de tots els modes («DATA D'ARA:
  2026-09-20 (diumenge)»), només el dia i no el rellotge: el motor local de
  Halogen desa el punt de represa del seu KV cache al final del system prompt,
  i un valor que canvia cada minut el tornava a invalidar a cada torn (2 s
  contra ~88 s en un torn de seguida, xifres del mateix motor). L'hora exacta
  la dona `date`.
- **`date` i companyia són lectura segura**: `date`, `uname`, `hostname`,
  `whoami`, `id`, `uptime`, `free`, `nproc`, `lsblk` i `locale`. Abans `date`
  no hi era i el mode xat responia «EINA BLOQUEJADA: fora de la llista
  segura» a qualsevol pregunta sobre el dia, i el model acabava responent de
  memòria. Els encadenaments, les redireccions i `sudo` continuen denegats.
- **El teclat 🧠 Rol del bot surt del config**: un rol nou (com `halogen`) hi
  apareix sol, darrere dels tres de sempre, i el reviewer no s'hi posa mai.
  Abans la llista era fixa al codi (chat/think/code) i un rol només es podia
  triar escrivint `/role`.

## v1.5.0 — 2026-09-18 — navegador, cerca web pròpia, pla viu i context per model

- Banc d'evals des del binari: `gregal --config evals/example.yaml --eval
  evals [--tasks t1,t6] [--eval-baseline]` corre les tasques en còpies,
  pinta veredicte, passos, tokens i segons, escriu `results/summary.json`
  i compara amb `evals/baseline.json` (codi 1 si hi ha regressions). El
  run.sh i el run.sh duplicaven la lògica i no donaven cap xifra.
- Finestra fora de línia: quan el backend no respon, una franja vermella
  ho diu a dalt amb «Torna-ho a provar», els sondejos s'espaien i en
  tornar la connexió tot es refresca sol. Abans: «Failed to fetch» dins
  d'una bombolla i sondejos cada segon en silenci.
- Finestra: capa de lectura. Text de conversa a 15 px amb interlineat
  1,7 (abans 14 px), columna de lectura de 760 px compartida per
  missatges i composer, missatges de l'agent sense icona ni reixa, els
  teus com a bombolla suau, composer amb més aire i ombra, etiquetes en la
  mateixa lletra que el text (fora monoespai en majúscules), «Segoe UI
  Variable» a Windows, i l'inspector plegat per defecte (botó ▥ a la
  capçalera, es recorda). El que separava la finestra de Claude i ChatGPT
  era la densitat, no el color.
- Reintents del client LLM per a proveïdors que arrenquen sota demanda:
  sis intents amb espera exponencial (1 s → 20 s, uns cinquanta segons en
  total; abans tres intents en mig segon), `Retry-After` respectat als
  429, i l'avís «proveïdor HTTP 502 · reintent 2/6 d'aquí a 4 s» visible
  al bloc de feina del TUI, a la barra del torn de la finestra i com a pas
  al headless. Vist en viu amb `localhost`: la primera pregunta
  passava i les següents morien al tercer 502 sense dir res.
- Selector de projecte a la finestra: en comptes d'un `prompt()` amb una
  llista numerada, un quadre amb els recents, un navegador de carpetes
  del servidor (`/api/dirs`: engrunes, unitats, repositoris git primer) i
  ruta a mà; a l'escriptori també el diàleg del sistema. S'obre des del
  nom del projecte a la capçalera i des de «Projecte…» a Fitxers. Les
  converses de la barra lateral duen el nom del projecte quan no és el
  d'ara: obrir-ne una canvia de workspace i ara se sap cap a on.
- Diagnòstic després d'editar: a més de la sintaxi, `go vet`,
  `py_compile`, `node --check` o `bash -n` sobre el fitxer tocat
  (`hooks.diag: full`, defecte; `syntax` per tornar a només sintaxi).
- Conversa més neta: les crides d'eina diuen «read internal/x.go» o
  «bash go test ./…» en comptes del JSON; les lectures que van bé es
  pleguen a una línia («✓ read 42 línies»; el cos sencer és a Ctrl+L);
  el codi en línia de les respostes ja no porta espais de farciment.
- Pressupost de passos tou: el `max_steps` és el tros base, i mentre el
  model avanci (eines noves executades des de l'últim tros) i digui
  CONTINUA se li concedeixen trossos de 20 passos tantes vegades com
  calgui (topall de seguretat de 25). Abans eren dos trossos de deu i les
  tasques llargues quedaven a mitges. Les regles de mode code li diuen que
  acabi el checklist en comptes d'estalviar passos.
- Ratolí: amb `mouse: off` el programa ja arrenca sense capturar-lo (a
  Windows, activar-lo i desactivar-lo després no sempre alliberava la
  selecció del terminal); pista de Shift+arrossegar per seleccionar amb la
  roda activa.
- Cerca web sense cap servei local: `web_search` consulta DuckDuckGo i
  Bing alhora (i SearXNG si hi ha `SEARXNG_URL`), fusiona i desduplica
  els resultats i els guarda deu minuts en memòria. Abans depenia d'un
  SearXNG a 127.0.0.1:4000 i sense ell la cerca no existia.
- Lector web per a agents (`internal/tools/webread.go`): `web_fetch`
  torna el contingut principal de la pàgina en markdown compacte (títol,
  data, autor, capçaleres, llistes, codi, taules i enllaços; sense menús,
  peus, banners, formularis ni imatges). Opcions `find` (només els
  paràgrafs que contenen un text), `max_chars` i `raw`. Les vies de
  recuperació (render local i Wayback) corren en paral·lel i el render
  només s'intenta si hi ha un Firecrawl escoltant.
- Eines en paral·lel dins d'un pas: les lectures (read, grep, glob,
  web_*, gh_*, office_read…) que el model demana juntes s'executen alhora
  (fins a 6) al TUI, al headless, al web/escriptori, al Loop i als
  subagents; les escriptures i el bash continuen en ordre i l'historial
  no canvia.
- Pla viu: en executar un `/plan` aprovat, els passos numerats passen al
  checklist abans del primer pas del model; si el model marca un pas fet
  sense dir quin és el següent, el primer pendent passa sol a «en marxa»;
  i passades cinc eines sense `todowrite`, l'últim resultat del pas porta
  un recordatori perquè actualitzi el checklist.
- Eina `browser`: navegador real (Chrome/Edge de la màquina via Chrome
  DevTools Protocol, sense extensions ni serveis) amb perfil propi del
  gregal i finestra visible. Obre, llegeix la pàgina en markdown amb
  elements interactius numerats, clica, escriu, prem tecles, captura
  pantalla i executa JavaScript. Mirar passa sol; actuar demana permís;
  en mode consulta i xat només mira. Prova viva: `GREGAL_LIVE_BROWSER=1`.
- Complement d'Office redissenyat amb el llenguatge de l'Office (Segoe UI,
  reixa de 4 px, tema clar/fosc del programa): accions ràpides sobre la
  selecció (reescriu, corregeix, resumeix, tradueix…), «Substitueix la
  selecció» i «Insereix», resultat amb format de debò al Word
  (`insertHtml`) i taules a l'Excel com a matriu de cel·les.
- Complement d'Office: mode consulta («Només consulta: no toca el
  document») per a documents compartits: l'agent respon al panell, sense
  inserir ni substituir, amb accions de lectura (resumeix, explica,
  revisa, punts clau, preguntes, verifica) i el torn en mode xat al
  servidor (`/api/agent` accepta `mode: chat|inspect` per restringir un
  torn).
- `office_edit`: `append` (paràgrafs amb format al final d'un docx o de
  l'última diapositiva d'un pptx) i `set_range` (bloc de cel·les xlsx
  d'una vegada).
- `--doctor` comprova quins motors de cerca web responen, si hi ha navegador per a `browser`, i quina finestra
  de context s'aplica a cada rol (config, model o defecte).
- Finestra de context per model: `context_window: 0` vol dir «la que
  declari el proveïdor» (`/v1/models`: `context_length`, `max_model_len`,
  `n_ctx`…), detectada a l'arrencada i apresa dels errors de context
  excedit. Abans un 0 volia dir 32K (8K en local) fos quin fos el model.
- Compactació per etapes a mig torn (`internal/agent/histcompact.go`),
  al TUI, al headless, al web/escriptori i als subagents: retall de les
  sortides d'eina velles, després resum dels passos anteriors amb el
  model (com opencode), i retall segur només si el resum falla. Abans es
  tallava cegament als quatre últims missatges.
- Detector de bucles per fitxer (`DoomSig`): quatre `write` de fitxers
  diferents a la mateixa carpeta ja no es bloquegen com a «eina repetida».
- `PlanCalls` unifica la decisió de permís i repetició de les crides d'un
  pas per al headless, el web i els subagents.

- `/verify on|off|toggle` al TUI (drecera de `verify mode auto/off`, es desa),
  amb indicador persistent `✓ verif` a la barra quan el revisor passa sol.
- Reintent automàtic acotat d'errors de servidor (5xx/429/xarxa) a mig torn
  al TUI, al headless (`-p`) i al backend web/escriptori: guia el model
  perquè regeneri el pas en comptes de matar la feina feta.
- Regles de mode code per a models petits: noms de fitxer exactes (sense
  traduir ni inventar) i comprovar dades del repo amb eines abans de respondre.
- Client LLM tolerant amb models locals: camps `thought`/`thinking`,
  extracció de `<think>`, i `question` que accepta `question:` i opcions com
  a strings o mapes (`internal/tools/question.go`).
- Evals reproduïbles contra `localhost`: `evals/example.yaml` +
  `.\evals\run.sh`; prova viva del mode pla (`TestStrixPlanLive`).
- Fletxa ↓/↑ de l'historial: navegació d'un en un sense salts, no es
  duplica en drenar la cua, no s'encalla en entrades multilínia ni la
  segresta el popup de `/ordres`; bloc de feina en viu d'una sola línia.
- Cadenat de permisos a la barra: 🔒 tancat (cal permetre) per defecte,
  🔓 obert en `/permissiu` (l'agent tira sol).
- Crida d'eina escrita al text: en comptes de tancar el torn, el TUI,
  l'headless i el web guien el model perquè la refaci estructurada
  (acotat; el text no s'executa mai).
- Vista del TUI d'alçada constant en escriure: el viewport cedeix les
  files visuals reals dels popups i del composer (inclòs l'embolcall);
  ja no fa scroll ni amaga blocs a cada tecla.
- Mode goal amb eines de només lectura: com que el prompt li promet
  read/grep/glob, ja passa per l'agent (la política denega escriure);
  abans anava sense `tools` i la crida arribava en text i matava el torn.
- Preguntes seleccionables a tots els modes: el prompt de xat i de goal
  ja diu que les decisions van amb l'eina `question` (mai llistes en
  text pla); el TUI obre el seleccionable igual a code, xat i goal.
- Menú d'accions en desar l'objectiu (Executa/Edita/Llista/Esborra amb
  fletxes); Executa del menú /goal ja no penja el torn (el Cmd anava
  a parar enlloc: ara va per deferred amb l'estat del pas).
- `/mouse` es desa (`mouse: on|off` al config): la tria entre roda i
  selecció nativa sobreviu reinicis.
- Sessions que es reprenen de debò: `/resume N` (o clic a la fila) amb
  llista numerada, restaura rol, conversa i resum, treu duplicats,
  avisa de rol perdut o altre directori; en obrir diu quantes n'hi ha.
- El seleccionable es navega: ↑↓ mouen el ressaltat (amb volta),
  Enter el confirma; escriure text continua sent resposta lliure.
- El seleccionable té fila de text lliure (✎, sense número): mostra en
  directe el que escrius al composer, retallada perquè no mogui el layout.
- Barra lateral de scroll a la conversa (rail + polze de posició),
  sempre visible i sense moure res.
- Escriure no mou la conversa: el KeyMap de pager del viewport (u, d,
  k, j, b, f, espai…) buidat; l'scroll va per dreceres explícites i la
  roda. Ctrl+U/D només desplacen amb el composer buit (amb text són
  edició) i el Tab només ensenya en buit.
- La capçalera porta la mar en feina: el buit viatja (swell) mentre
  l'agent treballa; en repòs, mar plana.
- El torn no es tanca amb el checklist a mitges: si el model anuncia i
  s'atura amb todos pendents, es guia i continua (acotat, als 3 fronts);
  doble Enter encuat una sola vegada.
- Checklist que no avançava: els models petits diuen `completed` o
  `in_progress` en comptes de `done`/`working` i tot queia a `pending`;
  ara s'accepten els àlies (vist en viu amb el Qwen) i la descripció de
  l'eina fixa els estats exactes.
- Office sense markdown imprès: `office_create` converteix `#`, `-`,
  `**`, `*`, taules i enllaços a format Word/PowerPoint de debò (el
  codi entre ``` queda literal); `office_edit`/`set_cell` netegen a
  text pla.
- Pressupost de passos decidit pel model: en esgotar `max_steps` se li
  demana `CONTINUA n` o `FINAL`; fins a 2 extensions de 10 passos, només
  si ha executat eines de noves (lectures i tests també compten), als 3
  fronts. El `%` de context és l'ompliment real de l'última petició.
- Rutes Windows a Git Bash (`C:\...` → `/c/...` automàtic a l'eina
  `bash` i processos, mai als hooks): els `ls` amb rutes absolutes ja
  funcionen; el web ancora el model al Directori de treball (res de
  Downloads sense que ho demanin).
- Doom difús: a més de 3 idèntiques, 4 seguides amb la mateixa plantilla
  (repro8.mjs → repro9.mjs…) també aturen amb guia.

Millores prioritàries d'estabilitat, recuperació d'errors i consistència:

### Sessions segures de reprendre
- Persistència completa de l'estat de sessió (`Workspace` i resum `Compacted`) tant al backend web com al TUI. En reobrir una sessió es restaura el directori de treball i el resum compactat sense perdre context.
- Escriptura atòmica via fitxer temporal (`.tmp` + rename) evitant corrupció per fallades durant el desat.
- Timestamps a nivell de milisegon a `NewName()` per evitar sobreescriptures de fitxers en desats automàtics simultanis.

### Inicialització i recuperació de l'aplicació d'escriptori
- Capçalera d'autorització `Authorization: Bearer <TOKEN>` a la comprovació de salut `ping()` de l'escriptori quan hi ha token configurat.
- Canal IPC `gregal:retry-backend` que reinicia el servidor backend quan l'usuari prem «Reintenta» a la pantalla d'error.
- Disseny de `error.html` adaptat a la paleta oficial de `docs/identitat.md` (`#7FD4C1`, `#0F172A`, `#111C2F`), amb suport bilingüe català/anglès.
- Menús natius i diàlegs d'Electron bilingües segons l'idioma configurat.

### Consistència d'interacció i traduccions
- Traducció completa de la llista de converses (`convs.js`): agrupació temporal (avui, ahir, últims 7 dies, aquest mes, anteriors), confirmació d'esborrat, estats de càrrega i d'error en català i anglès (15 noves claus amb paritat estricta).
- Correcció d'execució de hooks `post_edit` en entorns Windows (`internal/agent/hook.go`) via resolució dinàmica de shell (`sh.exe`/`cmd.exe`).
- Eliminació de falsos positius a `doctor.go` amb permisos Unix en Windows.

## 1.4.3 — 2026-09-16 — recuperar el que has escrit, i un TUI que es llegeix de dia

Aquesta tanda surt de comparar Gregal amb el que es dona per fet en una
eina d'aquestes el 2026 i quedar-se amb el que faltava de debò.

### Edit i patch fallaven sempre en fitxers de Windows

Un fitxer sortit d'un git amb `core.autocrlf` —aquest repositori mateix—
té `\r\n`, i el model escriu el bloc amb `\n`, que és el que fa tothom. La
comparació era exacta: no casava **mai**. El que rebia el model era «bloc
no trobat», sense cap pista, i el que feia llavors era tornar-hi igual o
reescriure el fitxer sencer. A `patch` feia més mal encara, perquè aplica
diverses edicions seguides i n'hi ha prou que en falli una perquè no se'n
faci cap.

Ara la comparació ignora els finals de línia i el fitxer es desa amb els
que tenia. I quan de debò no hi és, l'error distingeix els tres casos, que
es resolen de maneres diferents: hi és amb un altre sagnat, hi ha la línia
però no el context, o no hi ha res semblant.

### Quan el model escriu la crida d'eina al text

Alguns models, quan la crida estructurada no els surt, l'escriuen al cos
del missatge (`<tool_call><function=write>…`). Per a l'agent el torn s'ha
acabat, i això anava tal qual a la pantalla: markup en comptes de la feina
feta, i sense cap indici que el model ho havia intentat i no havia pogut.
Ara es treu, es manté la frase que l'acompanya i es diu què ha passat. No
s'executa: ve del model i el nostre client no l'ha validada.

### Recuperar el que ja has escrit

- **Ctrl+R al TUI**: cerca a l'historial per trossos, com al bash. Amb ↑/↓
  el recorries d'una en una, i per trobar aquella ordre llarga de fa dos
  dies havies de picar la fletxa vint vegades. Enter la posa al camp i no
  l'envia.
- **↑/↓ al composer de la finestra**: no hi havia historial **gens**. Per
  repetir un missatge o corregir-ne un de llarg l'havies de tornar a
  escriure sencer.

### Un TUI que es llegeix de dia

En un terminal de fons clar, el text apagat de la paleta fosca és `#475569`
sobre blanc: la meitat de la pantalla no es llegeix. Ara hi ha `theme:` al
config i `/theme` per canviar-lo en calent. Els colors surten de la taula
«Paleta clara» de `docs/identitat.md`, la mateixa que fa servir la
finestra; no n'hi ha cap d'inventat.

## 1.4.2 — 2026-09-16 — que sàpiga quina màquina trepitja

### Set passos de dotze buscant el compilador

Mirant un torn real amb el model de debò, sobre un projecte Go a Windows:
de dotze passos, **set** se'n van anar buscant on era el compilador. `go
version` fallava —el toolchain no és al PATH d'aquesta màquina— i a partir
d'aquí va provar `which go.exe`, `ls /c/Go`, `find /c -maxdepth 3 -name
go.exe`, `echo $PATH`… fins a trobar-lo al pas dotze, amb el pressupost ja
esgotat. La feina va sortir bé, però per sort.

No és res que pugui deduir: o li ho diem, o ho busca. Ara el system prompt
porta el sistema, amb què s'executen les ordres (a Windows és Git Bash, no
cmd) i les eines trobades. El cas que importa no és «hi és» sinó «hi és
PERÒ no és al PATH», amb el directori a punt per enganxar.

Mateixa tasca, mateix model: **7 passos en comptes de 12**, sense esgotar
el pressupost, i els tests que escriu passen.

### El bash tenia trenta segons i compilar en fred en costa trenta-sis

Mesurat en aquest mateix repositori amb la cau de Go buida, `go build
./...` triga 36s. Amb el topall a 30s, la primera compilació d'una sessió
es tallava **sempre**, i el que arribava al model era un «timeout (30s)»
sec del qual no es pot saber si el codi està trencat o si només ha faltat
temps. Ara són dos minuts, i quan salta, el missatge diu per on sortir-se'n.

### La barra del TUI canviava informació viva per ajuda fixa

Pintant la pantalla de debò a 110 columnes —una amplada de terminal de les
normals— el mesurador de context no hi era: les pistes de teclat li havien
pres el lloc. Ara les pistes van les últimes, i cedeixen per graons
—llargues, curtes, cap— com ja feia l'esquerra.

De passada, la caixa de pregunta i la de checklist es quedaven a 78
columnes mentre el menú, la confirmació i el composer feien tota l'amplada.

### L'anglès arribava a la barra lateral i s'aturava abans de la pantalla

Obrint la finestra amb `lang: en` i mirant-la: el menú en anglès i tot el
que hi ha al mig en català. La benvinguda, les quatre targetes d'inici, els
xips Codi/Xat/Objectiu, els buits de GitHub, Fitxers, Office, Objectius i
Grafs, i les dreceres. És literalment la primera pantalla. Comptat amb un
recorregut del DOM: **29 rètols abans, 0 després**.

El TUI tenia el mateix forat i s'ha tancat igual: 70 literals que no
passaven per T().

Tres coses que hi han sortit pel camí:

- Traduir en carregar el mòdul no funciona: la taula de modes cridava la
  traducció abans que existís i petava la pàgina sencera. És el mateix
  error que ja havia passat amb la paleta d'ordres del TUI.
- `refresh()` posava l'idioma al final, després de pintar.
- Tres claus dels grafs eren al diccionari en totes dues llengües i no les
  feia servir ningú. Hi ha una prova nova de claus mortes.

## 1.4.1 — 2026-09-16 — la finestra ja no oblida en silenci

### La conversa deixava de cabre-hi i ningú ho deia

El TUI compactava des del principi: en acabar el torn, si la conversa
passava del 75% de la finestra, la resumia i es quedava els darrers
missatges. La finestra —que és on es passen les hores— no ho feia mai.
Una sessió llarga hi creixia sense límit fins que el model retallava el
context pel seu compte, sense dir-ho, o petava.

Ara compacta **abans** de cada torn, així el que ve ja hi cap en comptes
d'assabentar-nos que no hi cabia quan ja és tard. I es diu: surt un
missatge al fil i una entrada a l'activitat. Perdre detall en silenci és
el que fa que després no s'entengui per què l'agent ha oblidat una cosa
dita fa estona. Comprovat contra l'endpoint real amb una finestra de 2000
tokens: torns 1-3 sense tocar res, i al quart la compactació.

### Amb «lang: en» al config, la finestra sortia en català

El TUI respectava el config i la finestra no: només mirava el navegador
i, si no hi havia res desat, es quedava en català per sempre. Ara mana la
tria del navegador quan n'hi ha, i el config quan no.

### L'anglès estava fet a mitges

Els rètols fixos es traduïen, però tot el que l'app diu **en marxa** —la
cua, el torn aturat, els passos esgotats, el permís caducat, les entrades
d'activitat— eren frases en català escrites al codi. Amb la interfície en
anglès, cada missatge que et deia alguna cosa et sortia en una altra
llengua.

51 claus noves i una prova nova: es comprovaven els `data-i18n` de la
pàgina, però ningú mirava els `gregalT('clau')` del codi. Fent-ho s'hi van
escapar catorze claus que no arribaven mai al diccionari i pintaven
literalment `ui.passos` a la pantalla; com que els dos diccionaris hi eren
igual de buits, la prova de paritat passava tan tranquil·la.

### Rebobinar una pestanya retallava la conversa de l'altra

Els fitxers ja anaven filtrats per workspace, però les marques de conversa
—on retallar l'historial en rebobinar— compartien sac, indexades pel seq
global. Amb dues pestanyes obertes, un rewind a la sessió A podia trobar
la marca de la B i retallar-li la conversa a una llargada que no era seva:
missatges perduts sense cap avís.

### Detalls

- `--addr 0.0.0.0:9000` ja no respon «too many colons in address»: si
  l'adreça porta port, mana ella.
- L'avís d'«exposat a la LAN sense token» ja no salta amb `::1` ni
  `127.0.0.2`, que són loopback. Un avís que salta quan no toca s'ignora.

## 1.4.0 — 2026-09-16 — que les eines treballin on toca, i tot en dos idiomes

### Les eines treballaven al directori equivocat

Canviar el projecte d'una pestanya canviava els panells de fitxers però
**no on treballaven les eines**: `bash` s'executava al directori on
s'havia engegat el servidor, i `read`/`write`/`edit`/`grep`/`glob` amb
rutes relatives hi resolien també. A l'escriptori, una pestanya que havia
canviat de projecte llegia i escrivia a l'altre.

La resolució es fa en un sol lloc, sobre el JSON dels arguments, i no als
dotze casos: n'hi hauria hagut prou que un s'oblidés. Amb el TUI i el
headless —una sola sessió— no canvia res.

### Tres coses més que es creuaven entre pestanyes

- **Els processos de segon pla** duien tots l'etiqueta `"agent"`: no
  sortien al Terminal de la pestanya que els havia engegat i tancar-la no
  els matava. El comentari deia que el web hi posava l'id de sessió i no
  era veritat.
- **La checklist** era global: la sessió que començava un torn esborrava
  la de la que estava treballant, i un `todoread` podia tornar la llista
  de l'altra conversa.
- **Canviar de pestanya** feia cinc peticions en sèrie, dues d'elles
  duplicades i dues cares (`git diff` i l'arbre de fitxers) que es feien
  encara que estiguessis mirant la conversa. Ara en fa una d'esperada:
  mesurat, 239 ms.

### L'agent avisa si una edició deixa el fitxer trencat

Escrivia un JSON amb una coma de més i l'eina responia «escrit, 412
bytes» i tan amples. Ara `write`, `edit` i `patch` comproven Go, JSON i
YAML i ho diuen al mateix resultat. Només sintaxi i en procés: 0,9 ms en
un `.go` de 400 funcions.

### El TUI ja no es torna espès

Mesurat amb 20.000 línies: el `strings.Join` de la conversa costa 0,5 ms
i el `vp.SetContent` del mateix text, 6,3 ms. El temps se n'anava tot al
viewport i creixia amb la sessió. Ara la pantalla s'acota a 6.000 línies
i una sessió de 40.000 es queda a 2,2 ms. No es perd res: la conversa
sencera es desa a disc.

### Català i anglès a tot arreu

- **La finestra**: les vuit pàgines, no només la navegació.
- **El TUI sencer**: pancarta, insígnies, composer, barra d'estat,
  `/help`, el desplegable de `/`, els menús, els proveïdors, els
  objectius i tots els missatges d'estat i d'ús. 267 claus.

Tres tests ho sostenen: els dos diccionaris han de tenir les mateixes
claus, cap clau pot quedar sense fer servir, i les descripcions d'ordres
han de seguir l'idioma actiu. El tercer va néixer d'un error real: com a
variable de paquet, les traduccions s'avaluaven abans que es fixés
l'idioma i el desplegable es quedava en català per sempre.

### Colors fantasma al TUI

Quatre colors escrits a mà fora de la paleta, entre ells dos `#526DFF`
—el mateix blau genèric que ja vam treure de la web. El test d'identitat
existia però només mirava `theme.go`, i vivien als altres fitxers; ara
mira tot el paquet.


## 1.3.2 — 2026-09-15 — canviar de pestanya deixa d'esperar

Amb dues sessions treballant, canviar de pestanya es quedava penjat
segons. Feia cinc peticions en sèrie —sessions vives, `/api/state`,
`/api/state` un altre cop (per al transcript, que ja venia a la primera),
el `git diff` i l'arbre de fitxers— i les dues últimes, que són les
cares, es feien encara que estiguessis mirant la conversa i no aquells
panells. Amb l'agent escrivint al disc, encara més lentes.

Ara en fa **una** d'esperada i la resta van després sense bloquejar; els
panells cars només es carreguen si els estàs veient. Mesurat: 4
peticions, 239 ms, i ni diff ni arbre.

La barra de pestanyes també: el sondeig de cada 5s demanava la llista i
`render()` la tornava a demanar, i repintava encara que no hagués canviat
res.

### Sobre les preguntes amb opcions: és el model

Mesurat amb el mateix banc, mateixa tasca ambigua, 8 mostres:

| model | crida `question` |
|---|---|
| Qwen38-Flash-FAST | **8/8** |
| Ornith-1.5-35B | 6/8 |
| **Nex-2.5-mini** | **0/8** |

El `roles.code` per defecte apuntava a `Nex-2.5-mini`, que és el que mai
no la crida. No hi ha res a arreglar al codi: cal triar un model que
faci tool-calling de debò per al rol que fa la feina.


## 1.3.1 — 2026-09-15 — el tema, els models i «qui respon»

### «rol: chat» desapareix: ara és «Qui respon», dins la píndola del model

Era un desplegable a la barra lateral que deia el **nom intern d'una
entrada del config**, i no hi havia manera de saber què feia. A més era
un segon control per a la mateixa pregunta que ja responia la píndola de
dalt: quin model contesta.

Ara és allà, amb **Automàtic** (i quin ha triat ara mateix) i cada rol
amb el model que duu a sota, dit en pla: *per parlar*, *per pensar*, *per
programar*, *per revisar*.

### Llistar models amb un proveïdor apagat

Baixa de 10s a **4s** per proveïdor: llistar models és un GET a un
endpoint que ha de ser instantani, i amb 10s un proveïdor que s'empassa
la connexió feia esperar tota la llista.

I l'error deixa de ser el de Go en cru —«dial tcp [::1]:8089: connectex:
No connection could be made because the target machine actively refused
it»— per dir el que necessites saber: **«localhost:8089 no accepta
connexions (el servidor no està engegat?)»**.

### Els botons de la finestra segueixen el tema

Els botons de minimitzar, maximitzar i tancar els pinta el **sistema**,
no el CSS, i el color anava escrit a mà al `main.js` amb els valors del
tema fosc. Amb el tema clar quedava un requadre negre a dalt a la dreta
enmig d'una barra clara.

Ara la pàgina li passa els colors del tema actiu (els mateixos tokens que
la fila de pestanyes, que és la que fa de barra) en arrencar, en canviar
de tema i quan el sistema canvia. L'alçada també surt d'una constant
compartida, que abans hi havia el 38 escrit a dos llocs.

La finestra tampoc no neix fosca: els colors d'arrencada —els que es
veuen abans que la pàgina carregui— surten del tema del **sistema**, que
és el que el gregal segueix si no tries res. Abans, a qui va en clar li
feia un flaix negre. Un test lliga aquests colors a la paleta de
`docs/identitat.md`: és exactament així com s'havien desenganxat.


## 1.3.0 — 2026-09-15 — que les eines es cridin quan toca

Una tanda de fer que el que ja hi havia surti de debò a la pantalla, amb
els números al costat: tot el que diu aquesta entrada està mesurat contra
el model real, no per impressió.

### L'agent pregunta en comptes d'endevinar

El mode `code` **no tenia cap regla** al prompt: l'únic que llegia el
model era «primer la solució», i davant d'una petició ambigua endevinava
i es posava a escriure fitxers. L'eina `question` —amb tot el circuit fet
a la web i al TUI— no la cridava mai.

Mesurat amb el projecte ja explorat, o sigui amb la decisió damunt de la
taula, 16 mostres per variant:

| | tasca ambigua | tasca clara (control) |
|---|---|---|
| sense regla | **0/16** | — |
| regla base | 14/28 (50%) | 0/12 |
| regla afinada | **11/16 (69%)** | 0/12 |

El control importa: la regla no el torna preguntaire. I el segon
paràgraf parla de fitxers perquè el mode de fallada observat era aquest —
la meitat de les vegades que no preguntava, triava ell (SQLite, JSON, una
interfície) i escrivia el fitxer.

**La descripció de l'eina no mou res**: provada llarga en català, curta i
en anglès imperatiu, totes donen el mateix. Queda escrit al codi perquè
ningú ho torni a provar.

### Les eines que es perdien

Provades **soles, sense competència, totes es criden 3/3** —`todowrite`,
`patch`, `delegate`, `gh_issue` i `bash_background` incloses. Els
esquemes i les descripcions són correctes: el que es perd, es perd contra
les altres 22. Dues arreglades fent que la **germana** els cedeixi el
pas: `bash` avisa que si la comanda pot passar dels 30s se'n vagi a
`bash_background` (2/3 → 3/3), i `edit` que per a diversos canvis al
mateix fitxer vagi a `patch`.

El `todowrite` depèn del model i no del text: **Nex-2.5-mini 4/4** (0/4
sense la regla), Qwen38-Flash-FAST 1/4, i els dos Ornith **0/4** amb
quatre formulacions. Si el checklist no surt, el que falla és el model
del rol `code`.

### Els passos esgotats ja no passen per feina acabada

Quan el loop esgotava `agent.max_steps`, el web i el TUI feien la síntesi
final i prou: a la pantalla arribava igual que una feina acabada. Ara ho
diuen tots dos (`steps_exhausted`) i diuen què fer.

### L'escriptori

- **Un sol commutador de mode**, al composer, amb els tres modes visibles
  i un color cadascun. N'hi havia tres alhora dient el mateix i cap
  ensenyava on eres.
- **Permisos**: un cadenat al costat, amb el detall al hover i un menú
  que explica els tres modes.
- **«rol: chat» deia què era, no què feia.** Ara diu «respon: automàtic»
  o el model fixat — i **s'hi pot tornar**: abans, triar-ne un apagava el
  router per a tota la sessió sense cap manera de desfer-ho.
- **Temps del torn** en viu i, en acabar, quant ha trigat.
- **Cua de missatges**: amb un torn en marxa, Enter encua en comptes de
  bloquejar-te. Aturar el torn la buida i ho diu.
- **El checklist, en un desplegable** just sobre el xat. L'agent el
  reescriu a cada pas i cada reescriptura deixava una targeta nova.
- **Pestanyes de sessió a dalt de tot**; a l'escriptori passen a ser
  elles la barra de títol, que si no els botons de finestra hi cauen
  a sobre.
- **Revisor automàtic com a interruptor** a Preferències. Passava un
  segon model per sobre de cada resposta i des de la finestra no hi havia
  manera d'apagar-lo.
- **Idioma triable** (català i anglès): canvia la finestra i també la
  llengua de les respostes de l'agent. Cobreix navegació, Preferències i
  el composer; la resta de pàgines i el TUI, encara no.

### Office

- **La subfinestra s'eixampla** amb l'agafador de l'esquerra (i amb
  fletxes), amb mínim i màxim que es recalculen.
- **El full es llegeix**: capçaleres A·B·C i números de fila, fixes en
  desplaçar-se, i **pestanyes de fulls a baix** com a l'Excel. Ensenyava
  20 columnes fixes i perdia la resta sense dir-ho.
- **Una ruta amb espais ja no es perd**: «EWEC EDH Project Plan.xlsx» es
  desava amb els espais i el lector de `@ruta` talla al primer espai, o
  sigui que el model rebia mitja ruta i deia, amb raó, que el fitxer no
  existia. Ara el temporal es desa sense espais i les mencions admeten
  `@"entre cometes"`.

### TUI

- **`/graf`** per mirar i executar els grafs del projecte. No els edita:
  dibuixar vol arrossegar. Els passos surten a mesura que acaben i Esc
  talla.
- Les accions dels missatges són icones dibuixades, no text.


## 1.2.0 — 2026-09-15 — grafs

Un **graf** és un procediment dibuixat: quins passos es fan, en quin
ordre, i quina fletxa se segueix segons el que hagi sortit. Es dibuixa a
la vista **Grafs** (Ctrl+G) i es desa a `.gregal/flows/` **del projecte**,
perquè «com es revisa un PR aquí» és del repositori i ha de viatjar-hi.

Val la pena dir què **no** és, perquè amb el mateix nom es venen tres
coses: un graf d'execució no és un graf de dependències ni un graf de
coneixement, i **tenir-lo no et dona memòria ni context**. Serveix quan
el procediment es repeteix, quan un pas ha de veure el que ha fet
l'anterior, i quan després vols poder mirar d'on ha sortit un resultat.
Per a una pregunta d'un sol cop no val la pena i no s'ha de fer servir.
Tot això és a `docs/grafs.md`.

- **Editor visual.** Passos d'agent, d'eina i de nota; s'arrosseguen pel
  llenç i es connecten estirant des del punt de sota d'un pas fins a un
  altre. A cada fletxa s'hi escriu quan s'agafa.
- **Llenguatge de condicions curt a posta**: `clau`, `!clau`, `==`, `!=`
  i `conté`. Tot el que demani més ha de ser un pas d'agent, no mitja
  gramàtica nova.
- **L'estat passa de pas a pas** amb `{{clau}}`, més `last` i
  `<id>.error`.
- **Després d'un pas que falla només valen les fletxes amb condició
  escrita**; sense cap, s'atura i diu quin pas ha estat. Continuar per la
  fletxa sense condició passant un valor buit era la manera silenciosa
  d'equivocar-se.
- **Sostre de 100 passos.** Els cicles hi són a posta —reintenta fins que
  els tests passin— i sense sostre un cicle mal posat dona voltes.
- **Executar il·lumina els passos** a mesura que passen i deixa el resum
  a la conversa: un graf que s'executa i desapareix no serveix de gaire.
  Amb «sense demanar permís» les eines que en demanen passen; les
  denegades continuen bloquejades.
- El motor (`internal/flow`) no coneix ni model ni eines, així que es
  prova sencer sense cap proveïdor; el TUI i el mode `-p` hi poden
  endollar el mateix agent quan els toqui.

També:

- **Al tema fosc, `--hover`, `--raised`, `--edge` i `--red-soft` es
  definien amb ells mateixos** (`--hover:var(--hover)`). CSS descarta una
  definició circular i el token es queda sense valor: tots els *hover* del
  fosc eren transparents, sense cap error a la consola. Hi ha test.
- `max_steps` admetia fins a 50 tot i que la configuració ja deixava
  posar-ne més.


## 1.1.9 — 2026-09-15 — tema clar i preferències

- **Tema clar**, amb la seva taula a `docs/identitat.md`: els accents
  s'enfosqueixen perquè l'escuma i l'arena del fosc no passen AA sobre
  blanc. Surt del sistema si no tries, i el segueix si el sistema canvia.
  Fer-lo va destapar **24 colors escrits a mà** en `rgba()` fora dels
  tokens —entre ells **dos blaus de la paleta antiga** i una pila de vels
  `rgba(255,255,255,…)` que en clar són blanc sobre blanc. Ara són tokens,
  i el test d'identitat mira també els `rgba` de color, que el tema clar
  defineixi tots els tokens del fosc, i que els valors surtin del document.
- **Preferències** (⚙ o Ctrl+,): un sol lloc amb seccions i tries d'un
  clic. Aparença (tema, densitat, mida del text: mouen tot l'espaiat i la
  tipografia alhora), Conversa, Agent, i els proveïdors com a secció.
  Abans el ⚙ només obria proveïdors, la vista vivia al menú ⋯ i els
  permisos a la capçalera.
- **El clip obre un menú**: imatges, **carpeta com a context** (tria una
  carpeta del projecte i en posa l'arbre al missatge) i **canviar de
  projecte** (diàleg natiu a l'escriptori).
- **Icones d'enviar i adjuntar dibuixades**, no emoji: l'emoji canvia de
  forma i de color a cada sistema i desentonava.

## 1.1.8 — 2026-09-15 — l'escriptori recorda de què heu parlat

L'escriptori tenia el motor però li faltava el que fa que ChatGPT o
Claude es facin servir cada dia: saber quines converses has tingut i
poder-hi tornar.

- **Historial de converses.** Es desen soles en acabar cada torn (abans
  només clicant «Nova sessió», i cada cop en creava una còpia), sempre
  al mateix fitxer: si tanques l'app enmig, no perds res. Cada una té
  títol, tret del primer missatge teu. A la barra lateral, agrupades per
  data (Avui / Ahir / Aquesta setmana / Aquest mes / Abans), amb cerca,
  l'actual marcada i esborrar en passar-hi per sobre. Reprendre'n una la
  continua en comptes de clonar-la. Les converses velles, que no tenien
  títol, se'l treuen del contingut.
- **Una targeta per crida d'eina**, que evoluciona: en curs → ✓ feta o
  ✗ fallada, plegada, amb la sortida a dins i els errors oberts. Abans
  n'hi havia dues i la primera duia «pas 1 · 1 eines», que es repetia
  sol i ja sortia a Activitat.
- **Accions al missatge** en passar-hi per sobre: Copia a tots dos, i
  «Torna-ho a provar» a la resposta.
- **Botó «↓ Al final»** quan has pujat a llegir.
- **Ressaltat de sintaxi** als blocs de codi (comentaris, cadenes,
  números, paraules clau). Era el que més distància marcava.
- **Dreceres**: Ctrl+N conversa nova, Ctrl+F cerca converses, Ctrl+K al
  camp d'escriure, i Escape atura el torn des d'on sigui.

## 1.1.7 — 2026-09-15 — el TUI deixa de saltar

- **Es pot fer scroll.** PageUp/PageDown no estaven implementades
  enlloc, tot i que la barra d'estat anuncia «Pg↑↓ scroll» des de
  sempre: pujar a llegir la conversa era impossible si no sabies
  la drecera no documentada (shift+fletxa). Ara hi són, amb
  ctrl+home/ctrl+end als extrems; home/end es queden per al cursor
  dins del text. I la roda del ratolí ve activada de sèrie, com a
  opencode (`/mouse` la desactiva per seleccionar text del terminal).
- **Escriure ja no et baixa al final.** `View()` forçava anar al
  final cada cop que canviava l'alçada del viewport, i l'alçada
  canvia cada cop que s'obre o es tanca l'autocomplete: escrivint
  et tornava al final a cada tecla i el text apareixia i
  desapareixia. Ara només se segueix el final si ja hi eres.
- **Una sola animació, i que digui alguna cosa.** N'hi havia tres
  alhora en tres punts de la pantalla: la marea a la capçalera, el
  shimmer «TREBALLANT» al composer i el spinner a la barra. Queda
  el spinner del composer, que ara diu què passa («agent pas 7/40 ·
  bash go test ./...»), quant fa que hi és i que Esc atura. La
  barra d'estat no repeteix la frase: només el comptador de passos.
- **Tres veus a la conversa.** `TU ›` i `GREGAL ›` eren dues
  etiquetes bessones i una crida d'eina pesava igual que la
  resposta. Ara el teu missatge porta barra i fons propis (`▌`), el
  que fa l'agent —pensament, crides i resultats— va en un rail
  tènue (`│`), i la resposta no porta cap adorn. El pensament passa
  al rail: era el que es confonia amb el principi d'una resposta.
- **La resposta ja no arrossega una cua d'espais.** Glamour omple
  cada línia fins a l'amplada amb espais pintats: un bloc de fons
  en terminal fosc, i una cua a qualsevol còpia del text.
- **El sostre de passos passa de 10 a 40** (rang 1–200). Deu és poc
  per a una tasca de codi real —llegir quatre fitxers, un grep i
  editar-ne dos ja te'ls menja— i el torn s'acabava a mitges. Esc
  atura el torn, que és el control de debò.
- El marc del composer sortia dues columnes més ample que la
  capçalera: `Width` és el contingut i la vora hi suma 2.

## 1.1.6 — 2026-09-15 — capçalera neta, navegació a la lateral

- Les seccions (Agent, Objectius, Activitat, GitHub, Office)
  viuen a la barra lateral plegable, que recorda si la vols
  oberta o tancada; la capçalera queda en burger + projecte +
  model + mode + permís + menú ⋯ (Pla, Compara, Desfés, Vista).
- L'Esc amb el menú obert el tanca sense aturar el torn.

## 1.1.5 — 2026-09-14 — estil Claude Code + reintent per length

- Preguntes triables, todos amb checks, cua de tasques i onada pro.
- Reintent automàtic quan el raonament es menja el pressupost:
  la tasca ja no mor per `length`.
- Web: el model de configuració es tria d'una llista;
  desktop: `build-backend` troba Go fora del PATH.

## 1.1.4 — 2026-09-14 — Office amb l'agent + barra pròpia

- Office com a subfinestra amb l'agent i `@docx` de debò;
  selector de models (pill + `/model` al composer).
- L'escriptori estrena barra superior pròpia.

## 1.1.3 — 2026-09-14 — cap eina fantasma

- **El mode xat prometia eines que no tenia.** El prompt deia "POTS
  llegir el disc amb read/glob/grep" (des de la 0.9.17), però xat i
  consulta anaven per `ChatStream`, que no porta `tools`. Un model que
  s'ho creu —DeepSeek— escrivia la crida en el seu format natiu (DSML)
  dins de la resposta, i l'usuari veia
  `<｜DSML｜invoke name="glob">…` imprès en comptes d'una llista de
  fitxers. Ara **tot mode que promet eines passa per l'agent** (TUI i
  web): code amb permís, xat i consulta amb la política de només-lectura
  que ja hi era. Goal es queda parlant, que és el que ha de fer. El preu:
  el xat perd el text en directe (l'agent no fa streaming); recuperar-lo
  és el primer punt de la fase H del pla 1.2.
- **Xarxa de seguretat per al DSML.** Si un proxy filtra el markup igualment,
  `ChatWithTools` el converteix en `tool_calls` normals i el loop continua;
  `ChatStream` el treu del text i l'error diu quina eina volia cridar i per
  què no pot. El markup no arriba mai a la pantalla. Provat amb la cadena
  exacta que va sortir al terminal.
- **El xat recupera el text en directe (H0).** Nou `ChatStreamWithTools`:
  la via amb eines fa streaming i reconstrueix les `tool_calls` dels
  deltes de l'SSE (acumulades per índex: l'ordre d'arribada no és l'ordre
  de les crides). L'agent —i per tant el xat i la consulta— tornen a
  ensenyar el text mentre creix, al TUI (mateix streamer que abans) i a la
  web (bombolla viva que es tanca a cada crida d'eina). Amb això ja no hi
  ha cap via que hagi de triar entre veure el text créixer i poder cridar
  eines; el pas anterior d'aquesta mateixa versió havia pagat el streaming
  per la correcció, i ara no cal.
- **Escriptori: el markdown del model es pinta de debò.** El renderitzador
  només feia codi, negreta i salts de línia: les respostes sortien amb els
  `##` i els guions en cru. Nou `app/md.js` (títols, llistes amb nivells,
  cites, regles, taules, enllaços només http(s), èmfasi, caselles de tasca,
  codi amb capçalera i «copia»), amb tot el text escapat abans de construir
  res i tests propis.
- **Escriptori: Canvis, Fitxers i Terminal es quedaven a mitja pantalla**
  amb el xat visible a sobre, com una làmina. `setView` sí que posava
  `hidden` al xat, però `#main.on` té més especificitat que la regla
  d'agent d'usuari i la trepitjava. `[hidden]{display:none!important}` i
  els panells ocupen tota l'alçada.
- **Escriptori: detalls que feien pobre.** El fil d'Ariadna (`/ main`)
  trepitjava les pestanyes en amplades mitjanes; «mode codi» es partia en
  dues línies; l'`input` del Terminal era blanc sense tema; les targetes
  d'eina duien el glif ≋ cadascuna (cinc en columna per un torn) i el text
  de treball d'un pas sortia dues vegades (bombolla viva i «Pensant…»). Tot
  resolt: la branca s'amaga per sota de 1100px, els botons no es parteixen,
  els inputs dels panells porten el tema, les targetes van sense glif
  alineades sota el text, i el text de treball es queda apagat a la seva
  bombolla sense repetir-se.
- **TUI: cap línia més ampla que el terminal.** Test que pinta la vista
  sencera a sis amplades i onze estats; va trobar la capçalera (projecte o
  branca llargs) i la barra d'estat desbordant a 60 columnes. La capçalera
  escurça amb «…» i la barra cedeix per graons (context → model → rol)
  abans que l'estat del torn.
- **Windows: el Terminal de l'escriptori no podia engegar cap procés.**
  `/api/exec` feia `sh -c` a pèl i al portable moria amb «"sh": executable
  file not found in %PATH%»; l'eina `bash` de l'agent, en canvi, tirava
  amb `cmd`. Nou `internal/shell`, un sol criteri per a tots dos: `sh` si
  és al PATH, el `sh` del Git for Windows si hi és instal·lat (entén `&&`,
  pipes i `npm run dev`), i `cmd /C` com a últim recurs. `GREGAL_SHELL` ho
  força. Trobat conduint el portable, no els tests: a la màquina de
  desenvolupament `sh` sí que hi era.
- **Mòbil: les pestanyes Agent/Objectius/… no es veien.** Dues causes
  sumades: per sota de 760px van a una fila sota la capçalera, però la
  barra de sessions (G1) va arribar més tard i ocupava el mateix lloc; i
  l'`overflow:hidden` que J4 va posar a `#top` per a l'onada retallava la
  fila, que és filla de la capçalera però viu fora de la seva caixa. Ara
  van a sota de la barra de sessions i `#top` no retalla (l'onada es
  retalla dins de la seva franja). Verificat amb `elementFromPoint` al
  mig de la fila: torna la pestanya, no el fons.
- **Escriptori: selector de models.** El pill «model …» de la capçalera és
  un botó (i `/model` al composer): llista el que cada proveïdor anuncia
  ara mateix (`/api/models`), agrupat, amb filtre, fletxes, l'actual
  marcat i els proveïdors que no responen dits en clar; triar-ne un el
  valida al servidor (`/api/model`) i el fixa per al rol actiu només en
  aquesta sessió, com el `/model` del TUI. I al quadre de configuració
  (⚙ → Providers i rols), «Assigna rol» tria el model d'una llista que
  s'omple amb el que anuncia el provider triat (i canvia amb ell), amb
  «altre…» per escriure'l a mà si cal. Abans a la web s'havia d'escriure
  el nom a mà a tots dos llocs.
- **Escriptori: l'Office deixa de ser un formulari.** Deixa caure un
  .docx/.xlsx/.pptx (o tria'l) i s'obre en una **subfinestra lateral** que
  es queda mentre parles amb l'agent; hi ha peticions ràpides per tipus
  (resumeix, corregeix, tradueix, explica les dades…) i una lliure, que
  s'envien a l'agent amb el document adjunt (`@ruta`) perquè hi treballi
  amb `office_read`/`office_edit`; quan acaba el torn el document es
  rellegeix sol i es pot baixar tal com ha quedat. La pujada torna la ruta
  del fitxer al disc (`path`) per fer-ho possible. En pantalles estretes la
  subfinestra ocupa tot l'ample. Nou `app/office.js`.
- **Web: `@document.docx` adjuntava el zip.** `expandMentions` llegia els
  office com a text pla (bytes del zip al prompt); ara passa per
  `OfficeRead`, com al TUI. I una ruta absoluta de Windows (`C:\…`) es
  tractava com a relativa i s'enganxava darrere del workspace: el `:` i la
  `\` ara són vàlids a la menció i `IsRooted` decideix. Amb test que munta
  un .docx real en memòria.
- **Escriptori: barra superior pròpia.** La finestra ja no porta la barra
  de títol de Windows a sobre, d'un altre color: la capçalera de l'app fa
  de barra (arrossegable pels buits) i els botons minimitza/maximitza/tanca
  els pinta el sistema dins la mateixa franja amb els colors de la marea
  (`titleBarStyle: hidden` + `titleBarOverlay`). Al Mac, lloc per als
  semàfors a l'esquerra. Al navegador no canvia res.

## 1.1.2 — 2026-09-14 — identitat única + dist-win que sí porta backend

- Una sola paleta Gregal (`docs/identitat.md` com a font de veritat):
  la web i l'escriptori deixen el blau genèric i passen als colors
  de la marea; Android com a comandament (pla 1.2).
- `dist-win` produïa un portable sense backend i sense signatura:
  ara l'exe de Windows porta el backend integrat i compila sense
  signar (winCodeSign no s'extreu a Windows).
- `package-lock` de l'escriptori al dia (era de la 0.9.6).

## 1.1.1 — 2026-09-14 — cap torn en blanc

- **Un torn que no responia i no deia per què.** Amb un model de raonament
  i `max_tokens` curt, el pensament es menja tot el pressupost i la
  resposta ja no hi cap: el proveïdor tanca amb `finish_reason: "length"`
  i zero tokens de contingut. `ChatStream` no mirava mai el
  `finish_reason` i tornava `("", nil)`, o sigui que el TUI pintava una
  línia en blanc i no hi havia res a què agafar-se. Ara l'error diu quants
  caràcters ha pensat i que cal pujar `max_tokens` del rol. Si hi ha text,
  encara que sigui tallat, es conserva: val més mitja resposta que un
  error. `Chat` i `ChatWithTools` tenien el mateix punt cec; a
  `ChatWithTools` el contingut buit amb `tool_calls` continua sent vàlid,
  que el torn és la crida. El TUI, com a xarxa de seguretat, ja no pinta
  mai un torn buit.
- **La línia d'onada era una barra massissa.** `waveLine` repetia "≋" a
  amplada completa i era el més pesant de la pantalla. Ara l'onada
  s'esmorteeix cap als extrems: separa igual i sembla volguda.
- **A la benvinguda hi faltava el mode CONSULTA**, que existeix, té
  insígnia i surt al composer. També tenia una pancarta de 32 columnes
  fixes que no seguia l'amplada i línies en blanc descordades. Ara hi són
  els quatre modes, alineats.

## 1.1.0 — 2026-09-14 — l'escriptori deixa de ser un mirall

Fins ara l'app d'escriptori era una finestra Electron sobre la webapp, i
la webapp tenia una sola conversa. Aquesta versió obre la porta a fer-hi
la feina diària sense tornar al TUI.

- **Sessions en paral·lel (G1).** El servidor deixa de tenir una única
  conversa: `X-Gregal-Session` (o `?session=`) tria pestanya, cadascuna
  amb la seva conversa, mode, rol i permisos. Sense id tot va a la sessió
  `default`: Android, VS Code i Telegram no s'han de tocar. S'acaba el
  409 "ja hi ha un agent en marxa" quan el que volies era una altra cosa.
  Nou `POST /api/agent/cancel` per aturar el torn des de qualsevol client.
- **Workspaces (G2).** El directori de treball era `os.Getwd()` a
  l'arrencada; ara és per sessió i es canvia amb `/api/workspaces`, amb
  recents desats a `~/.local/share/gregal/workspaces.json`. Dues
  pestanyes, dos repositoris. Amb més d'una sessió viva, `/rewind` i els
  checkpoints es filtren al workspace propi per no desfer la feina de
  l'altra pestanya.
- **Fitxers i diffs (G3).** `/api/tree`, `/api/file` i `/api/diff`
  (fitxers → hunks → línies, en JSON). Totes rebutgen `..`, rutes
  absolutes i `~`.
- **Revisió de canvis amb accepta/descarta (G4).** Pestanya *Canvis*:
  diff acolorit per hunk i `/api/diff/discard` per desfer-ne un de sol
  (`git apply -R` del pegat), un fitxer sencer, o esborrar un fitxer nou.
  És el flux que abans només existia al terminal.
- **Arbre de fitxers i visor (G5)** amb marca d'estat git, i doble clic
  per posar el fitxer al missatge.
- **Composer de debò (G6).** Ordres `/` amb menú, `@fitxer` amb
  autocompleció real (abans era text del placeholder) i HUD de cost.
- **Electron natiu (G7).** `contextIsolation` + `preload` (abans la
  finestra no ho declarava), notificació quan acaba un torn sense focus,
  *Obre projecte…* amb diàleg natiu, *Sessió nova* (Ctrl+T), *Canvis*
  (Ctrl+D), mida i posició recordades i auto-update opcional amb
  electron-updater.
- **Processos en segon pla (G8/H3).** `/api/exec` + pestanya *Terminal*, i
  les eines `bash_background`, `bash_output` i `bash_kill`: el bash de
  l'agent té 30 s, això no. Es maten per grup (matar `sh` deixava els
  nets vius) i amb la sessió. Mateixa política de permisos que el bash:
  el terminal no és una porta del darrere.
- **UI modular.** `index.html` ja no creix: el que és nou viu a
  `internal/web/app/*.js` com a mòdul ES, servit des de l'`embed.FS`.
- **Contracte i CI (fase F).** `docs/api-contract.md` documenta totes les
  rutes i events; un test (`TestContracteAPIDocumentat`) falla si se
  n'afegeix una sense documentar. La CI feia servir Go 1.24 amb un
  `go.mod` que en demana 1.27 — ara llegeix `go-version-file`. I el test
  `/attach` tenia una ruta absoluta de la màquina de desenvolupament: mai
  hauria passat a la CI.
- **Contenció de rutes a Windows.** `filepath.IsAbs` hi demana lletra
  d'unitat, de manera que `/etc/passwd`, `\Windows\System32\…` o
  `C:fitxer` passaven per relatives i s'unien tan tranquil·lament al
  projecte: `safeJoin` (`/api/tree`, `/api/file`, `/api/diff`) i
  `InsideProject` (el pas automàtic de write/edit dins del projecte) les
  deixaven entrar. Ara ho mira `tools.IsRooted`, que no depèn del GOOS on
  corre. Només afectava el binari de Windows, que és el que distribueix
  `dist-win`.
- **Descartar un hunk ja no reescriu el fitxer sencer.** Amb
  `core.autocrlf=true` (el que ve de sèrie al Git de Windows) el
  `git apply -R` de G4 convertia tot el fitxer a CRLF: desfeies una línia
  i te'n quedaven setze de modificades. Si el fitxer és LF pur, l'apply va
  amb el filtre apagat.
- **Els tests ja no toquen el `$HOME` de debò.** Aïllaven el directori
  d'usuari només amb `HOME`, però `os.UserHomeDir()` a Windows llegeix
  `USERPROFILE`: `go test ./...` llegia —i sobreescrivia—
  `~/.local/share/gregal/history`. També hi havia tests que construïen
  JSON enganxant rutes sense escapar (`\U` de `\Users` no és cap escapada)
  i tests de POSIX (permisos 0600, backends de porta-retalls, servidors
  MCP amb shebang) que a Windows petaven en comptes de saltar-se.
- **Animacions de la marea al TUI.** El buit de la capçalera entre el logo
  i la versió no feia res; mentre hi ha feina hi passa una onada. A
  "TREBALLANT" hi passa una cresta d'escuma per sobre i al costat el temps
  del torn: saber que fa 2m04s que hi és treu l'angoixa de no saber si
  s'ha penjat. El context passa de percentatge a barra amb resolució de
  vuitens, que batega per sobre del 80%. En repòs no s'hi encén res: el
  tick va a mig segon i qualsevol animació hi parpellejaria.

## 0.9.17 — 2026-09-14 — verificació només amb feina real

- L'auto-verificació només corre si el torn ha tocat fitxers
  (journal o git diff): en xat pur ja no surt el bloc VEREDICTE.
- Regla anti-soroll al revisor: sense canvis ni eines → APROVAT
  d'una línia; mai CAL REVISAR dient alhora "res a validar".
- Mode xat: el prompt deia "no inspeccionis" però la política
  permet llegir — ara diu POTS llegir amb read/glob/grep i
  l'assistent ja no s'inventa límits.

## 0.9.16 — 2026-09-14 — avís Model access del Zen

- Quan el Zen torna 500 "Internal server error" (model
  desactivat al workspace — Model access, típic sense zero
  data retention), l'error ja diu on activar-lo en comptes
  de deixar-te endevinant.

## 0.9.15 — 2026-09-14 — permissiu + office a l'escriptori

- Mode permissiu (l'agent tira sense demanar; deny bloqueja):
  TUI `/permissiu [on|off]` (cobreix agent + /write + /edit),
  API `POST /api/permissive`, botó ◇/◆ a la web (només sessió).
- Web/desktop: vista Office (puja docx/xlsx/pptx, llegeix,
  posa valors a cel·les, cerca/substitueix, baixa el fitxer).
- Desktop 0.9.15 amb backend integrat al dia.
- Fix: /api/state es penjava (mutex) en afegir-hi permissive.

## 0.9.14 — 2026-09-14 — errors de provider sencers

- Els errors HTTP del provider (401/500/…) ara mostren el cos
  SENCER (fins a 2000 caràcters, abans 200-300) i amb un sol
  prefix: el motiu real (model inexistent, clau dolenta, …)
  ja es llegeix al TUI sense haver d'endevinar.

## 0.9.13 — 2026-09-14 — gregal config

- NOU `gregal config`: assistent per posar claus api (tria
  provider, enganxa la clau sense eco, la prova contra
  /models i la DESA al disc, permanent). `config list` i
  `config set-key <prov> --stdin [--persist]` per scripts.
- El config ara força 0600 a cada Save (abans només en crear).
- Neteja: el probe GET /models vivia copiat 3 cops (web,
  telegram, tui); ara és `llm.ProbeModels` compartit.

## 0.9.12 — 2026-09-14 — claus al TUI

- En triar un provider sense clau (/model, menú de models), el
  TUI ho avisa i la següent línia es menja com a secret: sense
  eco a la pantalla, sense historial d'entrada, sense conversa.
- /provider key <nom> sense clau també la demana; amb clau
  inline funciona com abans. Esc cancel·la la captura.
- Literal = només memòria (mai al disc, test que ho afirma);
   o buit persisteix al config.

## 0.9.11 — 2026-09-14 — office

- Word/Excel/PowerPoint (.docx/.xlsx/.pptx) amb stdlib, sense
  dependències: `office_read` (text/dades) i `office_edit`
  (set_cell xlsx amb números numèrics, replace docx/pptx fins i
  tot amb text partit en fragments). Tot passa pel journal:
  /rewind ho desfà. Formats antics (.doc/.xls/.ppt) deriven a
  LibreOffice amb la comanda exacta.
- TUI: /read i @adjunts entenen office; /permissions els llista
  (read=allow, edit=demana permís).
- API nova /api/office/{upload,read,edit,download} per a l'app
  (el fitxer viu al mòbil: puja en base64, edita al servidor,
  baixa el resultat). Android 0.6.8 amb diàleg Office complet.

## 0.9.10 — 2026-09-13

- Diff sense "+" fantasma: el salt final no compta com a línia.

## 0.9.9 — 2026-09-13

- Alt+Enter fa salt de línia de debò (el textarea ignora l'enter amb
  modificador; s'insereix a la posició del cursor). També Shift+Enter.

## 0.9.8 — 2026-09-13 — edició TUI

- Identitat "la marea": paleta pròpia (escuma #7FD4C1, arena #F2E7C9,
  blau llacuna), onada ≋ sota la capçalera, pancarta de benvinguda,
  eines amb capçalera ≋ nom i resultat ✓/✗ nom.
- Diffs inline: cada write/edit/patch de l'agent mostra el canvi
  (+ verd / − vermell, retallat a 40 línies). Nova ordre /diff
  (canvis de la sessió + git).
- Entrada multilínia (textarea, alçada 1..6): Enter envia,
  Alt+Enter fa salt de línia. Suggeriments només en 1 línia.
- Esc atura la feina en curs (com Ctrl+C). El raonament intern
  ja no cau en silenci: comptador "ha pensat N caràcters".

## 0.9.7 — 2026-09-13

- Cancel·lació amb Ctrl+C al TUI: la primera pulsació cancel·la la crida
  activa (xat, agent, revisor, comparació o compactació), la segona surt.
- Comparació paral·lela cloud (`/api/parallel`, TUI, web, Android): mateix
  missatge a tots els rols alhora per triar model amb dades.
- Rol inicial coherent amb el mode a tots els clients (code→code…).
- Escriptura del pla completada: E1 tancada aquí (tots els fronts).

- TUI amb popup central `/settings` (mode, rol, model, revisor, permisos,
  providers i sessions), navegació amb Esc i animació de treball.
- Integració read-only de GitHub: `gh_issue` i `gh_pr` per consultar issues,
  PRs, diffs i checks sense exposar una shell arbitrària.
- Menú de providers amb prova de connexió, models vius i assignació al rol
  actiu; claus sempre emmascarades.
- App desktop 0.9.6: backend gestionat, port dinàmic, token local compatible,
  arrencada sense flaix blanc i pantalla d'error útil.

- Bearer-only a `/api/*`, `gregal --doctor`, rewind segur i clients desktop,
  Android i VS Code actualitzats.
- Router per tasca, pressupost, memòria persistent, multimodalitat, MCP,
  revisor estricte i delegació read-only en paral·lel.

[0.9.6]: https://releases.example.org/gregal
