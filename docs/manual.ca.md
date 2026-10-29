# Manual d'usuari de Gregal

[English](manual.md)

Gregal és un agent de programació amb interfície de terminal, escriptori i
web. Envia les peticions al model a l'endpoint configurat per al rol actiu.
Aquesta guia cobreix la configuració habitual i els límits de seguretat; per
als detalls que depenen de la compilació, consulta l'ajuda integrada i les
referències enllaçades.

## Requisits

- Go 1.27.1 o posterior per executar-lo des del codi font.
- Un endpoint de model compatible amb OpenAI i identificadors de models
  disponibles en aquell endpoint.

L'endpoint i el model han d'admetre les capacitats que vulguis fer servir,
com ara l'ús d'eines. La validació de la configuració no prova la connexió,
les credencials ni la disponibilitat del model.

## Inicialització i configuració

Des de l'arrel del repositori, crea la configuració inicial:

```sh
go run . init
```

En sistemes Unix, la ruta predeterminada és
`~/.config/gregal/config.yaml` i el fitxer es crea amb permisos `0600`.
L'ordre també crea `AGENTS.md` al directori del projecte actual si encara no
existeix. Tria una altra ruta amb `go run . init --config=/path/to/config.yaml`.

Defineix l'URL del proveïdor i els identificadors dels models per als rols
`chat`, `think`, `code` i `reviewer`. Les URL `local` i `local-direct`
generades fan referència a serveis de loopback que Gregal no inicia. L'URL
`cloud` generada és un exemple: substitueix-la, i també els identificadors de
model, pels valors que admeti el teu endpoint. Pots assignar diversos rols a
un mateix proveïdor si té les capacitats que necessiten.

El proveïdor cloud d'exemple llegeix la clau d'una variable d'entorn:

```yaml
providers:
  cloud:
    base_url: https://api.example.org/v1 # substitueix-ho pel teu endpoint
    api_key: ${GREGAL_CLOUD_API_KEY}
```

Defineix la variable a l'entorn des d'on iniciaràs Gregal, o fes servir un
gestor de secrets. No desis credencials ni configuració privada al repositori.

### Catàleg de models

Gregal guarda una còpia del catàleg de [models.dev](https://models.dev)
(finestra de context, sortida màxima, preus, i si el model admet eines,
raonament i adjunts) a `<data>/models-catalog.json`. La refresca en segon
pla un cop al dia; la primera vegada espera com a molt 4 segons.

- **Finestra de context.** Surt, per aquest ordre, del `context_window` del
  rol, del que declara el proveïdor a `/models`, del catàleg i d'una taula
  interna. `/stats` diu d'on surt. Els servidors locals sempre fan servir la
  finestra amb què s'han engegat.
- **Cost.** Els rols amb un model sense preu a `cost:` prenen el del
  catàleg, així que el cost estimat surt sense haver de configurar preus.
- **Correspondència.** El proveïdor es busca per l'adreça base (per exemple,
  opencode zen → `opencode-go`). Si no en coincideix cap, el model es busca
  pel nom, amb la finestra més petita de les coincidències i sense preu.

`models_catalog: off` el desactiva: no es fa cap petició a models.dev.

### Protocols natius de proveïdor

Per defecte tots els proveïdors parlen el protocol Chat Completions
compatible amb OpenAI. Hi ha dos protocols natius que s'activen amb `type`:

```yaml
providers:
  anthropic:
    type: anthropic            # API Messages d'Anthropic (POST <base_url>/messages)
    base_url: https://api.anthropic.com/v1
    api_key: ${ANTHROPIC_API_KEY}
  openai-responses:
    type: openai-responses     # API Responses d'OpenAI (POST <base_url>/responses)
    base_url: https://api.openai.com/v1
    api_key: ${OPENAI_API_KEY}
```

`type` admet `openai` (o buit, el valor per defecte), `anthropic` i
`openai-responses`. El motor, el failover, els reintents (429/5xx, i el 529
d'Anthropic) i els avisos de progrés no canvien; només canvia el format del
cable.

- **anthropic**: prompt de sistema, eines, blocs `tool_use`/`tool_result` i
  streaming. Els blocs de pensament (amb la signatura) es tornen intactes dins
  d'una cadena de crides d'eina. El prompt caching marca amb `cache_control`
  el prompt de sistema, la llista d'eines i l'últim missatge; el consum
  inclou les lectures i escriptures de cache. El `think` del rol es tradueix a
  pensament adaptatiu amb resum visible i `output_config.effort`
  (`low`/`medium`/`high`/`max`); `think: no` demana `effort: low`. No s'envia
  `temperature`, perquè els models Claude actuals rebutgen els paràmetres de
  mostreig.
- **openai-responses**: missatges, crides de funció i streaming amb l'API
  Responses i `store: false`. Es demanen els items de raonament xifrats i es
  tornen intactes davant de les crides que van produir; els resums de
  raonament arriben com a text de raonament. `think` es tradueix a
  `reasoning.effort`.

Els camps opcionals (pensament, esforç, temperature, reasoning, include) es
treuen sols si un model els rebutja amb HTTP 400, i ja no se li tornen a
enviar durant el procés. El protocol es tria per `base_url`: si dos
proveïdors comparteixen URL amb tipus diferents, mana el que fan servir els
rols, i la configuració es rebutja si els rols fan servir tots dos.

## Interfície de terminal

Valida la configuració i inicia Gregal:

```sh
go run . --check-config
go run .
```

`--check-config` només comprova l'estructura i les referències de la
configuració. A la interfície de terminal, `/help` mostra les ordres
disponibles i `/model` obre un selector amb els models anunciats pels
endpoints configurats.

## Interfície web local

Inicia el servidor a loopback:

```sh
go run . --serve --addr 127.0.0.1:8097
```

Obre `http://127.0.0.1:8097/` al navegador. Gregal rebutja adreces que no
siguin de loopback si no s'ha configurat un token o autenticació d'usuari. El
servidor no ofereix TLS; configura HTTPS abans de fer-lo accessible fora
d'una màquina local de confiança.

El servidor escriu al terminal l'inici i el final de cada torn i equip
(estat, temps, mai credencials). Amb `--log-file ruta` (o la variable
`GREGAL_LOG_FILE`) a més ho desa en un fitxer en mode append: així, quan
una execució s'atura, el terminal o el fitxer diuen si ha acabat, ha
fallat o s'ha aturat, i quant ha durat.

Al compositor, tria **Pregunta** (respon i llegeix, no canvia res),
**Planifica** (investiga i proposa un pla que aproves) o **Actua** (fa canvis).
El botó del cadenat diu què pot fer Actua sense preguntar: **Demana abans de
canviar**, **Aprova les edicions** (per defecte) o **Autonomia total**. El
terminal i l'Android mostren els mateixos tres modes i permisos; vegeu
[Modes i permisos](modes.ca.md). Objectiu i depuració són modes avançats
(`/goal`, `/debug`). La selecció manual del rol és una opció avançada del
selector de models; Gregal pot triar-lo automàticament segons la feina.

## Taller 2D

La vista **Taller 2D** de la barra lateral representa les sessions obertes
com estacions de treball. Cada estació mostra el projecte i si la sessió
treballa, està en repòs o espera una resposta. Clica una estació per obrir
la seva conversa; **Qui em necessita?** obre la primera amb una interacció
pendent coneguda pel navegador actual.

Les estacions reutilitzen les sessions existents. Les alertes depenen de
les preguntes i aprovacions rebudes en aquest client; no són un registre
global de totes les interaccions obertes des d'altres navegadors.

El Taller també segueix els subagents reals: selecciona'n un per veure la
tasca, el model, l'eina, els passos consumits, el resultat o l'error. La sala
animada i les targetes mostren estats observats; els passos no són un percentatge
de finalització. Es conserven fins a 32 subagents recents fins al torn següent.
La sala es pausa quan queda oculta i respecta la preferència de moviment reduït.
**Rols i relleus.** Per sobre de les badies, la sala mostra un personatge per
rol de la sessió actual: Coordinació, Recerca, Construcció, Proves i Revisió.
Cada pas del coordinador s'atribueix a un rol segons les eines que fa servir
(planificar i `todowrite` → Coordinació; `read`, `grep`, `glob`, `web_search`,
`web_fetch` i ordres de shell de només lectura → Recerca; `write`, `edit`,
`patch`, `office_edit`, `git_commit` i la resta d'ordres de shell →
Construcció; ordres de test, build i lint → Proves; checkpoints autònoms i
verificació automàtica → Revisió). Quan el rol canvia, un sobre vola d'una
taula a l'altra amb una etiqueta curta («2 fitxers per editar», «Proves per
executar») i xips amb els fitxers; el personatge que el rep camina fins a la
seva taula, la pantalla s'encén i una bombolla diu què fa («Llegint
src/stats.py», «Executant les proves»). El resultat de les proves queda com
una insígnia verda o vermella a la taula de Proves. Els rols lliures esperen
drets i atenuats al costat de la taula. Els subagents reals seuen en una fila
del fons amb el nom del subagent definit per l'usuari. L'atribució és només
visual: el coordinador continua sent un sol agent.

Clica un personatge, una taula o un rol de la llegenda per obrir-ne el panell:
les accions del rol en aquest torn, els fitxers tocats (clica'ls per veure'ls;
Construcció també enllaça amb Canvis), les ordres amb el resultat i la durada,
els tokens consumits mentre tenia el torn i, per a Recerca, l'encàrrec i el
resum de l'investigador real. La tira de colors de sota la sala té un segment
per pas; clica'n un per destacar aquell pas i el seu personatge. La tira i els
relleus també funcionen a la **Repetició**.

