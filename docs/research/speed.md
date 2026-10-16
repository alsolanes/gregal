# Velocitat dels torns: on va el temps i què s'ha canviat

Data: 2026-10-16. Mesures amb `deepseek-v4.1-flash` via opencode zen, a
Windows 11, amb `GREGAL_DATA_DIR`, `GREGAL_AUTH` i `GREGAL_MEMORY` aïllats i
un config de prova (tots els rols al mateix model, `verify: manual`).

## Com s'ha mesurat

- **Headless (`gregal -p`)**: el banc `bench/` (8 tasques: Python, Go,
  HTML/CSS/JS, t9 amb test ocult) amb `--eval bench --repeat N`. Base: 32
  tirades (dues passades de `--repeat 2`) amb el codi d'`origin/main` més la
  instrumentació. Després: 32 tirades (`--repeat 4`) amb els canvis.
- **Instrumentació nova**: `RunResult.timings` (surt a `gregal -p --output
  json` i als logs de l'eval) desglossa el temps de paret en setup (sondeig de
  la finestra + mapa), model, eines, motor (inclosos revisor, crític i
  apostes), compactació, ampliació, síntesi i esperes de reintent. Cada crida
  del `trace` porta `ms`. La instrumentació no canvia el comportament.
- **Web**: servidor aïllat (`--serve --dir <còpia de bench/fixtures/t6 amb
  git>`), un client SSE que envia la tasca a `/api/agent` en mode permissiu i
  apunta quan acaba el scout, quan arriba la primera crida del coordinador i
  quan acaba el torn. El workspace es reinicia (`git reset --hard`, `git clean`)
  abans de cada tirada.
