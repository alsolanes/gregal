# Pla de millora de la portada

## Diagnòstic

- En estat buit competeixen almenys 7 capes: barra superior, pestanyes, fila de cares, salutació, compositor i el bloc `TodaySection` (amb fins a 3 targetes) més el resum del matí.
- La barra superior i les pestanyes són navegació, però la fila de cares sembla una segona navegació: no queda clar si Inici mostra el Ventet, els vents o les converses.
- La fila horitzontal ocupa molt espai i té diversos estats visuals; empeny la salutació i el compositor, que haurien de ser el centre de l'acció.
- Quan hi ha conversa, `CompanyScreen` encara conserva la barra/pestanyes i `VentsOpening` passa a mostrar capçalera + `VentetThread` + compositor: dues pantalles se superposen conceptualment.
- A la captura amb cares, `Ventet` apareix entre la fila de personatges i el missatge: és una tercera capçalera, mentre que el fil ja hauria d'explicar qui parla.
- A la captura sense cares, desapareix una peça però queda el mateix problema: el Ventet, el fil, la delegació i la feina en marxa competeixen sense una entrada única ni una conclusió destacada.
- `TodaySection` barreja novetats, feina, recordatoris i briefing sota la salutació; és informació útil, però no és el mateix nivell que “parlar amb la colla”.

## Pla prioritzat

### P0 — Separar inequívocament Inici buit i conversa

**Ara** → `home` decideix entre `VentsOpening` buit i el mateix `VentsOpening` amb `chat`, però manté el chrome de la portada i canvia massa coses dins del composable.

**Hauria de ser** → dos modes visibles: “Inici” = una sola invitació a parlar amb el Ventet; “Conversa” = fil del Ventet amb context, sense cares, salutació ni targetes d'avui.

**Per què** → el primer toc ha de tenir un protagonista; obrir una conversa no pot semblar que s'ha afegit un fil a una portada existent.

**Canvis concrets** →

- `CompanyScreen.kt`, branca `val home`: derivar un `HomeMode.Empty/Chat` a partir de `capSt.history`/`capUi.busy` i passar-lo explícitament; en `Chat`, no construir ni `HomeFacesRow` ni `TodaySection`.
- `VentsAskHome.kt`, `VentsOpening`: dividir internament la branca `chat != null` en una superfície de conversa completa; conservar només `chatHeader`, `VentetThread` i `AskComposer`, amb `imePadding` i scroll que segueixi el final.
- `CompanyScreen.kt`, capçalera de la pantalla: decidir si les pestanyes es mantenen com a navegació fixa o es redueixen a una barra compacta en `Chat`; no deixar que el fil aparegui sota una fila de cares o una capçalera d'Inici.
- `VentetChat.kt`, `VentetThread`: mantenir el fil únic compartit, però fer visibles al seu lloc les propostes, acords i `CapLinkCard`; el fil ha de ser la font de veritat, no un resum duplicat a la portada.

### P0 — Fer de la portada buida una única invitació

**Ara** → `VentsOpening` mostra cares a dalt, una salutació al centre, `TodaySection` a sota i el compositor al final.

**Hauria de ser** → salutació + compositor com a bloc hero; l'activitat queda com una sola secció secundària, sota demanda o molt més discreta.

**Per què** → la pantalla ha de respondre en un segon “què hi puc fer ara?”: parlar amb el coordinador.

**Canvis concrets** →

- `VentsHomeParts.kt`, `HomeFacesRow`: treure-la de la portada per defecte; conservar-la al directori/calaix i oferir “Tots” com a acció de menú. Si es vol accés ràpid, substituir-la per una sola pastilla del Ventet.
- `VentsAskHome.kt`, `VentsOpening`: fer que `top` sigui opcional i no estructural; reservar l'espai vertical principal a `OpeningGreeting` i `AskComposer`.
- `VentsAskHome.kt`, `TodaySection`: convertir-la en una secció “Per reprendre” amb un màxim de 1–2 entrades, sense repetir “Avui” ni el briefing com una segona portada. El resum del matí només apareix si té contingut accionable.

