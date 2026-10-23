# Vents: memòria, coneixement i continuïtat

Proposta de producte i arquitectura, 23 d’octubre de 2026. Aquest document
descriu el comportament proposat; no afirma que ja estigui implementat.

Estat d’implementació, v1.15.0: l’espai compartit amb Converses i Projectes,
els tres equips de 16 especialistes, els perfils de Memòria i Coneixement,
la biblioteca amb arxiu/recuperació/assignació/exportació, els documents,
les rutines diàries de lectura i l’historial de feina amb resultats reals
ja tenen implementació al servidor i a la interfície web d’escriptori/mòbil.
Comparteixen les API i els fitxers de memòria/coneixement existents d’Android.
La resta del document continua sent el full de ruta: no hi ha un servei al núvol
ni execució amb Gregal tancat, i la previsualització aïllada de fitxers HTML
no substitueix el servidor de desenvolupament d’una aplicació complexa.

## Objectiu

Cada vent té dos sistemes visibles i editables: **Memòria** i **Coneixement**.
La informació pertany a l’usuari i es conserva independentment del personatge.
Eliminar un vent no elimina el que ha après. Es pot restaurar el vent o assignar
una selecció d’aquella informació a un altre, sense començar de zero.

## Què tenim realment

- Android, a `WindProfileScreen.kt`, mostra memòria pròpia editable i una
  biblioteca de coneixements amb nom, descripció i contingut. L’API ja té
  `/api/winds/memoria` i `/api/winds/coneixements` per a aquests dos sistemes.
- `wind-memory/<windID>.md` desa records sobre l’usuari; `wind-skills/<windID>/`
  desa coneixements consultables sota demanda. L’índex dels coneixements va al
  context i el contingut complet es consulta amb l’eina `skill`.
- També existeix `wind-knowledge/<windID>.md`: apunts temàtics apresos de
  converses, amb fonts. S’ha d’integrar visualment dins de Coneixement, juntament
  amb els documents, sense perdre la seva procedència.
- Hi ha una memòria personal general compartida. No és equivalent a la memòria
  pròpia de cada vent i no s’ha de copiar automàticament a tots els vents.
- El perfil web actual retorna coneixement temàtic i memòria personal general;
  no retorna la memòria pròpia ni la biblioteca de coneixements. Els encàrrecs
  nous llegeixen aquest perfil incomplet: la paritat ha d’incloure el que el
  vent utilitza durant una tasca, a més del que es mostra a la pantalla.
- `WindStore.Delete` treu el vent del catàleg i deixa els fitxers separats al
  disc. Però les API exigeixen un vent existent: la informació queda sense
  accés normal. Això no constitueix una funció de recuperació.
- Els identificadors actuals deriven del nom. Tornar a crear el mateix nom pot
  reutilitzar l’identificador; no ha de substituir un procés explícit de recuperació.
- Els resums actuals de memòria i apunts tenen límits i poden podar informació
  antiga. El resum del context no ha de ser l’única còpia del coneixement durador.

## Experiència proposada

El perfil de cada vent té **Activitat · Memòria · Coneixement**; el caràcter
s’edita des d’una acció discreta. No cal configurar aquests apartats per començar.

| Sistema | Què conté | Com s’utilitza | Accions visibles |
| --- | --- | --- | --- |
| Memòria | Preferències, decisions i context durador de la relació amb aquell vent | Resum breu pertinent al xat o encàrrec | Afegir, corregir, oblidar, veure origen, recuperar versió |
| Coneixement | Documents, apunts amb fonts, guies, procediments i lliçons verificades | Cerca i consulta sota demanda | Adjuntar fitxer, cercar, editar, actualitzar fonts, assignar a un altre vent |

Exemples: «Prefereixo que les webs siguin en català» és Memòria;
«Guia d’estil de la cafeteria» és Coneixement. La web creada continua sent un
fitxer del projecte, amb un enllaç des de l’activitat: no se’n desa una còpia
sencera dins de la memòria.

Una indicació discreta com «Record nou: prefereixes exemples curts · Desfés»
fa visible l’aprenentatge sense obligar a obrir un editor. Un record pot mostrar
la conversa d’origen i la data. Un coneixement pot mostrar la font i l’última
verificació. La informació ambigua no es desa com si fos un fet.

## Biblioteca i recuperació

Una entrada **Biblioteca dels vents** reuneix els records i coneixements, amb
filtres per vent, projecte, tipus i estat actiu/arxivat. No afegeix cap pas al
camí principal d’escriure un encàrrec.

Flux concret d’un esborrament accidental:

1. L’usuari elimina Marinada. Desapareix de la colla activa, i la seva identitat
   i els dos sistemes queden arxivats. Es mostra «Vent eliminat · Desfés».
2. Biblioteca → Arxivats mostra «Informació de Marinada», amb dates, quantitats
   i vista prèvia. També s’hi poden recuperar fitxers antics sense vent associat.
