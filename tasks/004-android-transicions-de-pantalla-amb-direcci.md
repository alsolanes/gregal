---
title: "Android: transicions de pantalla amb direcció"
status: done
depends_on: []
---

## Why

Ara les pantalles canvien amb el fos per defecte d'`AnimatedContent`, igual endavant que enrere. Una transició que entra per la dreta en obrir i per l'esquerra en tornar dona sensació d'app moderna i orienta.

## Acceptance criteria

- [x] Obrir una pantalla que apila (la pila d'enrere creix, també des del calaix, que apila) la fa entrar des de la dreta amb fos; tornar enrere (la pila decreix) la fa entrar des de l'esquerra; un canvi amb la pila igual (p. ex. `showVentet` des del Ventet) només fa fos.
- [x] Sense moviment (`windMotionEnabled` fals), el canvi de pantalla és immediat, sense transició.

## Out of scope

- Transicions d'elements compartits i animacions dins de cada pantalla.

## Notes

- Fitxer: `MainActivity.kt` (l'`AnimatedContent` de les pantalles) i `GregalViewModel.kt` (`backStack`).
- La direcció surt de comparar la mida de `backStack` abans i després del canvi (`navDirection` i `screenTransition` a `MainActivity.kt`). `ScreenSwitcher` hi té l'`AnimatedContent` amb estat `pantalla to profunditat` i la pantalla com a clau, perquè `go()` i `back()` actualitzen pila i pantalla en ordre diferent i buidar la pila no ha de recrear la pantalla visible. `ScreenSwitcherUiTest` mira on és la pantalla que entra uns 100 ms després.
- Sense moviment, `ScreenSwitcher` no fa servir `AnimatedContent`: fins i tot amb `EnterTransition.None`/`ExitTransition.None` la pantalla vella hi queda un fotograma de més.
- Tornar enrere amb la pila buida (cap als Vents, `Screen.Company`) deixa la pila igual, així que només fa fos.
- Follow-up: avui el calaix navega amb `vm.go()`, que apila, així que canviar d'arrel des del calaix fa la pila més gran i llisca endavant; el criteri del fos sol s'aplica quan la pila queda igual (p. ex. `showVentet` des del Ventet). Si el calaix ha de ser un canvi d'arrel de debò, cal que no apili.
- No provat en un dispositiu: el moviment real (durades 260/220/160 ms) només s'ha comprovat amb proves JVM.
