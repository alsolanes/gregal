---
title: "Android: el progrés diu quin fitxer escriu el model i què pensa"
status: todo
depends_on: []
---

## Why

Paritat amb la 183: a la web, mentre el model treballa, ara surt «Escrivint index.html · 12k caràcters», «Preparant bash · 40 caràcters» o «Pensant · 67k caràcters» amb l'última frase del que pensa. Android pinta l'event `progress` (`progressLine` a `GregalViewModel.kt`) però no fa servir `tool_name`, `tool_target` ni `tail`.

## Acceptance criteria

- [ ] Amb `tool` > 0 i `tool_target`, la línia diu «Escrivint <fitxer> · <mida> caràcters» / «Writing <file> · <size> characters»; sense fitxer, «Preparant <eina> · …», i sense nom d'eina, «Preparant una eina». Test unitari.
- [ ] Quan només raona, la línia diu «Pensant · <mida> caràcters» i, a sota, la cua del raonament (`tail`). Test unitari.
- [ ] Un servidor vell sense aquests camps segueix mostrant el que mostrava. Test unitari.

## Out of scope

El servidor i la web.

## Notes

- `docs/api-contract.md` (camps de `progress`), `internal/web/app/model-progress.js` per als textos exactes, `android/app/src/main/java/xyz/solanes/gregal/GregalViewModel.kt` (`progressLine`). Textos a `AppStrings.kt` a mà.
- Treball lleuger: només les classes de test que toquis (`./gradlew testDebugUnitTest --tests ...`) i `check.py --fast`; la suite sencera i la revisió es fan un sol cop per a tot el lot.