3. L’usuari pot **Restaurar Marinada** o **Assignar a un altre vent**.
4. Per assignar-la a Mestral, selecciona Memòria, Coneixement o elements concrets.
   Un resum explica què s’afegirà i quins possibles conflictes hi ha.
5. Es creen còpies assignades a Mestral, conservant origen i historial. L’arxiu
   original continua disponible, i l’operació es pot desfer.

Per defecte, assignar significa **copiar**, perquè editar la informació de
Mestral no canviï inesperadament la de Marinada. «Compartir i mantenir
sincronitzat» pot arribar després com una opció explícita per a guies comunes.
Els records contradictoris es presenten per resoldre; no se sobreescriu la
memòria de destí ni se li demana al model que resolgui silenciosament el conflicte.

No hi ha una caducitat automàtica dels arxius. «Eliminar definitivament» és una
acció independent que enumera què s’eliminarà. Arxivat, historial de versions
i còpia de seguretat són funcions diferents.

## Persistència i compatibilitat

- Identificadors estables independents dels noms per a vents i elements.
- Cada element té tipus, contingut, dates, revisió, procedència, àmbit i estat.
  Els elements de coneixement poden afegir fonts i data de verificació.
- Una assignació vincula l’element amb un vent; eliminar el personatge no
  desencadena una supressió dels continguts.
- Cada còpia conserva el seu origen i l’identificador de l’operació per desfer-la.
  L’historial conserva les revisions anteriors sense dependre de l’extracció del model.
- El magatzem durador conserva el contingut complet; el context del model usa
  resums i cerca sota demanda per evitar costos i latència creixents.
- Primer es llegeixen els fitxers Markdown existents i es crea un manifest amb
  els seus vincles. La migració és repetible, verifica continguts i conserva els
  originals; no es requereix una base vectorial per oferir la primera versió.
- La biblioteca és de l’usuari: els registres d’altres usuaris o servidors no es
  barregen. Android i desktop comparteixen contracte quan fan servir el mateix
  servidor; en modes locals diferents, la transferència és explícita via exportació.
- Exportació/importació d’un paquet llegible amb manifest, records,
  coneixements i procedència. Importar previsualitza les assignacions i evita
  sobreescriure dades existents.
- La migració detecta informació que ja ha quedat sense un vent al catàleg.
  Es mostra com a informació recuperada, sense inventar el caràcter perdut.

## Millores d’utilitat, en ordre

1. **Paritat Android-desktop i ús real durant els encàrrecs.** Llegir i editar els
   dos sistemes i consultar els coneixements pertinents en xats i tasques.
2. **Biblioteca, arxivat i reassignació.** Donar continuïtat a la informació i
   recuperar també els fitxers que avui han quedat sense vent.
3. **Aprenentatge útil visible.** «Recorda això», «Desa com a coneixement» i
   desfer; després d’una tasca, retenir decisions i lliçons verificades, amb
   enllaços als fitxers i comprovacions reals, no tot el text generat.
4. **Assignació per allò que saben.** Triar qui ajuda a partir dels coneixements
   i del projecte, a més del nom o l’especialitat. Explicar-ho breument: «Marinada
   té les teves fonts sobre cafeteries».
5. **Continuar feines i rutines amb context.** Activitat amb encàrrecs, resultats
   i punts pendents; rutines que actualitzen un coneixement existent i indiquen
   què ha canviat, sense duplicar notes a cada execució.

## Ampliació: ajudants persistents i companyia quotidiana

Les referències funcionals són tres agents proactius: Grok Bot (xAI), pel treball
per missatge, la continuïtat, els relleus i els procediments reutilitzables;
ChatGPT dots (OpenAI), per les responsabilitats continuades d’un agent sempre
actiu, la vista d’activitat i les regles sobre què pot fer sol; i Claude Duet
(Anthropic), pel fil únic, el resum del matí i les aprovacions. La direcció que
se’n deriva per a Android i desktop és a «Direcció Duet». Dot de New Computer
és un producte diferent amb un nom semblant i no és la referència d’aquest
document; el seu anunci de tancament sí que recorda per què cal garantir la
recuperació i l’exportació de la informació de Gregal.

Les funcions següents són propostes per a Gregal, no una afirmació que tots els
productes de referència les tinguin ni que Gregal ja les implementi.

### 1. Una conversa pot convertir-se en feina

Escrius «Marinada, investiga cafeteries semblants a la meva i prepara una web».
El vent recupera el context pertinent, resol la cerca i demana ajuda a Tramuntana
si cal crear fitxers. La conversa mostra la feina en marxa i el resultat. Pots
escriure «fes-la més càlida» al mateix fil, sense copiar el resultat a un altre xat.

Conversar, executar i preparar una rutina comparteixen el mateix punt d’entrada.
El perfil i les opcions avançades són accessoris. Una incògnita només provoca
una pregunta quan és necessària per avançar; la resta es resol amb el projecte
obert, les preferències desades i valors per defecte visibles.