- **Micro-mesures** (test d'un sol ús, no inclòs): engegar `sh`, fotos git,
  diagnòstics post-edició, mapa del projecte, sondeig de `/models`, latència
  del model amb i sense streaming.

## On va el temps (abans)

Banc headless, 32 tirades, 661 s en total:

| Fase | Part del temps | Notes |
| --- | --- | --- |
| Model | 76 % | 2,4-3,3 s per crida de mitjana; 5,25 crides per tasca |
| Eines | 23 % | `go test` (compilar i enllaçar a Windows) és el 72 % de les eines |
| Setup | 0,6 % | sondeig de `/models` 0,6 s a la primera tirada de cada procés; mapa 60-100 ms |
| Motor, compactació, ampliació, síntesi | ~0 % | cap compactació ni ampliació al banc |

Latència del model (prompt de 5,7k tokens, 97 % servit de la caché, 43
tokens de sortida): **temps fins al primer token 1,2-1,8 s**, total 1,4-1,9 s.
L'streaming no és més ràpid en total (el headless no en fa servir i no cal).
La generació és curta; el que pesa és el cost fix de cada volta. Per tant, la
palanca és **fer menys voltes**, no fer-les més ràpides. La caché de prompt
ja va bé: 90-92 % de tokens d'entrada servits de la caché a totes les
tirades, amb el prefix estable de P2.

Per crida d'eina (trace amb `ms`, 16 tirades):

| Eina | Crides | Mitjana |
| --- | --- | --- |
| `bash` amb `go test`/`go vet` | 17 | 3,3 s |
| `patch` | 5 | 0,86 s (inclou `go vet` post-edició) |
| `edit` | 13 | 0,54 s (ídem; `.py`: ruff/pyflakes ~0,7 s) |
| `bash` amb `python` | 17 | 0,47 s |
| `read` | 47 | ~0 ms |

Web, tasca t6 (5 fitxers), 3 tirades amb el scout per defecte i 3 sense:

| | Torn sencer | Primera crida del coordinador | Scout |
| --- | --- | --- | --- |
| Amb scout (abans, `auto`) | 44-82 s (mediana 57 s) | 18,9-44,6 s | 16,9-42,9 s |
| Sense scout | 26-34 s (mediana 32 s) | 2,3-3,1 s | — |

Amb scout, el coordinador fa les mateixes crides d'eina (6-8) que sense: el
resum no li estalvia feina en un projecte que ja veu sencer al mapa.

## Què s'ha trobat

1. **Sortida de Python en cp1252 a Windows.** Quan la sortida va a una
   canonada, Python escriu amb la pàgina ANSI. «TOT BÉ» arribava al model com
   «TOT B�», i el model, que no se'n refiava, tornava a córrer els tests amb
   `; echo "exit=$?"` o amb `PYTHONIOENCODING=utf-8`. Era un pas sencer de
   més (una volta de model i una execució) en 9 de les 32 tirades base (tests tornats a córrer després de l'última edició).
2. **El scout de la web** és un subagent sencer (fins a 8 passos) que corre
   abans del primer pas en gairebé tota tasca de més de vuit paraules. En un
   projecte petit costava 17-43 s i el coordinador feia igualment les seves
   lectures.
3. **Foto git abans de cada `bash` que pot escriure** (`python …`, `sed -i`,
   generadors): ~9 processos de git, 0,69 s de mitjana (0,31 s quan no havia
   canviat res). Dos `rev-parse` (arrel i índex) es repetien a cada foto i el
   `for-each-ref` del prune també.
4. **Sondeig de `/models`** a cada procés: 0,44-0,71 s abans del primer pas de
   cada `gregal -p`. opencode zen (i OpenAI) no hi declaren la finestra, de
   manera que el model no quedava mai «conegut» i la web el tornava a
   sondejar cada deu minuts.
5. **Inici de torn en fila** (web, TUI, Telegram): foto git, mapa del
   projecte i sondeig, un darrere l'altre. Al TUI, al fil de la interfície. En
   mode xat la web calculava el mapa i el llençava.
6. **El verificador de la web** (`verify.mode` auto/both/strict; la plantilla
   d'`init` posa `both`) corria després de *tots* els torns, també dels que
   només responien una pregunta, i el torn no s'acabava fins que tornava. El
   TUI ja ho limitava als torns amb canvis.

Mirat i descartat (o deixat per més endavant):

- **Eines en paral·lel**: les de lectura ja corren juntes (`RunCalls`, fins a
  6); les que escriuen van en fila a posta. El model ja agrupa lectures (4
  `read` al primer pas a 6 de 8 tasques).
- **Streaming al headless**: no escurça el total (mesurat).
- **Caché de prompt**: 90-92 % d'encerts; el prefix no es trenca entre passos.
- **Compactació, ampliació, síntesi, reintents**: 0 al banc.
- **`go vet` després de cada edició d'un `.go`** (~1 s): dona al model els
  errors de tipus abans de córrer els tests, i treure'l podria costar
  qualitat. Es podria agrupar per paquet quan un mateix pas fa diverses
  edicions; al banc les edicions solen anar en passos separats.
- **Compilació de `go test` a Windows** (3,3 s per crida): és del projecte,
  no de l'arnès.
- **MCP al headless**: `mcp.Setup` espera tots els servidors abans del primer
  pas (fins a 30 s si un penja). Amb el servidor de demo són ~80 ms; amb
  servidors lents caldria connectar-los en segon pla, però les eines han
  d'estar a les specs del primer pas i canviar-les després trencaria la
  caché de prefix.
- **Rol petit per a tasques laterals**: ja existeix (P12 de
  `harness-techniques.md`); al banc no hi ha crides laterals.

## Què s'ha canviat

| Canvi | On | Efecte mesurat |
| --- | --- | --- |
| Python escriu UTF-8 a la canonada (`PYTHONIOENCODING=utf-8` a `bash` i `bash_background` a Windows; no `PYTHONUTF8`, que canviaria la codificació per defecte d'`open()`), i la sortida que no és UTF-8 es llegeix com a cp1252 | `internal/shell/env.go`, `internal/tools/tools.go`, `internal/procs/procs.go` | reexecucions dels tests després de l'última edició: 9 → 1 en 32 tirades |
| El scout no s'engega en projectes de menys de 40 fitxers (`agent.scout: always` el manté) | `internal/agent/scout_gate.go`, `internal/web/motor_web.go` | web t6: primera crida del coordinador de 18-45 s a 2-3 s |
| Foto git: arrel i índex recordats per directori; prune només quan hi ha més refs que `checkpoint_keep` | `internal/tools/gitcheckpoint.go` | 689 → 402 ms per foto; 310 → 179 ms sense canvis |
| Sondeig de `/models` dels remots recordat un dia a `<data>/windows.json` | `internal/agent/window_disk.go`, `window.go` | −0,4-0,7 s per `gregal -p`; sense sondejos repetits a la web |
| Foto git, mapa i sondeig alhora a l'inici del torn; sense mapa en mode xat | `internal/web/serve.go`, `agent.CheckpointIMapa` (TUI, Telegram) | −0,2-0,6 s per torn amb git |
| Verificador de la web només si el torn ha canviat alguna cosa (journal o diff diferent del d'abans) | `internal/web/serve.go` | −1 crida de model als torns de pregunta |
| `timings` i `ms` per crida al resultat del headless | `internal/agent/run.go` | (mesura) |

## Resultat

Banc headless, mateixes 8 tasques, 32 tirades abans i 32 després (codi
final):

| | Abans | Després | Canvi |
| --- | --- | --- | --- |
| Segons per tasca (mitjana) | 20,6 | 17,9 | −13 % |
| Segons per tasca (mediana) | 16,5 | 12,3 | −25 % |
| Segons per tasca sense t6 (mitjana) | 18,5 | 13,8 | −25 % |
| Passos per tasca sense t6 | 5,07 | 4,68 | −8 % |
| Temps de model per tasca sense t6 | 14,4 s | 10,6 s | −26 % |
| Temps d'eines per tasca sense t6 | 3,9 s | 3,1 s | −21 % |
| Reexecucions dels tests després de l'última edició | 9 | 1 | |
| PASS | 27/32 | 27/32 | igual |

Medianes per tasca (segons): t1 11,2 → 8,9; t3 10,8 → 7,7; t4 16,5 → 12,8;
t5 13,5 → 12,2; t7 20,4 → 16,0; t8 29,8 → 26,4; t9 14,8 → 11,8. **t6 33,0 →
46,6**: no té Python ni scout, i el que hi passa és del model. `gofmt -l`
assenyala els finals de línia CRLF del fixture i el model, en dues de les
quatre tirades, s'hi entreté (11 i 14 crides d'eina en lloc de 5). Ja passava
abans, amb menys freqüència: amb 4 tirades per tasca la variància del model
pesa més que qualsevol canvi de l'arnès en aquesta tasca. t8 falla 3 de 4
abans i després (és la tasca difícil del banc) i t9 (test ocult amb
separadors de milers) va entre 2/4 i 4/4 segons la passada, abans i després.

Una passada intermèdia amb `PYTHONUTF8=1` a més de `PYTHONIOENCODING` va
donar 20,6 → 16,0 s de mitjana. Es va treure `PYTHONUTF8` pel risc d'`open()`
descrit a la taula, no pel temps.

Tasques de Python amb el fixture dins d'un repo git (on hi ha fotos de pas),
t1, t3, t5, t7 i t9, 10 tirades abans i 10 després: 14,6 → 12,8 s de mitjana,
13,9 → 11,2 s de mediana, reexecucions 5 → 0. El temps de cada `python …`
no baixa (873 → 874 ms de mediana): cada tirada del banc és un procés nou en un
directori nou, i la primera foto sempre paga els `rev-parse` i el prune. La
millora de la foto es veu en sessions llargues (web, TUI), on el mateix
directori fa moltes fotos (689 → 402 ms a la micro-mesura). (Aquesta passada
tenia encara `PYTHONUTF8`; les dues fallades de t9 són el cas `'2.000'` del
test ocult, de raonament.)

Web, t6, 3 tirades després dels canvis (config per defecte, scout `auto`):

| | Torn sencer | Primera crida del coordinador |
| --- | --- | --- |
| Abans (scout) | 44-82 s, mediana 57 s | 18,9-44,6 s |
| Després | 27-57 s, mediana 42 s | 2,3-2,5 s |

El final del torn depèn de quanta feina faci el coordinador (9-11 crides
d'eina en aquestes tirades); el que l'arnès controla, el temps fins a la
primera acció, baixa de 19-45 s a 2,5 s.

## Què més es podria fer

- Connectar els servidors MCP del headless en segon pla amb un sostre curt i
  afegir-ne les eines al pas següent (cal decidir com no trencar la caché).
- Agrupar el `go vet` post-edició per paquet quan un pas fa diverses
  edicions al mateix paquet.
- Preparar la foto git de pas mentre el model pensa (després de cada eina
  que escriu) i fer servir aquesta si no ha passat res més: estalviaria els
  ~0,4 s que queden abans de cada `python …`.
- A la web, el verificador podria córrer després de `done` sense retenir el
  torn (com al TUI), però el veredicte va pel mateix stream SSE.