### P1 — Crear la pastilla de context del Ventet

**Ara** → `HomeThreadHeader` és una fila amb cara, nom i estat; el nom/model no queda tractat com el context de la conversa i l'estat pot semblar una barra afegida.

**Hauria de ser** → una pastilla compacta amb cara, `Ventet`, model/rol i estat integrat en el fons/halo, amb un únic accent.

**Per què** → dona orientació immediata sense introduir una altra capçalera i fa coherent el retorn d'Inici a Conversa.

**Canvis concrets** →

- `VentsAskHome.kt`, `HomeThreadHeader`: convertir la `Row` en una superfície `GregalShape.pill` de baixa alçada; usar `GregalType.heading/bodySmall`, un sol accent del Ventet i estat curt (“Pensant…”, “A punt”).
- `VentsSpectacle.kt`, `WindHalo`: aprofitar `mood`/animació com a estat visual; no afegir indicadors independents ni rings decoratius.
- `AppStrings.kt`: afegir les variants catalanes necessàries per a model/rol i estat; evitar “Open the chat”, placeholders i frases repetides en aquesta zona.

### P1 — Fer visibles els resultats, no només les notificacions

**Ara** → comptadors a pestanyes i `CapDecisionDone` en petit; la feina en marxa pot quedar com una línia i el resultat principal es perd entre el text del fil.

**Hauria de ser** → pendents amb número, però acords/conclusions en una targeta o nota curta i llegible, amb la següent acció clara.

**Per què** → el valor de Ventet és coordinar; l'usuari ha de veure què s'ha decidit sense reconstruir-ho llegint tot el xat.

**Canvis concrets** →

- `VentetChat.kt`, `CapDecisionDone` i `CapNote`: elevar l'acord resolt a una fila destacada del fil, amb qui ho fa, estat i enllaç a la conversa resultant.
- `VentsHomeParts.kt`, `RunningWorkLine`: mostrar “N feines en marxa” només quan existeix i amb accés d'una sola icona/gest; no competir amb el hero buit.
- `VentsAskHome.kt`, `AskTabs`: mantenir números només per pendents reals (`unread`/`today`) i no usar badges per comunicar estats que ja tenen una targeta visible.

### P2 — Polir densitat, microdetalls i copy

**Ara** → hi ha moltes superfícies i escales simultànies; el compositor usa “Pregunta el que vulguis” però altres branques encara exposen textos anglesos o duplicats.

**Hauria de ser** → una jerarquia de tinta/Muted/accent, icones soles per a accions i copy català consistent.

**Canvis concrets** →

- `Theme.kt`: reutilitzar `GregalType` i la paleta existent; només ajustar espaiat, alçades i pesos dels composables de portada, sense crear una identitat nova.
- `VentsAskHome.kt`, `AskComposer` i `TodayCardView`: reduir vores/ombres i deixar l'accent per a focus, enviament i estat; mantenir resposta tàctil immediata.
- `AppStrings.kt`: revisar claus de la portada i xat per eliminar anglès cru, frases repetides i placeholders; conservar `ca/en`.

## Decisions que ha de prendre el propietari

1. En obrir una conversa, les pestanyes “Inici / Avui / Xats” han de quedar visibles com a navegació fixa, o el xat ha d'ocupar també aquesta franja?
2. La portada buida ha d'oferir accés ràpid als vents amb una sola pastilla del Ventet, o els vents han de viure exclusivament a “Xats”/directori?
3. Quin resultat és prioritari a la portada: el briefing del matí o les feines/acords pendents? Només un pot ser el bloc secundari principal.

## Validació

- Fer una captura real en quatre estats: buit, amb pendents, conversa oberta i resposta amb delegació.
- Executar `./gradlew testDebugUnitTest -Pgregal.plotsPreview` i `./gradlew assembleRelease -Pgregal.plotsPreview`.
- Revisar a la captura que no hi hagi solapament, scroll inicial inesperat, text anglès ni cap badge sense una acció associada.
