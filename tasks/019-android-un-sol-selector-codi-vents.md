---
title: "Android: un sol selector Codi | Vents"
status: done
depends_on: [017]
---

## Why

`main` va rebre per una altra banda un selector Vents | Codi al calaix (2053bd80, que també recorda l'última pantalla de cada mode i afegeix una drecera a la capçalera dels Vents), i `assistent-v2` el de la tasca 017. Fusionats tal com eren, el calaix en tindria dos.

## Acceptance criteria

- [x] `assistent-v2` porta el commit de `main` i el calaix només té un selector: el de `main` (`ModeSwitcher`, `vm.switchArea`), que fa més coses.
- [x] De la tasca 017 es queda el que el selector de `main` no feia: sense l'interruptor «Eines de programació», la pàgina de Permisos en tots dos modes, i Enrere des de la portada del mode (xat a Codi, vents a Vents) surt de l'app en lloc de portar a l'altre mode.
- [x] Els textos del selector i de la drecera són a `AppStrings.kt` en català i en anglès, no escrits al codi.
- [x] Els tests de la 017 (`DrawerModeSwitchTest`, `BackStackTest`) i els de `main` (`ModeSwitcherTest`, `ModeSwitchViewModelTest`, `ModeNavigationTest`) passen sobre el selector que queda.

## Out of scope

- Canviar on és el selector o com es veu.
- Publicar una versió (el commit de `main` ja puja els números d'Android a 1.94.1-vents / 1.63.0).

## Notes

- Fet a la fusió de `origin/main` a `assistent-v2`: el conflicte d'Enrere (`back()`) es resol amb la portada del mode (`home`) i desant-la com a última pantalla (`rememberAreaScreen`). Es treuen `ModeSwitch` (Drawer.kt) i `vm.switchMode` de la 017.
- `DrawerModeSwitchTest` buscava les opcions com a botons de ràdio (`ChoicePills`); el selector que queda les marca com a pestanyes (`Role.Tab`).
- Textos nous: `app.mode.toVents` / `app.mode.toCode` (la drecera); el selector fa servir `app.mode.vents` / `app.mode.code` de la 017.
- Després d'aquesta fusió, `main` pot avançar fins a `assistent-v2` sense conflictes.
