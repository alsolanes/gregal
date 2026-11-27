---
title: "Els fitxers grans s'escriuen per parts i el torn acaba la feina"
status: todo
depends_on: [184]
---

## Why

Gregal ha de fer la feina sempre. Un model que vol escriure un joc sencer (desenes de milers de tokens) en un sol `write` topa amb el límit de sortida del proveïdor, i fins ara el torn acabava sense fitxer després de 37 minuts. La 184 fa que aquest error es vegi; aquesta fa que no passi i que, si passa, el torn sen surti sol.

## Acceptance criteria

- [ ] `write` accepta `append: true`, que afegeix `content` al final del fitxer (creant-lo si no existeix). La descripció de `write` diu que un fitxer de més de ~300 línies s'escriu per parts: el primer tros amb `write` i la resta amb `append: true`, cada crida per sota de ~8.000 tokens. Test Go de l'eina i del text de la descripció.
- [ ] El missatge de tall de crida (`promptCridaTallada`) i l'error de la 184 diuen de fer servir `append: true` per continuar. Test.
- [ ] Test d'extrem a extrem amb un proveïdor fals que talla qualsevol resposta de més de N caràcters de sortida (com faria el límit real): se li demana un fitxer més gran que N i el torn acaba amb el fitxer sencer al disc, escrit en diverses crides, sense intervenció de l'usuari. Fa servir el motor real (`internal/agent` o el de la web), no un mock de l'eina.
- [ ] En autonomia total, quan el torn s'acaba amb la feina a mitges per un error que el model pot resoldre sol (com aquest), la resposta final no demana permís per tornar-ho a provar: el detector de crides repetides dona primer una instrucció de canvi d'estratègia concreta (per a `write`: escriure per parts amb `append`) abans de tancar. Test Go.

## Out of scope

Canviar el límit de passos o el model per defecte.

## Notes

- `internal/agent/loop.go` (definició i `case "write"`), `internal/llm/continua.go`, `internal/agent/motor.go` ~1077 (`app.senseAccionsNoves`), el detector de crides repetides (cerca «REPEATED TOOL CALL»).