### 2. Encàrrecs que continuen i es poden reprendre

Cada encàrrec té identitat, conversa, carpeta, estat, resultats i punts pendents.
La vista «En marxa» mostra què està fent cada vent, fitxers canviats, comprovacions
i qui revisa. Una comprovació pendent no es presenta com si hagués passat.

Tancar la vista no cancel·la la feina. Els estats i punts de represa es desen per
poder recuperar-la després d’una desconnexió o reinici. Reprendre comprova què
ja s’ha executat abans de repetir accions. Pausar i cancel·lar són accions diferents.

Això requereix evolucionar l’execució actual: en local només es pot continuar
mentre el servei estigui actiu. Treballar amb l’ordinador apagat requereix un
servidor o executor remot disponible; la interfície ha de mostrar on s’executa
i si està connectat. No es promet treball 24/7 a partir d’un procés local tancat.

### 3. Iniciativa basada en coses que importen

Propostes curtes amb un motiu verificable: «La font que seguim ha canviat el
preu», «L’última vegada vam deixar això pendent» o «He trobat una novetat del
projecte que segueixes». Cada proposta ofereix una acció concreta o es pot descartar.

Les rutines es poden demanar amb llenguatge natural: «Cada divendres actualitza
aquesta comparativa i avisa’m només si hi ha canvis». El vent presenta un resum
de freqüència, origen i resultat esperat abans d’activar-la. Les execucions tenen
historial, i la notificació retorna al seu fil i al lliurable corresponent.

La iniciativa es pot activar per projecte o tema i té controls senzills:
«Només quan t’ho demani», «Novetats útils» i «Silencia fins demà». Els avisos
s’agrupen i es dedupliquen; no s’omple el xat amb comprovacions sense novetats.

### 4. Ensenyar una manera de treballar

Després de corregir un resultat, pots dir «Fes-ho així a partir d’ara» o
«Guarda aquest procediment». El vent extreu una guia curta amb els passos,
entrades, resultat esperat i comprovacions. Es pot editar i reutilitzar en altres
encàrrecs o rutines, amb una versió anterior recuperable.

Un procediment és un element del **Coneixement**, amb tipus «procediment».
Una preferència de com vols el resultat és **Memòria**. Això conserva els dos
sistemes principals i permet transferir els procediments quan canvies de vent.
L’aprenentatge a partir d’una demostració amb navegador o pantalla arribaria
després del guardat de procediments a partir de feina ja realitzada.

### 5. Col·laboració que evita fer de coordinador

Un vent pot cercar, un altre construir i un altre revisar dins del mateix
encàrrec. El relleu conté objectiu, resultats, fonts i pendents. La persona veu
una explicació breu de qui hi participa, pot intervenir i pot continuar parlant
mentre avancen les parts que no depenen d’una decisió seva.

Els vents comparteixen el context necessari de la feina. Participar en una
rotllana no dona accés automàtic a tota la memòria personal dels altres vents.
La revisió comunica defectes concrets, correccions i comprovacions reals. Els
encàrrecs senzills no han de pagar el cost d’una colla sencera.

### 6. Resultats que es poden utilitzar

La peça principal del final és el lliurable: web navegable, document, fitxer,
comparativa amb fonts o canvi de codi. Té accions pertinents: obrir, descarregar,
continuar, comparar versions o convertir el procés en rutina. L’explicació del
vent acompanya aquesta peça i no la substitueix.

Cada projecte té una prestatgeria de resultats amb les seves versions i carpeta.
Les connexions a eines externes es fan quan un encàrrec les necessita i després
es reutilitzen. Les primeres tasques locals han de funcionar amb el model i el
projecte que ja estan oberts, sense un assistent llarg de configuració.

### 7. Una relació amb continuïtat i caràcter

Una petita vista «El nostre fil» permet reprendre temes i decisions importants.
És una presentació amb enllaços a la Memòria, el Coneixement i les converses,
no un tercer sistema on es duplica informació. Els resums personals es poden
corregir i es distingeix allò que has dit de les inferències del vent.

Cada vent manté una veu, un ritme i un aspecte recognoscibles. Pot discrepar,
fer una pregunta pertinent o suggerir ajuda, i recordar una preferència sense
recitar-la a cada resposta. La tendresa ve del to, els detalls i la continuïtat;
no de missatges que pressionin per tornar ni d’afirmacions de sentiments que
no es poden verificar.

### 8. Android i desktop com a dues portes al mateix fil

Pots dictar un encàrrec a Android i trobar al desktop la mateixa feina, fitxers,
memòria, coneixements i estat quan comparteixen servidor. Des del mòbil també
pots veure el progrés, aturar la feina, revisar el resultat i respondre un punt
pendent. Una notificació obre directament la conversa corresponent.

