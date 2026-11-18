---
title: "Android: estil Vidre amb fons viu"
status: todo
depends_on: []
---

## Why

L'usuari vol una opció a la configuració de l'app Android perquè tota l'app tingui un aire modern de vidre (glassmorphism), sense perdre els temes ni el mode clar/fosc actuals.

## Acceptance criteria

- [ ] A Preferències → Aparença hi ha un interruptor «Vidre» (en anglès «Glass»), apagat per defecte; en encendre'l queda desat i torna encès en reobrir l'app.
- [ ] Amb Vidre encès, la paleta de qualsevol tema i mode té les superfícies (`surface`, `surfaceHi`) translúcides i l'accent i el text iguals; el text (`ink`, `muted`) sobre la superfície composta damunt del fons manté un contrast de 4,5:1 o més.
- [ ] Amb Vidre encès, el fons de l'app (`SeaBackdrop`) pinta unes taques de llum amb els colors de la paleta (accent, aqua, lila); amb Vidre apagat el fons és exactament el d'ara.
- [ ] Les taques es mouen lentament només si el moviment expressiu és actiu i Android no té les animacions eliminades; si no, queden quietes.

## Out of scope

- El desenfocament real del que hi ha darrere (tasca 002).
- La web, l'escriptori i la resta de clients.

## Notes

- Fitxers: `Theme.kt` (paleta i `scheme()`), `Marea.kt` (`SeaBackdrop`), `MainActivity.kt` (`Prefs`, arrel), `GregalViewModel.kt`, `PrefsScreen.kt`, `AppStrings.kt`.
- Menús, diàlegs i fulls de Material es queden gairebé opacs perquè es llegeixin damunt del contingut.
- Paritat: només Android. La web i l'escriptori no tenen estil Vidre.
