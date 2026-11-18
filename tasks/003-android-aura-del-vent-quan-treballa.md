---
title: "Android: aura del vent quan treballa"
status: todo
depends_on: []
---

## Why

Els vents ja tenen cara, respiració i insígnies d'estat, però mentre treballen no es nota prou a primer cop d'ull. Una aura de llum del color del vent fa visible que hi ha feina en marxa.

## Acceptance criteria

- [ ] Una cara de vent gran (64 dp o més) en un estat de feina (pensant, cercant, llegint, escrivint, treballant, discutint) mostra una aura del color del vent darrere seu; en repòs, content, parlant, fet, error o novetat no en té.
- [ ] L'aura apareix i desapareix amb un fos en canviar d'estat, i gira i batega només si el moviment està permès (`windMotionEnabled`); sense moviment queda quieta.
- [ ] Les cares petites de les llistes (menys de 64 dp) no tenen aura.

## Out of scope

- Canviar les insígnies, la cara o el creador de vents.

## Notes

- Fitxer: `WindFace.kt`.