El text continua sent suficient per utilitzar-ho tot. La veu és una via ràpida
d’entrada i lectura del resultat. Les captures i fitxers adjunts s’associen a
l’encàrrec i només s’incorporen al Coneixement quan té sentit conservar-los.

### Distribució visual

- **Inici de Vents:** una caixa per parlar o encarregar feina; després, només
  allò que requereix atenció, feines en marxa i fils recents.
- **La colla:** retrats petits amb estat real; clic per parlar, i perfil per
  consultar Memòria, Coneixement i activitat.
- **Al mateix fil:** conversa, passos de feina, relleus i lliurables. Els detalls
  tècnics queden plegats, amb progrés comprensible sempre visible.
- **Biblioteca:** records, coneixements, procediments, arxivats i recuperació.
- **Rutines:** accessibles des del fil i en una vista senzilla per pausar-les o
  veure'n els resultats. No ocupen la meitat de la pantalla d’inici.

L’estil manté superfícies suaus, contrast llegible, espai i retrats tendres.
Animacions breus reflecteixen espera, feina o finalització observades. A mòbil,
una columna; a desktop, el lliurable es pot obrir al costat de la conversa.

### Revisió estètica i de distribució — abans d’aprovar el disseny

La direcció és una eina quotidiana tranquil·la, amb identitat pròpia i resultats
al centre del recorregut. La colla aporta calidesa amb retrats petits i un to
proper. Les referències de Grok Bot serveixen per al treball al mateix fil i la
relació entre ajudants; no s’atribueix al seu disseny una distribució que no s’ha
pogut comprovar a la seva aplicació. La maqueta és una proposta original de Gregal.

**Una jerarquia lateral, sense llistes duplicades.** Es conserva el rail global
estret de Gregal i una única barra contextual per als vents. Aquesta barra conté
Inici, Feines, Biblioteca, Colles i converses recents. La llista de membres no es
repeteix al centre permanentment: s’obre quan consultes una colla. Projectes duen
la icona de carpeta; converses, la de conversa. Ajuda i configuració queden
accessibles al peu del rail i no depenen de desplegar «Més».

**Dues composicions del mateix producte.** La maqueta permet comparar «Conversa»,
amb entrada i fils recents, i «Projecte», amb feina i resultat al costat. En el
producte final han de ser moments del recorregut, no modes que l’usuari hagi de
configurar abans de començar.

**Inici.** Una salutació breu, un retrat petit i una caixa d’escriure dominant.
Tres exemples discrets i dos o tres fils recents ajuden a començar. Les colles
completes, rutines, editors de memòria i graelles d’especialistes no ocupen
simultàniament aquesta pantalla. La primera acció es veu sense desplaçar-la.

**Conversa i execució.** El fil mostra qui respon, el resultat i un resum curt
del progrés. Les crides tècniques i comprovacions s’expandeixen sota demanda.
Les targetes corresponen a un encàrrec o lliurable, no a cada missatge. Els
estats «Treballant», «Esperant», «Cal una decisió», «Aturat» i «Preparat» porten
text i icona, i mai es distingeixen només amb color. Els canvis no empenyen el
contingut ni mouen la posició de lectura de manera inesperada.

**Caixa d’escriure.** Text, adjunts, carpeta, model/opcions i enviament formen un
sol component. El model és un control discret integrat a la barra inferior del
component. No té una fila pròpia ni un selector prominent a la capçalera. En
pantalles estretes, les opcions secundàries passen al menú compacte del component.

**Resultats.** A desktop, la vista prèvia apareix en un panell dret que es pot
tancar, mantenint el fil al costat quan hi ha espai suficient. A mòbil, passa
a una vista de resultat amb retorn al fil. No es col·loquen dos scrolls estrets
ni un resultat enorme sota una conversa que desapareix fora de pantalla.

**Colles i perfils.** Les colles s’identifiquen pel nom i especialitat. Els
membres es presenten en files compactes amb retrat, missió breu i acció per parlar
o obrir el perfil. El color identifica suaument el personatge i no omple tota
la pantalla de blocs diferents. Memòria i Coneixement tenen pestanyes amb el
mateix pes; el caràcter es pot editar sense ocupar el centre del perfil.

**Biblioteca.** Llista llegible amb tipus, contingut, vent associat i procedència.
Els filtres són Tot, Memòria, Coneixement i Arxivats. Recuperar o assignar es
resol amb una finestra curta que identifica el destí i què es copiarà. La
confirmació apareix al costat de l’acció, es pot desfer i conserva l’arxiu original.

**Android.** Quatre destinacions principals: Inici, Feines, Colla i Biblioteca.
La navegació lateral es replega; la capçalera mostra el context actual. La veu
i els adjunts són accessibles sense tapar el text ni el botó d’enviar. El teclat
no ha d’ocultar la caixa d’escriure i tornar d’un resultat conserva el fil.

