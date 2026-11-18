---
title: "Android: vidre esmerilat a Android 12+"
status: done
depends_on: [1]
---

## Why

El vidre llueix de debò quan el que passa per sota es veu desenfocat. Android ho permet a partir de l'API 31 (Android 12); l'app arriba fins a l'API 26.

## Acceptance criteria

- [x] Amb Vidre encès a Android 12 o més, la capçalera i el compositor del xat pinten a sota, desenfocat, el contingut que tenen darrere (una vora nítida del contingut queda difuminada sota la barra).
- [x] En aquest cas la llista de missatges passa per sota de la capçalera i del compositor (el marge passa a ser interior), i el darrer missatge es pot continuar llegint sencer per sobre del compositor.
- [x] Per sota d'Android 12, o amb Vidre apagat, no es desenfoca res i el xat queda com ara (la llista s'atura a les barres).

## Out of scope

- Desenfocar altres pantalles, fulls o diàlegs.
- Cap dependència nova (res de Haze): es fa amb `GraphicsLayer` i `RenderEffect` de la plataforma.

## Notes

- Fitxers: `Screens.kt` (`ChatScreen`), un modificador nou al costat de `SeaBackdrop` a `Marea.kt`.
