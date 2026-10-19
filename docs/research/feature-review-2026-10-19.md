# Funcions candidates per a Gregal

Revisió del 2026-10-19 sobre `bfc611b` (v1.12.2). Lectura de codi i
documentació oficial externa; no és una auditoria exhaustiva ni un benchmark.
Les millores de rendiment són hipòtesis per validar. Les durades són estimacions
de desenvolupament d'un primer increment, amb proves, per una persona familiar
amb el projecte; no inclouen totes les plataformes ni desplegaments empresarials.

## Punt de partida

Gregal ja té eines de lectura paral·leles, delegació, scouts automàtics,
subagents configurables que poden editar amb les guardes corresponents,
worktrees, equips amb grafs, checkpoints, best-of-N, verificació independent,
goal loops, límits, detecció de crides repetides, retries i fallback, LSP,
repo map amb graf de referències i PageRank, deduplicació de lectures,
compactació, prompt caching, hooks, memòria amb aprovació i skills progressives.
No cal presentar aquests mecanismes com si faltessin.

Hi ha documents de recerca anteriors que descriuen mancances ja resoltes.
Per establir l'estat actual s'ha prioritzat el codi per sobre d'aquests documents.

## Prioritats

| # | Increment | Valor principal | Esforç orientatiu |
|---|---|---|---|
| 1 | Recuperació durable de feina després d'un reinici | Fiabilitat de tasques llargues | 5–10 dies |
| 2 | Backend local gestionat des de VS Code | Instal·lació i ús quotidià | 2–4 dies |
| 3 | Revisió dels canvis de cada torn dins de VS Code | Control i qualitat | 5–8 dies |
| 4 | Delegació automàtica amb pressupost i resultats verificables | Velocitat i cost | 3–6 dies |
| 5 | Descoberta i càrrega d'eines MCP sota demanda | Tokens i latència | 2–4 dies |
| 6 | Estat estructurat que sobrevisqui a la compactació | Qualitat en tasques llargues | 3–5 dies |
| 7 | Verificació incremental amb evidències vinculades al canvi | Qualitat i temps de verificació | 3–5 dies |
| 8 | Autocompletat amb model i pressupost propis | Fluïdesa de VS Code | 2–4 dies |
| 9 | Taller 2D amb dependències i accions sobre tasques | Comprensió del progrés | 3–5 dies |
| 10 | Acceptació visual de canvis de frontend | Qualitat funcional i estètica | 3–6 dies |
| 11 | Adaptador ACP | Accés des de més editors | 5–10 dies |

### 1. Recuperació durable

Evidència: `internal/runs/runs.go` documenta que la cua i les claus
d'idempotència són en memòria. `internal/web/serve.go` marca com a fallides les
execucions que troba inacabades després de reiniciar el servidor. Persistir
events permet reconstruir què va passar, però no equival a reprendre l'executor.

Proposta: desar la petició admesa, el workspace, l'estat del torn, les decisions
pendents i la clau d'idempotència. Recuperar els torns encara no iniciats i oferir
«Reprendre» per als interromputs des d'un punt consistent. Registrar les eines
iniciades i acabades: una escriptura o ordre externa amb resultat desconegut no
s'ha de repetir automàticament. No prometre execució exactament una vegada per
a eines externes arbitràries.

Validació: matar el procés amb feina en cua, durant una lectura, durant una
edició i durant una aprovació. Comprovar que no es perd la petició ni es duplica
cap efecte ja confirmat. És diferent de reconnectar el flux després de canviar
de pestanya, que Gregal ja suporta.

Referència: [continuïtat en agents llargs, Anthropic](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents).

### 2. Backend gestionat per VS Code

Evidència: `vscode-gregal/README.md` demana iniciar el backend i deixar el terminal
obert. `src/controller.ts` configura una connexió, però no gestiona el procés.