**Llenguatge visual.** Superfícies opaques coherents amb el tema, base clara
càlida o fosca de carbó, contrast llegible, accents vegetals i colors suaus als
retrats. Sans serif per llegir i treballar; serif editorial reservada als títols
d’entrada, sense aplicar-la als controls. Radi moderat, ombres suaus i una sola
acció principal per context. Tipografia i botons accessibles sense dependre de
passar-hi el ratolí. La tendresa no redueix la llegibilitat ni converteix la feina
en una pantalla de joc.

**Comprovacions per donar-lo per bo.** A 320, 390, 768 i 1024 píxels: cap solapament
ni scroll horitzontal, controls principals visibles, jerarquia equivalent en clar
i fosc, focus de teclat recognoscible i objectius tàctils d’uns 44 píxels. S’han de
revisar explícitament els estats buit, llarg, en marxa, amb error i amb resultat;
seleccionar un vent, mostrar progrés o obrir el teclat no pot desplaçar tot el shell.

La maqueta interactiva usa exemples locals i no executa tasques ni modifica
memòria real. Permet recórrer Inici, Colles, perfils, Biblioteca, arxivats,
reassignació i resultat. Aquesta passada valida la direcció visual; les proves
de dades reals i del comportament natiu d’Android corresponen a la implementació.

### Ordre d’entrega ampliat

1. **Continuïtat real:** paritat de Memòria/Coneixement, arxivat i reassignació,
   ús d’aquesta informació en tasques i seguiment al mateix fil.
2. **Feina delegada fiable:** encàrrecs persistents, represa, revisió, lliurables
   utilitzables i continuïtat Android-desktop.
3. **Iniciativa útil:** rutines per conversa, avisos de canvis, procediments
   apresos de correccions i coordinació basada en coneixements.
4. **Més abast:** veu, connexions externes, aprendre per demostració i execució
   remota opcional quan es necessiti continuar amb l’ordinador apagat.

El primer recorregut complet a assolir és: «Fes la web amb l’estil que vam
acordar» → cercar la guia guardada → crear i revisar → obrir la web → corregir
al mateix fil → conservar la correcció útil → arxivar el vent → assignar la
informació a un altre → continuar el projecte sense repetir el context.

## Direcció Duet: un sol fil per a Android i desktop

Claude Duet, publicat el 21 d’octubre de 2026, organitza un agent proactiu al
voltant d’un sol fil continu. Comença ell amb un resum al matí, calla fins que
cal alguna cosa de l’usuari, continua la feina amb els dispositius apagats i
demana aprovació abans d’enviar, pagar, signar o esborrar. Els Vents d’Android i
desktop adopten aquesta lògica. Quan hi entri en conflicte, aquesta secció té
prioritat sobre la navegació de «Distribució visual», la revisió estètica,
l’apartat d’Android i l’«Ordre d’entrega ampliat». La resta de funcions del
document (Memòria, Coneixement, biblioteca, arxivat, reassignació i colles) es
mantenen.

### Principis

| Idea de Duet | Com s’aplica als Vents |
| --- | --- |
| Un sol fil: una resposta ràpida i un projecte d’una setmana comencen al mateix lloc | El **Fil** és la pantalla principal. Encàrrecs, progrés, resultats i aprovacions hi apareixen com a targetes. Els canals per projecte o tema són opcionals i comparteixen memòria; no són compartiments privats |
| Tres destinacions: Chat · Tasks · Make | **Fil · Feines · Fet**. Colla, Memòria, Coneixement, Biblioteca i Rutines s’obren des del perfil o de la capçalera i no ocupen la navegació principal |
| Resum del matí | Un «Bon dia» breu al fil: feines en marxa i bloquejades, resultats de rutines, novetats dels vents, recordatoris del dia i aprovacions pendents. Cada element enllaça a la seva feina o font |
| Calla fins que cal | Només es publica allò que demana atenció o aporta una novetat verificable. L’usuari decideix de què vol saber, cada quan i «Silencia fins demà». Els avisos s’agrupen i es dedupliquen |
| Continua amb el dispositiu apagat | L’execució pertany al servidor (`gregal --serve`), no a la finestra oberta. Desktop i Android en són clients. La interfície mostra on s’executa i si hi ha connexió |
| Prepara sol; abans d’actuar fora de la feina, pregunta | Llegir, redactar i editar fitxers versionats del projecte no requereix aprovació. Esborrar, escriure fora del projecte, publicar, fer push o enviar missatges mostra l’acció exacta en una targeta «Aprova / Ara no» |
| Només l’usuari tanca una feina | L’estat final d’un encàrrec és «Preparat per revisar». L’usuari l’accepta o demana canvis al mateix fil |
| Fa servir el context que ja tens | Els vents utilitzen la memòria personal, les skills i els connectors MCP de Gregal sense configuració addicional. La memòria de cada vent hi afegeix caràcter i relació; no substitueix la general |
| Veus què fa i el pots redirigir | Un resum comprensible del progrés i accions per afegir context, canviar de direcció o aturar la feina mentre avança |
| Judici sobre qui ho fa i quant hi dedica | El router amb model que tria qui parla en una conversa també tria qui fa un encàrrec i quin esforç necessita |

