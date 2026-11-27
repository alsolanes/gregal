---
title: "Android: «A fons» amb models triats i explicació de cada agent"
status: done
depends_on: [178]
---

## Why

Paritat amb la 178: a Android també s'han de poder triar els models d'«A fons» i veure què fa cada agent.

## Acceptance criteria

- [x] A Preferències de Vents, una secció «A fons» amb un selector de model per a cada agent (Directe, Fonts, Advocat del diable) i la síntesi, amb «Automàtic», que llegeix i desa amb el mateix endpoint de la 178. Test unitari (parse i payload).
- [x] A la vista de la Tempesta del xat (`Tempesta.kt`), cada agent ensenya la descripció que envia el servidor, en la llengua de l'app. Test unitari de `TempestaAgent.listOf`.
- [x] Un servidor vell sense el camp de descripció segueix funcionant (no surt res). Test.

## Out of scope

Canvis al servidor (178).

## Notes

- `android/app/src/main/java/xyz/solanes/gregal/Tempesta.kt`, `VentsChatChrome.kt`, `GregalApi.kt`. Strings a `Strings.kt`/`AppStrings.kt` a mà (no regen-strings.py). Tests amb ANDROID_HOME.
- La secció és a Preferències › Models (`PrefsScreen.kt`), com a la web, i surt també en mode Vents simple. Els noms i les descripcions dels agents del selector són a `AppStrings.kt` (`app.tempesta.*`) amb el mateix text que `tempesta-prefs.js`; les del tauler del xat vénen del servidor (`desc_ca`/`desc_en`).
- La llista de models és la de `/api/models?configured_only=1` (la que ja fa servir l'app), no la sencera com a la web.
- Si el GET falla (servidor anterior a la 178), la secció no surt i hi ha un avís; un POST rebutjat (400/502) avisa i deixa la tria d'abans.
- Follow-up: amb un servidor anterior a la 178, cada visita a Preferències › Models mostra l'avís de «no s'han pogut llegir»; potser seria millor callar el 404.
- Follow-up: `uv run scripts/check.py` no corre els tests d'Android quan el canvi només toca `android/`; només ho recorda.
