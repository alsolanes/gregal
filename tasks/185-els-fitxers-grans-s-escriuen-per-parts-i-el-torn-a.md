---
title: "Els fitxers grans s'escriuen per parts i el torn acaba la feina"
status: done
depends_on: [184]
---

## Why

Gregal ha de fer la feina sempre. Un model que vol escriure un joc sencer (desenes de milers de tokens) en un sol `write` topa amb el límit de sortida del proveïdor, i fins ara el torn acabava sense fitxer després de 37 minuts. La 184 fa que aquest error es vegi; aquesta fa que no passi i que, si passa, el torn sen surti sol.

## Acceptance criteria

- [x] `write` accepta `append: true`, que afegeix `content` al final del fitxer (creant-lo si no existeix). La descripció de `write` diu que un fitxer de més de ~300 línies s'escriu per parts: el primer tros amb `write` i la resta amb `append: true`, cada crida per sota de ~8.000 tokens. Test Go de l'eina i del text de la descripció.
- [x] El missatge de tall de crida (`promptCridaTallada`) i l'error de la 184 diuen de fer servir `append: true` per continuar. Test.
- [x] Test d'extrem a extrem amb un proveïdor fals que talla qualsevol resposta de més de N caràcters de sortida (com faria el límit real): se li demana un fitxer més gran que N i el torn acaba amb el fitxer sencer al disc, escrit en diverses crides, sense intervenció de l'usuari. Fa servir el motor real (`internal/agent` o el de la web), no un mock de l'eina.
- [x] En autonomia total, quan el torn s'acaba amb la feina a mitges per un error que el model pot resoldre sol (com aquest), la resposta final no demana permís per tornar-ho a provar: el detector de crides repetides dona primer una instrucció de canvi d'estratègia concreta (per a `write`: escriure per parts amb `append`) abans de tancar. Test Go.

## Out of scope

Canviar el límit de passos o el model per defecte.

## Notes

- `internal/agent/loop.go` (definició i `case "write"`), `internal/llm/continua.go`, `internal/agent/motor.go` ~1077 (`app.senseAccionsNoves`), el detector de crides repetides (cerca «REPEATED TOOL CALL»).
- `write` amb `append: true` fa servir `tools.Append` (O_APPEND, crea el fitxer i la carpeta). Passa per la mateixa guarda de lectura que `write` (`GuardaEscriptura`): afegir a un fitxer existent que la sessió no ha llegit ni escrit falla igual, i després de cada tros `Coneix` torna a apuntar el fitxer, així que write + append + append… passa sol. El resultat continua començant per «escrit …» («(N bytes afegits al final)») perquè la web (`runs.js`) reconeix els write que han anat bé per aquest prefix.
- Tall que se'l menja el pensament (run 364): `LengthError.raonatSobretot` (pensament > text + arguments que han arribat) afegeix a `promptCridaTallada` que no redacti el fitxer pensant, que n'escrigui ja el primer tros amb `write` i continuï amb `append: true`.
- No s'ha baixat el raonament del reintent. El botó existeix (`llm.WithThink` → `reasoning_effort`), però caldria passar el context a la `do` de `continuaTallat`. Això canvia la línia de la signatura d'una funció que ja té complexitat ciclomàtica 25, i golangci-lint ho compta com a problema nou (gocyclo > 15).
- Criteri 4: la guia de crides repetides (`DoomGuide`) ara diu l'estratègia per a `write` (per parts, amb `append: true`). El test (`TestDoomGuiaDeWriteDiuEscriurePerParts`) comprova que, en mode autònom, la tercera crida repetida rep aquesta guia. El model la rep abans del tancament: el torn no es tanca fins al segon pas en què totes les crides són repetides. Que la resposta final del model no demani permís no es pot comprovar amb un model fals. El que es comprova és que se li dona l'estratègia concreta per resoldre-ho sol.
- El test d'extrem a extrem (`TestFitxerGranTallatPelLimitAcabaEscritPerParts`) fa servir `RunNonInteractiveExIn` (el motor `Torn`) en mode code sense aprovacions. El proveïdor fals té un límit fix de 1.500 caràcters de sortida (pensament + arguments). El model fals redacta el fitxer sencer (~3.500 caràcters) pensant fins que la conversa li diu `append: true`; llavors l'escriu en quatre trossos.
- La confirmació del TUI (`previewCanvi`) i el revisor automàtic (`vistaCrida`) ensenyen un append com el que s'afegeix, no com una reescriptura del fitxer.
- Follow-up: un tall que només porta pensament (sense text ni crida, finish length) no passa per `continuaTallat`: `escalaLength` el reintenta amb més pressupost i el mateix prompt, i si no cap, el torn acaba amb error. Si el proveïdor no tanca la crida amb el path com zen, aquest cas encara pot deixar la feina a mitges.
- Follow-up: baixar `reasoning_effort` al reintent d'un tall raonat. Primer cal partir `continuaTallat` per sota de gocyclo 15, i després passar el context a la `do`.
- Follow-up: l'error de `GuardaEscriptura` diu «write would rewrite it whole», que no és cert per a un append a un fitxer no llegit.
- Follow-up: el journal apunta l'append com a operació «write» (`SnapOp(path, "write")`). L'undo funciona perquè desa el contingut sencer d'abans, però l'etiqueta no distingeix entre reescriure i afegir.
- Follow-up: els resums d'eina de la web i d'Android (`tool-summary.js`, `Work.kt`) compten les línies de `content` d'un append com si fos un fitxer nou («+N»). És correcte com a línies afegides, però no diuen que és un append.
