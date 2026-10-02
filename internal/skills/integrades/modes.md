---
nom: modes
descripcio: els cinc modes de treball (code, consulta, xat, objectiu, autònom), què permet cadascun i com es canvien
---

El mode diu què pots tocar. Es canvia amb Shift+Tab al TUI, amb
`/mode <nom>` o amb `--mode` a la línia d'ordres, i el d'arrencada surt
de `mode:` al config.

- **code** — accés total: llegir, crear, editar i executar, amb
  confirmació per a les eines delicades segons `permissions`.
- **inspect** (consulta) — explorar i explicar. Llegir sí; escriure no,
  i de shell només el que és clarament de lectura. Aquí no es demanen
  permisos: el que no és segur, no es fa i s'explica.
- **chat** (xat) — conversa amb lectura. Pots llegir el disc, però no
  modificar res. Si cal editar, digues que passin a mode code.
- **goal** (objectiu) — concretar una feina amb preguntes fins que
  quedi escrita, i executar-la quan ho diguin. L'objectiu es desa i es
  recupera amb `/goal`.
- **autonomous** (autònom) — treballar per fites sense preguntar a cada
  pas: es fan comprovacions cada `checkpoint_every` passos i un revisor
  hi passa cada `review_every`, amb un topall de temps
  (`max_minutes`). És el mode per a feines llargues.

Shift+Tab fa el cicle entre code, consulta, xat i objectiu. L'autònom
s'ha de demanar explícitament (`/mode autonomous`): és el que pot estar
hores treballant sol i val més que no s'hi entri per error.

## Quan et bloquegin una eina

En xat i consulta, intentar escriure no és un error teu: és el mode. No
insisteixis ni busquis la volta amb la shell. Digues què faries i que
cal passar a mode code.
