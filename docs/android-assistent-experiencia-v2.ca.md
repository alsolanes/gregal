# Gregal personal: renovació completa de l'experiència

Pla vigent per a implementació · revisió 2.1 · 11 de novembre de 2026

## 1. El canvi de direcció

**Gregal és un assistent personal amb conversa, responsabilitats i ordinador propi.** Pot investigar, preparar resultats, fer feina autoritzada i continuar quan l'usuari no hi és. El mòbil és el lloc on parlar-hi, veure què ha fet i intervenir quan cal.

El primer pla millorava una col·lecció de pantalles. Aquest pla defineix un producte continu: la persona connecta les eines, encarrega una responsabilitat i rep feina feta. L'assistent no necessita que li preguntin constantment si hi ha novetats.

No cal que l'usuari entengui «cors», «jobs», «rotllanes», «MCP», modes d'inspecció ni planificadors. Aquestes peces poden continuar al codi, però el producte ha de parlar de converses, feina, connexions i resultats.

### Decisions que substitueixen el pla anterior

| Abans | Ara |
|---|---|
| Portada amb salutació i botons per demanar | Obrir directament la conversa principal, amb context i aportacions reals |
| Assistent · Avui · Xats com a tres superfícies principals | Conversa principal; accessos contextuals a feina, ordinador i connexions; historial al menú |
| Resposta de l'assistent sempre sense caixa | Conversa personal amb bombolles suaus; documents i respostes llargues amb superfície de lectura apropiada |
| Resum a la portada i safata separada | Un resultat canònic en el fil, amb projecció a l'activitat; no duplicar missatges |
| Proactivitat centrada a avisar de novetats | Detectar una necessitat, fer la preparació útil i tornar amb un resultat o decisió |
| Ordinador i connexions àmplies ajornats | Ordinador persistent i recorregut de connexió formen part del producte central |
| Push només si el pilot ho demana | Lliurament en segon pla amb objectiu de latència explícit; transport definit abans de l'entrega completa |
| Poliment incremental com a entrega final | Primera entrega vertical completa, després extensió de capacitats |

S'ha acceptat el radi de targetes **15 px a la maqueta**, que es tradueix inicialment a **15 dp en Compose**. No significa que totes les bombolles o el compositor hagin de tenir el mateix radi.

## 2. Referents, evidència visual i límits

### Què adoptem de dots

L'article oficial descriu ordinador propi inspeccionable, treball persistent i context entre interaccions. També diferencia exploració proactiva de lectura de feina amb accions: autonomia no vol dir actuar sense límits sobre qualsevol compte. [Presentem els dots](https://openai.com/ca-ES/index/introducing-dots/).

S'han inspeccionat visualment dos recursos oficials de la [pàgina de dots](https://chatgpt.com/features/dots/): conversa mòbil i connexions. En el primer s'observa identitat central amb avatar, menú i trucada laterals, bombolles grises de l'assistent, bombolla blava de l'usuari i un document lliurat al mateix fil. En el segon, connexions amb icona, nom i descripció curta. Aquestes observacions són sobre imatges de presentació, no una auditoria de l'app real.

Aplicació a Gregal: identitat persistent i propera; xat personal; resultat tangible dins la conversa; connexions comprensibles d'un cop d'ull. No copiar mascota, marca ni colors exactes.

### Què adoptem de Grok Bot

