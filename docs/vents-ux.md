# Vents: companys, converses i memòria

## Interacció

- El mode **Vents** obre les converses. **Els meus vents** és la gestió separada de companys.
- Una única acció **+** permet crear vent, xat individual amb nom o rotllana.
- El creador comença amb una galeria de vuit personalitats. Triar-ne una obre nom, interès i caràcter editables; l'aparença detallada és opcional. Editar un vent existent obre directament les seves dades, no la galeria.
- Tocar la capçalera del vent obre **Caràcter / Memòria / Activitat**. Els coneixements es poden corregir o buidar amb confirmació; la memòria personal es mostra separadament i és de només lectura en aquesta fitxa.
- Cada resposta d'una rotllana porta nom i cara del parlant. La capçalera permet consultar perfils i modificar membres.
- El botó enrere tanca primer la subpantalla, després torna per les pantalles visitades. No s'apila la connexió inicial com a destí al qual tornar.

## Persistència i compatibilitat

Els catàlegs i el coneixement viuen a l'origen actiu: al dispositiu en mode local, o al servidor en mode remot. Dos clients del mateix compte i servidor consulten el mateix estat; no és una còpia automàtica de les dades locals al servidor.

Els comptes autenticats disposen de `users/<nom>/` per als seus vents, rotllanes, coneixements i memòria personal. El mode sense comptes / token legacy «Tothom» manté el comportament compartit anterior. La memòria global legacy no es copia silenciosament als comptes nous.

Els ids existents dels vents no canvien en reanomenar-los: l'actualització envia un `id` explícit i conserva historial i coneixements. No es fa cap esborrat dels vents ni dels historials previs.

Les converses amb nom reutilitzen el magatzem de rotllanes: un membre és un fil individual; dos o més, una rotllana. Cada creació genera un id independent, encara que el títol coincideixi. L'historial continu anterior de cada vent continua disponible.

Els missatges de grup serialitzen identificadors de parlant, mantenint compatibilitat de lectura amb historials antics que només portaven `Nom: text`. La conversa individual incorpora els torns previs al context del model, no només els desa al disc.

En canviar de compte o servidor, Android buida l'estat dels companys i descarta respostes de generacions anteriors. Els editors no confirmen el desat fins haver llegit el resultat del servidor.

## Memòria i límits

La memòria personal i els coneixements temàtics són diferents. L'aprenentatge temàtic automàtic no és una autorització per inventar preferències personals. Les fonts extretes han de ser dominis presents a l'intercanvi; això evita promocionar citacions inventades, però no certifica per si sol la veracitat d'un fet.

L'edició explícita de coneixements substitueix tot el text del vent (màxim 24 KiB); text buit significa oblidar aquests coneixements, no esborrar la conversa ni la memòria personal. Les noves rutes i els cossos es documenten a `api-contract.md`.

## Verificació reproduïble

## Pass Android: al nivell de les apps de companys

Referents: Claude Duet, els companys de Grok i OpenAI dot. Criteri: el vent és el protagonista i la maquinària no es veu.

- **Portada:** les cares dels teus vents a dalt (un toc hi parla; «Tots» obre el directori), la salutació al mig i el camp a baix, on arriba el polze. Amb Vents net, la capçalera només porta el menú, el títol, les pestanyes i el `+`. La píndola de servidor i les icones d'Explora i del directori desapareixen (la connexió la diu el calaix), i la cerca només surt a «Xats».
- **Calaix (Vents net):** Vents, Els meus vents, Explora i Notes. Programats, Desats, Wiki, Biblioteca i Feines viuen a Explora. L'entrada «Ventet» desapareix, perquè el Ventet és la mateixa portada. Les eines de codi queden plegades a «Codi».
- **Fitxa del vent:** primer el que sap de tu (memòria), després programats, desats i calaix, amb quatre pestanyes com a molt. El prompt, el nivell, les xifres i les fonts de l'ofici queden plegats a «Instruccions i ofici».
- **Espera:** cap rellotge ni «Pas N». La franja de dalt només surt si el vent fa una feina que es pot dir (cerca, llegeix, una eina) o si fa 20 s que no arriba res. Mentrestant, al fil el vent «escriu» amb la seva cara i els punts. Aturar és al camp de text.
- **Lectura:** en acabar el torn, una resposta que no cap a la pantalla es mostra des del seu inici, i surt «Últims ↓».
- **Respostes sense caixa:** el que escrius tu va en bombolla. El que respon el vent és text net sobre el fons, a tota l'amplada. El color del vent és a la cara, als enllaços i als xips de resposta.
- **Xat net:** amb Vents net no surt la píndola de mode («Actua»), que continua al `+`. A la capçalera hi ha un sol «⋯» amb noms (Perfil, Converses, Resumeix a la wiki) en lloc de dues icones soles.
- **Explora** és un full que puja des de baix, no una llista dins de la capçalera.

## Pass Android 1.44

- La portada és conversa-primer: el Ventet es pot fixar a l’inici des de Preferències, però per defecte no desplaça els fils. La gestió de Vents continua al directori i la creació passa per una sola acció `+`.
- Els Programats consideren també les rutines LLM reals en el seu estat buit. Des del perfil, el cor del Vent es desa i es torna a llegir abans de deixar l’interruptor, inclòs quan es reactiva un job existent.
- El canvi d’origen buida totes les memòries auxiliars de Vents i cancel·la el polling d’experts; les respostes tardanes d’un altre servidor no poden repoblar el perfil actual. La càrrega de portada usa resums i deixa l’última organització bona si el GET falla.
- La memòria pròpia manté l’edició oberta si el desat falla. El moviment decoratiu és quiet per defecte i es pot activar amb **Moviment expressiu**; les cares i indicadors continuen mostrant treball real.
- Els xats individuals del creador són d’un sol vent; les rotllanes mantenen selecció múltiple. El preview Vents és `1.44.0-vents` / `vc145`.

La direcció d’interacció pren com a referència la separació entre autoria i context de [SillyTavern Character Design](https://docs.sillytavern.app/usage/core-concepts/characterdesign), la gestió de converses de [Kindroid Groupchats](https://docs.kindroid.ai/docs/groupchats) i la memòria editable de [Kindroid](https://kindroid.ai/v2/docs/memory). Són referències d’usabilitat, no promeses de capacitat ni còpia de producte.

```sh
# Go (en un entorn amb Go 1.27.1)
go test ./...

# Desktop: frontera DOM/API de prova, sense model real
node --test internal/web/winds.test.js

# Android: inclou proves Robolectric/Compose
cd android
./gradlew testDebugUnitTest -Pgregal.skipBackend -Pgregal.plotsPreview
```

`gregal.skipBackend` només s'ha d'usar per lliurar un APK si les dues llibreries natives ja s'han recompilat des dels mateixos fonts amb l'NDK. Verificar package, versionCode/versionName, signatura i la llibreria de l'ABI abans de distribuir.

La QA visual ha d'incloure galeria clara/fosca, text ampliat, edició, memòria real, rotllana, teclat i enrere. Les animacions decoratives respecten la desactivació d'animacions del sistema i es poden desactivar a les proves de UI amb `LocalWindMotion` sense suprimir cap interacció.
