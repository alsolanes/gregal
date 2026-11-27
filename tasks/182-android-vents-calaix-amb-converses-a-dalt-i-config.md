---
title: "Android Vents: calaix amb converses a dalt i Configuració com a pàgina"
status: todo
depends_on: []
---

## Why

Paritat amb la 180 i la 181. Al calaix d'Android, «Configuració i més» (`DrawerGroup` a `Drawer.kt` ~249) també desplega opcions dins el calaix, i les converses queden avall.

## Acceptance criteria

- [ ] L'ordre del calaix de Vents és: Nova conversa, Cerca (si n'hi ha), Avui; Converses; Vents; la resta d'espais; i a baix una sola fila «Configuració» / «Settings». Test de UI de Compose o unitari de l'ordre.
- [ ] La fila «Configuració» no desplega res: obre una pantalla de Configuració amb les subseccions que avui hi ha al grup (Preferències, l'ordinador, A prop teu, la guia, Els meus vents, i Programar des del mòbil), cadascuna amb icona, nom i una línia d'explicació, i cada una obre el que obre ara. Test.
- [ ] El botó enrere del sistema des d'una subsecció torna a Configuració. Test.
- [ ] Textos a `Strings.kt`/`AppStrings.kt` en ca i en, editats a mà. Comprovat a l'emulador (emulator-5554).

## Out of scope

La pantalla de Preferències per dins.

## Notes

- `android/app/src/main/java/xyz/solanes/gregal/Drawer.kt` (`DrawerGroup`, seccions ~125-260), `MainActivity.kt` (Screen). Tests amb ANDROID_HOME (memòria del projecte). No hi afegeixis cap dependència.
