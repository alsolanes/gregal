---
title: "Android Vents: calaix amb converses a dalt i Configuració com a pàgina"
status: done
depends_on: []
---

## Why

Paritat amb la 180 i la 181. Al calaix d'Android, «Configuració i més» (`DrawerGroup` a `Drawer.kt` ~249) també desplega opcions dins el calaix, i les converses queden avall.

## Acceptance criteria

- [x] L'ordre del calaix de Vents és: Nova conversa, Cerca (si n'hi ha), Avui; Converses; Vents; la resta d'espais; i a baix una sola fila «Configuració» / «Settings». Test de UI de Compose o unitari de l'ordre.
- [x] La fila «Configuració» no desplega res: obre una pantalla de Configuració amb les subseccions que avui hi ha al grup (Preferències, l'ordinador, A prop teu, la guia, Els meus vents, i Programar des del mòbil), cadascuna amb icona, nom i una línia d'explicació, i cada una obre el que obre ara. Test.
- [x] El botó enrere del sistema des d'una subsecció torna a Configuració. Test.
- [ ] Textos a `Strings.kt`/`AppStrings.kt` en ca i en, editats a mà. Comprovat a l'emulador (emulator-5554).
- [x] (Afegit pel coordinador, a petició de l'usuari: veure abans les converses amb els vents.) Sota «Converses», les converses de la més nova a la més antiga, a soles i en grup (com `renderThreads` a la web), amb el nom i la cara del vent (els grups, la icona de grup); tocar-ne una l'obre. Se'n veuen cinc; si n'hi ha més, «Mostra'n més (N)» / «Show more (N)» desplega la resta al lloc i «Mostra'n menys» / «Show less» la plega. La conversa oberta sempre es veu (i és l'única fila marcada). «Totes les converses» / «All conversations» (l'antiga «Xats») queda l'última i obre el panell de converses. Sense converses, només aquesta fila. Tests a `DrawerConversationsTest.kt`.

## Out of scope

La pantalla de Preferències per dins.

## Notes

- `android/app/src/main/java/xyz/solanes/gregal/Drawer.kt` (`DrawerGroup`, seccions ~125-260), `MainActivity.kt` (Screen). Tests amb ANDROID_HOME (memòria del projecte). No hi afegeixis cap dependència.
- Fet: `Screen.Settings` i `SettingsScreen.kt`; la fila «Configuració» és al peu fix del calaix (amb «Canvia de servidor»), i queda marcada a Configuració i a Preferències. Tests a `SettingsPageTest.kt`.
- El calaix d'Android no té cap fila de Cerca, així que l'ordre comença amb Nova conversa i Avui.
- Enrere: `openVentsPanel` recorda si el panell s'ha obert des de Configuració, i `closeVentsPanel` (que fan servir `back()` i la portada quan tanca el panell sola) hi torna. Obrir una conversa des del panell és anar endavant i no hi torna. «A prop teu» ja no passa per la portada (va directe a Automatitzacions), perquè Enrere torni a Configuració.
- `VentsWebParityTest` ha canviat perquè el requisit ha canviat: l'ordinador ja no es veu desplegant «Configuració i més» al calaix, sinó obrint la pàgina Configuració.
- Emulador: NO comprovat. emulator-5554 té 485 MB lliures a /data, per sota del llindar d'instal·lació d'Android (INSTALL_FAILED_INSUFFICIENT_STORAGE), i alliberar-hi espai vol dir esborrar apps o dades. Comprovat només amb captures de Robolectric (calaix i pàgina, ca i en) fora del repositori.
- Converses: es fa servir `ventsInbox` (el mateix que la llista de la portada) filtrat per `hasMessages`, i per això hi ha una fila per vent, que obre el fil que tingui triat, i no una per fil com a la web (`/api/winds/threads`). La fila de Gregal, a Vents, només queda marcada a la portada o amb el seu fil obert si no surt a Converses.
- `SettingsPageTest` ha canviat perquè el requisit ha canviat: l'ordre ara té «Totes les converses» on hi havia «Xats».
- Follow-up: el comentari de `VentsAskHome.kt:66` encara diu que les converses són a «Xats».
- Follow-up: el test d'Enrere crida `vm.back()`/`closeVentsPanel()`; no hi ha cap test de Compose que passi pel BackHandler real de `CompanyScreen` amb el panell obert des de Configuració.