**Investigador d'inici de torn.** En els modes code, autonomous i debug, un
torn que demana investigar (no salutacions, confirmacions ni edicions
mínimes) comença amb un subagent investigador real de només lectura (només en
projectes de 40 fitxers o més: en un de més petit el mapa del projecte ja en
dona tots els fitxers i el scout només afegiria una tirada abans del primer pas;
`agent.scout: always` l'engega igualment): fins a 8
passos i 4 minuts, amb el model del rol `delegate` si n'hi ha, o el de la
sessió (o un subagent de només lectura definit per l'usuari amb el nom
`researcher`). El seu resum, amb camins de fitxer, s'afegeix a la tasca perquè
el coordinador no repeteixi l'exploració. Si falla o s'atura, el torn continua
igual. Els seus tokens compten en el torn. Es desactiva amb `agent.scout: off`
(per defecte `auto`).

**Explorador per a preguntes.** En mode consulta (inspect), la mateixa regla
fa que una pregunta sobre un projecte de 40 fitxers o més comenci amb un
**explorador**: un subagent de només lectura amb només `read`, `grep`, `glob`
i `symbols`, el mapa del projecte i fins a 8 passos, que busca al codi i
torna la resposta amb cites `camí:línia`. El coordinador respon a partir
d'aquest informe i només llegeix el que l'informe marca com a no verificat o
no cobreix. En preguntes sobre el repositori del mateix Gregal ha reduït els
tokens d'entrada unes tres quartes parts i el temps gairebé a la meitat, amb
cites com a mínim igual de precises ([docs/research/explore.md](research/explore.md)). Les seves lectures
queden dins del subagent: la pregunta següent de la mateixa conversa no les
torna a enviar. Un subagent de només lectura definit per l'usuari amb el nom
`explorer` el substitueix; `agent.scout: off` també l'apaga. Amb `gregal -p
--output json`, el resultat porta `explorer`, `explorer_report` i
`timings.explore_ms`.

**Subagents en paral·lel.** Quan la tasca anomena diverses àrees
independents (una llista «(1) … (2) …», línies numerades o amb guions, camins
de carpetes diferents, o una pregunta sobre «la TUI, el servidor web i el bot
de Telegram»), l'investigador es parteix: un per àrea, fins a 3, tots alhora,
i els seus resums arriben al coordinador agrupats per àrea. També es demana
al coordinador que enviï les investigacions independents a `delegate` en una
sola crida, perquè corrin juntes. Com a màxim corren 4 subagents alhora per
torn (8 per procés); la resta surten **En cua** fins que hi ha lloc. Quan els
subagents van a un servidor local (Ollama, llama.cpp, LM Studio o qualsevol
URL de localhost) corren d'un en un, perquè amb un sol slot només farien cua
dins del servidor. `agent.parallel_subagents: N` canvia el límit. A la sala,
els fills que treballen junts seuen l'un al costat de l'altre a la fila del
fons amb una etiqueta «3 alhora»; a la badia, cada fill té una barra sobre
l'eix de temps del torn (les barres que se solapen han treballat juntes), i
la capçalera conserva «fins a N alhora» quan acaben.

Els equips (vista Team) també apareixen al Taller com a badies en directe:
coordinador, agents amb tasca i resultat, graf del flux, detall, cua i Atura
compartits; és una sola sala 2D. La vista Team conserva el seu circuit separat
de col·laboració sense eines.
Cada badia mostra el consum real del torn (tokens de pujada i baixada, i cost
només si el model té preu; si el proveïdor no declara consum, només el nombre
de crides) i un historial d'inspecció amb els passos del torn. Una badia
ocupada es pot aturar allà mateix, i el títol de la pestanya compta les
sessions que esperen resposta. Si el torn només està encuat, l'Atura ho diu
en comptes de fer veure que ha parat. El resum mostra el cost total de les
sessions amb preu (o «cost n/d»), l'edat de les dades i un historial
d'inspecció per badia; el botó d'atenció recorre els pendents i els fallits
tenen secció pròpia.

En mode Code i Autònom, el coordinador té instruccions de delegar automàticament
investigacions i revisions independents quan aportin valor. Els subagents poden
llegir i cercar; les edicions i decisions queden al coordinador. Cada torn web
admet fins a quatre treballadors simultanis, amb un màxim de 12 passos i vuit
minuts cadascun. Cancel·lar el pare també atura els fills. Sostre global:
vuit subagents vius per procés en tots els clients; els resums que tornen
van retallats amb marca quan passen de 6000 caràcters.

Pots afegir un rol `delegate` al mapa `roles` de la configuració per utilitzar
un model més petit o ràpid d'un proveïdor ja configurat:

```yaml
roles:
  delegate:
    provider: el_teu_proveidor
    model: el_teu_model_rapid
    max_tokens: 2048
```

Sense aquest rol, a la web s'utilitza el model actiu del coordinador. Es
respecten els permisos habituals (`permissions.tools.delegate: deny` ho
desactiva). El model decideix quan delegar: pot afegir consum i és útil sobretot
quan les investigacions són independents.

Un rol opcional `small` (o `fast`), amb els mateixos camps, passa les tasques
laterals a un model més petit: els resums de compactació, les propostes de
notes del projecte i l'investigador previ al torn. Si el model petit falla,
ho fa el principal. Sense aquest rol, no canvia res.

## Extres: repetició a la sala, Wrapped, to de #retro, penell, veu i imatges

Tots els extres es poden apagar a **Preferències → Extres**, i tota animació
s'atura amb l'opció del sistema de *reduir moviment*.

**Repetició a la sala.** La pàgina d'Agents (pestanya En directe) té un botó
**Repetició**, i cada grup de feina acabat de la conversa també. Reprodueix
un torn acabat a la sala 2D a partir del registre durable d'events: el
coordinador pensant, fent servir eines (amb el nom) o revisant, esperant
aprovacions, i els fills delegats entrant, treballant i lliurant. Reprodueix o
pausa (Espai), arrossega la barra i tria 1×, 4× o 10×; les esperes llargues
(per exemple una aprovació oblidada uns minuts) s'escurcen a 15 segons.
**Baixa el clip** grava la repetició sencera com a vídeo WebM animat
(navegadors amb `canvas.captureStream` i `MediaRecorder`; si no, el botó no
surt). Els grups de feina d'una conversa reoberta després de recarregar es
relacionen amb els torns per ordre (aproximat); el selector del diàleg permet
triar-ne un altre.

**Gregal Wrapped.** Un cop per setmana la pàgina d'inici mostra una targeta
amb el més destacat dels últims set dies (amaga-la amb **Ara no** fins la
setmana que ve, o apaga-la). **Més → Estadístiques** obre la vista completa de
la setmana o el mes: torns, converses, crides d'eina per tipus, fitxers
canviats, tests que han passat a verd (seqüències de check fallit → correcte),
el bug més tossut (el torn amb més errors abans del verd), la ratxa de dies
més llarga i l'actual, el model preferit i els tokens estimats (el cost només
si el config té preus). **Baixa la imatge** desa un PNG per compartir. Tot es
calcula en local amb les converses desades i el registre d'events
(`GET /api/stats`).

**Revisió de l'equip a #retro.** En obrir #retro, els agents que han fet la
feina expliquen com l'han implementada i, en paral·lel, un equip de revisors
que no l'ha feta la mira cadascun des del seu angle: **Disseny** (aspecte,
UI/UX, accessibilitat; només si ha canviat alguna cosa visible), **Codi**
(bones pràctiques), **QA** (errors, casos límit, proves que falten),
**Funcionalitat** (si fa el que s'ha demanat) i **Seguretat**. Cadascun deixa
un veredicte d'una línia i fins a tres troballes amb gravetat (alta, mitjana,
baixa), rebuts i un arranjament que pots engegar com un torn nou amb
**Arregla-ho**. La barra de dalt compta les troballes i les filtra per
gravetat. Un revisor que no hi té res a dir calla, i un que falla o no arriba
a temps no atura la resta.

**To de #retro.** Els agents de #retro poden parlar en to *neutral*,
*amable*, *sarcàstic* o *mentor*. Tria el to per defecte a Preferències (o
deixa *El del config*, que fa servir `retro.persona` del fitxer de
configuració) i canvia'l per canal amb el selector de la capçalera de #retro.
Només canvia el to: les regles de fonamentació (rebuts, res inventat,
honestedat amb les fallades) són les mateixes.

```yaml
retro:
  persona: mentor   # neutral | friendly | sarcastic | mentor
```

**Penell.** Apagat per defecte; s'activa a **Preferències → Extres →
Penell**. Un penell petit al peu de la barra lateral mostra l'estat de
Gregal: en repòs assenyala els 45° NE (el vent gregal) i es gronxa una mica,
mentre treballa gira a ràfegues, quan hi ha una aprovació pendent apunta cap
al camp de text amb la punta en ambre, quan un torn acaba torna al NE amb un
llampec verd i, si falla, queda encallat, vermell i rovellat. Clica'l per
enviar un missatge ràpid a la sessió actual (l'esborrany del camp es
conserva); el quadre del missatge ràpid també té **Amaga**, que apaga el
penell de la barra. A l'aplicació d'escriptori, **Penell flotant
(escriptori)** obre el mateix penell en una finestreta sempre visible: un
clic curt torna a l'app i, mantenint premut i movent, l'arrossegues on
vulguis (no surt de la pantalla i, si el deixes a prop d'una vora, s'hi
enganxa). En passar-hi per sobre surten tres controls: **plega** (el deixa
en un puntet de 32 px; clica'l per desplegar-lo, o prem Esc per plegar-lo),
**enganxa a un racó** (el més proper i, després, el següent en sentit
horari) i **amaga** (apaga la preferència; es torna a activar a
Preferències). Recorda la posició i si és plegat. Amb *reduir moviment* el
penell queda quiet i només en canvien l'angle i el color.

**Veu i imatges.** Si el navegador suporta la Web Speech API (Chrome, Edge,
l'aplicació d'escriptori), el camp mostra un micròfon: mantén-lo premut per
parlar o clica un cop per començar i un altre per acabar; el text provisional
surt en directe al camp, en l'idioma de la interfície. Les captures
enganxades i les imatges deixades anar sobre el camp s'afegeixen als adjunts
normals (com a molt 3 imatges de 8 MB). En navegadors basats en Chromium el
reconeixement de veu el pot processar el servei en línia del fabricant.

## Dades i execució d'eines

Els prompts, el context de conversa i el codi o els fitxers inclosos en una
petició al model s'envien a l'endpoint configurat per al rol. Un proveïdor
anomenat `local` només és local si el servei s'executa realment a la teva
màquina; un proxy local pot reenviar les peticions a un altre lloc. La consulta
web i els connectors MCP configurats també poden fer peticions de xarxa.

`write` només crea fitxers nous o reescriu fitxers que l'agent ha llegit (o
escrit) en aquesta conversa i que no han canviat des d'aleshores. Per
substituir un fitxer existent que no ha vist, primer l'ha de llegir, o
canviar-lo amb `edit` o `patch`. És la mateixa regla que aplica Claude Code,
i evita que reescriure un fitxer sencer s'emporti els canvis que mentrestant
hi hagis fet tu, un formatador o un altre agent.

Les eines de fitxers i de shell s'executen a la màquina host dins del projecte
seleccionat. Per defecte (`sandbox.mode: off`) les úniques proteccions són
controls de l'aplicació: el classificador d'ordres de shell, les regles dures
(per exemple `sudo`, `curl` cap a una altra màquina, escriure a camins del
sistema) i les confirmacions. Les regles dures miren el programa que executa
l'ordre, no les paraules dels arguments (`grep -rn curl .` es pot fer);
`curl`/`wget` cap a `localhost`, `127.x` o `[::1]` demanen permís com
qualsevol altra ordre, tret que passin per un proxy o vagin a una shell. Aturen errors habituals, però són comprovacions de patrons, no
aïllament: una ordre aprovada, o que el classificador considera segura,
s'executa amb tots els permisos del teu usuari. Revisa els canvis i les ordres
proposats. Per executar codi no fiable, fes servir un checkout temporal, un
contenidor o una màquina virtual.

### Regles de permís: globs i per subagent

`permissions.tools` dóna una decisió per eina (`allow`, `ask`, `deny`). Per
afinar més, una eina també pot dur **patrons**, en línia o a `permissions.rules`:

```yaml
permissions:
  tools:
    bash:                        # un mapa en lloc d'una decisió = patrons
      "git *": ask               # tota ordre git pregunta...
      "git status*": allow       # ...menys aquestes (mana l'última)
      "rm *": deny
  rules:
    edit:
      "docs/**": allow
      "*.env": deny
  subagents:                     # el que canvia per a un subagent amb nom
    explorer:
      tools: {bash: deny}
```

- **Què es compara.** `bash`/`bash_background`: l'ordre, etapa per etapa
  (`&&`, `||`, `;`, `|`). Eines amb `path` (`write`, `edit`, `read`, `lsp`…):
  el camí relatiu al projecte, amb `/`; un patró sense `/` també coincideix amb
  el nom del fitxer. `web_fetch`: l'URL. La resta: només hi coincideix `*`.
- **Globs.** `*` (i `**`) coincideixen amb qualsevol cosa, també `/`; `?` amb
  un caràcter. Dins d'una eina mana l'**últim** patró que coincideix, en
  l'ordre del fitxer, i guanya la decisió simple de l'eina; sense coincidència
  s'aplica la decisió simple i després la de sempre.
- **Seguretat del shell.** Cap patró no relaxa un bloqueig dur (`sudo`,
  `rm -rf /`, camins crítics). Un `allow` només es dóna si totes les etapes
  tenen un patró `allow` i l'ordre no porta `$(...)`, cometes inverses ni
  redirecció a fitxer: `"git *": allow` no deixa passar `git log > ~/.bashrc`.
- **Modes.** Els modes de només lectura (xat, inspect, goal) ignoren els
  patrons. L'autònom no pregunta mai: un patró `ask` passa, un `deny` denega.
- **Per subagent.** `permissions.subagents.<nom>` accepta `tools` i `rules`, que
  s'apliquen damunt dels generals en aquell subagent (els definits per
  l'usuari i `explorer`): un `tools.<eina>` seu substitueix els patrons generals
  d'aquella eina, i els seus patrons van darrere i manen.

### Sandbox de shell (opcional)

La secció `sandbox` confina les eines `bash` i `bash_background` (i les
ordres `!` de la interfície de terminal) a nivell del sistema operatiu. El
classificador, les regles dures i les aprovacions continuen aplicant-se a
sobre. Les eines de fitxers (`write`, `edit`, `patch`), els hooks, els
servidors MCP i el navegador no queden dins del sandbox.

```yaml
sandbox:
  mode: workspace      # off (per defecte) | workspace | strict
  network: "off"       # on (per defecte) | off
  allow_write: []      # camins extra on es pot escriure (~ i $VARS s'expandeixen)
  backend: auto        # auto | bwrap | landlock | sandbox-exec | docker
  image: alpine:3.20   # només per al backend docker
```

- `workspace`: només es pot escriure al workspace de l'ordre, al directori
  temporal, a la memòria cau del teu usuari (perquè les compilacions i els
  gestors de paquets continuïn funcionant) i a `allow_write`. Si no hi ha cap
  backend, Gregal avisa un cop (a la sortida de l'eina i a `gregal --doctor`)
  i executa les ordres **sense sandbox**.
- `strict`: com `workspace`, sense la memòria cau de l'usuari i amb un `/tmp`
  privat a Linux. Si no hi ha cap backend, o el backend no pot tallar la
  xarxa quan es demana `network: off`, les ordres de shell es deneguen.

Què garanteix cada backend:

| Backend | Plataforma | Sistema de fitxers | Xarxa tallada | Notes |
|---|---|---|---|---|
| `bwrap` (bubblewrap) | Linux, primera opció d'`auto` | tot el sistema de fitxers en només lectura, binds d'escriptura per als camins permesos, `/dev` i `/proc` nous, espai de noms de PID propi | sí (`--unshare-net`) | cal el binari `bwrap` i espais de noms d'usuari sense privilegis |
| `landlock` | Linux 5.13+, l'usa `auto` si no hi ha `bwrap` | escriptures limitades als camins permesos; la lectura no es limita | només TCP, i només amb l'ABI 4 de Landlock (Linux 6.7+); UDP no queda cobert | l'aplica un procés auxiliar `gregal __sandbox-exec` |
| `sandbox-exec` | macOS, el tria `auto` | escriptures limitades als camins permesos; la lectura no es limita | sí (els sockets Unix continuen permesos) | Apple el marca com a obsolet però encara l'inclou |
| `docker` | qualsevol sistema; l'única opció a Windows; mai no es tria sol | del host només es munten el workspace i `allow_write` (mateix camí; `C:\x` passa a `/c/x`) | sí (`--network none`) | les ordres corren dins d'`image` amb les seves eines, no les del host; `strict` hi afegeix arrel de només lectura, `/tmp` en tmpfs i cap capability |

A Windows no hi ha backend automàtic: posa `backend: docker` amb Docker en
marxa. Sense això, `workspace` executa sense sandbox amb un avís i `strict`
denega les ordres de shell. Cap backend no limita CPU, memòria ni temps més
enllà dels límits de les eines, i cap no amaga els secrets que es poden llegir
dins del workspace o (llevat de docker) a qualsevol lloc que el teu usuari
pugui llegir. El backend actiu surt a `gregal --doctor`, a `/api/state`
(`sandbox`) i al peu del menú de permisos del compositor web.

### Cerca web i lectura de pàgines

`web_search` pregunta primer a [Exa](https://exa.ai), pel seu servei MCP
públic (el mateix que fa servir OpenCode): no cal compte ni clau, la cerca és
semàntica i cada resultat ja porta el text de la pàgina, així que sovint
l'agent no l'ha d'obrir. Un SearXNG propi (`SEARXNG_URL`) hi va en paral·lel.
DuckDuckGo i Bing, que Gregal llegeix raspant, només són la reserva quan
aquells no troben res.

Sense clau, Exa té un límit diari per IP. Amb clau no n'hi ha: defineix
`EXA_API_KEY`, o desa-la com a `exa` a `keys:` de `~/.config/gregal/auth.yaml`.
La clau viatja en una capçalera, mai a la URL.

Un motor que bloqueja (captcha, HTTP 403 o 429, el límit gratuït d'Exa) queda
en pausa 15 minuts, el doble cada cop seguit fins a 2 hores, o el que digui el
seu `Retry-After`, i la cerca fa servir els altres. Si tots estan en pausa,
l'eina diu a l'agent que no ho torni a provar en aquell torn. Els resultats de
Bing que no comparteixen cap paraula amb la consulta es descarten.

`web_fetch` llegeix la pàgina directament. Si falla, prova alhora Exa (la seva
còpia de la pàgina), el Wayback Machine i, si `FIRECRAWL_URL` està definit, un
render de Firecrawl; després, archive.today. Les pàgines que no arriben en viu
porten etiqueta. Si el lloc rebutja els agents (HTTP 403/429 o una comprovació
anti-robots) i ningú més en té còpia, l'agent rep l'avís clar de no tornar-ho
a provar amb `web_fetch` ni amb el navegador; l'eina del navegador afegeix el
mateix avís quan cau en una comprovació anti-robots.

### Navegador de l'agent

L'eina `browser` condueix una pàgina Chromium de debò (JavaScript, apps locals
a `localhost`, llocs on has iniciat sessió). On corre ho decideix
`browser.mode`:

```yaml
browser:
  mode: auto   # auto (per defecte) | app | headless | window | chrome
```

| Mode | Comportament |
| --- | --- |
| `auto` | `app` si l'app d'escriptori ha engegat aquest backend i hi ha registrat el pont del navegador; si no, `headless` |
| `app` | s'obre el panell de navegador de la mateixa app al costat de la conversa i l'agent el condueix; ho veus en directe i hi pots clicar o escriure quan vulguis. Sense pont torna a `headless` i el resultat de l'eina ho diu |
| `headless` | el Chrome o l'Edge de la màquina sense finestra, amb el perfil propi del Gregal (`~/.config/gregal/browser`) |
| `window` | el mateix navegador amb finestra visible; útil per iniciar sessió un cop en un lloc dins del perfil del Gregal |
| `chrome` | el teu Chrome, amb les teves sessions, a través de l'extensió de Gregal (vegeu més avall). Si no està connectada, l'eina diu com activar-la |

`GREGAL_BROWSER_HEADLESS=1` força `headless` digui el que digui el config.
A l'app d'escriptori el pont és un punt HTTP només de loopback que serveix
l'app, amb port i token aleatoris que passa al backend per l'entorn quan
l'engega; només accepta ordres DevTools de pàgina i mai navega a l'adreça del
mateix backend. Una app connectada a un backend que no ha engegat ella
(`GREGAL_URL` o un servei que ja corria) no té pont i l'agent fa servir
`headless`. El panell de l'agent comparteix galetes amb la pàgina Navegador de
l'app. Les captures també surten a la targeta de l'eina del web.

**Com veu i actua l'agent.** `read` torna la pàgina com un arbre
d'accessibilitat: una línia per element amb el rol, el nom i l'estat, i una
ref com `[ref=e12]` a tot allò on pot actuar. La ref va lligada al node de la
pàgina i val mentre l'element hi sigui. `click`, `type`, `select`, `hover`,
`drag`, `upload` (un fitxer del projecte a un camp de fitxer) i `scroll` fan
servir la ref; `press` accepta tecles i combinacions (`Enter`, `Control+a`);
`wait` espera un text, que en desaparegui un, o que la pàgina s'aturi; `click`
amb `x`/`y` clica píxels d'una captura (apps de llenç). Cada acció torna només
el que ha canviat. `batch` fa fins a 15 passos en una crida. `console` i
`network` mostren errors i peticions; `tabs`, `tab_new`, `tab_select` i
`tab_close` gestionen pestanyes, i un clic que n'obre una hi porta l'agent.
`alert`/`confirm`/`prompt` es responen sols (cancel·lar per defecte) i es diu;
`dialog accept:true` accepta el següent.

**Permisos.** Llegir amb el navegador propi del Gregal corre directament. La
primera acció que canvia alguna cosa en un web nou (clicar, escriure, triar…)
pregunta un cop; després el lloc queda permès durant la sessió (`localhost` mai
pregunta). Clicar una cosa que envia, paga, compra, esborra o publica, i
acceptar un diàleg de la pàgina, pregunten sempre, també amb aprovació
automàtica o autonomia total. El Gregal mai escriu en camps de contrasenya,
targeta o codis d'un sol ús: ho escrius tu. En mode xat o inspecció no es pot
actuar mai.

**El teu Chrome.** Al panell lateral de l'extensió de Gregal, «Deixa que
Gregal faci servir aquest navegador» demana a Chrome el permís de depuració en
aquell moment i es connecta a l'app d'escriptori (un websocket local que només
accepta l'extensió). Després demana-li al Gregal que faci una cosa «al meu
Chrome»: l'agent treballa en pestanyes del grup «Gregal», amb les teves
sessions, i només allà (arrossega-hi una pestanya per compartir-la). Cada web
nou pregunta primer, també amb aprovació automàtica. Chrome mostra la seva
barra «està depurant aquest navegador»; Atura al panell o Cancel·la a la barra
ho acaben, i l'agent no s'hi torna a enganxar fins que ho tornis a activar.
`browser.mode: chrome` fa que el teu Chrome sigui el de per defecte.

### Mode de permís amb revisor automàtic

En mode revisor automàtic (`auto-review`), quan una crida d'eina necessita
aprovació, la decideix un model revisor ràpid en comptes de preguntar-te. Fa
servir el rol `reviewer`, o `chat` si no n'hi ha. Rep la tasca, el context
recent, les regles del projecte (`AGENTS.md`, `.gregal/memory.md`) i la crida
exacta: el diff d'una edició o l'ordre d'una crida de shell. Respon `allow`,
`deny` o `ask`:

- `allow`: la crida s'executa. L'activitat i la targeta de l'eina mostren
  «Revisor ha aprovat: motiu».
- `deny`: la crida es bloqueja i el model rep el motiu.
- `ask`, un temps esgotat (`permissions.review_timeout_s`, per defecte 30 s),
  un error o una resposta il·legible: apareix la confirmació habitual amb el
  motiu del revisor.

El revisor només veu les crides que altrament preguntarien. Les denegades per
les regles dures, pel classificador o per `permissions.tools: deny` no hi
arriben mai, i per tant no les pot aprovar. En mode autònom no hi intervé:
allà no es pregunta res. El revisor també és un model i es pot equivocar o
ser manipulat pel contingut del context: redueix les preguntes, no és una
barrera de seguretat. Combina'l amb el sandbox per a la feina desatesa.

Activa'l amb `permissions.mode: auto-review` al config (web, terminal,
execucions no interactives i Telegram), des del menú de permisos de la web
(**Revisor de permisos** a les Preferències de la web o l'Android, només
aquesta sessió) o amb `/autoreview` a la
interfície de terminal.

El mode autònom no demana permís mai: decideix ell i ho fa tot sol. El que
altres modes preguntarien hi passa sense esperar ningú. Només continua
bloquejat el que està prohibit: les ordres que el classificador denega (`sudo`,
descàrregues passades a un shell, formatar discos, esborrar l'arrel…) i les
regles `deny` de `permissions.tools`. Per impedir alguna cosa en mode autònom,
posa-la a `deny`; un `ask` del config no hi fa preguntar.

### Execucions no interactives (`-p`)

`gregal -p "<tasca>"` fa un torn sense interfície:

- `--dir <camí>` fixa el workspace (eines, mapa del projecte, verificació).
  Sense, es fa servir el directori actual. En mode `code`, les escriptures
  dins del workspace no necessiten `--auto-approve`; la resta d'accions que
  pregunten sí.
- `--max-steps N` (1-200) és un límit dur: en arribar-hi l'agent resumeix el
  que ha fet i el que queda, i la sortida JSON porta `"exhausted": true`.
  Sense el flag val `agent.max_steps` del config, i l'agent pot demanar més
  passos mentre avança.
- El mode autònom treballa sol i no pregunta res: tot el que necessita fer
  ho fa, i `question` queda desactivada (l'agent explica les seves
  suposicions). Només el que està prohibit (classificador i regles `deny`)
  continua bloquejat. `--mode autonomous` no necessita `--auto-approve` i no
  s'hi pot combinar.
- No s'ofereixen les eines que necessiten una persona (`question`, `preview`,
  `office_open`, `browser`). Si l'agent necessita una decisió, s'atura i la
  pregunta a la resposta final en lloc d'endevinar.
- `--output json` sempre imprimeix un objecte JSON, amb `"error"` si el torn
  falla. Amb `--verify`, `"verified"` hi és sempre i s'hi inclouen els passos,
  les eines i el trace de totes les rondes de reparació.
- `--output stream-json` imprimeix una línia JSON a mesura que passen les
  coses (`step`, `tool_call`, `tool_result` amb la sortida tallada a 4000
  caràcters, `warning`, `error`, `diff`, `note`) i, al final, el mateix
  objecte que `--output json` amb `"type": "result"`. El progrés continua
  anant a stderr.
- Cada tirada es desa com una sessió, a la mateixa llista que les del TUI i
  la web, i el nom surt per stderr (`"session"` al JSON). `-c`
  (`--continue`) continua la sessió més recent desada en aquesta carpeta, ja
  sigui del `-p`, del TUI o de la web; `--resume <nom>` en continua una de
  concreta. `--no-session` no desa la tirada.
- El que arriba per stdin s'afegeix a la tasca, fins a 256 KB:
  `git diff | gregal -p "revisa aquest diff"`. Amb `-p -` la tasca sencera es
  llegeix de stdin.
- `--model proveidor/model` (o només un model del proveïdor del rol code) fa
  servir aquest model per als rols code, chat i think de la tirada, sense
  canviar el config.

Codis de sortida: `0` correcte, `1` el torn ha fallat (proveïdor, configuració
o cancel·lació), `2` flags invàlids (es comproven abans de cap crida al model),
`3` la comanda de `--verify` continua fallant després de l'última ronda (o, amb
`--goal`, l'objectiu no s'ha confirmat).

### Objectius en bucle (`--goal`)

`gregal --goal <id|last|"text">` executa un objectiu fins que un verificador
independent el confirma amb proves, en lloc de creure's el «fet» de l'agent.
S'executa en mode autònom si no hi poses `--mode`. A cada ronda: l'agent
treballa; corren les comprovacions (`--verify` o `agent.autonomous.verify`; si
en falla una, la sortida guia la ronda següent i no es gasta cap verificador);
un verificador de només lectura (rol `reviewer`) mira els fitxers de debò, amb
el diff i la sortida davant, i contesta criteri per criteri amb prova. Només es
dóna per complert amb `VERDICT: MET` i tots els criteris `MET` amb prova.
S'atura, amb un resum parcial, per `--goal-rounds` (4, màx. 10), `--max-steps`
(per ronda; total 300), `--goal-minutes` (60), `--goal-cost` o si dues rondes
seguides deixen el projecte igual. Les fitxes d'objectiu del terminal i la web
encara llancen un torn normal.

Els subagents utilitzen el projecte i la sessio actius, hereten les restriccions
configurades i s'aturen quan es cancella el torn pare. Les ordres de segon pla
iniciades per eines de l'agent pertanyen al torn i s'aturen quan acaba o es
cancella; els processos iniciats manualment al terminal son independents.

Aprovar una ordre shell o permetre explicitament una eina dona acces real amb
els privilegis de l'usuari del backend. Els permisos d'escriptura dins del
projecte no limiten les ordres shell, la xarxa ni les lectures a aquest projecte.
No executis repositoris no fiables ni tasques desateses amb permisos amplis en
una maquina amb dades sensibles; utilitza un entorn aillat.

## Git: checkpoints, worktrees, commits i pull requests

**Checkpoints.** En un repo git, gregal fa una foto de tot el working tree
(fitxers seguits i fitxers nous no ignorats) a l'inici de cada torn i abans de
cada ordre de shell que no és de només lectura. Les fotos són commits en refs
amagats (`refs/gregal/checkpoints/<sessió>/<n>`) escrits amb un índex
temporal: HEAD, el teu índex, les branques i l'stash no es toquen mai. Desfer
(`/rewind`, el botó Desfés de la web) ara també desfà el que han canviat les
ordres de shell; `/checkpoints` llista les fotos com a `gN` i `/rewind gN` (o
`/api/rewind-to {"git": N}`) hi torna: es restauren els fitxers que difereixen
i s'esborren els creats després que no són ignorats. Els ignorats no es toquen
mai. Fora de git, el journal de canvis funciona com sempre. Configuració:
`git.checkpoints: on|off` (per defecte on) i `git.checkpoint_keep` (refs que es
guarden per repo, per defecte 50). No es fa foto si hi ha massa fitxers nous
(més de 5000, o un de més de 50 MB): afegeix-los al `.gitignore`.

**Sessions aïllades (worktrees).** A les pestanyes de sessió de la web, `⎇＋`
obre una sessió nova a `<repo>/.gregal/worktrees/<nom>` en una branca nova
`gregal/<nom>` des de HEAD. Es copien els fitxers d'un `.worktreeinclude` a
l'arrel del repo (un glob per línia, p. ex. `.env`). La pestanya duu la marca
`⎇ nom`. A Canvis, **Porta al repo principal…** ensenya el diff del worktree i
l'aplica com a pegat al working tree principal (sense commit) o fusiona la
branca (només amb el principal net i la feina en commits; amb conflictes
s'avorta). Res no es força; treure un worktree amb canvis sense commit es
refusa. Al terminal: `/worktree new <nom>`, `/worktree list`,
`/worktree diff|apply|remove <nom>`, `/worktree exit`. Headless i TUI:
`--worktree <nom>` (el crea si cal o el reprèn).

**Commits i pull requests.** L'agent té dues eines que sempre demanen permís i
que es deneguen en els modes xat, consulta i objectiu: `git_commit` (per
defecte fa commit només dels fitxers canviats en aquesta sessió, bash inclòs;
el que tinguis staged en altres fitxers queda fora) i `gh_pr_create` (fa servir
`gh`; a la branca per defecte primer crea una branca `gregal/<títol>`, fa push
sense forçar i obre la PR). La pàgina Canvis té els diàlegs **Fes commit…** i
**Crea una PR…**, amb un botó **Suggereix** que redacta el missatge a partir del
diff amb el rol chat. Hi ha una GitHub Action reutilitzable per revisar PRs:
vegeu [github-action.md](github-action.md).

## Mode depuració, Prova N maneres, crític del pla i apostes

**Mode depuració.** Tria **Depuració** al menú ⋯, `/debug`,
`/mode debug` al terminal o `--mode debug` amb `-p`. L'agent treballa com en
mode code (mateixa política d'escriptura) amb un contracte de depuració: escriu
2-4 hipòtesis ordenades, afegeix instrumentació temporal mínima on cada línia
porta la marca `GREGAL-DEBUG`, executa ell mateix la reproducció si li dones
una ordre o n'hi ha una d'òbvia (si no, et demana que reprodueixis i s'atura),
llegeix l'evidència, arregla la causa, treu tota la instrumentació i passa les
comprovacions. La resposta final diu de cada hipòtesi si s'ha confirmat o
descartat i amb quina evidència. Gregal n'imposa la neteja: abans de tancar
el torn busca línies `GREGAL-DEBUG` noves al workspace i torna l'agent a la
feina perquè les tregui (dues vegades com a molt; després tanca amb un avís).
Els projectes que ja contenen la marca no en surten afectats: només compten
les línies afegides durant el torn.

**Prova N maneres (best-of-N).** Al menú `⋯` de la web, **Prova N maneres** agafa
la tasca del compositor, demana N (2-4) i, opcionalment, un model per intent
(un nom de rol o `provider/model`). Gregal crea N worktrees des de HEAD
(`.gregal/worktrees/bestof-<id>-<k>`), hi executa la mateixa tasca a través de
la cua d'execucions compartida, passa la verificació configurada
(`agent.autonomous.verify`) a cada worktree i demana al rol reviewer una nota
de 0 a 10 amb rúbrica. La targeta de comparació mostra fitxers i línies
canviades, comprovacions, nota i cost de cada intent i marca el recomanat.
**Aplica** porta l'intent triat al working tree principal com a pegat (sense
commit) i treu els altres worktrees; **Descarta-ho tot** els treu tots. Els
intents corren sense interacció: les accions que demanen permís només passen
si la sessió és permissiva. Headless:
`gregal -p "tasca" --best-of 3 [--best-of-models code,openai/gpt-x] [--verify "go test ./..."]`
imprimeix la comparació i deixa els worktrees perquè els apliquis o els
treguis; amb `--best-of-apply` aplica directament el recomanat. La
recomanació prefereix primer les comprovacions en verd, després les **proves
creuades** (els tests que ha escrit cada intent es passen contra el codi dels
altres: guanya l'arranjament que també passa els tests dels rivals), després
els **vots** (intents que arriben al mateix pegat es reforcen), i després la
nota, el diff més petit i el cost més baix.

**Crític del pla.** Quan **Pla** proposa un pla, el rol reviewer el revisa
contra la tasca i el mapa del projecte i en llista els passos que falten, els
arriscats i les suposicions no comprovades. La targeta del pla mostra
**Crític: N objeccions** (desplegable); executar el pla passa les objeccions a
l'agent, i en mode autònom l'agent revisa el pla una vegada amb elles abans
d'executar-lo. Les tirades autònomes també passen pel crític la primera
llista de fites. `plan.critic: off` l'apaga.

**Apostes de confiança.** Abans d'executar una comprovació (tests, build, vet,
lint) l'agent diu amb quina probabilitat creu que passarà (paràmetre `bet` de
`bash`). Gregal la compara amb el resultat, en mostra un xip a la fila de
l'eina (`aposta 80% ✓`), l'afegeix a l'evidència del #retro i la desa a
`<data>/bets.jsonl`. `GET /api/calibration?scope=session|project|all` en torna
el resum de calibració (Brier, probabilitat mitjana prevista, taxa d'èxit,
franges). `agent.bets: off` les apaga; `agent.bets: side` també demana una
probabilitat al rol chat quan el model no n'ha donat (una crida petita de més
per comprovació).

## Hooks del cicle de vida

A més de `hooks.post_edit` (una ordre que corre després de cada escriptura o
edició, amb `{file}` substituït) i `hooks.diag`, la secció `hooks:` accepta
ordres per a sis events del cicle de vida. Funcionen a tots els clients
(terminal, web, escriptori, headless, Telegram) perquè viuen al motor compartit:

```yaml
hooks:
  post_edit: "gofmt -w {file}"
  session_start:
    - command: ./scripts/context.sh        # el stdout s'afegeix al primer prompt
  user_prompt:
    - command: ./scripts/check-prompt.sh   # exit 2 bloqueja el torn
  pre_tool:
    - command: ./scripts/guard.sh
      matcher: "bash|write|edit|patch"    # regex ancorada sobre el nom de l'eina
      timeout_s: 10
  post_tool:
    - command: ./scripts/lint-note.sh
      matcher: "write|edit"
  stop:
    - command: ./scripts/tests-green.sh    # pot demanar que l'agent continuï
  pre_compact:
    - command: ./scripts/backup-transcript.sh
  stop_max_continues: 3
```

Cada ordre corre amb l'intèrpret del sistema al workspace de la sessió, amb
`GREGAL_EVENT`, `GREGAL_SESSION`, `GREGAL_WORKSPACE` i `GREGAL_TOOL` a l'entorn,
i rep un JSON per stdin: `event`, `session`, `workspace`, `mode` i, segons
l'event, `tool` + `args` (pre_tool, post_tool), `result`/`error` (post_tool),
`prompt` (user_prompt, stop), `answer` i `stop_count` (stop), `messages` i
`tokens` (pre_compact).

| Sortida | Què vol dir |
|---|---|
| exit 0, stdout en text | Context per al model: s'afegeix al prompt (session_start, user_prompt) o al resultat de l'eina com a `[hook] …` (post_tool). |
| exit 0, stdout JSON | `{"decision":"allow\|deny\|ask","reason":"…","continue":true,"context":"…"}`. |
| exit 2 | Bloqueja: el motiu és stderr (o stdout). A `stop`, exit 2 vol dir «continua». |
| altra sortida, temps exhaurit | Avís que no bloqueja; l'acció continua. |

- `session_start` corre al primer torn d'una conversa nova.
- `user_prompt` pot afegir context o bloquejar el torn (el motiu surt com a
  error del torn).
- `pre_tool` corre després de la política de permisos: `deny` bloqueja la crida
  (la UI la mostra com a eina bloquejada i el model en rep el motiu), `ask`
  força una aprovació fins i tot per a eines que anirien directes (una
  aprovació recordada no la salta) i `allow` se salta la confirmació de la
  política. Cap hook no pot desfer una denegació de la política. Els subagents
  també apliquen pre_tool; allà `ask` bloqueja perquè ningú no pot respondre.
- `post_tool` pot enganxar una nota per al model al resultat de l'eina.
- `stop` corre abans que el torn acabi amb una resposta; `{"continue":true,"reason":…}`
  torna el motiu a l'agent com una instrucció nova, com a molt
  `stop_max_continues` vegades per torn (3 per defecte). No corre a les
  síntesis forçades (pressupost de passos exhaurit).
- `pre_compact` corre abans de resumir l'historial. No pot aturar la
  compactació.

El temps per defecte és de 10 s (`timeout_s`, fins a 600). Cancel·lar el torn
(Esc, Atura) mata el hook i els seus fills. Els hooks són ordres teves i de
confiança, com `post_edit`: corren amb els teus permisos. Al terminal,
`session_start`, `user_prompt`, `pre_tool` i `stop` corren síncrons dins del
motor: un hook lent atura la pantalla fins que acaba o se li acaba el temps.

## Connectors MCP

Gregal es connecta a servidors Model Context Protocol de dues maneres:

```yaml
mcp:
  files:                         # servidor local per stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/home/jo/notes"]
  linear:                        # servidor remot: Streamable HTTP
    url: https://mcp.linear.app/mcp
  intern:
    url: https://mcp.example.com/sse
    transport: sse               # HTTP+SSE antic (2024-11-05)
    headers: {X-Api-Key: "${INTERNAL_MCP_KEY}"}
  github:
    url: https://api.example.com/mcp
    oauth: {client_id: el-meu-client, scopes: [repo]}
```

- `transport` és opcional: `command` vol dir stdio i `url`, Streamable HTTP; si
  el servidor respon el primer POST amb 400/404/405, Gregal prova el transport
  SSE antic. Les capçaleres admeten `${VAR}`: s'expandeixen en connectar i es
  desen sense expandir.
- **OAuth 2.1.** Quan un servidor remot respon 401, Gregal en descobreix el
  servidor d'autorització (metadades del recurs protegit i del servidor
  d'autorització), s'hi registra dinàmicament si ho ofereix (si no, posa
  `oauth.client_id`) i fa servir el flux de codi d'autorització amb PKCE (S256)
  i el paràmetre `resource`. Els tokens es desen a
  `~/.local/share/gregal/mcp-oauth.json` (o `$GREGAL_DATA_DIR`) amb permisos
  0600 i es refresquen sols. L'arrencada no obre mai el navegador: un servidor
  que demana autorització surt com a **Cal autoritzar**.
  - Web i escriptori: pàgina **Connectors / MCP** → **Connecta**. La pàgina
    d'autorització s'obre al navegador i torna a `/api/mcp/oauth/callback` del
    servidor local; la pàgina s'actualitza quan el servidor ja és connectat.
  - Terminal: `/mcp connect <nom>` pinta l'URL que cal obrir; un servidor
    temporal a `127.0.0.1` rep el callback. `/mcp disconnect <nom>` n'oblida el
    token.
- **Importar d'altres eines.** `gregal mcp import` (i `/mcp import` al terminal)
  llegeix els servidors MCP de Claude Code (`.mcp.json`, `~/.claude.json`),
  Codex (`~/.codex/config.toml`) i opencode (`opencode.json[c]`), ensenya què
  s'afegiria i no escriu res fins que ho confirmes (`--from`, `--only`,
  `--dry-run`, `--yes`; al terminal `/mcp import yes [noms]`). Els desactivats,
  els noms ja existents amb una altra definició i els repetits es salten i es
  llisten. Els secrets no surten mai: la vista prèvia els mostra com a `***`.
  Els valors del fitxer d'origen es copien tal qual a `config.yaml` (0600), però
  les referències continuen sent referències (`bearer_token_env_var` de Codex
  passa a `Authorization: Bearer ${VAR}`) i el secret OAuth d'opencode no
  s'importa.
- Les **eines** surten com a `mcp_<servidor>_<eina>` i per defecte demanen
  confirmació (`permissions.tools` ho canvia). Un servidor marcat
  `readonly: true` declara que les seves eines només llegeixen: s'executen
  sense demanar permís, també als modes de només lectura (xat, objectiu,
  consulta) i en mode autònom. Una entrada explícita de `permissions.tools`
  continua manant. Posa-ho només a servidors que saps que només llegeixen.
- **Recursos.** Si un servidor connectat n'exposa, l'agent té
  `mcp_resource_list` (`server` opcional) i `mcp_resource_read` (`server`,
  `uri`).
- Els **prompts** són ordres `/mcp__<servidor>__<prompt>` al terminal (amb
  compleció) i a la paleta d'ordres de la web. Al terminal els arguments van
  després de l'ordre, en ordre o com a `clau=valor`; la paleta web els demana
  un per un i deixa el prompt expandit al compositor perquè el revisis.
- La pàgina **Connectors / MCP** mostra el transport, l'estat, l'autorització i
  quantes eines i prompts té cada servidor, amb els botons Connecta/Reconnecta,
  Desconnecta i Treu i un formulari per afegir-ne un (URL o ordre local). Afegir
  i treure escriuen `config.yaml` i, si el servidor té usuaris, només ho pot fer
  l'administrador.

## Gregal com a servidor MCP

`gregal mcp serve` fa de Gregal un servidor MCP per stdio, perquè un altre
client MCP li encarregui feina. Exposa tres eines:

- `gregal_run` — `task` (obligatori), `mode`, `workspace` (camí absolut; per
  defecte, on s'ha arrencat el servidor o `--workspace`), `auto_approve`,
  `max_steps`, `verify`. Fa un torn headless (com `gregal -p`) i torna un JSON
  amb `answer`, `changed_files` (de `git status` abans i després; fora de git,
  de les eines d'escriptura), `steps`, `tools_used`, `exhausted`, `verified` i
  `session` (el torn es desa com a conversa). Els torns van d'un en un; una
  cancel·lació MCP atura el torn.
- `gregal_sessions_list` — converses desades (`limit`, `workspace`).
- `gregal_session_get` — una conversa (`name`, `max_messages`).

Per afegir-lo a Claude Code:

```sh
claude mcp add gregal -- gregal mcp serve --workspace /camí/al/projecte
```

Altres clients (Cursor, Claude Desktop, un altre Gregal) fan servir el JSON de
sempre:

```json
{"mcpServers": {"gregal": {"command": "gregal", "args": ["mcp", "serve", "--config", "/home/jo/.config/gregal/config.yaml"]}}}
```

o, al `config.yaml` del mateix Gregal, `mcp: {gregal: {command: gregal, args: [mcp, serve]}}`.
Sense `auto_approve`, les accions que demanen aprovació es deneguen, igual que
a `gregal -p`; el mode `autonomous` no es pot combinar amb `auto_approve`. El
protocol fa servir stdout; l'avenç va a stderr.

## Base de dades de demo

Gregal porta una base de dades de demostració minúscula i determinista per
provar eines MCP, consultes i dashboards sense connectar res teu. Descriu un
comerç europeu fictici, Northwind Outfitters, amb les dates ancorades a
l'1-10-2026 (les comandes cobreixen els 24 mesos anteriors), de manera que els
resultats sempre són els mateixos:

| Taula | Files | Contingut |
|---|---|---|
| `regions` | 8 | nom, país, responsable |
| `customers` | 800 | segment (Consumer, SMB, Enterprise), regió, data d'alta |
| `products` | 60 | categoria, preu, cost, data de llançament |
| `orders` | 6000 | data, client, producte, canal, estat, promoció, quantitat, descompte, ingressos, cost, marge (amb estacionalitat, creixement i promocions) |
| `marketing_spend` | 120 | despesa, impressions, clics i conversions mensuals per canal |
| `support_tickets` | 1500 | prioritat, estat, categoria, hores de resolució, CSAT |

`gregal mcp demo` la serveix com a servidor MCP de només lectura per stdio. Es
genera en memòria en arrencar (pocs MB, pocs mil·lisegons) i no toca cap
fitxer. Eines:

- `list_tables` — taules amb el nombre de files, descripció i unions.
- `describe_table` (`table`) — columnes, tipus, taules unibles i cinc
  files d'exemple.
- `query` — una consulta estructurada, no SQL: `table`; `select` amb
  columnes o agregats `sum`, `avg`, `min`, `max`, `count(*)`, `count(col)`,
  `count_distinct(col)`, cadascun amb `… as nom` opcional; `filters`
  `[{col, op, value}]` amb `=`, `!=`, `>`, `>=`, `<`, `<=`, `in`, `not_in`,
  `between`, `contains`; `group_by` amb columnes o períodes `day()`, `week()`,
  `month()`, `quarter()`, `year()`; `order_by` com `"revenue desc"`; `limit`
  (100 per defecte, màxim 200). Les columnes de taules relacionades porten punt
  (`customers.segment`, `products.category`, `regions.name`) per un conjunt fix
  d'unions: comandes i tiquets → clients i productes, clients → regions. El
  resultat és JSON `{columns, rows, total_rows, truncated}`.

També exposa el recurs `demo://schema` (l'esquema en Markdown) i els prompts
`/mcp__demo__sales_dashboard` i `/mcp__demo__support_kpis`.

A la web i a l'app d'escriptori, **Connectors / MCP → Afegeix la base de dades
de demo** l'afegeix amb un clic (ordre: l'executable de Gregal que corre, amb
`mcp demo`, `readonly: true`), i la targeta de l'inici **Prova les dades de
demo** (també a la biblioteca buida de **Plots i dashboard**) l'afegeix si cal
i demana a l'agent un dashboard de vendes: ingressos mensuals, ingressos per
categoria i per regió i tiquets de suport per prioritat, com a gràfics
`gregal-plot`. Desa'ls amb **Afegir al dashboard**; un dashboard anomenat
«Demo sales» els manté junts. El connector es treu com qualsevol altre. Al
`config.yaml` queda així:

