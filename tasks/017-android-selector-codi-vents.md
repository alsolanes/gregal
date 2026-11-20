---
title: "Android: selector Codi | Vents"
status: todo
depends_on: []
---

## Why

A Android, Codi i Vents ja viuen a la mateixa app, però per passar d'un a l'altre cal anar a Preferències i girar l'interruptor «Eines de programació». Un selector a la vista ho fa d'un toc, com el de la web.

## Acceptance criteria

- [ ] A dalt del calaix hi ha un selector segmentat «Codi | Vents» (en català i en anglès), a l'app Gregal i a l'app Gregal Vents.
- [ ] Tocar «Vents» posa el calaix dels vents i obre la pantalla dels vents; tocar «Codi» posa el calaix complet i obre el xat de Codi. Sense recarregar ni perdre la connexió.
- [ ] L'app recorda l'últim mode triat; si no n'hi ha cap, Gregal Vents comença a Vents i Gregal a Codi, com ara.
- [ ] L'interruptor «Eines de programació» de Preferències desapareix (el selector el substitueix), i el seu text es treu de tots dos idiomes.
- [ ] La pàgina de Permisos de Preferències es veu en tots dos modes (la regla és comuna, tasca 018).
- [ ] Tests: el selector canvia el calaix i la pantalla; el mode es desa i es torna a carregar; el valor per defecte depèn del paquet.

## Out of scope

- Fusionar les dues apps en una (es mantenen els dos APK i les dues actualitzacions).
- La web: ja té el selector Codi/Vents (`#productSwitch`); que canviï sense recarregar la pàgina seria una tasca a part.

## Notes

- Avui: `simpleVents` (`MainActivity.kt` ~198, per defecte `packageName.endsWith(".vents")`), interruptor a `PrefsScreen.kt` ~143 (`vm.setSimpleVents(!it)`), textos a `AppStrings.kt` ~902 i ~2526, calaixos a `Drawer.kt` (`SimpleVentsDrawer` ~163, complet ~100), pantalla inicial a `GregalViewModel.kt` ~6335, Permisos amagats a `PrefsPage.kt` ~35 i `PrefsScreen.kt` ~128.
- Reaprofitar `simpleVents` com a estat del selector (canviar-ne el nom si queda confús), no afegir-ne un altre.
- `docs/separacio-codi-vents.ca.md` diu que a mòbil són dues apps separades: actualitzar-hi la fila del mòbil.
