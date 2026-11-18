---
title: "Android: transicions de pantalla amb direcció"
status: todo
depends_on: []
---

## Why

Ara les pantalles canvien amb el fos per defecte d'`AnimatedContent`, igual endavant que enrere. Una transició que entra per la dreta en obrir i per l'esquerra en tornar dona sensació d'app moderna i orienta.

## Acceptance criteria

- [ ] Obrir una pantalla que apila (la pila d'enrere creix) la fa entrar des de la dreta amb fos; tornar enrere (la pila decreix) la fa entrar des de l'esquerra; canviar d'arrel des del calaix (pila igual) només fa fos.
- [ ] Sense moviment (`windMotionEnabled` fals), el canvi de pantalla és immediat, sense transició.

## Out of scope

- Transicions d'elements compartits i animacions dins de cada pantalla.

## Notes

- Fitxer: `MainActivity.kt` (l'`AnimatedContent` de les pantalles) i `GregalViewModel.kt` (`backStack`).