```yaml
mcp:
  demo:
    command: /camí/a/gregal
    args: [mcp, demo]
    readonly: true
```

## Subagents, equip amb eines, preguntes al marge i memòria

### Subagents definits per l'usuari

Posa fitxers Markdown amb capçalera YAML a `.gregal/agents/*.md` (projecte) o
`~/.config/gregal/agents/*.md` (usuari; amb el mateix nom mana el projecte):

```markdown
---
name: test-runner
description: Corre els tests i informa de les fallades amb fitxer i línia
tools: [read, grep, glob, bash]
mode: inspect        # inspect (defecte, només lectura) o code (pot escriure)
max_steps: 15        # defecte 12, màxim 40
model: openrouter/qwen3-coder   # opcional: proveïdor/model o només el model
# role: chat         # opcional: un rol configurat
---
Corre primer l'ordre de test més acotada. Informa només de les fallades.
```

L'agent veu la llista (nom i descripció) al prompt de sistema i pot passar
`agent: test-runner` en una tasca de `delegate`. Els subagents en mode code
només escriuen si la sessió treballa en un worktree aïllat (`--worktree`,
pàgina de Canvis) o si aproves cada canvi (la targeta d'aprovació de sempre a
la web); si no, queden en només lectura. Al terminal no hi ha aprovació per a
subagents: fora d'un worktree són de només lectura. Els topalls continuen sent
4 per crida i 8 vius. `/agents` al terminal i la pàgina Agents (pestanya
Equip, «Agents definits») els llisten; el taller mostra el nom del subagent.

