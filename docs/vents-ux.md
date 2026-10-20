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