### On Gregal es diferencia

Duet és un sol personatge. ChatGPT dots permet triar un nom, un personatge o una
mascota per a cada dot. Gregal hi afegeix una colla: les veus dels vents conviuen
dins el mateix fil. El router decideix qui respon o treballa, una @menció
s’adreça a un vent concret i la capçalera mostra els retrats de la colla.

A ChatGPT dots, el context que conserva l’agent només s’elimina esborrant el dot
sencer. A Gregal, Memòria i Coneixement continuen sent visibles, editables i
recuperables element per element, tal com descriu la resta del document.

### El fil

- El compositor és un sol component: adjuntar, text, micròfon i mode veu.
- Missatges breus, en diverses bombolles quan convé, abans que un bloc llarg.
- **Targeta d’encàrrec:** títol, qui hi treballa, estat amb text i icona, darrer
  pas comprensible i accions per obrir, redirigir o aturar. Conserva el seu lloc
  cronològic i s’actualitza sense desplaçar la lectura.
- **Targeta de resultat:** primer el lliurable (web, document o fitxer), que
  s’obre al costat; el resum del vent i la revisió queden plegats a sota.
- **Targeta d’aprovació:** acció exacta, destinació, abast i caducitat, amb una
  acció principal i una de secundària.
- Els detalls tècnics (eines, arguments i sortides) viuen a «Activitat». Per
  defecte no mostren JSON ni contingut sencer de fitxers.

### Feines i Fet

**Feines** reuneix el que està en marxa, el que espera l’usuari, el que està
programat i l’historial, amb filtres per vent i projecte. Les rutines hi
apareixen com a feines recurrents, amb l’historial d’execucions. **Fet** és la
prestatgeria de lliurables de cada projecte, amb versions i enllaç a la
conversa d’origen.

### Android i desktop

Les dues apps són portes al mateix fil quan comparteixen servidor. Android
aprofita `WindWatch`, que ja consulta les novetats cada 15 minuts i envia una
notificació local, per al resum del matí, les feines acabades i les aprovacions.
Cada notificació obre la targeta corresponent. A mòbil, una barra inferior amb
les tres destinacions i sense el rail global de l’aplicació; a desktop, les
mateixes tres a la barra lateral i el lliurable obert al costat del fil.

### Punt de partida

Revisió del desktop del 23 d’octubre de 2026, abans d’aplicar aquesta direcció:

- Un encàrrec obre una vista separada. La conversa només en rep un enllaç al
  final, el vent no hi publica el resultat i una petició de seguiment feta des
  de la conversa no rep el resultat anterior ni la carpeta.
- La feina en marxa depèn que l’app i la connexió continuïn obertes.
- Un revisor automàtic decideix les aprovacions; va autoritzar sol un `rm -rf`
  de fitxers temporals dins del projecte.
- Un encàrrec queda com a acabat encara que el revisor hi demani correccions.
- El cor proactiu, els recordatoris i les novetats funcionen al servidor, però el
  desktop no els mostra.
- Les converses trien qui respon amb un router amb model; els encàrrecs, amb
  expressions regulars.

### Ordre d’entrega

1. **Fil únic** amb targetes d’encàrrec i de resultat, que substitueixen la vista
   de tasca separada.
2. **Execució al servidor**, persistent i represa després d’un reinici, amb
   notificacions a Android i desktop.
3. **Resum del matí i «Et necessita»**, a partir del cor, les novetats, els
   recordatoris i el check-in existents.
4. **Aprovacions per acció** i tancament de cada feina per part de l’usuari.
5. **Context compartit** de memòria, skills i connectors; després, caràcters
   amb aspecte i veu propis.

### Criteris d’acceptació

- «Fes una web per a la cafeteria» i, més tard, «fes-la més càlida» al mateix fil
  produeixen dues versions del mateix lliurable sense copiar context.
- Tancar el desktop durant un encàrrec no l’atura si el servidor continua actiu,
  i Android rep l’avís quan acaba o quan necessita una decisió.
- Cap esborrat, publicació, push ni enviament es fa sense una aprovació visible
  al fil.
- Un dia sense novetats no genera missatges proactius, a part del resum si està
  activat.
- Cada element del resum enllaça amb la seva feina o font.

## Colles predefinides: especialistes útils des del primer encàrrec

Les colles o squads són plantilles d’equip amb especialistes i formes de
col·laborar. S’activen amb un toc; no cal redactar personalitats, configurar
fluxos ni assignar models a cadascun. Un encàrrec mobilitza només els membres
necessaris. Tenir sis especialistes disponibles no implica sis crides al model.

### Colla quotidiana — la porta d’entrada al mòbil

