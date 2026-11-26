---
title: Vents comprova actualitzacions en tornar a primer pla
status: done
depends_on: []
---

## Why

L'app Vents només mira si hi ha versió nova quan el procés arrenca de zero. Si Android la manté viva de fons, una release publicada no surt fins que algú la tanca del tot o prem «Comprova actualitzacions».

## Acceptance criteria

- [x] En tornar a primer pla, l'app fa la comprovació silenciosa si l'última té més d'una hora.
- [x] Si l'última comprovació és de fa menys d'una hora, no torna a trucar el servidor.
- [x] L'arrencada segueix comprovant una vegada (la primera passada per primer pla), no dues.

## Out of scope

La caché de 6 h del manifest, la comprovació manual de Preferències i l'app Gregal normal (no té OTA).

## Notes

MainActivity.onStart, GregalViewModel.checkUpdate, Updater.kt. Només Android: web i escriptori tenen el seu propi actualitzador.
