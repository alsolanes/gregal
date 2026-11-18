---
title: "Android: vidre esmerilat a Android 12+"
status: todo
depends_on: [1]
---

## Why

El vidre llueix de debò quan el que passa per sota es veu desenfocat. Android ho permet a partir de l'API 31 (Android 12); l'app arriba fins a l'API 26.

## Acceptance criteria

- [ ] Amb Vidre encès, la llista de missatges del xat passa per sota de la capçalera i del compositor (ja no s'atura al seu límit) i el darrer missatge continua sencer visible per sobre del compositor.
- [ ] A Android 12 o més, la capçalera i el compositor del xat pinten a sota el contingut que tenen darrere, desenfocat.
- [ ] Per sota d'Android 12, o amb Vidre apagat, la capçalera i el compositor es veuen com a la tasca 001, sense desenfocament.

## Out of scope

- Desenfocar altres pantalles, fulls o diàlegs.
- Cap dependència nova (res de Haze): es fa amb `GraphicsLayer` i `RenderEffect` de la plataforma.

## Notes

- Fitxers: `Screens.kt` (`ChatScreen`), un modificador nou al costat de `SeaBackdrop` a `Marea.kt`.