La pàgina oficial proposa connectar-se a eines iniciant sessió, rutines i feina continuada. La FAQ especifica un ordinador persistent compartit pels bots d'un mateix usuari, amb fitxers, navegador i sessions; l'aïllament és per usuari. També mostra cessió de control durant el login i ensenyar una feina per convertir-la en rutina. [Grok Bot](https://x.ai/bot).

Aplicació a Gregal: un ordinador per persona, compte connectat reutilitzable, «pren el control» quan toca i continuació després. Els especialistes comparteixen recursos de la mateixa persona; no crear una màquina per cada vent.

### Limitació d'accés

Les dues pàgines d'entrada van retornar HTTP 403 al navegador automatitzat. S'ha pogut llegir el contingut públic amb l'eina web i inspeccionar els recursos gràfics oficials de dots al CDN. No s'han observat visualment els vídeos/interaccions de Grok Bot ni es presenten detalls inventats com si s'haguessin provat.

## 3. Una experiència completa, descrita amb un cas

Responsabilitat d'exemple: «Ajuda'm a preparar el viatge de divendres. Mira les reserves i avisa'm si falta res.»

1. Gregal identifica les eines disponibles. Si falta accés, mostra dins del fil una targeta «Connecta el correu» amb què necessita llegir i per què.
2. La persona connecta el compte pel flux adequat. Gregal comprova l'accés i reprèn l'encàrrec original; no demana que es torni a escriure.
3. Consulta reserves i informació actual. Desa al seu ordinador un itinerari o document útil. L'activitat es pot desplegar, però el fil no es converteix en un log d'eines.
4. L'usuari tanca l'app. La feina continua al servidor/ordinador personal.
5. Gregal detecta que falta el trasllat, compara opcions i deixa una proposta amb fonts i preus consultats. Pot preparar el document dins l'àmbit encarregat sense preguntar cada pas.
6. En arribar una reserva actualitzada, reconcilia el canvi i actualitza el resultat autoritzat. Escriu una aportació curta amb el canvi important.
7. Si cal comprar, reservar o comunicar una cosa no autoritzada, presenta la proposta concreta en línia. No simula que l'acció ja està feta.
8. La notificació, si correspon, conté la novetat i obre aquest punt exacte del fil. En tornar a l'app, no hi ha un popup intermedi de «tens notificacions».

No s'ha de donar aquest escenari per complet amb una maqueta, un prompt més llarg o un cron que envia text. Ha de funcionar amb execució, persistència, connexió, recuperació i resultat real.

## 4. Disseny del xat nou

### Entrada i capçalera

- Destinació inicial: fil de l'assistent principal. Una instal·lació nova veu una presentació breu de Gregal i la primera acció útil, no un formulari llarg de personalitats.
- Avatar original de Gregal, expressiu però compacte, al centre o agrupat amb el nom. Menú a l'esquerra; ordinador i àudio a la dreta quan estiguin disponibles.
- Nom personalitzable i estat discret, basat en feina real. «Preparant l'itinerari» té més valor que «En línia» permanent.
- No mostrar una icona de trucada si només tenim dictat. El control actual es diu «Dicta»; la veu contínua serà una capacitat diferent.
- Historial i especialistes al menú; un nou encàrrec dins del fil principal no obliga a crear ni canviar de xat.

### Missatges

- Bombolla de l'assistent amb fons neutre molt suau; usuari a la dreta amb accent marí. Grup de missatges consecutius sense repetir avatar i hora a cada línia.
- Cos 16 sp, interlínia 23–24 sp, ample adaptable; text secundari 12–13 sp llegible. Padding de bombolla 12–16 dp.
- Radis de bombolla 20–24 dp, targetes de resultats 15 dp, compositor 26–30 dp. Una sola família d'icones i accents coherents clar/fosc.
- Missatges curts per avenços significatius; resposta llarga només quan el resultat ho necessita. No fragmentar artificialment una mateixa resposta en deu missatges.
- Resultats amb miniatura/tipus, títol, breu contingut útil i acció d'obrir. Fitxer real i versió real; no una caixa decorativa que només repeteix la resposta.
- Citacions al costat de la informació o en detall desplegable. Les fonts i el moment de consulta formen part del resultat, no d'un popup.
- Propostes amb una acció principal i alternatives contextuals. Aprovació, pregunta i rebut al mateix fil amb estat durable.
- Alerta proactiva: mateixa autoria i llenguatge de conversa, amb un petit «He revisat…» o «Sobre el viatge». El separador «Novetats des de l'última visita» és local, no un altre missatge del model.

### Compositor i feina simultània

- Compositor fix a la zona del polze: text, adjunts, dictat, enviar. Aturar és contextual a la feina seleccionada.
- La persona pot escriure mentre una feina treballa. El sistema distingeix nou encàrrec, correcció i resposta a una pregunta pendent; no envia cegament qualsevol missatge a l'últim job.
- Una franja discreta «2 feines en marxa» obre activitat. No bloquejar la conversa perquè un navegador o una cerca estigui ocupat.
- Conservació de draft, IME, scroll manual i lectura des de l'inici de resposta. Les novetats no arrosseguen l'usuari al final si està llegint més amunt.

### Superfícies auxiliars

| Accés | Què mostra | Presentació |
|---|---|---|
| Feina | En marxa, esperant-te, resultats, responsabilitats actives | Full/pantalla amb enllaços al mateix fil |
| Ordinador | Escriptori/navegador, fitxers, qui té el control, pausa | Pantalla completa amb visor remot autenticat |
| Connexions | Apps connectades, compte, accés i salut | Llista de files netes, cercador quan el catàleg ho justifiqui |
| Programacions | Recordatoris i rutines | Vista única ja prevista al primer pla |
| Personalitza | Nom, aparença, veu disponible, com prefereixes treballar | Configuració opcional; cap pantalla inicial obligatòria |

«Avui» es conserva com a projecció secundària de feina i pendents, sense competir amb el fil. No s'esborren dades ni es trenquen deep links antics.

### Del xat actual al nou: canvis concrets a Compose

Punt de partida verificat al codi (11/11): la resposta de l'assistent es pinta **sense caixa** (`WindBubble`, `CompanyScreen.kt` ~l. 1840, `color = Color.Transparent`, 15 sp / 22 sp); la bombolla de l'usuari és `Sea` amb radi 20/20/20/6 i ample màxim 300 dp; la capçalera (`VentsChatHeader`, `VentsChatChrome.kt:49`) alinea cara + nom + subtítol a l'esquerra; el compositor (`VentsComposer`, `VentsChatChrome.kt:133`) té radi 28 dp i text 15 sp; les novetats entren com a `CapNote(st.brief)` al final del fil (`VentetThread`, `VentetChat.kt`), i la portada `VentsOpening`/`OpeningGreeting` (`VentsAskHome.kt`) fa de pantalla intermèdia.

| Element | Ara | Objectiu B1 | On |
|---|---|---|---|
| Bombolla assistent | Transparent, amplada completa | `SuperficieAlta` (`surfaceHi`: clar #E5E8F0, fosc #242833; si en clar resulta pesant, provar-la al 70 %), radi 22 dp amb cua 6 dp a baix-esquerra només a l'últim de la ratxa, ample màx. 84 % / 520 dp. Taules, codi i respostes > ~900 caràcters passen a «superfície de lectura» sense bombolla (regla en una funció pura testejable) | `WindBubble` |
| Bombolla usuari | `Sea`, 15 sp | Mateix accent, 16 sp / 23 sp, ample màx. 78 %, cua només a l'última de la ratxa | `WindBubble` |
| Ratxes | Separació 16/4 dp | 3 dp dins la ratxa, 14 dp entre parlants; hora una vegada per ratxa o separador «Avui · 09:41» si han passat > 30 min | `VentetThread`, `runKeyOf` ja existeix |
| Capçalera | Fila esquerra | Avatar 44 dp centrat + nom 15 sp SemiBold a sota; botó menú (esquerra) i ordinador (dreta) de 44 dp circulars amb vora `Line`; subtítol només amb feina real («Preparant l'itinerari») | `VentsChatHeader` |
| Compositor | 28 dp, 15 sp | 28 dp es manté, text 16 sp, botó enviar 40 dp ple `Ink`/`OnInk`; adjunts dins el «+»; franja «2 feines en marxa» de 32 dp just a sobre, tocable | `VentsComposer`, `VentsWorkProgress` |
| Resultat lliurat | `CapLinkCard` 16 dp genèrica | `ResultCard` nova: icona de tipus 40 dp, títol 15 sp SemiBold, subtítol 12 sp, xevró; radi **15 dp**, vora 1 dp `Line`, fons `Superficie`; sota, línia 12 sp «✓ Preparat al seu ordinador · Veure feina» | fitxer nou `ResultCard.kt` |
| Proposta/decisió | `Surface` tenyida + `CapDecisionRow` | Botó principal *outline* amb accent (radi 15 dp) i alternatives com a text; quan es resol, es converteix en rebut d'una línia, no desapareix | `VentetChat.kt` |
| Novetats | `CapNote(st.brief)` al final | Separador local «Una novetat per a tu» davant del primer missatge no llegit; el missatge proactiu és una bombolla normal amb prefix curt («Sobre el viatge ·») | `VentetThread` |
| Portada | `VentsOpening` amb salutació | Si hi ha historial, s'obre el fil directament. Instal·lació nova: el mateix fil amb un missatge de presentació i 2–3 suggeriments, no una pantalla a part | `VentsAskHome.kt`, `VentsActivity.kt` |

Tokens a `Theme.kt` (`GregalShape`, l. 174, ja té `xs 8 · sm 12 · md 14 · lg 20 · xl 28 · pill`): afegir `bubble = 22.dp` (amb variant de cua 6 dp) i `card = 15.dp` —o bé moure `md` de 14 a 15 dp si la revisió visual de la resta de pantalles ho accepta—; el compositor continua amb `xl`; controls circulars de 44 dp. Tipografia del fil: `chatBody = 16.sp/23.sp`, `chatMeta = 12.sp/16.sp`. Cap valor nou hardcodejat fora d'aquests tokens; totes les paletes de `GregalTemes` (Nit, Calma, Paper, Grafit, en clar i fosc) han de passar contrast AA amb les dues bombolles.

Moviment: entrada de missatge nou amb `fadeIn + slideInVertically(8.dp)` de 180 ms; streaming sense salts de mida (ja resolt amb `richBlocksLive`, no regressar-hi); cap animació en bucle a l'avatar mentre l'usuari llegeix. Respectar «reduir animacions» del sistema.

Què s'elimina o s'amaga al recorregut principal: `CapNote` com a resum, l'estat `WindMood.NEWS` com a reclam, `AskTabs` com a navegació primària, la petició de permís de `MainActivity.kt:197` i `CompanyScreen.kt:186-198`. El codi pot quedar si altres pantalles el fan servir; el que no pot és aparèixer al camí principal.

## 5. Ordinador personal: decisió tècnica concreta

### Primera arquitectura que implementarem

**Revisat l'11/11:** dues peces separades, no una.

- **Amfitrió (sempre encès, lleuger):** el servidor Gregal amb els comptes, les converses, l'estat durable (decisions, encàrrecs, connexions), els esdeveniments, els avisos, el gateway i la **publicació de webs**. És el que veu Android i és l'únic que ha d'estar sempre disponible.
- **Ordinador (un per persona, s'encén quan cal):** una VM Linux amb el runner, navegador amb perfil persistent, escriptori lleuger i carpeta de treball. Es pot aturar o hibernar quan no s'usa. L'amfitrió l'encén quan una feina el necessita (wake on demand), i l'atura passat un temps d'inactivitat configurable.

Quan l'ordinador està aturat, la conversa, els resultats ja lliurats, les decisions pendents i les webs publicades continuen funcionant. Només esperen les feines que necessiten navegador o terminal; passen a `waiting_computer` i el fil ho diu. El pilot són dos comptes, la Carla i l'Aleix, cadascun amb el seu ordinador.

**Política d'encesa (requisit de l'usuari: poden passar moltes hores entre sessions i no es vol gastar res inútilment):**

- **Estat normal: ordinador desat a disc** (suspensió/hibernació de la VM, o aturada si l'hipervisor no ho permet). Zero CPU i zero RAM; només ocupa disc. Les sessions del navegador i els fitxers es conserven.
- **S'encén només quan una feina necessita navegador, terminal o fitxers de l'ordinador**, o quan la persona l'obre per mirar o prendre el control. Obrir l'app, xatejar, llegir resultats, aprovar decisions o veure webs publicades **no** l'encén.
- **La proactivitat no l'encén per defecte.** Les comprovacions periòdiques (cerques web, correu per IMAP, calendari ICS, connectors API) corren a l'amfitrió, com fa avui el cor dels vents. Només una responsabilitat que necessiti un lloc amb sessió iniciada al navegador desperta l'ordinador, i agrupa aquestes mirades perquè no el desperti cada poc.
- **Es desa sol passats 10 minuts sense feina ni visor obert** (configurable per compte). Abans de desar-se, acaba o posa en espera la feina en curs.
- **Represa visible:** si una feina el necessita i està desat, l'encàrrec passa a `waiting_computer`, el fil diu «Engegant el teu ordinador…» i continua sol quan està a punt. Objectiu del pilot: ≤ 30 s des de suspensió.
- **Mesura:** l'amfitrió registra hores enceses per ordinador i per què s'han encès. Es mostren a la pantalla de l'ordinador perquè es vegi que no gasta en buit.

La primera entrega no necessita un servei de provisioning multi-cloud. Tampoc es pot presentar com a «ordinador personal» una pestanya headless sense fitxers, persistència, visibilitat ni recuperació.

### Capacitats mínimes de l'ordinador

- Navegació, lectura, clic, formularis, fitxers i descàrregues amb les eines existents.
- Terminal/execució només dins l'ordinador dedicat i l'abast autoritzat, reutilitzant el runner i sandbox existents.
- Directori de resultats persistent i accessible des d'Android amb control d'accés, versió i MIME.
- Visor remot per veure i prendre el control. Proposta inicial: un transport consolidat com noVNC darrere del gateway autenticat; l'agent continua usant CDP per al navegador. No exposar VNC/CDP directament a Internet.
- Un sol propietari del control interactiu: agent o persona. Lease curt amb renovació i revocació; prendre el control pausa les accions de l'agent, no només la imatge.
- Fitxers i sessió sobreviuen al reinici. El navegador usa un perfil per compte/ordinador, amb pestanyes per feina i exclusió de mutacions simultànies incompatibles.
- Desconnectar una app revoca l'accés des de Gregal i tracta la sessió del navegador explícitament. No confondre «treure una icona» amb tancar un compte.

### Què ja existeix i què falta

| Peça | Evidència actual | Treball necessari |
|---|---|---|
| Navegador propi | `internal/tools/browser.go`: Chrome/Edge + CDP + perfil a disc | Worker dedicat, salut, exposició segura al mòbil |
| Perfils per sessió | `launchChromeScoped` deriva subdirectori del scope | Definir compte/ordinador com a propietari de login; no un login nou per feina; migrar sense barrejar comptes |
| Cessió de control | `browser_handoff.go` funciona en modes visibles i pont desktop; rebutja headless | Handoff remot real amb Android, lease, represa i expiració |
| Sandbox | `internal/sandbox` té backends com bubblewrap/Landlock | Usar-los dins el worker; no assumir que aïllen un servei multiusuari complet |
| Fitxers/resultats | Workspace, previews i apps existents | Adreça canònica del resultat, versions, descàrrega i vista mòbil |
| Eines d'escriptura | Runner i gates existents; mode interactiu diferenciat a `plan.go` | Execució durable vinculada a encàrrec; cap desactivació global de gates |
| Esdeveniments | `/api/v2/events`, stream i cursor ja tenen proves | Extensió/reutilització per feina de l'assistent; no un segon bus d'esdeveniments |

### Límits del pilot i extensió

Primer, navegador + terminal + fitxers al mateix ordinador visible; no prometre automàticament control universal de totes les apps natives. Si un servei no admet login en aquest entorn, mostrar que no està suportat i oferir connector quan existeixi.

L'ordinador propi de l'usuari és una connexió addicional i explícita, no el valor per defecte. El backend local Android es manté per casos locals, amb capacitats declarades; no es diu que és un ordinador remot sempre encès.

Abans d'escala: cost per usuari, hibernació, recursos, backups, actualitzacions del worker, eliminació de dades i supervisió. El pilot ja necessita límits de temps/disc i credencials protegides, però no un control plane distribuït construït des de zero.

### Publicar webs pròpies (requisit del pilot)

Ús principal: la Carla i l'Aleix, cadascú des del seu compte, creen webs i les publiquen en subdominis de `selsol.top` (per exemple `aliments.selsol.top`), o només a la xarxa local. Han de poder-les verificar, publicar i obrir encara que el seu ordinador estigui aturat.

**Principi:** l'ordinador construeix i previsualitza; l'amfitrió serveix. Publicar **copia** una versió a l'amfitrió (com ja fa `publish`/`share_site.go`, que desa els fitxers i no serveix la carpeta d'origen). Res del que es publica depèn que la VM estigui encesa.

| Peça | Decisió |
|---|---|
| Domini | Un sol registre DNS comodí `*.selsol.top` cap al túnel Cloudflare de l'amfitrió, i una regla d'ingress comodí cap a Gregal. Gregal tria la web per la capçalera `Host`. Una web nova no toca DNS ni Cloudflare |
| Noms | Registre `nom → owner, visibilitat, tipus, versió actual`. El primer compte que reclama un nom el té; l'altre no el pot sobreescriure. Noms reservats: `www`, `gregal`, `api`, `lan`, `preview`. Validació de format DNS (a-z, 0-9, guió) |
| Visibilitat | `public` (per Internet via túnel), `lan` (només a la xarxa local) o `private` (cal sessió del propietari). Es canvia sense tornar a publicar |
| Xarxa local | L'amfitrió serveix les webs `lan` només a peticions d'adreces privades i mai pel túnel. Nom: `aliments.lan.selsol.top`, amb un registre comodí `*.lan.selsol.top` cap a l'IP local de l'amfitrió (sense proxy de Cloudflare). Alternativa si el router bloqueja DNS amb IPs privades: `http://<ip-amfitrió>/w/aliments/` |
| Estàtiques | Versions immutables a l'amfitrió. Publicar = pujar versió i apuntar-hi el nom. «Desfés» torna a la versió anterior a l'instant. Es conserven les 10 últimes |
| Dinàmiques | Corren al gestor d'apps de l'amfitrió (`apps.go`), no a la VM, amb límits de memòria i CPU per app i el mateix enrutament per `Host`. Si una app necessita massa recursos per a l'amfitrió, es diu clarament; no es publica servida des de la VM |
| Verificar | Abans de publicar, previsualització privada a `aliments.preview.selsol.top` (cal sessió del propietari) o dins l'app. Després de publicar, l'amfitrió comprova l'URL real (HTTP 200, `index.html`, enllaços interns) i el resultat entra al fil com a rebut: «Publicat a aliments.selsol.top · versió 4 · comprovat». Si falla, no es canvia la versió activa |
| Permisos | Publicar és una capacitat per compte: «pot publicar a `*.selsol.top`». No requereix els «poders de servidor» d'administrador que avui exigeix l'eina `tunnel`. Publicar en `public` per primer cop un nom nou és una decisió que es confirma al fil; les actualitzacions del mateix nom, no |
| Accés des del mòbil | Una llista «Webs» amb nom, visibilitat, versió, estat de l'última comprovació i obrir, compartir o desfer |

Què es reutilitza: `publish` i `share_site.go` (còpia i versions), `apps.go` (apps dinàmiques), `tunnels.go` (túnel amb nom i ingress). Què falta: enrutament per `Host`, registre de noms amb owner, visibilitat `lan`/`private`, versions amb desfer, comprovació posterior i ingress comodí. Va al paquet **B11 — Publicació**, que depèn només de l'amfitrió i no espera B2 ni B3. El domini base és configuració de la instal·lació (§15): `selsol.top` és el del nostre desplegament, no un valor del codi.

## 6. Connectar apps: dos camins, una experiència

### Camí A — Connector estructurat

Preferir API/MCP quan hi hagi una integració real, mantenible i amb permisos suficients. `internal/mcp/oauth.go` ja implementa OAuth/PKCE i refresc per MCP remot; `callback.go` i `mcp_web.go` són punts d'entrada existents. Això no implica que qualsevol servei comercial ja estigui suportat ni que el magatzem actual sigui adequat a multiusuari sense revisar-lo.

Flux: «Connecta» → consentiment al navegador segur del sistema → callback associat a usuari i petició → prova de capacitat → «Connectat com…» → represa de l'encàrrec. El codi de retorn no ha de quedar al xat ni en logs visibles. Android ha de poder recuperar l'estat si es tanca durant el consentiment.

### Camí B — Sessió al navegador de Gregal

Per llocs sense connector: «Inicia sessió a l'ordinador de Gregal» → visor remot → la persona introdueix credencials/2FA → «Ja està» → comprovació de sessió → l'assistent continua. No transportar cookies del navegador personal del telèfon ni demanar la contrasenya al xat.

Durant credencials, suspendre captura/lectura i eines del model, buffers i traces sensibles; no basta amb un prompt que digui que no les miri. El visor ha de mostrar clarament quan la persona té control exclusiu. Conservar només metadades de connexió necessàries.

### Model de connexió proposat

`id, owner, method(api|browser), accountLabel, status, capabilities, lastCheckedAt, authRef, computerId?`.

`authRef` és una referència interna opaca; mai tokens/cookies al payload d'Android o context del model. Estats: disponible, connectant, connectat, necessita sessió, error, desconnectat. Múltiples comptes del mateix servei han de tenir identitat explícita.

La mateixa pantalla expressa «pot llegir», «pot preparar» i «pot enviar sota aquesta regla». Separar capacitat de l'eina i autorització de la persona: tenir una sessió oberta no autoritza totes les accions.

Per la primera entrega, certificar **un connector real disponible** i **un lloc amb login al navegador** mitjançant comptes de prova. La UI no ha d'ensenyar Gmail, Drive o altres marques com a connectables si la integració concreta no s'ha completat. IMAP/ICS existents continuen sent vies vàlides, sense presentar-los com OAuth.

## 7. Proactivitat que fa feina

### Tres nivells explícits

1. **Explorar:** consultar fonts permeses i detectar canvis rellevants respecte de responsabilitats/preferències actives. Lectura i anotacions locals mínimes; no escriptura arbitrària sobre comptes externs.
2. **Preparar:** executar feina interna reversible que encaixa amb l'encàrrec: comparar, ordenar fitxers del worker, generar un document o un esborrany local.
3. **Actuar:** aplicar canvis externs que estan autoritzats per l'encàrrec o una regla concreta. Quan l'autorització falta, tornar amb una decisió revisable. Cap interruptor global de «fes qualsevol cosa».

Una responsabilitat conté objectiu, fonts, què pot fer sol, què ha de consultar, pressupost i condició de finalització/pausa. Es crea conversacionalment i es mostra amb llenguatge normal: «M'encarrego de preparar els viatges i et consultaré abans de reservar».

### Activació i continuïtat

- Esdeveniments de connectors quan existeixin; polling incremental per la resta; rellotge per compromisos temporals. Reutilitzar jobs, no cridar el model cada minut per preguntar si hi ha feina.
- Primer filtratge determinista de canvis/duplicats; després model quan hi hagi informació nova i una responsabilitat relacionada.
- Guardar darrera comprovació i cursor independentment de si s'ha notificat; una feina no perd informació per haver quedat en silenci.
- L'assistent pot reprendre feina incompleta i actualitzar resultats del mateix encàrrec; el model no reinterpreta cada comprovació com una nova tasca des de zero.
- Memòria de preferències confirmades i feedback utilitzable; no transformar qualsevol conversa casual en una responsabilitat.

### Quan parla

Un missatge proactiu ha de portar un resultat, un canvi important o una decisió que la persona pot prendre. «He trobat dues opcions amb el pressupost que em vas dir» és útil; «vols que busqui opcions?» repeteix una feina ja delegada.

No limitar la feina autònoma al nombre de notificacions. Pressupost de compute, límit de concurrència i pressupost d'atenció són controls diferents. Agrupar resultats poc urgents i respectar el silenci; recordatoris explícits mantenen la semàntica acordada.

Conservar les correccions F01–F03 del primer pla: silenci real del heartbeat, següent control validat i urgència no inferida només del tipus de job.

### Converses i aprovacions que sobreviuen

Troballa nova: `windAskWait` usa un mapa i canal en memòria amb expiració de 20 minuts. El GET pending recupera la pregunta mentre viu aquell procés; no acredita durabilitat després de reinici.

Cal persistir decisió pendent, context mínim, referència a feina i efecte proposat. La feina passa a `waiting_user` i allibera recursos; en respondre, el runner valida que l'acció encara és aplicable i continua des del punt segur. No mantenir un goroutine esperant tota la nit. Resoldre la decisió dues vegades només produeix un efecte.

Les regles actuals de `browser_gate.go` fan que determinades accions sempre preguntin. Revisar-les amb un model d'autorització explícit i proves de límits; no eliminar-les per fer la demo més fluida. Els permisos de lloc guardats per sessió tampoc equivalen a una autorització persistent per compte/objectiu.

## 8. Notificacions sense interrupcions artificials

### Confirmat al codi

- `MainActivity.onCreate` demana POST_NOTIFICATIONS en una arrencada nova sense permís.
- `CompanyScreen` pot tornar-lo a demanar quan troba jobs actius; `notifAsked` viu en `remember`, no és una decisió durable d'onboarding.
- `NoticeHost` mostra snackbars. No s'ha identificat un únic popup universal de «tens avisos» en aquest recorregut; s'ha de reproduir el cas concret en QA abans d'atribuir-lo a un component.

### Experiència nova

- Eliminar petició de permís automàtica d'arrencada. Demanar el permís del sistema una vegada en context: primer recordatori o opció d'avisar quan acabi una feina.
- Després d'una denegació, no insistir en cada sessió. Preferència durable i accés discret a configuració.
- A l'app oberta: inserir el missatge al fil; si estàs llegint més amunt, indicador de novetats sense salt de scroll. No snackbar que anuncia un altre lloc on mirar.
- En un altre fil: petit indicador/enllaç de novetat; cap modal obligatori.
- Fora de l'app: notificació amb contingut útil i deep link a message/result ID. En obrir, marcar lectura/reconciliar sense tornar a anunciar l'avís.
- Errors de servei es mostren al lloc afectat; snackbars només per confirmacions curtes amb desfer quan aportin valor.

### Transport

En primer pla, reutilitzar stream d'esdeveniments amb cursor i replay. En segon pla amb Android/GMS, proposta de producció: FCM amb identificador opac, després lectura autenticada del contingut des del backend. La configuració del projecte i credencials de servidor és una dependència real d'entrega, no s'ha fet aquí.

Per entorns sense GMS, mantenir polling/mirror com a mode declarat i mesurar-ne límits; un transport alternatiu és una decisió posterior. No prometre el mateix SLA ni mantenir un socket en segon pla com si Android el garantís. No posar contingut privat en el push per defecte.

SLO de pilot proposat, no garantia d'Android: missatge disponible al fil en ≤2 s després de persistir-lo amb client actiu; notificació de feina remota amb mediana ≤15 s i p95 ≤60 s en el conjunt de dispositius GMS i condicions normals provats. Mesurar Doze i documentar excepcions, inclòs force-stop i dispositiu sense xarxa.

## 9. Contracte mínim compartit

Reutilitzar IDs de sessió/run/job i magatzem durable d'esdeveniments on siguin aplicables. Els noms següents són conceptes de contracte a mapar abans d'afegir models, no APIs que ja existeixin:

| Concepte | Camps imprescindibles | Invariant |
|---|---|---|
| Encàrrec | owner, conversationId, runId, objective, state, resultRefs, allowedScope | La desconnexió del mòbil no cancel·la per accident la feina |
| Responsabilitat | objective, sources, allowedActions, cadence/trigger, budget, paused | Només treballa dins l'abast encarregat |
| Ordinador | owner, id, health, capabilities, controlOwner, leaseExpiry | Un usuari no veu perfils/fitxers d'un altre |
| Connexió | owner, method, account, status, authRef, capabilities | Secret fora de model i UI; reconnexió reprèn la feina |
| Decisió | id, runId, proposedEffect, revision, state, expiresAt? | Aprovació lligada a l'efecte concret; idempotència |
| Missatge | id, conversationId, source, relatedRunId, resultRefs, createdAt | Una publicació canònica, moltes vistes del mateix resultat |
| Esdeveniment | cursor, owner, kind, targetId, revision | Replay ordenat, deduplicació i lectura consistent |

Estats d'encàrrec: `queued → running → waiting_user / waiting_connection / waiting_computer → running → completed / failed / cancelled`. El heartbeat d'una responsabilitat no és un encàrrec visible nou si no hi ha canvi útil.

Cancel·lació: el mòbil sol·licita al backend, el backend atura eines/runner i confirma l'estat final. Pèrdua de connexió, pausa, tancar visor i cancel·lar feina són accions diferents.

## 10. Paquets per als agents

Els paquets B substitueixen l'ordre A del primer pla. Les correccions, tests i components d'A es reutilitzen dins d'aquests paquets. No s'han llançat agents ni s'ha desplegat infraestructura: el lliurable continua sent el pla per implementar.

### B0 — Integrador i primera entrega vertical (P0)

> **Fet (11/11):** contracte, inventari, identificadors de missatge, F01/F02 i fixtures a [assistant-contract.ca.md](assistant-contract.ca.md).

**Objectiu:** fixar el contracte, una sola base de treball i l'escenari de demostració real.

- Inventari de dades/rutes existents i extensió mínima dels models de la secció 9.
- Reservar `GregalViewModel.kt`, `GregalApi.kt`, `CompanyScreen.kt`, `MainActivity.kt`, `AppStrings.kt` i contracte API a un integrador; la resta lliuren components i canvis acotats.
- Crear fixtures de feina, decisió, connexió expirada i ordinador aturat. Incorporar F01/F02 del primer pla com a regressions prioritàries.
- Acceptació: el recorregut no depèn de textos simulats; IDs i propietat definits; compatibilitat amb dades actuals documentada.
- Esforç orientatiu: 1–2 jornades.

### B1 — Conversa personal i resultats integrats (P0 de producte)

**Fitxers:** `VentsChatChrome`, `WindMessageUi`, `VentetChat`, `VentsAskHome`, `Theme`, `GregalUi`; integració via B0.

- Implementar capçalera, bombolles, agrupació, compositor i resultats amb 15 dp. Cap portada separada si ja existeix historial.
- Missatges proactius dins del fil; aprovar/corregir/obrir resultat en context.
- Menú d'historial i accés a feina/ordinador/connexions; no barra inferior de tres dashboards.
- Acceptació: llegibilitat i targets a 320 dp/200%; teclat i scroll sense salts; missatge nou no duplica el d'Avui; es pot escriure mentre hi ha feina.
- Dependències: fixtures B0; no esperar infraestructura per construir el component, però no donar l'entrega per completa sense integrar-la. Esforç: 3–5 jornades.

### B2 — Ordinador persistent i supervisor (P0)

**Fitxers:** `internal/tools/browser*`, `internal/sandbox`, workspace/runs existents; configuració de desplegament dedicada.

- Pilot amb una VM Linux, navegador visible en escriptori remot i volum persistent; arrencada/reinici supervisats.
- Identitat de worker i owner, registre de salut, capacitats, carpeta de resultats i autorització a nivell de transport.
- Perfil compartit per usuari/ordinador i pestanyes per feina; lease per interacció; cap concurrència cega sobre el mateix navegador.
- Acceptació: tancar Android no interromp una feina; reiniciar worker conserva fitxers/login quan el servei ho permet; un altre owner rep denegació; worker aturat produeix estat honest.
- Dependències: B0. Esforç: 5–8 jornades, segons infraestructura existent; és el principal salt de capacitat.

### B3 — Visor, control humà i connexió per navegador (P0)

**Fitxers:** `browser_handoff.go`, pont de transport i noves peces Android d'ordinador, via B0 per navegació/API.

- Visor remot autenticat amb sessió curta, prendre/deixar control, estat d'agent i feedback de connexió.
- Handoff durable que posa la feina en espera; pausa real del model/eines/captures durant credencials.
- Comprovar login sense exposar secrets i continuar l'encàrrec inicial. Gestió d'expiració, 2FA, cancel·lació i xarxa perduda.
- Acceptació: login fet al mòbil arriba al navegador del worker; l'agent no actua mentre l'humà controla; tancar/reobrir conserva el punt d'espera; no credencials en events/logs/model.
- Dependències: B2 i contracte B4. Esforç: 4–7 jornades.

### B4 — Feina i decisions durables (P0)

**Fitxers:** `internal/runs`, `internal/events`, `internal/web/winds_ask.go`, rutes de runs/stream, `plan.go` i gates.

- Desacoblar execució durable del cicle de vida de la petició mòbil; no alterar sense necessitat el streaming interactiu actual.
- Persistir esperes, efecte proposat i punt de represa; alliberar goroutines i revalidar abans d'executar.
- Reutilitzar cursor/replay; connectar missatge, run, resultat i decisió amb idempotència.
- Acceptació: esperar tota la nit, reiniciar backend i respondre continua correctament; resposta duplicada no executa dues vegades; cancel·lar atura realment; una acció revisada exigeix decisió de la revisió nova.
- Dependències: B0. Esforç: 4–7 jornades.

### B5 — Connexions comprensibles i reutilitzables (P1)

**Fitxers:** `internal/mcp/oauth.go`, `callback.go`, `mcp_web.go`, `PrefsScreen`, `ProviderSetup`; models de connexions via B0.

- Pantalla de connexions amb compte, estat i permisos efectius; separar OAuth/API de sessió browser.
- Completar un connector real amb retorn mòbil, refresc, revocació i represa de la petició.
- Auditar owner del token store actual abans d'emprar-lo en un servidor compartit; no anunciar OAuth universal.
- Acceptació: compte connectat usable en una segona feina sense login repetit; desconnexió efectiva; múltiples comptes distingibles; tancar app durant auth no perd l'encàrrec.
- Dependències: B0/B4; via browser B3. Esforç: 3–6 jornades més qualsevol aprovació externa del proveïdor, que no es pot donar per resolta.

### B6 — Responsabilitats i preparació proactiva (P1 central)

**Fitxers:** jobs, `winds_heartbeat.go`, `winds_action.go`, `winds_avui.go`, memòria existent.

- Responsabilitat explícita sobre fonts connectades; detecció incremental; pressupost de compute i atenció separats.
- Convertir canvi rellevant en feina preparatòria autoritzada i després missatge/resultat. Política d'accions per encàrrec, no permís global.
- Deduplicació, silenci, pausa i finalització; «per què ho has fet?» mostra l'encàrrec i les fonts pertinents.
- Acceptació: canvi de fixture dispara preparació real sense prompt nou; cap canvi no crida ni notifica innecessàriament; pausa ho atura; una font no pot concedir-se permisos.
- Dependències: B4, fonts B5 o web pública; límits B0. Esforç: 4–6 jornades.

### B7 — Continuïtat de notificacions i stream (P1)

**Fitxers:** `WindWatch`, `GregalNotifications`, `NotificationOrigins`, `VentsOffline`, `JobAlarms`; `MainActivity`/`CompanyScreen` via B0.

- Una única política de permís contextual i denegació durable; retirar disparadors duplicats.
- Connexió de primer pla amb replay; push GMS amb ID opac i fetch autenticat; fallback explícit existent.
- Notificació obre missatge/resultat exacte; supressió si ja s'està llegint; llegit/pendent coherents entre vistes.
- Acceptació: sense popup d'avís en tornar, sense duplicat push+polling, cap avís d'altre compte; mesures de latència en Doze i condicions normals.
- Dependències: B4 i credencials/registre FCM de desplegament. Esforç: 3–5 jornades.

### B8 — Programacions, memòria i ensenyar rutines (P1/P2)

**Fitxers:** `VentsActivity`, `ProgramatsScreen`, `AutomatitzacionsScreen`, `WindProfileScreen` i rutes existents.

- Incorporar A3: recordatoris deterministes, editar/ajornar/cancel·lar sobre mateix ID i dates entenedores.
- Preferències de treball visibles i corregibles. Una correcció de l'usuari es pot aplicar als encàrrecs relacionats sense repetir configuració.
- Primera versió de «fes-ho així la propera vegada»: guardar instruccions i referències de resultat en una rutina revisable. No prometre aprenentatge general per vídeo.
- Demostració visual d'un workflow és una extensió posterior: només esdeveniments seleccionats i no sensibles, resum editable i prova abans d'activar.
- Acceptació: rutina pausada no corre; editar memòria no altera credencials; recordatori simple no necessita LLM en disparar-se.
- Dependències: B4/B6. Esforç: 3–5 jornades per primera versió; captura de demostracions fora d'aquesta estimació.

### B9 — QA d'experiència i operació (P0 transversal)

- Comparar amb captures/recorreguts previs, no només tests de cadenes. Mateix nivell de poliment a xat, resultats, login i esperes.
- Provar el cas complet de la secció 3 i els escenaris de la secció 11 en Android real.
- Registrar bateria, latència, errors, durada i cost per encàrrec. Logs operatius mínims, sense secrets ni historial privat per defecte.
- Pilot single-user abans de qualsevol claim multiusuari. Segona identitat de prova per demostrar separació de dades.
- Acceptació: tota la ruta funciona amb backend, connector/lloc de prova i worker reals; cap etiqueta «connectat»/«acabat» basada només en estat optimista de UI.
- Dependències: transversal. Esforç: 3–5 jornades + correccions.

### B12 — Instal·lació portable i projecte obert (P0 transversal)

**Fitxers:** `deploy/linux/{setup.sh,gregalctl.sh,install.sh}`, `deploy/ordinador/`, ordres `gregal setup`/`gregal doctor` noves a `cmd`/`main.go`, `README*.md`, `docs/getting-started*.md`, `docs/install*.md` nous. Detall a la §15.

- Un sol camí d'instal·lació de l'amfitrió amb tres preguntes (domini o només LAN, compte administrador, proveïdor de model); la resta té valors per defecte que funcionen.
- L'ordinador personal s'afegeix amb una ordre (`gregalctl computer add <compte>`) sobre un *backend* d'ordinadors connectable: **contenidor local** per defecte (Podman/Docker, sense hipervisor ni xarxa privada a configurar) i VM (libvirt o proveïdor) com a opció. Mateix contracte `status|wake|save|stop` que ja fa servir `internal/ordinador/host.go`.
- `gregal doctor`: comprova i explica en llenguatge planer què falla (model, DNS, túnel, ordinador, push, espai), sense mostrar secrets.
- Cap valor propi del nostre desplegament al codi ni als valors per defecte: domini base, noms de compte, IDs de paquet de push i URL d'actualització vénen de configuració.
- Còpia de seguretat i restauració en una altra màquina (`gregalctl backup`/`restore`) que porta comptes, fils, decisions, webs publicades i perfils d'ordinador.
- Acceptació: en una VM Debian/Ubuntu neta, de zero a xat funcionant + ordinador + una web publicada en ≤15 min amb ≤5 valors introduïts; restaurar el backup en una segona màquina funciona sense editar fitxers; `doctor` detecta cada error provocat de la llista de la §15; el repo passa l'auditoria pública existent (`scripts/audit-public.cjs`).
- Dependències: es construeix sobre B2/B11; les normes de la §15 s'apliquen des d'ara a tots els carrils. Esforç: 3–5 jornades.

### B13 — Veure què fa l'agent a l'ordinador (P1, després de la primera release)

Petició del 11/11: com fan altres assistents amb *computer use*, al visor de l'ordinador s'ha de veure on actua la IA.

- Cursor de l'agent ben visible (color viu, amb un halo que dura uns instants on clica) i diferent del cursor de la persona; el text que escriu, ressaltat al camp; desplaçaments i arrossegaments amb una traça breu.
- Una franja curta amb l'acció en curs («Clica "Reserva"», «Escriu l'adreça») i el pas de l'encàrrec, en llenguatge planer.
- Funciona al visor del mòbil i de la web, i també en la reproducció de les últimes accions d'una feina (captures amb el punt marcat) quan no s'hi estava mirant.
- Origen de les dades: les eines del navegador de l'ordinador (`internal/tools/browser*`, `browser_ordinador.go`) emeten esdeveniments `computer_action` amb coordenades i tipus pel bus existent; el visor els dibuixa a sobre del noVNC. Res de secrets: el text de camps de contrasenya es mostra emmascarat.
- Quan la persona pren el control (B3), els ressalts de l'agent s'aturen.
- Acceptació: en una feina real es veu cada clic i cada camp escrit al lloc correcte amb un retard inferior a 300 ms; cap contrasenya visible; la reproducció posterior coincideix amb el que ha passat.

## 11. Entregues i definició de «full upgrade»

### Entrega 1 — Fil + ordinador + feina que torna

B0/B1/B2/B3/B4, amb correccions de notificacions in-app de B7. Demostració: donar un encàrrec, connectar un lloc al navegador remot, tancar l'app, veure el resultat real al tornar, obrir el fitxer, corregir-lo i veure'n una nova versió. Una decisió pendent ha de sobreviure al reinici.

### Entrega 2 — Fonts connectades + iniciativa + avís útil

B5/B6/B7 i programacions de B8. Demostració: una responsabilitat encarregada detecta un canvi, prepara un resultat i el porta al fil sense un nou prompt; el telèfon rep l'avís amb transport real.

### Entrega 3 — Poliment i extensió

Memòria, rutines, accessibilitat, rendiment i operació. Veu dúplex i demostracions complexes només després que aquests recorreguts siguin bons. Aquesta tercera entrega no serveix d'excusa per ajornar el disseny: B1 ha de tenir qualitat des de la primera.

Les estimacions són jornades d'esforç orientatives, no calendari ni promesa; no se sumen mecànicament si hi ha paral·lelisme. Colls d'ampolla: worker/visor, esperes durables, aïllament d'identitat, accés als proveïdors i push real. Fins que no passen les entregues 1 i 2, no anomenar el canvi «full upgrade».

### Matriu mínima d'acceptació

| Prova | Evidència exigida |
|---|---|
| Feina amb app tancada | Run continua i lliura fitxer real al mateix fil |
| Login remot al mòbil | Sessió al worker, cap secret al model/log i represa automàtica |
| Segona feina amb mateix compte | Reutilitza sessió sense tornar a configurar-la |
| Dues feines que usen navegador | Control serialitzat/pestanyes coherents, cap formulari enviat al lloc equivocat |
| Prendre control | Agent aturat efectivament fins retornar el control |
| Pregunta pendent i reinici | Decisió recuperada i continuació segura, sense duplicar efectes |
| Nou missatge mentre treballa | Conversa usable, nova petició ben associada, draft conservat |
| Canvi a font vigilada | Resultat preparat sense nou prompt; cap acció externa fora d'abast |
| Notificació amb app oberta/tancada | Missatge en línia o notificació contextual; cap modal de «tens avisos» |
| Permís denegat | Cap repetició automàtica del diàleg en cada arrencada |
| Cancel·lació, xarxa perduda, worker caigut | Estats diferents i verídics; reconciliació de resultat incert |
| Dos usuaris | Cap sessió, fitxer, token, resultat ni push creuat |
| Clar/fosc, 320 dp, font 200%, TalkBack | Tot el recorregut accessible; accions no tapades pel teclat |
| Actualització d'instal·lació existent | Historial, IDs, recordatoris, connexions i preferències preservats |

## 12. Guia d'execució per als agents

### Carrils i ordre

| Ona | Carrils en paral·lel (worktree propi cadascun) | Per què poden anar junts |
|---|---|---|
| 0 | **B0** sol | Fixa contracte, fixtures i fitxers reservats; tota la resta en depèn |
| 1 | **B1** (Android UI) · **B4** (Go: runs/esperes) · **B2** (worker/infra) | Fitxers disjunts: Compose vs `internal/runs`/`winds_ask.go` vs desplegament/`internal/tools/browser*` |
| 2 | **B3** (visor + handoff) · **B7** (notificacions) · **B8** (programacions) | B3 necessita B2+B4; B7 necessita B4; B8 només toca pantalles pròpies |
| 3 | **B5** (connexions) · **B6** (proactivitat) | Necessiten esperes durables i, B6, fonts de B5 |
| Transversal | **B9** QA a cada final d'ona | Captures i matriu de la secció 11 |
| Transversal | **B12** instal·lació portable (després de B11) | Normes de la §15 a tots els carrils des de l'ona 2 |

L'integrador de B0 és l'únic que fusiona a `main` i l'únic que toca `GregalViewModel.kt`, `GregalApi.kt`, `CompanyScreen.kt`, `MainActivity.kt`, `AppStrings.kt`/`Strings.kt` i `docs/api-contract.md`. Els altres carrils li lliuren un diff acotat amb la crida exacta que necessiten.

### Plantilla de brief per a cada agent

```
Paquet: B<n> — <títol>          Ona: <n>      Worktree: sí
Llegeix: docs/android-assistent-experiencia-v2.ca.md §<seccions> i el paquet B<n>.
Fitxers teus: <llista>. Fitxers reservats (no els toquis, demana-ho a l'integrador): <llista B0>.
Fet quan: <criteris d'acceptació del paquet, copiats literalment>.
Proves: go test ./internal/... afectats; Android amb ANDROID_HOME definit (./gradlew testDebugUnitTest);
        captura clar/fosc a 360 dp i 320 dp/200 % per a qualsevol canvi visual.
No: prometre capacitats que la secció 6 diu que no existeixen; desactivar gates; secrets a logs/model.
```

### Normes del repositori que tots els carrils han de respectar

- Textos d'interfície: editar `Strings.kt` a mà mantenint la paritat ca/en amb `internal/web/app/i18n.js` (malgrat la capçalera «NO EDITAR A MÀ»); no executar `regen-strings.py`.
- Paritat: tot el que es vegi a Android ha d'existir també a la web/escriptori de Gregal o quedar anomenat com a buit al lliurament.
- Proves en viu: entorn aïllat amb `GREGAL_DATA_DIR`, `GREGAL_AUTH` i `GREGAL_MEMORY` propis; mai sobre les dades reals de l'usuari.
- No fer servir `~/gregal/gregal.exe` del terminal com si fos el binari de l'app d'escriptori (desfasament de versions conegut).
- Commits sense línies d'atribució a assistents; sense publicar releases: les fa l'integrador localment.
- Canvis de prompt dels vents: passar la bateria de qualitat existent abans i després.
- Portabilitat (§15): tota opció nova té un valor per defecte que funciona o surt a l'assistent d'instal·lació; res de dominis, noms de persona, rutes ni IPs del nostre desplegament al codi; tota dependència externa (FCM, VM, Cloudflare) és opcional i el sistema funciona, degradat i ho diu, sense ella.

### Primera tasca per a cada carril de l'ona 1 (per arrencar sense ambigüitat)

- **B1:** `ResultCard.kt` + nous tokens a `GregalShape` + bombolla de l'assistent amb fons, amb test pur de la regla «bombolla vs superfície de lectura». Captura abans/després del mateix fil.
- **B4:** persistir `windAskWait` (avui mapa en memòria, 20 min) a disc amb `id`, `runId`, `proposedEffect`, `revision`; test que reinicia el servidor entre pregunta i resposta.
- **B2:** script reproduïble de la VM pilot (navegador visible + noVNC darrere del gateway + volum persistent) i un endpoint de salut del worker; cap canvi a Android.

## 13. Llenguatges i llibreries

Revisat l'11/11 a petició de l'usuari. **Conclusió: no es canvia de llenguatge. Sí cal posar al dia la pila d'Android i afegir tres llibreries concretes.**

### Backend: continua en Go

- Són ~144.000 línies de codi i ~75.000 de proves. Reescriure-les aturaria el producte mesos i no resol cap dels colls d'ampolla del pla (worker, esperes durables, push, aïllament).
- Go és precisament el que permet el backend local d'Android: un sol binari que es compila creuat i s'executa dins l'app (`LocalServer.kt`). Amb Node, Python o la JVM això seria molt més pesat.
- La concurrència del runner, de la cua i dels streams SSE encaixa bé amb Go. Les dependències són poques i estables.
- **Canvi recomanat, fora de B4:** l'estat durable viu en fitxers JSON amb bloqueig entre processos (`crosslock.go`). Per a encàrrecs, decisions i connexions, quan creixin, convé SQLite amb `modernc.org/sqlite` (Go pur, sense cgo, compatible amb la compilació per a Android). B4 comença amb fitxers per no barrejar decisions; la migració a SQLite és un paquet a part, quan hi hagi tres o més entitats amb consultes per owner i estat.

### Android: Kotlin + Compose es mantenen; la pila està endarrerida

| Ara | Proposta | Per què |
|---|---|---|
| Kotlin 1.9.24, compilador Compose 1.5.14, AGP 8.5, Compose BOM 2024.05 | Kotlin 2.x amb el plugin del compilador de Compose, AGP actual, BOM actual | *Strong skipping* per defecte (menys recomposicions en streaming), `Modifier.animateItem` a `LazyColumn` (l'entrada de missatges de B1), transicions compartides, Material 3 recent |
| `org.json` a mà (182 `optString` a `GregalApi.kt`; B0 ha hagut de tocar 6 llocs per afegir un camp) | `kotlinx.serialization` amb models `@Serializable` i camps opcionals per defecte | Un camp nou és una línia; les proves de contracte poden deserialitzar directament les fixtures de `docs/assistant-contract/fixtures` |
| `HttpURLConnection` a mà | OkHttp (SSE amb `okhttp-sse`, WebSocket per al visor de B3) | Reconnexió, timeouts i streaming fiables; el visor remot el necessita igualment |
| Fitxers JSON propis (`VentsOffline`, preferències) | Mantenir-los; DataStore només per a preferències noves (permís de notificació de B7) | No cal Room: la font de veritat és el servidor |
| Sense push | Firebase Cloud Messaging (B7) | Únic transport de segon pla fiable en dispositius amb GMS |
| `GregalViewModel.kt` de 6.000 línies | Dividir-lo per domini (conversa, feina, connexions, ordinador) a mesura que cada paquet hi toqui | Menys conflictes entre carrils; no cal una reescriptura de cop |

Que no cal: Flutter/React Native (es perdria el backend Go incrustat i tota la UI actual), Room, Hilt/Koin (amb un sol ViewModel per domini n'hi ha prou) ni una llibreria de markdown externa (`Md.kt` ja gestiona streaming sense salts, i és fàcil regressar-hi).

Ordre: l'actualització de Kotlin/Compose, kotlinx.serialization i OkHttp és un paquet propi (**B10**) que va **després de fusionar B1** i abans de B3/B7, que són els que en treuen més profit. Es fa en una sola branca, sense canvis de comportament, i passa els 500+ tests unitaris existents.

## 14. Estat d'aquesta revisió

- S'han revisat els referents sol·licitats, dues imatges oficials de dots i els punts de codi que condicionen autonomia, navegador, OAuth, esperes i notificacions.
- S'ha substituït la direcció del pla anterior i definit feina concreta per a agents.
- S'ha preparat una nova proposta interactiva amb fil viu, resultat, ordinador i connexions. És una simulació de disseny, no un worker operatiu.
- Afegides la taula de canvis concrets de Compose (§4) i la guia d'execució per agents (§12), contrastades amb `WindBubble`, `VentsChatHeader`, `VentsComposer`, `VentetThread`, `GregalShape` i `windAskWait` reals.
- Maqueta revisada al navegador: conversa, ordinador i «Pren/Retorna el control» funcionen; sense errors de consola.
- No s'ha modificat codi funcional ni provisionat infraestructura; es preserven els canvis locals previs.
- Les proves del primer informe són el baseline d'aquella revisió, no una validació de les capacitats noves proposades aquí.

## 15. Portabilitat: un projecte obert que s'instal·la en poca estona

Requisit afegit el 11/11: Gregal ha de quedar com un copilot obert útil per a qualsevol persona, no com un muntatge per a la nostra màquina. Ha d'estar format per unes interfícies, un backend i les peces opcionals, tot integrat i fàcil de portar a un sistema nou amb una instal·lació senzilla i poca configuració.

### Composició final

| Peça | Què és | On corre | Obligatòria |
|---|---|---|---|
| **Backend** | Binari Go únic `gregal` (API, comptes, fils, esdeveniments, jobs, gateway, publicació). Porta la web dins (`internal/web/app`) | Amfitrió: Linux, Windows, macOS; també dins d'Android | Sí |
| **Interfícies** | Web/escriptori (`internal/web/app`, `desktop/`), Android Vents (`android/`), terminal, VS Code, extensió de Chrome. Totes parlen el mateix `docs/api-contract.md` | Clients | Almenys una (la web ja ve amb el backend) |
| **Ordinador personal** | `gregal-ordinador` + escriptori + Chrome, un per compte | Contenidor local (defecte) o VM | No: sense ell, les feines que el necessiten queden en `waiting_computer` i ho diuen |
| **Publicació** | Webs per `Host` sota un domini base configurat | Amfitrió | No: sense domini, només LAN o `/w/<nom>/` |
| **Push** | FCM per a Android | Servei extern | No: sense credencials, connexió de primer pla i comprovació periòdica |
| **Model** | Qualsevol endpoint compatible amb OpenAI | Extern o local | Sí, un |

### Instal·lació objectiu

```
curl -fsSL <release>/install.sh | sudo sh      # o: descomprimir i sudo sh setup.sh
  → domini? (buit = només xarxa local)
  → nom de l'administrador
  → proveïdor de model i clau (o «després», des de la web)
sudo gregalctl computer add <compte>           # opcional: ordinador en contenidor
sudo gregal doctor                             # què funciona i què falta
```

- **Configuració en un sol lloc:** `/var/lib/gregal/config` (fitxer llegible i comentat) generat per l'assistent. Les 85 variables `GREGAL_*` actuals es mantenen com a sobreescriptura per a desenvolupament, però cap instal·lació normal n'ha de necessitar cap.
- **Domini base:** un valor (`publish.domain`); la publicació, les previsualitzacions (`<nom>.preview.<domini>`) i la LAN (`<nom>.lan.<domini>`) en deriven. Al nostre desplegament és `selsol.top`; no apareix al codi.
- **Ordinador:** el backend per defecte és un contenidor amb volum persistent (aturar-lo allibera la memòria; el perfil i els fitxers queden al volum). Si l'amfitrió no té Podman/Docker, `computer add` ho diu i proposa instal·lar-lo o fer servir VM. La VM de `deploy/ordinador` queda com a backend avançat amb el mateix contracte.
- **Actualitzar:** tornar a executar l'instal·lador amb el binari nou; mai cal tocar configuració a mà ni migrar dades a mà.
- **Portar a una altra màquina:** `gregalctl backup` → copiar → `install.sh --restore fitxer`. Inclou comptes, fils, decisions, recordatoris, webs i volums d'ordinador; els secrets van xifrats amb una clau que es mostra un sol cop.
- **Clients:** l'app Android i l'escriptori troben el servidor a la LAN o amb una URL + invitació; cap compilació pròpia necessària. Qui faci un fork pot canviar l'ID de paquet i el canal d'actualitzacions amb dues propietats de Gradle.

### Projecte obert

- Llicència MIT ja present; mantenir `CONTRIBUTING.md`, `SECURITY.md` i l'auditoria pública (`scripts/audit-public.cjs`) verds abans de cada release.
- Documentació en anglès per defecte i català: «Install», «Add a computer», «Publish a site», «Connect Android», «Backup and move».
- Res que només funcioni amb els nostres comptes: proves amb dues identitats genèriques, fixtures sense dades personals, exemples amb `example.org`.
- Errors provocats que `doctor` ha de detectar i explicar: model inaccessible o clau incorrecta, DNS del domini que no apunta a l'amfitrió, túnel caigut, ordinador que no arrenca, sense Podman/Docker, push sense credencials, disc ple, rellotge desfasat.

## 16. L'assistent principal fa la feina (repensar el Ventet)

Revisat l'11/11 a petició de l'usuari: el Ventet és el que s'obre per defecte, però avui l'únic que fa és repartir.

### Què passa avui

`/api/winds/cap` decideix amb **una sola crida al model, sense eines** (`capModelCall`): o respon amb text, o proposa passar-ho a un vent, una rotllana o una colla i espera «Endavant». No pot cercar, ni programar un recordatori, ni vigilar res, ni fer servir l'ordinador ni les connexions. Tot el que és feina de debò surt del fil principal cap a un altre xat.

### Com ho fan els referents

- **Claude Duet:** un fil principal on l'agent fa la feina ell mateix amb el seu ordinador al núvol, les connexions i la memòria del compte; t'escriu primer (resum del matí, comprovacions periòdiques); canals opcionals per temes; demana aprovació només abans d'actuar fora (enviar, pagar, signar, esborrar). [MacRumors](https://www.macrumors.com/2026/10/21/anthropic-claude-duet/), [Zapier](https://zapier.com/blog/claude-duet/).
- **OpenAI dots:** un agent persistent amb ordinador propi que treballa en segon pla, fa recerca proactiva només de lectura i torna quan necessita una decisió. [Presentem els dots](https://openai.com/index/introducing-dots/), [TechCrunch](https://techcrunch.com/2026/10/26/how-openai-turned-its-coding-tools-into-dots-an-always-on-personal-assistant/).
- **Grok Bot:** bots amb nom sobre un ordinador compartit; el patró recomanat és un «cap de gabinet» que és amo del resultat i crida especialistes només quan cal la seva part, i et demana només les decisions de criteri. [Grok Bot 101](https://x.ai/bot/guides/grok-bot-101), [handbook](https://github.com/0xkaize/grok-bot-handbook).

Patró comú: **un sol agent principal que treballa i és amo del resultat**; els especialistes són ajuda en segon pla, no una porta que cal triar; l'aprovació és per a accions externes, no per decidir qui ho fa.

### Proposta (paquet B14)

1. **Gregal és un agent, no un encaminador.** El fil principal fa servir el mateix bucle que els vents (eines, ordinador, connexions, recordatoris, vigilàncies, decisions durables i represa): cercar, preparar, programar i vigilar els fa ell, al mateix fil.
2. **Els especialistes són ajudants.** Gregal els consulta amb una eina (la `wind` existent, o una colla per a feina llarga) en segon pla; el resultat torna al fil principal com a targeta «Ho ha preparat Marinada». Sense «Endavant» per triar qui ho fa.
3. **Aprovació només per actuar fora:** enviar, comprar, publicar, esborrar o escriure fora del projecte (decisions de B4).
4. **T'escriu primer al mateix fil:** el resum del matí, les vigilàncies (B6) i els recordatoris (B8) arriben al fil principal, no a fils separats. Els xats amb cada vent queden com a «canals» opcionals al menú.
5. **Nom i cara de Gregal** per defecte (personalitzables), i el «Ventet» desapareix com a concepte separat.
6. **Obertura espectacular:** en obrir l'app, la lluïssor floreix al voltant de la cara de Gregal i s'esvaeix quan apareix el fil (≤ 1 s, res amb «reduir animacions»); torna, suau, mentre treballa.

Compatibilitat: l'historial del Ventet es conserva com a historial del fil principal; `/api/winds/cap` continua responent per a apps antigues. Acceptació: «recorda'm demà a les 9…», «vigila aquesta web» i «cerca'm X» es resolen dins del fil principal sense canviar de xat ni prémer «Endavant»; una feina llarga torna al fil amb el resultat; enviar o comprar demana permís.
