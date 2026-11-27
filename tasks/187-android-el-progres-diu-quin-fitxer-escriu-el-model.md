---
title: "Android: el progrés diu quin fitxer escriu el model i què pensa"
status: done
depends_on: []
---

## Why

Paritat amb la 183: a la web, mentre el model treballa, ara surt «Escrivint index.html · 12k caràcters», «Preparant bash · 40 caràcters» o «Pensant · 67k caràcters» amb l'última frase del que pensa. Android pinta l'event `progress` (`progressLine` a `GregalViewModel.kt`) però no fa servir `tool_name`, `tool_target` ni `tail`.

## Acceptance criteria

- [x] Amb `tool` > 0 i `tool_target`, la línia diu «Escrivint <fitxer> · <mida> caràcters» / «Writing <file> · <size> characters»; sense fitxer, «Preparant <eina> · …», i sense nom d'eina, «Preparant una eina». Test unitari.
- [x] Quan només raona, la línia diu «Pensant · <mida> caràcters» i, a sota, la cua del raonament (`tail`). Test unitari.
- [x] Un servidor vell sense aquests camps segueix mostrant el que mostrava. Test unitari.

## Out of scope

El servidor i la web.

## Notes

- `docs/api-contract.md` (camps de `progress`), `internal/web/app/model-progress.js` per als textos exactes, `android/app/src/main/java/xyz/solanes/gregal/GregalViewModel.kt` (`progressLine`). Textos a `AppStrings.kt` a mà.
- Treball lleuger: només les classes de test que toquis (`./gradlew testDebugUnitTest --tests ...`) i `check.py --fast`; la suite sencera i la revisió es fan un sol cop per a tot el lot.
- La cua del raonament ja no va enganxada a «Pensant…»: `progressLine` torna un `Triple` (clau, text, cua) i la targeta de treball (`ChatWork.kt`) la posa a la línia de sota. S'ha substituït l'assert antic de `WorkTest.progressAndDurations` que esperava «Thinking… …plan the layout» en una sola línia: el requisit ha canviat.
- Un servidor vell sense `tool_name` ara diu «Preparant una eina · N caràcters» en lloc de «Preparant el pas següent…», com demana el primer criteri (i com fa la web); la resta de línies (esperant, escrivint, eines, revisió, compactació) no canvien.
- Follow-up: les altres línies de `progressLine` (Executant, Revisant, Esperant el model…) encara tenen el text en línia en/ca, no a `AppStrings.kt`.