Proposta: detectar o instal·lar un binari compatible, comprovar el checksum,
arrencar-lo en segon pla i mostrar «Connectat / Iniciant / Error» amb accions
per reparar i veure el log. Distingir processos propietat de l'extensió de
servidors compartits o remots; no aturar aquests últims en tancar una finestra.
Preveure extension hosts SSH/WSL/container i equips amb instal·lació manual.

Validació: màquina sense backend, reinici de VS Code, dues finestres,
port ocupat, binari incompatible i servidor remot.

### 3. Revisió dels canvis per torn

Evidència: `vscode-gregal/src/extension.ts` ofereix staging/revert de línies via
Git i checkpoints. Això no és una proposta de canvi pendent per cada tasca.

Proposta: una llista de fitxers del torn, diff natiu, proves associades i accions
«Aplica / Descarta» per bloc. Separar acceptar una proposta d'afegir-la al staging
de Git. Fer servir un workspace aïllat o una capa de canvis pendent i comprovar
la versió del document abans d'aplicar-la, preservant les edicions de l'usuari.

Validació: fitxer brut abans del torn, edició manual durant el torn, dos agents
editant el mateix fitxer i descart parcial sense perdre canvis previs.

### 4. Delegació que compensi

Evidència: `internal/agent/delegate.go`, `subagent_parallel.go` i `internal/team/`
ja resolen bona part de l'execució. `internal/config/router.go` usa heurístiques
i ja permet escalar després de revisions fallides. No cal afegir un altre
orquestrador general.

Proposta: ampliar aquesta base amb subtasques delimitades, fitxers assignats,
dependències, criteris d'acceptació i pressupost compartit pare/fills. Delegar
exploració i implementacions independents a models més petits; escalar quan
falla una comprovació objectiva. Per editar en paral·lel, aïllar els canvis i
integrar-los amb verificació. Mantenir l'execució directa per a feines curtes.

Validació: comparar sense delegació, delegació actual i nova política, amb
mateix model coordinador i tasques. Mesurar temps fins a resultat correcte,
cost total incloent fills, percentatge d'èxit i conflictes d'integració.