| Especialista | Definició bàsica útil | Resultat esperat |
| --- | --- | --- |
| Compres i productes | Entén ús, pressupost i restriccions; contrasta preus, compatibilitat, manteniment i disponibilitat amb fonts actuals | Opcions comparables, recomanació raonada, enllaços i data de consulta |
| Llibres i cultura | Recorda lectures i gustos; compara estil, temàtica, llargada i edicions; distingeix dades de valoracions | Selecció curta amb motius personals, sense espòilers per defecte |
| Esports | Segueix equips, competicions i interessos; consulta resultats i calendaris actuals | Resum, properes cites i avisos dels esdeveniments que segueixes |
| Música i descobertes | Aprèn gustos, artistes coneguts i què vols descobrir; compara discos i cerca concerts | Descobertes explicades, llistes o enllaços, sense repetir sempre els mateixos artistes |
| Organització | Converteix una conversa en llista, recordatori, encàrrec o rutina; recupera pendents reals | Pla breu o rutina amb freqüència i resultat esperat clars |

Els personatges coneguts es poden conservar: Xaloc per música, Ponent per
llibres i Gregal per organització; Marinada pot encarregar-se de la recerca i
comparació. Els noms i aspectes són editables. No es reassignen especialitats
d’un vent existent ni s’omplen records personals sense una decisió de l’usuari.

La colla quotidiana és la suggerida al mòbil, però està disponible al desktop.
Cap colla queda vinculada exclusivament a un dispositiu.

### Colla de ciència de dades

| Especialista | Definició bàsica útil | Comprovació que aporta |
| --- | --- | --- |
| Preparació i qualitat de dades | Inspecciona esquema, mancants, duplicats, unitats i procedència; prepara transformacions reproduïbles | Informe de qualitat i traçabilitat de les transformacions |
| Estadística i regressions | Formula la pregunta, defineix variables i supòsits; ajusta regressions i explica efectes i incertesa | Diagnòstics, residus i límits de la interpretació; no confon correlació amb causalitat |
| Machine learning | Compara una base senzilla amb models candidats; considera classificació, regressió, agrupament i mètriques pertinents | Separació entrenament/validació, absència de filtració d’informació i comparació amb la base |
| Sèries temporals | Estudia tendència, estacionalitat, horitzó i dades disponibles al moment de predir | Validació temporal, comparació amb prediccions ingènues i intervals d’incertesa |
| Visualització i revisió | Explica els resultats amb gràfics i conclusions; contrasta que dades, codi i relat concordin | Lliurable reproduïble, comprovacions realitzades i preguntes que segueixen obertes |

No hi ha un vent per cada algoritme. Les tècniques i procediments consultables
viuen al Coneixement de l’especialista. La primera resposta es basa en les dades
reals disponibles; si encara no hi ha dades, demana l’entrada necessària i no
presenta prediccions o coeficients inventats.

Exemple: «Preveu les vendes dels pròxims tres mesos amb aquest CSV». Qualitat
inspecciona el fitxer, Sèries temporals modela i valida, i Visualització presenta
la previsió amb les limitacions. No cal activar Machine learning si la tasca
ja queda ben resolta amb una alternativa més senzilla.

### Colla de software i ciberseguretat

| Especialista | Definició bàsica útil | Resultat esperat |
| --- | --- | --- |
| Producte, UX i accessibilitat | Concreta el problema de l’usuari i els criteris d’acceptació; revisa els recorreguts | Proposta o canvi amb criteris observables i recorregut accessible |
| Frontend | Construeix interfícies adaptables dins del disseny i eines del projecte | Interfície executable, estats coherents i comprovació visual |
| Backend i arquitectura | Dissenya dades, API, integracions i persistència amb criteri de simplicitat | Implementació coherent amb el projecte i migracions quan calguin |
| Qualitat i depuració | Reprodueix errors, revisa canvis i executa comprovacions pertinents | Defectes reproduïbles, correccions i proves amb resultat explícit |
| Seguretat d’aplicacions | Revisa autenticació, permisos, dades, dependències i superfície d’atac | Troballes amb evidència, impacte i correcció; diferencia sospita i vulnerabilitat comprovada |
| Infraestructura i lliurament | Prepara entorns, integració contínua, paquets i observabilitat | Build reproduïble, paquet utilitzable i diagnòstic dels errors |

La revisió de seguretat s’aplica als repositoris, entorns i sistemes que
formen part de l’encàrrec. Les comprovacions actives expliciten l’objectiu i
l’abast abans d’executar-se. Es pot ampliar després amb una colla específica
de ciberseguretat si les necessitats ho justifiquen: modelatge d’amenaces,
enduriment d’infraestructura i resposta a incidents, entre altres especialitats.

Exemple: «Afegeix inici de sessió a aquesta web». Backend implementa,
Frontend integra i Seguretat/Qualitat revisen els aspectes pertinents. El fil
mostra qui ha fet què i obre la web resultant al costat.

### Què porta cada plantilla de vent

