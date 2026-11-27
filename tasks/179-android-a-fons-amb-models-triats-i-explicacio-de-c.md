---
title: "Android: «A fons» amb models triats i explicació de cada agent"
status: todo
depends_on: [178]
---

## Why

Paritat amb la 178: a Android també s'han de poder triar els models d'«A fons» i veure què fa cada agent.

## Acceptance criteria

- [ ] A Preferències de Vents, una secció «A fons» amb un selector de model per a cada agent (Directe, Fonts, Advocat del diable) i la síntesi, amb «Automàtic», que llegeix i desa amb el mateix endpoint de la 178. Test unitari (parse i payload).
- [ ] A la vista de la Tempesta del xat (`Tempesta.kt`), cada agent ensenya la descripció que envia el servidor, en la llengua de l'app. Test unitari de `TempestaAgent.listOf`.
- [ ] Un servidor vell sense el camp de descripció segueix funcionant (no surt res). Test.

## Out of scope

Canvis al servidor (178).

## Notes

- `android/app/src/main/java/xyz/solanes/gregal/Tempesta.kt`, `VentsChatChrome.kt`, `GregalApi.kt`. Strings a `Strings.kt`/`AppStrings.kt` a mà (no regen-strings.py). Tests amb ANDROID_HOME.