Referència: [subagents i delegació, Claude Code](https://code.claude.com/docs/en/sub-agents).

### 5. MCP sota demanda

Evidència: `internal/mcp/manager.go` registra les specs de totes les eines
connectades; `internal/agent/loop.go` les afegeix a `SpecsAll()`. Hi ha filtratge
per mode, però no una descoberta per consulta equivalent a Tool Search.

Proposta: catàleg curt de servidors/capacitats i una eina de cerca que carregui
els esquemes necessaris per sessió. Conservar els permisos i els noms estables.
Preveure compatibilitat amb models oberts i un mode de càrrega completa; no
dependre exclusivament d'extensions d'un proveïdor. Mesurar l'efecte dels canvis
de catàleg sobre la caché del prompt.

Validació: cost d'entrada i selecció correcta amb catàlegs petits i grans,
incloent eina no descoberta al primer intent i servidor desconnectat.

Referència: [MCP Tool Search, Claude Code](https://code.claude.com/docs/en/mcp#scale-with-mcp-tool-search).

### 6. Estat estructurat de la tasca

Evidència: hi ha compactació (`compact.go`, `histcompact.go`), todos i memòria
duradora (`automem.go`). La memòria de projecte no ha de guardar l'estat temporal
de totes les tasques.

Proposta: un registre separat per tasca amb objectiu, decisions de l'usuari,
criteris pendents, fitxers alterats, proves vigents i següent pas. Actualitzar-lo
des dels events i exigir que cada criteri complet tingui evidència. Incloure'l
en compactar, reprendre o lliurar feina a un subagent, amb un límit de mida.

Validació: tasques que travessin diverses compactacions i reinicis, comprovant
que es preserven restriccions i no es repeteix feina ja feta.

### 7. Verificació incremental

Evidència: Gregal ja té diagnòstics després d'editar, `--verify`, revisors i un
goal loop amb proves per criteri (`internal/agent/goalloop.go`).

Proposta: seleccionar primer tests rellevants segons fitxers i dependències,
agrupar comprovacions després d'un lot d'edicions i fer la suite més àmplia al
punt d'integració. Vincular cada resultat al hash del codi comprovat: un test
verd anterior a una nova edició ja no demostra l'estat final. Recórrer a tota
la suite quan no es pugui delimitar l'impacte amb confiança.

Validació: temps de verificació i regressions detectades respecte a executar
sempre la suite completa. Afegir proves de reinici, desconnexió, aprovació i
canvi de sessió al banc existent, no només petites reparacions de funcions.

### 8. Autocompletat especialitzat

Evidència: `vscode-gregal/src/editor-features.ts` ja té debounce de 300 ms,
cancel·lació i context acotat; el README indica que fa servir el model de codi.

Proposta: rol propi per autocompletat, pressupost de latència i cua separada,
caché de continuacions i protocols fill-in-the-middle quan el model els suporti.
Descartar una resposta si la versió del document ja ha canviat. Evitar que
escriure al mateix servidor local prengui tots els slots d'una tasca agentiva.

Validació: latència p50/p95, acceptació de suggeriments, resultats obsolets,
consum i impacte sobre els torns llargs. No mesurar només tokens per segon.

### 9. Taller 2D operatiu

Proposta: reutilitzar els events i grafs existents per mostrar què fa cada
agent, què espera, què bloqueja altres tasques, últim progrés real i resultat
produït. Clicar un agent ha d'obrir el seu fil, diff o pregunta pendent.
Animar transicions reals: assignació, eina en curs, bloqueig, lliurament i fi.
No inventar percentatges a partir del nombre de passos; mostrar criteris
verificats. Oferir llista equivalent i respectar moviment reduït.

Validació: reconstrucció després d'un refresh, zero agents fantasma després
d'un error, sincronització amb la barra lateral i rendiment amb molts events.

### 10. Acceptació visual

Gregal ja disposa d'eines de navegador. L'increment és connectar-les amb un
procés de comprovació de frontend: executar recorreguts, capturar pantalles,
detectar errors de consola i revisar l'estat visual abans de donar la tasca
per acabada. Guardar evidències accessibles al fil. Una comparació d'imatges
necessita controlar dades i animacions per no generar falsos positius.

Validació: fixtures amb regressions funcionals, desbordaments, focus trencat,
estats buits i problemes de tema/contrast. Comparar detecció i falsos positius.

### 11. ACP

Proposta: un adaptador sobre els runs/events/permisos existents perquè Gregal
sigui accessible des d'editors compatibles. Fer una prova petita de prompt,
streaming, cancel·lació i aprovacions abans d'ampliar l'abast. MCP connecta
eines; ACP cobreix la comunicació editor-agent i no el substitueix.

Referències: [especificació i repositori ACP](https://github.com/agentclientprotocol/agent-client-protocol),
[integració d'editors de JetBrains](https://www.jetbrains.com/acp/).

## Seqüència recomanada

1. **Fiabilitat i ús diari:** recuperació durable, backend gestionat i banc de
   proves d'incidents. Revisió per torn com a següent increment d'experiència.
2. **Eficiència demostrable:** MCP sota demanda i delegació amb pressupost;
   validar per separat per atribuir els canvis de qualitat, temps i cost.
3. **Tasques llargues i experiència visual:** estat estructurat, verificació
   incremental, taller operatiu i acceptació visual. ACP quan hi hagi demanda
   real d'altres editors; autocompletat especialitzat segons ús de VS Code.

No prioritzaria una base vectorial per a tot el repositori sense mesurar primer
on fallen grep, LSP i el repo map actual. Tampoc activaria múltiples revisors
o subagents a totes les tasques: el sobrecost pot superar el benefici.

La comparativa d'Aider mostra una altra via útil per a models locals petits:
[separar arquitecte i editor](https://aider.chat/docs/usage/modes.html) i adaptar
el [format d'edició](https://aider.chat/docs/more/edit-formats.html). Ho reservaria
per a un experiment mesurat amb models locals, abans de convertir-ho en el
flux per defecte de Gregal.