### Equip amb eines

A la pàgina Agents, pestanya Equip, «Els rols fan servir eines» (activat per
defecte, el navegador ho recorda) deixa treballar els rols sobre el projecte:
l'investigador llegeix fitxers i el web, el revisor també fa comprovacions de
només lectura (`bash` en mode consulta) i el constructor edita amb la política
de permisos de la sessió. Les accions que demanen permís surten com una
targeta d'aprovació a la mateixa pàgina. El coordinador continua sense eines.
Un node del flux pot referenciar un subagent definit (camp «Subagent» de
l'editor) per fer servir el seu model, les seves eines i el seu prompt.

### Preguntes al marge i resums

Mentre un torn treballa, escriu `/btw <pregunta>` al composer (web o
terminal): el rol de xat respon a partir d'una còpia del transcript i de la
traça viva en una targeta al marge. El torn principal no s'atura i la
pregunta no entra a l'historial. `/recap [N]` resumeix les últimes N
peticions (3 per defecte). A la web, quan tornes a una sessió amb activitat
que no has vist, un botó «Resumeix» explica què ha passat mentre no hi eres.

### Memòria del projecte

Després d'un torn de codi que ha aplicat edicions, Gregal pot proposar fins a
dues notes duradores del projecte (targeta «Ho recordo?» a la web; `/keep
[n|all|none]` al terminal). Només les acceptades s'escriuen a
`.gregal/memory.md`. Preferències > Memòria del projecte llista les notes i
«Endreça les notes» proposa una llista consolidada (duplicats fusionats,
notes velles fora) i n'ensenya el diff abans d'escriure. Des de la línia
d'ordres: `gregal memory tidy [--dry-run] [--yes] [--dir ruta]` ensenya el
diff i pregunta abans d'escriure.

## Documents d'Office

Gregal crea, edita i revisa fitxers de Word, PowerPoint i Excel (`.docx`,
`.pptx`, `.xlsx` i les plantilles `.dotx`, `.potx`, `.xltx`) sense
necessitar Python ni Office:

- **Word**: el markdown es converteix en estructura de Word de debò (estils
  de títol, llistes numerades i de pics, taules amb capçalera, cites, codi,
  enllaços, imatges, índex, salts de pàgina), amb portada, capçalera i
  números de pàgina. L'edició conserva el format de cada fragment, també a
  capçaleres, peus, notes i comentaris. Els canvis es poden fer amb
  **control de canvis** (`track: true`; només es marquen les paraules que
  canvien) i amb **comentaris**.
- **PowerPoint**: un tema 16:9 propi amb nou layouts (portada, secció,
  contingut, dues columnes, comparació, xifra gran, imatge, només títol, en
  blanc), gràfics natius editables, taules amb estil i notes de l'orador.
  Cinc temes (`default`, `modern`, `nature`, `elegant`, `mono`) amb colors
  personalitzables.
- **Excel** (amb excelize): fórmules en lloc de valors, formats de nombre,
  colors de rol (entrada, fórmula, enllaç, hipòtesi clau, total), capçaleres
  fixades, taules, formats condicionals, llistes desplegables, comentaris i
  gràfics natius. `recalc` llista tots els errors de fórmula.
- **Plantilles**: un `.dotx`/`.docx` corporatiu conserva estils, capçalera,
  peu i marges, i els `{{marcadors}}` s'omplen amb `vars`; un `.potx`/`.pptx`
  aporta els seus layouts i colors (`office_read detail=layout` els llista).
- **Revisió visual**: `office_render` converteix el document en imatges
  (LibreOffice, o Word/PowerPoint a Windows) i comprova automàticament text
  que desborda, solapaments, formes fora de la diapositiva, text de mostra i
  errors de fórmula. L'agent té l'ordre de renderitzar i corregir abans de
  lliurar. Sense LibreOffice ni Microsoft Office només es fan les
  comprovacions estructurals; instal·la
  [LibreOffice](https://www.libreoffice.org) per a la revisió visual.
- **Seguretat**: cada escriptura passa pel journal (`/rewind` la desfà) i una
  edició que deixaria el paquet trencat es rebutja.

Les skills integrades `office-docx`, `office-pptx` i `office-xlsx` porten
les regles de format i disseny que segueix l'agent.

## Col·leccions de coneixement

La pàgina **Coneixement** (barra lateral, després de Fitxers) guarda
col·leccions de documents i notes que l'agent consulta amb l'eina
`knowledge_search` i cita com a `ruta · ubicació`. Cada col·lecció té un
abast:

- **Global**: disponible a tots els projectes. Es desa a
  `~/.local/share/gregal/knowledge/` (o `GREGAL_DATA_DIR`).
- **Aquest projecte**: es desa a `.gregal/knowledge/` dins del projecte, de
  manera que la definició (nom, descripció, fonts, notes) viatja amb el
  repositori. Les fonts de dins del projecte es desen amb rutes relatives; els
  fitxers d'índex s'ignoren a git automàticament i es refan a cada màquina.
- **Aquesta sessió**: només per a la conversa actual (a la interfície de
  terminal, per a la sessió del terminal).

Crea una col·lecció amb un nom, una descripció (ajuda l'agent a decidir quan
cercar-hi) i un abast, i enllaça-hi carpetes o fitxers («Navega…» fa servir
el diàleg del sistema a l'app d'escriptori i el selector de carpetes al
navegador) amb globs opcionals d'inclusió i exclusió com `*.md, docs/**` o
`**/*.pdf`. Gregal llegeix Markdown, text pla i codi, HTML (el contingut
principal), PDF (la capa de text), `.docx`, `.xlsx` i `.pptx`; les imatges,
els arxius comprimits i els binaris se salten, i els `.doc/.xls/.ppt` antics,
els PDF xifrats i els escanejats sense capa de text surten com a no suportats
o sense text. Carpetes com `.git`, `node_modules`, `vendor`, `dist` i les
ocultes no s'indexen.

Els documents es parteixen en fragments d'uns 1.500 caràcters que se
solapen i recorden on són: camí de capçaleres i rang de línies, pàgina del
PDF, full o diapositiva. La indexació va en segon pla amb una barra de
progrés i és incremental: **Reindexa** només rellegeix els fitxers nous o
canviats (per mida i data de modificació) i treu els esborrats; **Reindexa-ho
tot** ho rellegeix tot. Cada col·lecció llista els documents amb l'estat, els
fragments i quan es van indexar, i es pot desactivar (es conserva, però ni es
cerca ni la veu l'agent) o esborrar (se'n va l'índex; els fitxers font no es
toquen mai). També s'hi poden afegir notes curtes.

El quadre de cerca de dalt prova la recuperació a totes les col·leccions
activades o a una de sola. El rànquing és BM25 per paraules, sense distingir
accents ni majúscules, i funciona sense xarxa. Per a la cerca semàntica,
configura un model d'embeddings OpenAI-compatible; els vectors es desen amb
l'índex i els resultats combinen els dos rànquings:

```yaml
knowledge:
  embeddings: {provider: openai, model: text-embedding-3-small}
  max_files: 2000     # per col·lecció
  max_mb: 200         # llegits per col·lecció
  max_file_mb: 25     # els fitxers més grans se salten
```

El prompt de sistema llista les col·leccions activades (nom, abast i
descripció), perquè l'agent sàpiga quan cercar-hi, i li demana que citi els
passatges que fa servir. `knowledge_search` es permet a tots els modes.
`knowledge_note` (l'agent desa una nota) demana permís, es denega als modes
Xat, Consulta i Objectiu, i en mode Autònom passa sense preguntar (com tota la
resta); sense nom de col·lecció va a les
«Session notes» de la sessió.

La pàgina també ensenya les memòries integrades: la **memòria d'usuari**
(`~/.config/gregal/memory.md`, `/remember`) i les **notes del projecte**
(`.gregal/memory.md`, `/nota`) es poden editar i desar; si el fitxer ha
canviat entremig, Gregal el torna a carregar i t'ensenya la teva versió en
comptes de trepitjar-lo. `AGENTS.md`/`CLAUDE.md` es mostren només per llegir.
A la interfície de terminal, `/knowledge` llista les col·leccions i
`/knowledge search <consulta>` prova una cerca. En servidors amb usuaris,
només l'administrador canvia les col·leccions globals i la memòria d'usuari,
i les carpetes font han de ser dins de les arrels de l'usuari. L'API HTTP és
a `docs/api-contract.md`.

## Referències

### Prototips Web


A Equip, tria Crea una web, adapta la tasca, configura un model i inicia
l'equip. Els rols col.laboren amb text i codi, sense eines externes. Selecciona
el constructor o el revisor per obrir un HTML complet al panell lateral o
descarregar-lo com `website.html`. Els documents parcials no ofereixen preview.
La demo local es una simulacio i no utilitza cap provider.

La preview executa l'HTML en un iframe sense acces al mateix origen, amb CSP
que restringeix recursos externs, fetch i formularis. No es un sandbox complet
ni un desplegament. Revisa el codi abans d'obrir l'HTML descarregat fora de la
preview o publicar-lo.

- [Primers passos](getting-started.ca.md)
- [Contracte de l'API HTTP](api-contract.md)
- [Notes de compatibilitat](compatibility.md)
- [Política de seguretat](../SECURITY.md)
- [Com contribuir](../CONTRIBUTING.md)
## Limits Temporals Del Model

Un pas del model en streaming es talla quan el proveïdor no envia res de res
durant cinc minuts (Autonomous) o quatre (modes interactius): és un límit de
silenci, de manera que un model lent que va escrivint no es talla mai, i un
sostre dur (30 minuts, 60 en Autonomous) encara atura un pas desbocat. Els
passos sense streaming (Telegram, algunes crides laterals) mantenen un límit
de pas sencer de 20 minuts en Autonomous i quatre en interactiu.
`agent.model_timeout_s` permet configurar tots dos entre 1 i 1200 segons; `0`
selecciona el valor automàtic segons el mode. Els reintents comparteixen el
mateix termini i la cancel·lació continua activa. Les peticions autònomes
també respecten el pressupost restant de `max_minutes`. Quan s'esgota, es
descarten les eines pendents i es demana un resum final per separat, amb el
seu límit temporal existent. Un pas tallat per silenci es torna a demanar una
vegada (una petició encallada al servidor sol passar al segon intent); el
segon tall acaba el torn. Els subagents (`delegate`) tenen el mateix límit i
el mateix reintent.

Un proxy que respon HTTP 524 (Cloudflare: l'origen ha trigat més de ~100
segons) no es reintenta amb la mateixa petició: des d'aquell moment Gregal fa
en streaming totes les crides a aquell endpoint, perquè els bytes arribin de
seguida i el proxy no les talli.

## Plots al xat i dashboard

Demana a Gregal un gràfic amb dades del projecte. Els blocs `gregal-plot`
es mostren com a gràfics de línies, barres o dispersió dins del xat, amb
les dades consultables i exportació a PNG i JSON. Prem **Guardar plot**
o **Afegir al dashboard** per conservar-lo.

Obre **Plots i dashboard** a la barra lateral per consultar la biblioteca
del projecte o el seu dashboard. Pots importar JSON, editar dades i títols,
afegir notes, canviar l'amplada, ordenar targetes i eliminar plots.
Els plots es guarden al servidor per usuari i projecte i es conserven en
reiniciar. Són captures: les dades no s'actualitzen automàticament.

Crea i anomena diversos dashboards amb **Nou dashboard**. El selector
**Moure al dashboard** assigna cada plot a un tauler. Eliminar un dashboard
conserva els plots a la biblioteca. Pots cercar per títol, nota o sessió
d'origen i descarregar tots els taulers i plots amb **Exportar col·lecció JSON**.

## Mapa del projecte i cerca de símbols

Als modes code, autònom, consulta i objectiu, cada torn comença amb un mapa del
projecte enganxat al missatge: la llista de fitxers, l'estat de git i els
fitxers petits que la tasca anomena. En projectes Go, JavaScript/TypeScript,
Python, Java, C# i Rust el mapa també porta una **secció de símbols** compacta:
les funcions, mètodes, tipus, classes, interfícies, enums, traits i constants
de nivell superior dels fitxers més rellevants per a la tasca, una línia per
fitxer (`camí: Nom(f) Tipus.Metode(m) …`). La rellevància surt dels
identificadors de la tasca (partits per camelCase i snake_case) comparats amb
els noms dels símbols i els camins, amb un pes extra per als fitxers que estàs
canviant (git status) o que han canviat fa poc. La secció es queda dins d'uns
1.200 tokens; `GREGAL_REPOMAP_TOKENS` canvia el pressupost, i amb `0` l'apaga.

L'índex de símbols es construeix amb el parser del mateix Go per a Go i amb
patrons lleugers per als altres llenguatges. Segueix les mateixes regles de
fitxers que el mapa (la llista de git quan n'hi ha, sense `vendor` ni
`node_modules`, fitxers de fins a 256 KB), viu en memòria per workspace i només
torna a llegir els fitxers que han canviat de mida o de data. En repositoris
molt grans la primera construcció s'atura als 300 ms i el mapa avisa que
l'índex és incomplet; els torns següents continuen on s'havia quedat. El mapa
només es repeteix quan canvien la llista de fitxers o l'estat de git, o quan la
tasca anomena literalment un símbol (per exemple `MapaProjecte`): llavors només
s'hi afegeixen els símbols rellevants.

L'agent també té l'eina `symbols`, de només lectura i permesa a tots els modes
com `grep` i `glob`. Busca al mateix índex per nom (`query`, una subcadena sense
distingir majúscules com `Decide` o `Policy.Decide`, o una expressió regular
amb `regex: true`), `kind` (`func`, `method`, `type`, `struct`, `interface`,
`class`, `enum`, `trait`, `impl`, `const`, `var`), `file` (un glob com
`internal/**/*.go`), `exported` i `limit` (50 per defecte, màxim 200), i torna
`camí:línia tipus nom — signatura`.

### Servidors de llenguatge (eina `lsp`)

Quan hi ha un servidor de llenguatge instal·lat i al `PATH` (`gopls`,
`pyright-langserver`/`basedpyright-langserver`/`pylsp`,
`typescript-language-server`, `rust-analyzer`, `clangd`), l'agent té l'eina
`lsp`. Rep `path`, `line` (comptant des d'1, com la mostra `read`) i
`symbol` (el nom que hi ha a la línia) o `column`, i una `operation`:

- `definition`: on es defineix el símbol;
- `references`: tots els llocs on es fa servir, a tot el projecte (fins a
  80 línies);
- `hover`: tipus i documentació;
- `diagnostics`: errors i avisos del fitxer (no cal línia);
- `rename` amb `new_name`: canvia el nom del símbol a tots els fitxers, sense
  tocar strings, comentaris ni altres noms iguals. S'escriu com qualsevol
  edició (es pot desfer i `/rewind` funciona), en mode `code` demana permís
  com `edit` i en els modes de només lectura està denegat. Es refusen els
  canvis fora del workspace o en un fitxer que ha canviat mentre es
  calculava el rename.

Quan hi ha un servidor, les regles del mode també diuen al model quan triar
`lsp` (els usos d'un símbol concret, la definició o el tipus, un rename entre
fitxers), i l'explorador de només lectura també la té (el rename s'hi denega).

Els servidors s'engeguen el primer cop que calen, un per llenguatge i
workspace, i s'aturen amb la tirada. No es baixa res. Els diagnòstics que
surten després de cada edició continuen venint de les comprovacions per
línia d'ordres (`go vet`, `tsc`, pyright/mypy), que no necessiten cap
servidor engegat. Configuració:

```yaml
lsp:
  mode: off            # treu l'eina
  servers:             # canvia o afegeix un servidor per llenguatge
    java: {command: [jdtls], extensions: [.java]}
```

## Exportar i compartir converses

A la web i a l'app d'escriptori, **⋯ → Exporta** (conversa actual) o el botó
**⤓** d'una fila de la llista de converses (conversa desada) obre el menú
d'exportació:

- **Markdown (.md)** o **HTML autocontingut (.html)**: els missatges de la
  persona, els grups de feina de l'assistent (plegats, amb el nom, els
  arguments i el resultat retallat de cada eina) i les respostes finals. El
  mapa del projecte i la resta de context intern (prompt de sistema, context
  dels hooks, resums de compactació, avisos interns) no hi surten mai. L'HTML
  porta el CSS inline i cap script ni recurs extern.
- **Amaga claus, secrets i camins personals** (activat per defecte) substitueix
  per `[REDACTED]` les claus i tokens (`sk-…`, `ghp_…`, `xoxb-…`, `AKIA…`, JWT,
  `Bearer …`, `api_key=…`, claus privades PEM), els valors secrets del config
  (claus dels proveïdors, token de Telegram, env/capçaleres MCP) i les
  variables d'entorn amb nom de secret, i les carpetes personals per `~`.
  **Amaga també els correus** canvia les adreces per `[email]`.
- **Crea un enllaç per compartir** desa una instantània HTML de només lectura,
  sempre sanititzada, a `<carpeta de dades>/shares/` i en copia l'URL
  `/share/<id>`. L'id és aleatori (160 bits) i és l'única clau: qui tingui
  l'enllaç i pugui arribar al teu servidor Gregal la pot obrir, sense el token
  de l'API. Les pàgines es serveixen amb una Content-Security-Policy sense
  scripts. **Preferències → Enllaços compartits** les llista i les revoca (un
  enllaç revocat torna 404).

Al terminal, `/export` escriu la conversa actual en un fitxer Markdown al
directori de treball i en mostra el camí. `/export html` escriu HTML,
`/export md notes/xat.md` tria el camí, `--redact` sanititza com a dalt i
`--emails` també amaga els correus.
