# Millores següents: evidència, rànquing i resultats

Data: 2026-10-16. Codi de partida: `520ef9c` (abans de la v1.10.0), canvis
rebasats damunt de `Release 1.10.0`. Model: `deepseek-v4.1-flash` via
opencode zen, Windows 11, `GREGAL_DATA_DIR`, `GREGAL_AUTH` i `GREGAL_MEMORY`
aïllats, config de prova amb tots els rols al mateix model i `verify: off`.

## 1. Com s'ha buscat

- **Banc** `bench/` (8 tasques) amb `--eval bench --repeat 3`: 24 tirades
  amb el binari de base i 24 amb els canvis, amb la mateixa base de codi.
- **Tasques reals amb `gregal -p --output json`**, sobre còpies d'aquest
  repositori (un projecte Go gran) i de les fixtures:
  - A: pregunta en mode inspect («com detecta Gregal un bucle de crides
    repetides?»);
  - B: afegir el subordre `gregal version` amb un test;
  - C: tests nous per a `RecitaTodos`;
  - D: pregunta en català sobre els proveïdors;
  - E: aixecar el servidor de t4 en segon pla i provar-lo amb curl.
- **Lectura** dels logs de cada tirada (trace amb arguments i resum de cada
  eina), de `harness-techniques.md`, `speed.md` i les notes de les sessions
  germanes.

## 2. Què s'ha vist

