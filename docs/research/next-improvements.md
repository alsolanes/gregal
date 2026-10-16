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
| 5 | Preguntes més barates: un subagent d'exploració que torni un resum (com l'Explore de Claude Code) quan la pregunta toca molts fitxers, o lectures amb pressupost per pas | A, D | ●●●● | M | proposat |
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

- **Preguntes en projectes grans** (candidat 5): és el cost més gran que s'ha
  vist (fins a 440 k tokens d'entrada per una pregunta). Cal un disseny propi:
  quan s'activa, quin pressupost té el subagent, com es conserven les cites.
- **delegate sense que ningú el demani** (candidat 6): un bench de prompt amb
  tasques que tenen dues investigacions independents.
- Validar els canvis 1-3 amb un model local petit (Qwen3-Coder a Ollama), on
  els noms d'arguments d'altres arnesos són més freqüents.
