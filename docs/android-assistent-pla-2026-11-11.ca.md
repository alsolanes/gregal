# Gregal Android: un assistent que s'encarrega de les coses

Pla d'implementació per agents · 11 de novembre de 2026

> **Revisió de direcció:** després del feedback sobre dots i Grok Bot, el pla vigent és [Gregal personal: renovació completa de l'experiència](android-assistent-experiencia-v2.ca.md). Aquest document queda com a auditoria tècnica de base. La nova versió substitueix les decisions de navegació, xat sense bombolles, abast d'ordinador/connexions i seqüència d'entregues; conserva les regressions i proves útils identificades aquí. No executar els paquets A0–A8 sense aplicar aquesta revisió.

## 1. Decisió de producte

**Un assistent principal, una conversa fàcil d'obrir i una safata petita de coses que realment necessiten atenció.** Gregal ha de permetre dir «busca-ho», «recorda-m'ho» i «encarrega-te'n» i tornar amb un resultat, una data confirmada o una decisió concreta.

La base ja existeix. No cal construir un altre motor d'agents, un altre planificador, un altre sistema de memòria ni un altre catàleg de personalitats. Cal acabar els recorreguts, fer fiables els avisos i reduir les decisions que l'app exigeix abans de ser útil.

Àmbit principal: Gregal Vents per Android. S'ha inventariat també la superfície de codi, fitxers, documents, jocs i configuració per decidir on viuen; no es proposa eliminar-les ni transformar l'agent de programació en un producte diferent.

### Resultats que han de definir la primera entrega

1. Un usuari nou pot configurar la connexió existent i fer una primera petició sense haver de crear una colla ni triar especialistes.
2. «Recorda'm trucar demà a les 9» acaba en una targeta amb data, zona horària i accions d'editar/cancel·lar; només es diu que està desat després de l'èxit real.
3. «Busca tres opcions i recomana-me'n una» fa una cerca real, cita les fonts i presenta una comparació llegible al mòbil.
4. «Vigila aquest tema i avisa'm quan canviï» crea un seguiment visible i pausable; no envia missatges de farciment.
5. Una feina llarga es pot abandonar visualment i recuperar sense perdre'n l'estat ni haver d'endevinar quin vent la porta.
6. «Avui» serveix per decidir i actuar. No és un segon historial de tots els missatges.

## 2. Abast i evidència de la revisió

Base observada: commit `4b3b9f6`, amb canvis locals preexistents. En particular, `CompanyScreen.kt`, `GregalViewModel.kt`, `GregalApi.kt`, `VentetChat.kt`, rutes de cap/rotllanes i documentació ja tenien treball en curs. **No s'han modificat aquests fitxers.** Les referències següents són camins relatius al repositori i símbols per resistir canvis de línia.

S'han llegit els fluxos principals de navegació, portada, safata, programacions, notificacions, connexió, memòria, veu, offline, tema i parts del backend que els fan funcionar. Les pantalles secundàries s'han inventariat i revisat a nivell d'entrada/responsabilitat; això no és una certificació línia per línia de tot el codi.

No hi havia dispositiu ni emulador connectat a `adb devices`. Per tant, les conclusions visuals són una auditoria del codi Compose i una proposta de disseny; falten captures i proves tàctils reals abans de donar el redisseny per validat. La maqueta que acompanya el pla és una proposta amb contingut fictici, no una captura de l'app.

### Mapa de superfícies

Prefix Android: `android/app/src/main/java/xyz/solanes/gregal/`.

| Superfície | Implementació actual | Decisió |
|---|---|---|
| Entrada, connexió, origen local/remot | `MainActivity`, `Screens`, `ProviderSetup`, `LocalServer`, `ServerDiscovery` | Simplificar l'explicació; conservar el model local/remot |
| Portada, conversa principal, xats | `CompanyScreen`, `VentsAskHome`, `VentetChat`, `CompanyRoute`, `VentsInbox` | Una entrada clara a l'assistent, conservar fils i especialistes |
| Safata proactiva | `VentsAvui`, `winds_avui.go` | Fer-la la vista única d'atenció i decisions |
| Rutines, recordatoris i activitat | `ProgramatsScreen`, `AutomatitzacionsScreen`, `VentsActivity`, `JobsScreen`, `FlowsScreen` | Una vista de producte; mantenir eines avançades separades |
| Feines i resultats | `FeinesScreen`, `WorkScreen`, `ChatWork`, `PreviewScreen`, `Webs` | Resultats enllaçables des del xat i d'Avui |
| Perfil, personalitats i memòria | `WindProfileScreen`, `WindCreatorScreen`, `WindLibraryScreen`, `WindProfile`, `Winds` | Un assistent predeterminat; personalització opcional |
| Notes i coneixement | `Notes`, `WikiScreen`, `WindCalaix`, `CarpetaScreen` | Agrupar accessos com a «Desats», conservar tipus i dades |
| Veu i contingut ric | `VentsVoice`, `Md`, `ChatImage`, `PdfView`, `Plots`, `PlotCard` | Millorar lectura i dictat abans d'afegir veu en temps real |
| Bateria i notificacions | `JobAlarms`, `WindWatch`, `GregalNotifications`, `NotificationOrigins`, `VentsOffline` | Separar puntualitat, execució i lliurament; provar offline i reinici |
| Configuració i actualitzacions | `PrefsScreen`, `Updater`, `Theme`, `Brand`, `Strings`, `AppStrings` | Configuració progressiva, errors útils, coherència visual |
| Codi, Office, Retro, jocs i Racó | `Screens`, `OfficeScreen`, `RetroScreen`, `GamesScreen`, `CornerScreen`, `Drawer` | Conservar, sense competir amb l'assistent en la navegació principal |

## 3. Troballes prioritzades

«Confirmat al codi» no implica que s'hagi reproduït en un dispositiu. «A validar» identifica explícitament una hipòtesi.

| ID | Prioritat | Evidència i efecte | Correcció proposada |
|---|---|---|---|
| F01 | P0 | Confirmat al codi: `executeWindHeartbeat` comprova `out == "(res)"` abans de `windStripNext`. El prompt exigeix també `NEXT`. `(res)\nNEXT: ...` passa la comprovació i pot publicar `(res)` | Normalitzar primer; interpretar silenci després; prova de regressió sense missatge, targeta ni notificació |
| F02 | P0 | Confirmat: `windNextAt` accepta anys >2000; `executeWindHeartbeat` desa la data sense limitar-la al futur ni a una freqüència mínima | Validar i limitar el següent control; una data passada o exagerada no pot provocar bucles ni silenci indefinit |
| F03 | P1 | Confirmat: `avuiFromVoice` considera urgent qualsevol `feina:`, recordatori i briefing; el briefing retorna notificació activa explícitament | Definir una política de silenci diferenciada; acabar una feina no és sempre urgent |
| F04 | P1 | Confirmat: `CompanyScreen` passa `emptyList()` i `null` a recordatoris/connexions de `todayCards`, i només passa files amb novetats | La portada no ofereix totes les peces que la funció suporta. Evitar un segon resum divergent: consumir un resum d'Avui |
| F05 | P1 | Confirmat: hi ha «Programats», «Activitat/Automatitzacions», programats al perfil i jobs/flows avançats | Unificar entrades i terminologia; reutilitzar `/api/winds/automations`, sense migrar tots els magatzems |
| F06 | P1 | Confirmat: `WindWatch.INTERVAL_MS` és 15 minuts; `SOON_MS`, 3 minuts; hi ha comprovacions anticipades i recordatoris mirall | El lliurament d'una feina remota no és instantani. Mostrar estat honest i mesurar latència abans de decidir si cal push |
| F07 | P1 | Confirmat: `avuiDefaultActions` genera prompts per a ajornaments; `avuiRun` els envia al model | «Ajornar 1 h» pot gastar un torn i dependre de la interpretació del model. Acció determinista amb ID estable |
| F08 | P1 | Confirmat: coexisteixen `GregalType`, mides directes, `CardShape` de 20 dp i `GregalShape.md` de 14 dp | Assignar una funció a cada token; unificar les superfícies principals gradualment |
| F09 | P1 | Confirmat: `AskTabs` usa files de mínim 40 dp, lectura en veu alta té `size(28.dp)` i el calaix usa `CappedFontScale(1.5f)` | Validar àrees tàctils efectives de Compose, solapaments i text al 200%; no donar per fet que mida visible = mida tàctil |
| F10 | P1 | Confirmat: `VentsVoice` fa dictat amb `RecognizerIntent` i lectura TTS; no és una conversa de veu dúplex | Etiquetes de «Dicta» i «Escolta», text editable i aturada fiable. No prometre una trucada contínua |
| F11 | P1 | Confirmat: connexions de calendari via URL i correu via IMAP es configuren a `PrefsScreen.ConnectionsCard` | Exposar casos d'ús i prova de connexió; no presentar-ho com un catàleg OAuth de connectors ja disponible |
| F12 | P1 | Confirmat: `VentsOffline` ja guarda fils, cua d'enviament i recordatoris mirall per origen | Preservar-ho. A validar: reintents després d'una resposta perduda, doble avís i idempotència d'accions |
| F13 | P2 | Confirmat: `GregalViewModel.kt` té ~6.010 línies i `CompanyScreen.kt` ~2.388 en la còpia revisada | Extreure només les responsabilitats tocades; un sol integrador per aquests fitxers |
| F14 | P2 | Confirmat: `ChatSkeleton` té animació infinita; el producte ja té preferència de moviment expressiu | Auditar que tots els indicadors respectin reducció de moviment; no afegir un altre motor d'animació |
| F15 | P1 | Confirmat: `compileSdk`/`targetSdk` 34 i serveis `dataSync`; l'APK inclou backend Go | Preparar compatibilitat amb Android recents, condicionada a SDK/target. No concloure que el límit nou ja s'aplica igual a la build actual |

Fortaleses a protegir: streaming del Ventet en curs, resposta sense caixa, seguiment de scroll des de l'inici de resposta, errors d'enviament recuperables, origen per compte/servidor, memòria temàtica editable, limitació de passos de cerca, undo de targetes, hores tranquil·les i compatibilitat dels historials.

## 4. Direcció visual i distribució

### Recomanació: «Gregal serè»

Conservar la identitat marina i les cares dels vents, amb menys capes decoratives. Fons tranquil, text molt llegible, accent marí per a l'acció principal i avatar petit però recognoscible. La personalitat ha de venir de la veu, la cara i la qualitat de les propostes.

Alternatives considerades:

| Direcció | Avantatge | Cost / problema | Decisió |
|---|---|---|---|
| Gregal serè | Conserva identitat i components; lectura i accions clares | Requereix coherència entre pantalles | Recomanada |
| Conversa radical | Entrada gairebé buida i assistent sempre al centre | Rutines i pendents es poden tornar invisibles | Prendre'n la simplicitat del xat |
| Company expressiu | Avatar gran i més presència emocional | Consumeix espai i competeix amb informació útil | Opcional al perfil/veu; no portada predeterminada |

### Navegació recomanada

- Barra inferior amb **Assistent · Avui · Xats**. Reutilitzar els tres estats actuals; no mantenir simultàniament les tres pestanyes de dalt.
- «Assistent»: conversa principal persistent. En buit, salutació curta, tres exemples útils i compositor. Quan ja hi ha historial, obrir-lo directament.
- «Avui»: «Necessita resposta», «En marxa», «Més endavant». «Programacions» sempre accessible a la capçalera; no amagar recordatoris a Explora.
- «Xats»: llista de converses, cerca i acció nova conversa. Accés a «Els meus vents» aquí, sense obligar a gestionar-los.
- Menú secundari: Desats, Connexions, Personalitza, Configuració i Eines de codi. Jocs/Racó a Explora dins Personalitza o una entrada secundària; preservar accessos existents.
- En una conversa individual: amagar la barra inferior i mostrar enrere, identitat i menú. Tornar a la pestanya i posició anteriors.
- En tablets, rail amb les mateixes tres destinacions i màxim de lectura; no estirar una columna a tota l'amplada.

És una proposta de producte per a la propera entrega, no una descripció de l'app actual. Si la barra inferior falla a la QA amb teclat, mantenir temporalment `AskTabs` amb àrees tàctils adequades; no bloquejar fiabilitat per aquest canvi visual.

### Especificació d'estil executable

| Element | Valor inicial / criteri |
|---|---|
| Marges | 20 dp de pantalla; 16 dp en targeta; escala de separacions 4/8/12/16/24/32 |
| Lectura | 16 sp / 24 sp al xat; 14 sp / 20 sp secundari; no fer informació essencial d'11 sp |
| Capçaleres | 22–24 sp; salutació 28–32 sp només en buit, desapareix quan hi ha feina |
| Superfícies | Fons + superfície + elevació de full; evitar degradats a cada targeta |
| Radis | 16 dp targetes; 24 dp compositor/fulls; píndoles només per filtres i accions curtes |
| Accents | Paleta `Pal` existent; un accent principal i colors d'estat amb text/icona |
| Botons | Àrea efectiva de 48 × 48 dp com a objectiu; una acció de màxima jerarquia per targeta |
| Missatges | Usuari en bombolla; assistent sobre fons; eines en una línia desplegable; fonts al peu |
| Moviment | Transicions curtes 160–220 ms; cap animació decorativa permanent; honorar preferències del sistema |
| Avatar | 28–36 dp al xat, 40–48 dp al perfil; gran només si és protagonista d'una interacció específica |
| Densitat | Màxim tres elements prioritaris a la portada; la resta a Avui; evitar files horitzontals de xips truncades |

Validar contrast clar/fosc, TalkBack, mida de font 100/150/200%, 320/360/412 dp, teclat visible i pantalla horitzontal. Ajustar tokens des de `Theme.kt`, no fer reemplaç global de totes les mides.

### Components a consolidar

1. Compositor compartit per portada i conversa: adjuntar, dictar, escriure, enviar/aturar; retorn del text si falla.
2. Targeta d'acció: títol, motiu curt, data/origen, acció principal, alternativa i menú. Variants per recordatori, proposta i resultat.
3. Estat de feina: «Buscant fonts», «Llegint», «Preparant resultat», «Necessito una dada», «Fet»; només reflectir esdeveniments reals.
4. Resum de fonts: dominis i data de consulta, desplegable amb títols i enllaços reals; sense avals artificials de veracitat.
5. Estat buit/error/offline: una explicació i una sortida concreta, conservar l'última dada bona quan es pugui.

## 5. Comportament de l'assistent

### Cercar pel seu compte

Ja existeix la instrucció de cercar per dades actuals a `internal/companion/winds.go` i el bucle d'eines de lectura de `internal/web/plan.go`. Millorar-ne el recorregut i les proves, no crear un «research agent» paral·lel.

- Si la petició depèn d'actualitat, preus, horaris o fets incerts, cercar sense demanar «vols que ho busqui?».
- Si falten dades imprescindibles, fer una pregunta curta; si es pot avançar amb una hipòtesi reversible, explicitar-la i continuar.
- Retornar resposta + fonts + següent acció útil quan n'hi hagi. No posar suggeriments genèrics després de cada resposta.
- Reutilitzar el límit actual de passos. Per seguiments, afegir límit de temps i pressupost global d'usuari damunt dels jobs existents, amb valors explícits i configurables.
- Errors de cerca: «No he pogut comprovar el preu ara» amb reintent; mai presentar dades antigues com a noves.
- Les pàgines consultades són dades, no instruccions: no poden concedir permís per enviar missatges, canviar comptes o executar accions externes.

### Proactivitat que compensa

Començar amb tres casos: resum del matí, seguiment d'un tema demanat i seguiment d'una feina encomanada. El missatge ha de respondre «què ha canviat, per què m'importa i què puc fer».

Pipeline mínim sobre peces existents:

`job existent → eines de lectura → candidat → novetat/rellevància/horari/pressupost → Avui → notificació si toca`

- Autorització: cada seguiment té tema/objectiu explícit i un origen visible. Aprendre una preferència no crea automàticament una vigilància.
- Silenci és un resultat correcte. Normalitzar `(res)` abans de publicar i tractar errors com errors, no novetats.
- Deduplicació: clau per usuari/origen/seguiment + identificador de contingut normalitzat; no només pel títol generat pel model.
- Pilot proposat: màxim dues notificacions no sol·licitades al dia, 6 h de refredament per seguiment i horari tranquil 22:00–08:00 editable. Són decisions inicials de producte, no constants actuals ni valors universalment òptims.
- Els recordatoris explícits no consumeixen aquest cupo. L'usuari decideix si poden sonar en silenci; la UI ho ha de fer entenedor.
- Resultats ordinaris de feines durant silenci van a Avui. Una acció amb termini immediat pot demanar atenció segons preferència explícita.
- Agrupar troballes relacionades; una notícia repetida amb un altre titular no justifica una notificació.
- «Menys d'això», «Útil», «Pausa» i «Per què m'ho dius?» han d'ajustar regles existents o preferències simples. Sense un nou motor de scoring ML.
- Mostrar «Darrera comprovació», «Propera comprovació», «Darrera novetat» i «Pausat/error» al detall, no tota la telemetria al xat.

### Recordatoris i rutines

- Creació conversacional amb rebut: «Dijous 12 de novembre, 09:00 · Europe/Madrid». Resoldre «demà» segons la zona configurada de l'usuari.
- Si manca l'hora: proposar-ne una visible o preguntar quan sigui imprescindible; no amagar l'assumpció.
- «Ajornar», «Fet», «Canviar hora», «Pausa» i «Cancel·lar» criden accions estructurades. No enviar al model un prompt per moure un rellotge.
- Diferenciar «avisar-me» d'«executar una cerca i portar-me'n el resultat». Un recordatori simple no necessita una crida LLM en disparar-se.
- Separar estat de programació, d'execució i de lliurament. «Programat» no equival a «notificacions activades».
- No confirmar localment una operació remota fallida. En offline, mostrar pendent i l'efecte real que tindrà; conservar el mirror ja existent.
- Edició/cancel·lació amb revisió o equivalència que impedeixi aplicar una acció a una versió antiga. Retentar la mateixa petició no crea dues programacions.

### Feines amb continuïtat

- Un encàrrec simple va a l'assistent principal. Els especialistes existents es poden emprar internament; no crear una colla nova per defecte.
- Mateix identificador i enllaç entre conversa, feina, resultat i targeta d'Avui.
- Estats visibles: en cua, treballant, esperant resposta, fet, error, cancel·lat. No representar un error com «fet» amb text petit.
- Cancel·lar ha d'arribar al backend; ocultar un indicador no cancel·la la feina.
- Una acció externa necessita l'autorització que pertoqui segons l'encàrrec: preparar primer el resultat revisable; demanar aprovació només per al pas que no estigui autoritzat.
- Per correu, mostrar destinatari, assumpte i cos abans d'un enviament no autoritzat. No afegir permisos repetits a cerques o lectures ja encarregades.

### Memòria, veu i adjunts

- Distingir «Sobre tu», «Aquest assistent» i «Coneixement del tema»; avui hi ha diferents seccions i magatzems. No unificar-los destruint-ne els límits.
- Facilitar veure, corregir i oblidar cada tipus que l'API admeti. La memòria personal de només lectura necessita una ruta d'edició pròpia si s'incorpora a aquest flux; no simular-la modificant el coneixement del vent.
- No cal una base vectorial nova. Afegir procedència/data només on sigui necessari i compatible.
- Dictat editable abans d'enviar, lectura aturable, errors del reconeixedor visibles i pausa de TTS quan pertoqui pel cicle de vida/audio focus.
- No garantir que dictat/TTS siguin locals: depèn del motor del dispositiu.
- Compartir text/URL cap a Gregal: oferir «Pregunta», «Desa» o «Recorda-m'ho», mantenint la captura de notes existent. Fotos/PDF compartits són una ampliació posterior del manifest i gestió de permisos URI.

## 6. Arquitectura mínima i contractes

### Reutilitzar

- `internal/jobs`, `winds_heartbeat.go`, `winds_action.go`, `winds_automations.go` per execució i programació.
- `winds_avui.go` per la safata; ampliar aquest contracte, no crear una segona API de feed.
- `VentsOffline`, `NotificationOrigins`, `JobAlarms`, `WindWatch` per cache, origen i notificacions.
- `WindTurnAction`, `AvuiAction`, `WindAutomation` i identificadors existents per connectar UI i backend.
- `Theme`, `GregalUi`, `VentsChatChrome` per components. Extreure del ViewModel només l'estat/funcions que realment toqui cada entrega.

### Contracte a tancar abans d'implementar en paral·lel

L'agent integrador documentarà els camps actuals i la correspondència exacta abans d'afegir-ne cap. Les extensions següents són propostes, no rutes ja implementades:

| Necessitat | Extensió mínima proposada | Invariant |
|---|---|---|
| Accions deterministes | Ampliar accions d'Avui/programacions amb tipus, target ID, moment i clau d'idempotència | Repetir la mateixa acció retorna el mateix efecte |
| Data entenedora | Instant UTC + zona IANA per recurrències/calendari | Canviar zona o DST no duplica ni perd instàncies |
| Procedència | IDs de conversa/job/seguiment i, si existeix, resultat | Un avís obre l'objecte correcte i el seu origen |
| Salut del seguiment | Darrera comprovació, propera, error resumit, pausa | No afirmar «vigilo» si no hi ha executor disponible |
| Relevància | Clau de contingut + darrera notificació; pressupost per usuari | El silenci i la deduplicació s'apliquen abans d'enviar |
| Resultats amb fonts | Reutilitzar citacions actuals; estructura additiva si la UI la necessita | Enllaços provenen d'eines, no s'inventen al client |

Compatibilitat: clients antics ignoren camps nous; clients nous han de tolerar camps absents. Introduir capacitats només si cal distingir accions noves; evitar un framework de negociació. No esborrar històrics, no reassignar IDs de vents i no copiar dades entre orígens automàticament.

### Execució en segon pla

Local: el procés Go viu sota l'app i no equival a un servidor sempre encès. Remot: la feina pot continuar al servidor mentre Android dorm, però l'avís pot trigar a arribar. Comunicar-ho a la configuració i al detall d'una programació quan afecti la promesa.

Primer mesurar el mecanisme actual i corregir-lo. Reservar alarmes precises per avisos temporals justificats; estudiar feina diferible amb API adequada en pujar target SDK. No introduir FCM, infraestructura de push, scheduler nou i migració de WorkManager al mateix PR. Si el pilot requereix latència remota curta i el polling no l'assoleix, obrir una decisió específica de push amb manteniment i distribució definits.

## 7. Paquets d'implementació per a altres agents

Cada paquet acaba amb: canvi acotat, proves rellevants, captures si toca UI, compatibilitat documentada i llista de riscos reals. No fer commit/publicació automàtica ni tocar claus de signatura. Els agents són responsabilitats suggerides; no s'han creat agents en aquesta revisió.

### A0 — Integració, contracte i baseline (P0, S)

**Propietari:** integrador. **Dependències:** cap.

- Revisar canvis locals i acordar base compartida abans de modificar fitxers calents.
- Inventariar rutes actuals d'Avui, programacions i accions amb exemples de resposta.
- Reproduir baseline Android/backend i capturar 8 pantalles: alta, portada, Avui, xat actiu, recordatoris, perfil, offline i configuració.
- Fixar els models/extension points que consumeixen A2–A6.
- Fitxers reservats: `GregalViewModel.kt`, `GregalApi.kt`, `CompanyScreen.kt`, `MainActivity.kt`, `AppStrings.kt`, `docs/api-contract.md`.
- Acceptació: contracte curt amb dades d'exemple i instruccions de prova; cap agent ha d'inventar APIs ni escriure simultàniament als fitxers reservats.

### A1 — Cor silenciós, freqüència i pressupost (P0, M)

**Propietari:** backend de proactivitat. **Dependències:** A0 per política, correccions locals poden començar abans.

- Corregir F01 i normalització coherent de `NEXT` (majúscules, minúscules, espais i cometes acceptats pel parser).
- Rebutjar dates passades, aplicar mínim entre controls i màxim raonable; fallback explícit. Rellotge injectable en proves.
- Propagar errors de persistència quan alterin el resultat; no comunicar un avís no desat.
- Afegir deduplicació durable i pressupost per usuari sobre magatzems existents; només si no estan coberts per una capa superior.
- Fitxers: `winds_heartbeat.go`, `winds_heartbeat_next_test.go`; política compartida amb A4 a `winds_avui.go` via integrador.
- Acceptació: `(res)` + `NEXT` no crea missatge/targeta/notificació; dues fonts del mateix canvi no dupliquen; cap bucle amb dates errònies; canviar d'usuari no comparteix límits ni resultats.

### A2 — Sistema visual i navegació (P1, M)

**Propietari:** Android UI. **Dependències:** A0.

- Aplicar tokens proposats a capçalera, compositor, targetes, files i botons principals.
- Preparar barra Assistent/Avui/Xats i component d'entrada buit; lliurar composables a l'integrador.
- Reduir cares de portada a accés secundari si desplacen peticions i resultats; preservar directori complet.
- Fitxers: `Theme.kt`, `GregalUi.kt`, `VentsAskHome.kt`, `VentsChatChrome.kt`, `Drawer.kt`; integració de `CompanyScreen` per A0.
- Acceptació: cap doble navegació; historial i enrere conservats; compositor usable amb teclat; targets efectius accessibles; captures clar/fosc, text 200% i amplada estreta.

### A3 — Programacions i recordatoris de principi a fi (P1, L)

**Propietari:** recorregut de programacions. **Dependències:** A0; coordinar models amb A4/A5.

- Fer d'Activitat/Automatitzacions la pantalla única «Programacions» de producte; Programats i perfil hi enllacen amb filtre.
- Formulari curt com a alternativa al llenguatge natural: què, quan, repetició, avís. Configuració avançada plegada.
- Rebut de creació, edició, ajornament, pausa, cancel·lació i historial. Operacions deterministes, idempotents i vinculades al job original.
- Fitxers: `ProgramatsScreen.kt`, `AutomatitzacionsScreen.kt`, `VentsActivity.kt`, `winds_automations.go`, rutes existents de rutines/recordatoris després d'inventari A0.
- Acceptació: crear per xat i editar per UI funciona sobre el mateix ID; reintent no duplica; canvi de zona i DST provats; error de desat deixa recuperable l'edició; comptes no admin no carreguen rutes privilegiades innecessàries.

### A4 — Avui i política d'atenció (P1, M)

**Propietari:** safata. **Dependències:** contracte A0, accions A3, política A1.

- Reorganitzar prioritats i separar resultats, decisions i informació sense acció.
- Mateix resum a portada i Avui; cap llista paral·lela calculada amb altres criteris.
- «Fet», «Ajornar», «Pausa», «Menys d'això» amb rollback/feedback real. Preguntes obertes enllacen al fil exacte.
- Treure controls extensos d'hores tranquil·les del final permanent de la safata i posar-los a configuració accessible.
- Fitxers: `VentsAvui.kt`, `winds_avui.go`, tests relacionats; `CompanyScreen`/ViewModel via A0.
- Acceptació: repetir l'acció no duplica; errors no perden targetes; silenci funciona segons origen i preferència; safata buida diu que està al dia sense fabricar feina.

### A5 — Fiabilitat Android, offline i notificacions (P0/P1, L)

**Propietari:** plataforma Android. **Dependències:** A0; usa IDs A3/A4.

- Matriu local/remot, permís denegat, reinici, Doze, canvi de zona, pèrdua de xarxa i canvi de compte.
- Auditar cursor de novetats i actualització només després del tractament adequat; deduplicar mirall i avís remot.
- Validar les accions de notificació i deep links amb origen/compte correctes i intents repetits.
- Mesurar polling, bateria i latència. Presentar una recomanació de transport amb evidència, sense introduir infraestructura preventiva.
- Fitxers: `WindWatch.kt`, `JobAlarms.kt`, `VentsOffline.kt`, `NotificationOrigins.kt`, `GregalNotifications.kt`, manifest; integrador per activitat.
- Acceptació: cap avís d'un compte a un altre; reinici reprograma; recordatori mirall + recuperació genera una sola notificació; denegació explica el límit; force-stop no es presenta com una situació de lliurament garantit.

### A6 — Cerca i feines que tornen amb resultat (P1, M)

**Propietari:** agent/UX de feina. **Dependències:** A0, components A2.

- Provar intencions de cerca actual amb eines reals simulades i fallades; conservar els límits actuals de passos.
- Presentació de fonts i una línia d'activitat real. Resultat parcial explícit si s'ha aturat o esgota pressupost.
- Connectar feina, conversa, Avui i resultat amb IDs existents; no afegir un segon runner.
- Fitxers: `internal/companion/winds.go`, `internal/web/plan.go`, `winds_action.go`, `FeinesScreen.kt`, `ChatWork.kt`, `WindMessageUi.kt`.
- Acceptació: una consulta actual dispara cerca; coneixement estable no la força; cancel·lació real; errors de fonts no produeixen certeses inventades; resultat recuperable després de tancar/reobrir la vista.

### A7 — Primera experiència, memòria, dictat i compartir (P2, M)

**Propietari:** ús quotidià. **Dependències:** A0/A2; separar en PR petits.

- Primera experiència sense configurar una colla: connexió, prova i primera petició. Mantenir el selector de proveïdor existent i no prometre un servei allotjat que no existeix.
- Connectors explicats per ús: «Resum del calendari», «Resum del correu»; configuració/prova amb errors accionables, sense OAuth nou en aquest paquet.
- Ordenar memòria i accés a correcció; sense canviar silenciosament la semàntica dels magatzems.
- Dictat i lectura amb error visible; compartir text amb acció escollida i preservació de la nota actual.
- Fitxers: `ProviderSetup.kt`, `PrefsScreen.kt`, `WindProfileScreen.kt`, `VentsVoice.kt`, `Notes.kt`; `MainActivity` via A0.
- Acceptació: primera petició útil sense crear especialistes; motor de veu absent no deixa un control mort; text dictat no s'envia sol; no es perd cap captura compartida.

### A8 — QA, rendiment i entrega (P1, M)

**Propietari:** verificació. **Dependències:** revisions integrades.

- Executar la matriu de la secció 9 i les proves de regressió pertinents.
- Revisar scroll, IME, restauració d'estat, TalkBack, rotació i cancel·lació; comparar captures abans/després.
- Mesurar arrencada freda, temps fins a compositor, fluïdesa amb fils llargs, latència de notificació i consum del pilot. Registrar dispositiu/build/condicions.
- No modificar assertions per adaptar-les cegament al nou disseny; conservar comportaments importants i actualitzar només expectatives intencionadament canviades.
- Acceptació: cap P0/P1 obert del recorregut principal; APK amb backend de la mateixa revisió, ABI/package/signatura coherents; comprovació OTA separada. No publicar sense l'encàrrec corresponent.

## 8. Ordre, dependències i límits d'abast

Talles orientatives: S = 0,5–1,5 jornades, M = 2–4, L = 4–7; són estimacions d'esforç amb revisió i proves, no terminis garantits. A3/A5 concentren més incertesa.

1. **Entrega de fiabilitat:** A0 + correccions A1 + diagnòstic A5. Sense redisseny gran. Pot sortir sola.
2. **Entrega d'assistent útil:** A2 + A3 + A4 + A6, integració incremental per recorregut. Prioritat a recordatori, cerca i retorn de feina.
3. **Entrega de comoditat:** A7 i poliment d'A2; mesurar si realment cal push o veu contínua abans d'obrir aquests projectes.
4. **A8 verifica cada entrega**, no només l'última.

Paral·lelisme segur suggerit: màxim tres implementadors + integrador. A1 backend, A2 UI i A5 plataforma poden avançar alhora amb propietat clara. A3 i A4 no escriuen simultàniament contractes compartits. A6 ha d'esperar que s'estabilitzin els canvis actuals de streaming/planificació.

Instrucció comuna per copiar a cada agent:

> Implementa només el paquet assignat d'aquest pla. Llegeix primer els canvis locals i els contractes actuals. Reutilitza models, rutes, scheduler i components existents. No afegeixis una dependència o un nou servei si el problema es resol amb aquestes peces. No modifiquis els fitxers reservats de l'integrador sense coordinació. Conserva IDs, orígens i historials. Retorna canvi, proves, captures quan correspongui i limitacions observades. Si el codi ja resol un punt, verifica'l i evita duplicar-lo.

### Fora de la primera entrega

- Marketplace de plugins, motor de workflows nou, framework multiagent nou o migració integral d'arquitectura.
- Nova base vectorial, memòria automàtica invasiva o vigilàncies inferides sense encarregar-les.
- Avatar 3D, Rive com a dependència de producció o animacions contínues; el prototip existent queda experimental.
- Veu dúplex permanent, wake word, escolta en segon pla o trucades autònomes.
- Ampliar jocs/Racó, refer totes les pantalles de codi o afegir dashboards de mètriques al producte.
- Infraestructura push abans de mesurar; nova plataforma de connectors abans d'acabar calendari/correu existents.

## 9. Acceptació i verificació

### Escenaris de producte obligatoris

| Escenari | Resultat esperat |
|---|---|
| Instal·lació nova, backend local i proveïdor sense configurar | Guia curta, prova de connexió, error útil; no un 401 cru |
| «Recorda-m'ho demà a les 9» | Rebut real, hora local i zona; editar/cancel·lar pel mateix ID |
| «Ajornar 1 h» dos cops per reintent de xarxa | Una sola operació efectiva per clau; UI coherent |
| «Busca tres portàtils per sota de…» | Cerca real, fonts, comparació llegible, sense inventar preus |
| Seguiment sense novetats | Cap missatge ni notificació; propera comprovació vàlida |
| Mateixa novetat en dos controls | Un sol avís; historial de comprovacions conservat |
| Feina llarga, app en segon pla i reobertura | Estat i resultat recuperables; cancel·lació funcional |
| Hores tranquil·les | Informació ordinària a la safata; recordatoris segons preferència explícita |
| Remot apagat a l'hora del recordatori | Mirror quan sigui aplicable; no doble avís en recuperar |
| Canvi de compte/servidor amb resposta tardana | No repobla dades ni notificacions del nou origen |
| Sense xarxa després d'un enviament ambigu | No duplicar una acció irreversible; estat incert/reconciliació quan calgui |
| Notificacions denegades o app forçada a aturar | Límit explicat; no promesa de puntualitat impossible |
| Font 200%, TalkBack, amplada estreta, teclat | Contingut/accions accessibles; no ocultar informació amb truncament |
| Obrir avís, enrere, canviar pestanya | Fil correcte, retorn correcte i borrador conservat |
| Actualització sobre instal·lació prèvia | Historial, IDs, programacions, preferències i origen conservats |

### Proves automatitzades

Executar proves existents afectades: `AvuiTest`, `AvuiTrayUiTest`, `ProgramatsTest`, `AutomatitzacionsTest`, `WindWatchTest`, `WindWatchDueTest`, `JobScheduleTest`, `VentsOfflineTest`, `BackStackTest`, `CompanyNavigationTest`, `VentsAskHomeTest`, `VentsChatChromeTest`, `ProviderSetupTest` i les de streaming/scroll. Afegir proves de comportament per les regressions concretes, no tests que només busquin cadenes al font.

```powershell
# Android: tests JVM/Robolectric; no valida el backend empaquetat
cd android
# En aquest entorn l'SDK existeix a C:\Android, però ANDROID_HOME no estava definit.
$env:ANDROID_HOME='C:\Android'
.\gradlew.bat testDebugUnitTest '-Pgregal.skipBackend' '-Pgregal.plotsPreview'

# Des de l'arrel: subset exploratori executat en aquesta revisió
go test ./internal/web -run 'TestWindHeartbeat|TestAvui|TestWindsAutomations' -count=1
# Comprovació addicional: el test d'automatitzacions té aquest nom concret
go test ./internal/web -run 'TestAutomationsUnifiedList' -count=1
```

Abans d'entrega real, A8 ha d'executar els paquets Go modificats complets i les proves de dispositiu amb SDK/backend disponibles. El regex exploratori no és garantia de cobertura completa d'Avui/automatitzacions.

Objectius inicials del pilot, a mesurar i ajustar: compositor visible ràpid amb cache sense esperar xarxa; cap duplicat en escenaris de reintent; zero avisos sense contingut; cap pèrdua de borrador; reducció de passos per editar un recordatori; relació de feedback útil/no útil sobre proactivitat. Registrar diagnòstic mínim, sense enviar contingut privat a un servei d'analítica nou.

## 10. Referents consultats

Consulta: 11 de novembre de 2026. S'han usat pàgines oficials de producte/documentació, no s'ha fet una prova de primera mà de les apps competidores. Les decisions visuals d'aquest pla són propostes pròpies.

- [Claude Duet](https://claude.com/product/claude-duet): conversa contínua, treball que torna acabat i avisos quan toca revisar o decidir. Traslladar-ne la continuïtat i el respecte per l'atenció, sense assumir que Gregal ja té la seva infraestructura.
- [Grok i Grok Bot](https://docs.x.ai/grok/overview): combina conversa, veu, adjunts i accés a eines; presenta Bot com a delegació de feina en un ordinador persistent. Interessa l'accés directe a fer coses, no afegir personatges per omplir la portada.
- [OpenAI dots](https://chatgpt.com/features/dots/): responsabilitats persistents, progrés entre converses i treball revisable. Aplicació a Gregal: propietari clar d'un encàrrec, estat recuperable i control de pausa.
- [Android: alarmes](https://developer.android.com/develop/background-work/services/alarms): reservar precisió temporal per funcionalitats que la requereixin; verificar permisos i fallbacks segons versió.
- [Android: límits dels foreground services](https://developer.android.com/develop/background-work/services/fgs/timeout): revisar límits de `dataSync` en versions/targets nous; evitar promeses d'execució il·limitada al mòbil.
- [Compose: accessibilitat per defecte](https://developer.android.com/develop/ui/compose/accessibility/api-defaults): verificar àrees tàctils efectives i evitar solapaments; l'extensió automàtica de Compose no substitueix QA real.

## 11. Estat de verificació d'aquesta revisió

- Inventari i inspecció estàtica: realitzats segons l'abast de la secció 2.
- Backend, dues ordres exploratòries de la secció 9: **PASS** (`ok gregal/internal/web`). No verifiquen el cas nou F01, que requereix la prova de regressió d'A1.
- Android JVM/Robolectric: **BUILD SUCCESSFUL**, amb `:app:testDebugUnitTest UP-TO-DATE`; Gradle ha reutilitzat resultats vigents, no ha reexecutat tots els tests en aquesta revisió. Primer s'han corregit la citació dels flags en PowerShell i `ANDROID_HOME` només per al procés, sense canviar configuració del repositori.
- Informes Android reutilitzats: 105 suites, 514 tests, 0 fallades, 0 errors i 0 omesos. Són resultats de la cache validada per Gradle, no 514 execucions noves.
- Dispositiu, bateria, contrast renderitzat i interacció real: pendents; cap dispositiu connectat.
- Maqueta: comprovats sintaxi JavaScript, unicitat d'IDs, destinacions de navegació i HTML literal; no equival a validar Compose ni renderitzat en dispositiu.
- Implementació funcional: no iniciada. Aquest lliurable és el pla per a altres agents.