| Observació | On | Cost |
| --- | --- | --- |
| `edit` cridat amb `{"old": …, "new": …}` (els noms del `patch`). L'eina llegia `old_string`/`new_string` buits i responia «old i new són iguals» dues vegades | banc t6.r2 | 3 passos d'11; 2 de les 2 edicions fallides de tot el banc |
| El classificador de shell denegava per **subcadena**: `git log --format '%h %s'` («format »), `grep -rn curl .`, `grep -rn sudo …`, `rg 'ssh '`, `clang-format` | prova directa de `Classify` | el model rep «patró bloquejat» sense alternativa |
| `curl` cap a `127.0.0.1` denegat sempre, en tots els modes (també a l'autònom) | banc t6.r1, tasca E | 1-2 passos per tasca de servidor; el model reescriu les peticions amb `python http.client` i ho explica a l'usuari com a limitació |
| `gofmt -l` llista tots els fitxers per culpa del CRLF del checkout de Windows, i el model s'hi entreté | tasca B (3 passos), banc t6.r2 (2), `speed.md` (t6, 2 de 4 tirades amb 11-14 crides en lloc de 5) | 2-3 passos |
| `git status`/`git diff` en una carpeta sense git (exit 129, «not a git repository») | banc t8.r2, t6.r1, tasca E | 1 pas |
| t8 no podia passar mai: l'assert corria `go test ./...` sencer i `sincronitza_test.go` depèn d'un servei extern que la tasca diu que no es toqui. Les tres tirades feien el correcte (implementar `Retira`, no tocar el test, dir que continua fallant) | banc t8 | el banc mesurava 0/3 per a una conducta bona |
| Preguntes sobre un projecte gran: 9-10 passos seqüencials de grep/read, 170-440 k tokens d'entrada (87-88 % de caché) | tasques A i D | 50-90 s per pregunta |
| «Respon breu» i una resposta de 30 línies amb taula | tasca D | qualitat |
| t9 falla per raonament (`'2.000'` com a milers), no per l'arnès | banc t9.r3 | — |
| Un homòglif ciríl·lic (`а`) dins d'un comentari, i tres passos per trobar-lo | banc t8.r1 (base) | 3 passos, rar |

La resta del backlog de `harness-techniques.md` ja és al codi: P6 (perfils i
mostreig per família), P7 (eines per mode), P8 (crides en text i reparació
d'arguments), P9/P10 (`internal/tools/seguiment.go`), P11, P12 (rol `small`
a compactació i scout), P13, P14, P16, P17 (pyright/mypy), P18, P19, P20 i
P22. Queden P15 (format d'edició segons la mida del model) i P21
(arquitecte/editor), que pesen sobretot en local.

## 3. Rànquing

Impacte: ●●●●● molt alt … ● baix. Esforç: S (≤2 dies), M (≤1 setmana), L.

| # | Candidat | Evidència | Impacte | Esforç | Estat |
| --- | --- | --- | --- | --- | --- |
| 1 | **Arguments d'eina tolerants**: àlies d'altres arnesos (`old`/`new`, `file_path`, `filePath`, `oldString`, `cmd`…), també dins de les edicions de `patch` i a les vies Anthropic/Responses; `replace_all` a `edit`; `old_string` buit diu que falta | t6.r2 | ●●●● | S | fet |
| 2 | **Llista dura per programa, no per subcadena**; curl/wget a loopback demanen permís | `Classify`, t6.r1, tasca E | ●●●● | S-M | fet |
| 3 | **Fets del projecte al mapa**: sense git; finals de línia CRLF | B, t6, t8.r2, `speed.md` | ●●● | S | fet |
| 4 | **Assert honest de t8** | banc t8 | ●●● (mesura) | S | fet |
| 5 | Preguntes més barates: un subagent d'exploració que torni un resum (com l'Explore de Claude Code) quan la pregunta toca molts fitxers, o lectures amb pressupost per pas | A, D | ●●●● | M | fet ([explore.md](explore.md)) |
| 6 | El coordinador no fa servir `delegate` per si sol amb flash (nota de la sessió d'agents en paral·lel) | `agents-paralel.md` | ●●● | M | proposat |
| 7 | Respectar «breu»/«curt» a la resposta final (regla de prompt + mesura de llargada) | D | ●● | S | pendent |
| 8 | Connectar els MCP del headless en segon pla; agrupar `go vet` per paquet | `speed.md` | ●● | S-M | pendent |
| 9 | Avís de caràcters d'un altre alfabet (homòglifs) a les línies editades | t8.r1 | ● | S | pendent |
| 10 | P15 format d'edició per mida de model; P21 arquitecte/editor | backlog | ●●●● local / ● remot | M-L | pendent |
| 11 | Regla «prova altres formats d'entrada» en arreglar parsers (t9) | t9.r3 | ● (incert) | S | descartat per ara: és el model |

Els quatre primers tenen evidència directa, es poden mesurar al banc i són
S. Per això s'han fet primer. El 5 té més impacte potencial en l'ús diari,
però demana dissenyar com es resumeix i com no es perd precisió en les
cites `fitxer:línia`. Queda com a proposta pròpia.

## 4. Què s'ha fet

### 4.1 Arguments d'eina tolerants

- `internal/llm/textcalls.go`: `renombraAlies` dins de `RepairCalls`.
  Reanomena una clau que l'esquema no coneix quan correspon, sense
  ambigüitat, a un paràmetre que falta. Compara les claus normalitzades
  (minúscules, sense `_` ni `-`), més una taula d'àlies. Entra a les llistes
  d'objectes (`patch.edits`). Una clau coneguda o un objectiu ja present no
  es toquen mai.
- `internal/llm/native.go`: `RepairCalls` també a Anthropic i Responses
  (abans només passava per chat completions i Ollama).
- `edit`: `replace_all` (totes les aparicions exactes, CRLF inclòs). Si no
  n'hi ha cap d'exacta, torna a la via d'un sol bloc amb la cascada
  aproximada.
- `old_string` buit → «old_string és buit: posa-hi el bloc… (per crear un
  fitxer, write)», en lloc de «són iguals».
- Tests: `internal/llm/alies_test.go`, `internal/tools/edit_totes_test.go`.

### 4.2 Llista dura per programa

- `internal/tools/ordres_denegades.go`: un lexer de shell (cometes,
  escapades, continuacions de línia, `$( )`, accents greus, `${…}`,
  operadors). Una regla dura compta quan el programa és el que s'executa:
  - la primera paraula de cada etapa;
  - després d'un prefix (`env`, `xargs`, `timeout`, `nohup`, `-exec`,
    `then`/`do`…);
  - el nom d'un camí (`/usr/bin/curl`, `curl.exe`).
- Torna a la subcadena de sempre on no es pot saber quin programa
  s'executarà:
  - codi dins d'una cadena: `bash -c`, `python -c`, `node -e`, `cmd /c`,
    `eval`, `source`, `alias`, heretext, `$'…'`;
  - una ordre que la shell encara ha de formar: `$X`, `${IFS}`, `{a,b}`,
    comodins.
- curl i wget **només contra loopback** (`localhost`, `*.localhost`,
  `127.x.x.x`, `[::1]`, `0.0.0.0`) deixen de ser denegats i demanen permís
  com qualsevol ordre de fora de la llista segura. Es respecten les
  variables assignades abans a la mateixa ordre (`B=http://127.0.0.1:8080;
  curl $B/x`). Continuen denegats si:
  - alguna adreça no és local, o hi ha `-x`/`--proxy`, `--resolve`,
    `--connect-to` o `-K`;
  - wget porta `-e` o `-i`;
  - la comanda menciona un proxy;
  - la sortida va a una shell o a un intèrpret sense script.
- Els patrons `deny` del config continuen sent subcadenes.
- Tests: `internal/tools/ordres_denegades_test.go`, amb 66 casos: falsos
  positius d'abans, denegacions que s'han de mantenir, intents de colar-hi un
  programa i curl local/remot. Manual (ca/en) actualitzat.

### 4.3 Fets del projecte al mapa

`internal/agent/mapa_projecte.go`, `fetsProjecte`:

- «Not a git repository…: git status, diff and log fail here, so do not
  run them.»
- Si la majoria de fitxers de text (en mira fins a 12) tenen CRLF: «gofmt
  -l, prettier --check… list them only because of the CRLF; that is not
  caused by your edits.»

El text forma part de l'empremta del mapa: és estable dins del projecte i no
trenca la caché de prefix. Test a `mapa_projecte_test.go`.

### 4.4 Assert de t8

`bench/tasks/t8.assert` restaura els tests i corre `go test -skip
TestSincronitzaMagatzem ./...`. Després comprova que la resposta (`answer`
del log) diu que aquest test continua fallant. Fallen la fixture sense
`Retira` i una resposta que diu «tot passa».

## 5. Resultats

Banc, 24 tirades abans i 24 després, mateix model i mateixa base de codi:

| | Abans | Després |
| --- | --- | --- |
| PASS | 20/24 | **24/24** |
| t8 amb l'assert nou (als logs d'abans s'hi ha tornat a passar) | 3/3 | 3/3 |
| Edicions fallides | 2/33 (6 %) | **0/30** |
| `git status`/`diff` en carpetes sense git | 2 | **0** |
| Ordres bloquejades | 1 | **0** |
| Passos per tasca (mitjana) | 5,1 | 5,0 |
| Segons per tasca (mitjana / mediana) | 15,9 / 13,0 | 14,7 / 12,5 |
| Tokens d'entrada per tasca (mitjana) | 42,9 k | 41,6 k |
| Caché de prompt | 91 % | 91 % |

El PASS sobre el mateix criteri (assert nou aplicat a totes dues passades)
és 23/24 → 24/24: la diferència que queda és t9, que va de 2/3 a 3/3 i és
soroll del model (test ocult, raonament). Amb 3 tirades per tasca, passos,
temps i tokens estan dins del soroll. El que sí canvia de manera clara són
els errors que es podien comptar: edicions fallides, ordres bloquejades i
crides a git sense repositori.

Tasca E (servidor en segon pla + curl), una tirada de cada:

| | Abans | Després |
| --- | --- | --- |
| Peticions de prova | `curl` denegat; el model les reescriu amb `python http.client` i diu a l'usuari que curl «està bloquejat» | `curl` contra `127.0.0.1` funciona; un bucle amb `B=…; curl $B/…` encara es denegava, i això s'ha corregit després (cas als tests) |

## 6. Què queda

- **Preguntes en projectes grans** (candidat 5): fet després, a
  [explore.md](explore.md) (explorador en mode inspect i grep que entén
  regex; de 294 k a 65 k tokens d'entrada per pregunta).
- **delegate sense que ningú el demani** (candidat 6): un bench de prompt amb
  tasques que tenen dues investigacions independents.
- Validar els canvis 1-3 amb un model local petit (Qwen3-Coder a Ollama), on
  els noms d'arguments d'altres arnesos són més freqüents.

## 7. Segona ronda (v1.10.2)

Codi de partida: `8834a64` (Release 1.10.2, amb #34 i #36). Mateix model i
mateix entorn aïllat. La PR #38 (explorador per a preguntes) encara no és a
main i no entra en aquestes mesures.

### 7.1 Evidència

- **Banc** (24 tirades, `--repeat 3`): 22/24. Les dues fallades són de t9
  (test ocult, `"2.000"` com a milers). Cap edició fallida ni cap ordre
  bloquejada.
- **t9 amb 8 tirades**: 5/8. El model arregla el cas del test visible i no
  prova cap altre format.
- **Tasca E** (aixecar el servidor de t4 en segon pla, esperar que respongui,
  provar-lo amb curl i aturar-lo), 9 tirades amb main:
  - **a totes 9** el primer bucle d'espera es denegava. És el que el model
    escriu sempre: `for i in $(seq 1 30); do code=$(curl … 127.0.0.1 …); if
    [ "$code" = 200 ] …`.
  - En 3 de les 9 el model va abandonar la tasca: no va provar res, va
    deixar el servidor engegat i va escriure «fes-ho en una màquina on curl
    estigui permès».
  - En una va reescriure les peticions amb Python.
  - La causa: després de `do`, el lexer de #34 tractava totes les paraules
    com a ordres, i `"$code"` feia tornar la comanda a la comprovació per
    subcadena.
- **Esperes**: el model espera processos en segon pla amb `sleep 90` o
  `sleep 120` en un pas i `bash_output` en un altre (tasca real de canviar el
  port per defecte: 4 passos d'espera de 33).
- **Tasques reals amb `-p`**: preguntes, resum de commits, canvi del port per
  defecte a 22 fitxers. Cada `-p` començava de zero:
  - no hi havia manera de dir «ara fes-ho» sense repetir l'exploració;
  - no es podia passar res per stdin (`git diff | gregal -p …`);
  - no es podia triar el model per a una tirada.

### 7.2 Comparació de funcions amb Claude Code, Codex CLI i opencode

| Funció | Claude Code | Codex CLI | opencode | Gregal (abans) |
| --- | --- | --- | --- | --- |
| Continuar una tirada headless | `-c`, `--resume` | `exec resume --last` | `run --continue` | ✗ (no es desava) |
| Entrada per stdin | sí | `-` | sí | ✗ |
| Model per tirada | `--model` | `-m` | `-m` | ✗ (només config) |
| Esdeveniments en streaming per a scripts/IDE | `--output-format stream-json` | `--json` | `--format json` | ✗ (un objecte al final) |
| Esperar un procés de fons | BashOutput + Monitor | `yield_time_ms` | — | ✗ (sleep + lectura) |
| Checkpoints / rebobinar | `/rewind` | — | `/undo` | ✓ `/rewind`, `/checkpoints` |
| Hooks | sí | `notify` | plugins | ✓ hooks de cicle de vida |
| Ordres pròpies (`/`) | sí | prompts | sí | ✓ `usercmd` |
| Mencions `@fitxer` | sí | sí | sí | ✓ |
| Sessions: llistar, reprendre, cercar | sí | sí | sí | ✓ TUI/web (`/resume`, `/find`) |
| Extensió d'IDE | VS Code, JetBrains | VS Code | — | ✓ VS Code |
| Avís quan acaba un torn | hooks / campana | `notify` | — | ~ només l'app d'escriptori |
| Resposta en la llengua de l'usuari | sí | sí | sí | ~ la fixa `lang` |

El TUI i la web ja cobreixen gairebé tot el que tenen els altres. On Gregal
quedava enrere és el mode `-p`, que és el que es fa servir en scripts, CI i
extensions.

### 7.3 Rànquing

| # | Candidat | Evidència | Impacte | Esforç | Estat |
| --- | --- | --- | --- | --- | --- |
| 1 | **Bucle d'espera del servidor denegat** (paraules de control al lexer) | tasca E: 9/9 denegats, 3/9 abandonades | ●●●●● | S | fet |
| 2 | **Sessions del `-p`**: desar, `-c`, `--resume`, `--no-session` | tasques reals | ●●●● | S-M | fet |
| 3 | **stdin i `--model`** al `-p` | comparació | ●●● | S | fet |
| 4 | **`--output stream-json`** | comparació | ●●● | S | fet |
| 5 | **`bash_output` amb `wait`/`until`** | tasca del port, tasca E | ●● | S | fet |
| 6 | Regla de verificació per a arreglos de parsers (provar altres formats) | t9 | ●● | S | provat, sense efecte: retirat |
| 7 | Avís al terminal quan acaba un torn llarg (campana / OSC 9) | comparació | ●● | S | pendent |
| 8 | Respondre en la llengua del missatge (`lang: auto`) | pregunta en català, resposta en anglès | ●● | S | pendent |
| 9 | `"$(curl …)"` dins de cometes dobles encara torna a la subcadena | tasca E (1 de 6) | ● | M | pendent |
| 10 | El coordinador no fa servir `delegate` per si sol | ronda 1 | ●●● | M | pendent |

### 7.4 Què s'ha fet

1. **Paraules de control al lexer** (`internal/tools/ordres_denegades.go`).
   `do`, `then`, `if`, `while`, `until`, `else`, `elif`, `!` i `time` només
   marquen que la paraula següent és una ordre; ja no converteixen la resta
   de l'etapa en possibles ordres. `[` i `[[` es tracten com a ordres. Tests
   amb el bucle exacte del model, `until curl …`, i casos que s'han de
   continuar denegant (`while true; do curl https://…`, `then sudo …`).
2. **Sessions del `-p`** (`cli_session.go`, `RunOpts.Prior`,
   `RunResult.Convo`). Cada tirada es desa a la llista de sessions, la
   mateixa del TUI i la web. `-c` continua la més recent de la carpeta (de
   `-p`, TUI o web) i `--resume NOM` una de concreta. El nom surt per stderr
   i com a `"session"` al JSON. `--no-session` no desa. Les rondes de
   `--verify` desen la conversa de l'última.
3. **stdin**: el que arriba per una canonada o per una redirecció s'afegeix a
   la tasca dins de `<stdin>…</stdin>`, fins a 256 KB (si és més llarg es
   talla i es diu). Amb `-p -`, la tasca sencera ve de stdin. Un terminal no
   es llegeix.
4. **`--model`**: `proveidor/model` (si el prefix és un proveïdor del config)
   o un model del proveïdor del rol. S'aplica a code, chat i think, que són
   els que el router pot triar.
5. **`--output stream-json`** (`cli_stream.go`): línies `step`, `tool_call`
   (arguments com a objecte), `tool_result` (sortida tallada a 4000
   caràcters), `warning`, `error`, `diff` i `note`. L'última és `result`, amb
   el mateix objecte que `--output json`.
6. **`bash_output wait/until`** (`procs.WaitFor`). Espera fins que el procés
   acaba, fins que una línia conté `until` o fins a `wait` segons (màxim
   300). Torna una línia que diu com ha acabat l'espera.

### 7.5 Resultats

Tasca E, mateix prompt:

| | main (9 tirades) | Després (6) |
| --- | --- | --- |
| Tirades amb alguna ordre denegada | 9/9 (15 bloquejos) | 1/6 (1) |
| Tasca abandonada (cap prova, servidor encara engegat) | 3/9 | **0/6** |
| Endpoints provats (curl, o Python en una) | 6/9 | 6/6, totes amb curl |
| Passos (mitjana) | 11,0 | 13,3 |

Els passos pugen perquè ara la feina es fa: les tirades d'abans que
s'acabaven en 6 passos eren les abandonades. L'únic bloqueig que queda és una
funció de shell amb `"$(curl …)"` dins de cometes dobles (candidat 9); el
model s'ho va saltar amb crides soltes.

Sessions, stdin i `--model` amb el model real:
- `git log -5 --format='%h %s' | gregal -p "Summarise these commits in 3
  short bullets."` va respondre en 1 pas.
- Una pregunta («on es defineix el port per defecte?») i després `gregal -c
  -p "canvia'l a 8098 a tot arreu"`. El segon torn va començar per la
  feina, sense tornar a buscar on era el port, i va acabar canviant 22
  fitxers.
- Tests: `cli_session_test.go` (stdin, `--model`, desar i continuar amb `-c`
  i `--resume` contra un servidor fals, stream-json) i
  `internal/procs/waitfor_test.go`.

`bash_output wait`: el model l'ha fet servir en 2 de les 6 primeres tirades
amb el canvi. En general continua esperant dins de `bash` amb un bucle
(`sleep 0.5` + curl), que no costa passos. L'estalvi es veu en els
`sleep 90`/`sleep 120` de les esperes llargues de tests.

Regla de verificació per a t9: amb la regla, 6/8; sense, 5/8. En cap de les
8 tirades el model no va provar cap entrada més, així que la diferència és
soroll i la regla s'ha retirat.

Banc complet (24 tirades abans i 24 després):

| | main | Després |
| --- | --- | --- |
| PASS | 22/24 | 24/24 |
| Passos per tasca | 4,54 | 4,67 |
| Segons per tasca (mitjana / mediana) | 19,7 / 17 | 16,7 / 14 |
| Tokens d'entrada per tasca | 38,1 k | 39,7 k |
| Edicions fallides | 0 | 0 |

Cap canvi d'aquesta ronda toca les tasques del banc. Les diferències (t9
1/3 → 3/3, temps) són soroll del model, i les dues passades es van fer amb
altres feines corrent a la mateixa màquina. El banc confirma que no hi ha
regressions; la millora es veu a la tasca E.