1. Missió, especialitat i criteris de qualitat concrets.
2. Veu i aspecte propis, independents de les dades personals.
3. Coneixement inicial amb guies breus, procediments i fonts identificades.
4. Eines necessàries per al seu ofici, subjectes al context de treball existent.
5. Definició del lliurable i de quan convé demanar una segona mirada.
6. Memòria personal inicialment buida, que es construeix amb la relació real.

La plantilla d’un especialista no demostra que ja hagi executat una anàlisi,
tingui accés a una eina o conegui l’usuari. Les guies inicials tenen versió i
procedència; s’actualitzen sense sobreescriure records ni contingut afegit per
l’usuari. S’utilitza el model configurat i només es mostra una necessitat addicional
quan una capacitat imprescindible no està disponible.

### Activació, context i biblioteca

- «Afegeix una colla» ofereix Quotidiana, Ciència de dades i Software i seguretat,
  amb una explicació curta i un exemple real de què pot resoldre cada equip.
- Una colla es pot vincular a un projecte. Els encàrrecs fan servir la seva
  carpeta i coneixement rellevant sense preguntar-ho a cada missatge.
- Els especialistes disponibles no ocupen tota la pantalla: es mostren els
  favorits, els participants actius i un selector compacte de colla.
- Les definicions són lleugeres; el contingut extens només es consulta quan cal.
  Un model de planificació addicional no és obligatori per als casos senzills.
- Un mateix vent existent pot participar en més d’una colla sense duplicar
  silenciosament la seva identitat o memòria. Crear una còpia és una acció explícita.
- Cada vent conserva Memòria i Coneixement propis. Les guies de projecte
  compartides es presenten com a tals; afegir una colla no dona accés global
  a tota la memòria personal.
- Un procediment transferit a un altre vent conserva font, autoria original i
  revisions. Es pot assignar a tota una colla amb una acció explícita.
- Eliminar una colla no purga la informació. Si el vent participa en altres
  colles, continua actiu; els elements sense assignació queden recuperables a
  Biblioteca. L’esborrament definitiu és una operació diferenciada.

### Prioritat i criteris d’entrega de les colles

Primer, la colla quotidiana i el suport comú de plantilles, perquè ofereixen
continuïtat amb Android. Després, Software i seguretat sobre el motor de feina
existent. Ciència de dades arriba amb execució i lliurables verificables de dades,
i no només amb noms de personatges especialistes.

Una entrega és útil quan es pot activar una colla i completar un dels exemples
anteriors sense editar prompts ni configurar els agents individualment; es veu
quins membres han treballat i el resultat; i, després d’arxivar-ne un membre,
es poden recuperar els seus dos sistemes i assignar-los a un substitut.

## Criteris de la primera entrega

- Una preferència desada per Marinada arriba a les seves tasques, no només al xat.
- Un coneixement llarg es consulta quan cal, sense afegir-lo sencer a cada petició.
- Després d’eliminar Marinada i reiniciar, els seus dos sistemes continuen
  accessibles des de Biblioteca → Arxivats.
- Assignar-los a Mestral conserva la informació prèvia de Mestral, l’origen i
  l’arxiu original. Es poden veure i desfer els canvis.
- Canviar noms no altera les assignacions; crear un vent amb un nom antic no
  s’apropia de dades automàticament.
- Android i desktop mostren els mateixos continguts quan comparteixen servidor.

## Referències consultades

- Letta documenta blocs persistents que es poden crear independentment i
  assignar o desvincular d’agents. És una referència per separar dades i identitat,
  sense necessitat d’adoptar el seu motor:
  https://docs.letta.com/tutorials/attaching-detaching-blocks/
- Obsidian diferencia la recuperació per instantànies de les còpies de seguretat.
  És una referència per a versions recuperables i per no confondre arxivar un
  vent amb protegir-se d’una pèrdua del disc:
  https://obsidian.md/help/plugins/file-recovery
- Grok Bot descriu treball per missatge, context persistent, coordinació entre
  bots i aprenentatge de procediments. La proposta n’adapta aquestes idees:
  https://docs.x.ai/grok-bot/overview
- Les automatitzacions de Grok documenten programació des del xat, historial
  d’execucions com a converses i controls de notificació:
  https://x.ai/news/grok-automations
- Claude Duet descriu un agent proactiu amb un sol fil, resum del matí, feina en
  segon pla i aprovacions abans d’enviar, pagar, signar o esborrar. És la
  referència de la «Direcció Duet»:
  https://claude.com/resources/articles/claude-duet
  https://claude.com/product/claude-duet
- ChatGPT dots, d’OpenAI, documenta un agent sempre actiu amb responsabilitats
  continuades, activitat en marxa/programada/completada, regles sobre què pot fer
  sol, i un nom i personatge propis:
  https://help.openai.com/en/articles/20001530-getting-started-with-your-dot
- Dot de New Computer és un producte diferent de ChatGPT dots. El seu anunci de
  tancament reforça la necessitat d’exportació i recuperació:
  https://new.computer/
