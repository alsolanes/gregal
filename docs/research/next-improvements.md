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

## 8. Tercera ronda: contrast amb dues revisions externes (v1.10.2)

Dues revisions externes comparaven Gregal amb OpenCode, DeepSeek Harness
(dsh), ZCode, Aider, Claude Code, Cline, OpenHands, Cursor i altres. La
primera es va escriure sobre la v1.9.4. Aquí cada punt es contrasta amb
`origin/main` (v1.10.2) i les PR obertes #38 (explorador) i #39 (sessions
del `-p`), amb proves al codi. Estat: ✓ fet, ~ parcial, ✗ no hi és.

### 8.1 OpenCode

| Punt | Estat | Proves | Què falta |
| --- | --- | --- | --- |
| Catàleg de models (models.dev) | ✗ → **✓ aquesta PR** | abans: `config/cataleg.go:41` (10 proveïdors, sense preus ni límits), `agent/window.go:210` (taula a mà), preus només de `cost:` | — |
| Login OAuth sense clau (Copilot, ChatGPT, GitLab Duo) | ✗ | l'OAuth és només de MCP (`internal/mcp/oauth.go`) | login de proveïdor amb device code |
| Gateway Zen/Go i `/connect` | ✗ | el `/connect` del TUI (`tui/app.go:3316`) connecta al servei local de Gregal | no aplica: és un producte d'opencode |
| LSP | ~ → **~+ aquesta PR** | diagnòstics per CLI (`agent/diag_exec.go:9-14`, `diag_lang.go`, `diag_pytypes.go`) | servidors persistents per als diagnòstics (opencode els té desactivats per defecte, vegeu 8.5) |
| Sessions: compartir, revert, fork, fills navegables | ~ | `web/share.go:116`, `/rewind` (`tui/app.go:3018`), targetes dels fills a la sala | fork d'una sessió, refer, obrir un subagent com a sessió |
| Plugins, hooks, eines pròpies, ordres `/` | ~ | hooks `HooksCfg` (`config.go:328`, pre_tool pot denegar), `internal/usercmd`, subagents d'usuari, skills | eines pròpies sense MCP; plugins dins del procés |
| Extensió d'IDE amb selecció i `@fitxer#L37-42` | ~ | `vscode-gregal/src/extension.ts:52` (selecció en invocar), `mentionRe` (`tui/app.go:786`, `web/serve.go:1750`) | selecció i pestanya en viu, rangs de línies a les mencions |
| MCP: OAuth (DCR), remot, `mcp auth/debug` | ~ (quasi ✓) | `mcp/oauth.go:384` (RFC 7591), `transport_http.go:124`, `/mcp connect` al TUI | subordres `gregal mcp list/auth/debug` |
| Permisos amb glob i per agent | ~ | `PermissionsCfg` (`config.go:359`): allow/ask/deny per eina, `bash_allow` per prefix, `bash_deny` per subcadena | globs (`git *: ask`), permisos per subagent |
| Temes, tecles, paleta Ctrl+P, política d'empresa, SDK/OpenAPI | ~ | temes (`internal/tema`), Ctrl+K a la web, `docs/api-contract.md` + `contract_test.go` | tecles configurables, paleta al TUI, `.well-known`, OpenAPI, SDK TS |

### 8.2 DeepSeek Harness

| Punt | Estat | Proves | Què falta |
| --- | --- | --- | --- |
| Microkernel de plugins amb recàrrega en calent | ✗ | punts d'extensió solts: `RegisterExtra` (`agent/loop.go:259`), MCP, skills | arquitectura de plugins |
| Presets (minimal/standard/PTC/creator) | ~ | modes i prompt compacte (`config.go:78`, `llm/perfils.go:108`) | presets amb nom; PTC (un programa que crida eines en una sola volta) |
| Agent Teams: plantilla persistent, tauler, bústia, subagents continuables | ~ | `team/team.go:57`, `team/flow.go:255`, `delegate.go` | plantilla persistent, tauler compartit, missatges entre agents, continuar un subagent |
| Mètriques de trajectòria (TTFT, caché, tokens per torn) | ~ | `RunResult` (`agent/run.go:44`), `evals/stats.go`, `/stats` | TTFT; taxa de caché al resultat en viu |
| FS versionat (llegir abans d'escriure, `FS_STALE_VERSION`) | ~ → **✓ aquesta PR** | `tools/seguiment.go` (guarda per a les edicions aproximades) | abans `write` no comprovava res; ara sí |
| Compactació (llindar, cua literal, poda cap+cua) | ~ | `compact.go:20` (0,75), `histcompact.go` (emmascarament per blocs), `retallaCapICua` (bash) | cua per percentatge de tokens; resum que reaprofiti la caché |
| PTY persistents | ✗ | `internal/procs` (canonades); `bash_output wait` a #39 | escriure a stdin d'un procés, ConPTY |
| Sandbox | ✓ | `sandbox/backends.go` (bwrap, Landlock, Seatbelt, Docker) | — |
| E2B / sandbox remot | ✗ | — | backend remot |
| Feines programades | ✓ | `internal/jobs` (daily, interval, manual) | expressions cron |
| Webhooks | ~ | només de sortida (`web/retro.go:988`) | un webhook d'entrada que llanci un torn |
| Subagents externs (ACP, Codex, Claude Code) | ✗ | — | adaptadors |
| SDK | ~ | `client/` (Go), `harness/` (Go), `python/`, `gregal mcp serve`, `--output stream-json` (#39) | SDK TS, protocol JSON-RPC/ACP |
| `lsp_replace_symbol` | ✗ | `symbols` és de lectura | edició per símbol (el `rename` d'aquesta PR cobreix el cas més comú) |
| Edicions especulatives en streaming | ✗ | cascada aproximada a `tools/fuzzy.go` | aplicar els canvis mentre arriba el JSON |
| Cerca semàntica | ~ | `internal/knowledge` (BM25 + embeddings opcionals) | índex automàtic del codi |

### 8.3 ZCode

| Punt | Estat | Proves | Què falta |
| --- | --- | --- | --- |
| Goal amb verificador independent | ~ | autònom: `RunAutonomousChecks` (`autonomous.go:35`), revisor (`verify/verifier.go:129`), `--verify` (`verify_gate.go`) | que el mode goal faci servir aquest bucle fins que el verificador ho confirmi |
| Entorn tipus IDE (timeline, Cmd+K, terminal, DevTools) | ~ | converses per projecte i data, replay amb timeline, menú `/`, panell de processos | paleta difusa, terminal interactiu |
| Navegador amb selector d'elements → @context | ~ | eina `browser` (`tools/browser.go:849`), panell d'escriptori per CDP | selector d'elements |
| Vista prèvia (PDF, HTML, Mermaid, imatges, CSV) | ~ | `preview.js` (html, markdown, url), panell d'Office | PDF i imatges del workspace, Mermaid, CSV |
| Continuïtat multidispositiu | ~ | Android, Telegram | workspaces remots SSH/Docker, altres bots |
| Xat lateral, fork, editar un missatge | ~ | `/btw` (TUI, web), reintent, `/rewind` | fork des d'un missatge, editar-lo i tornar-lo a llançar |
| Memòria, automatitzacions, nivells de raonament, context llarg | ~/✓ | `automem.go`, `internal/jobs`, `Role.Think`, compactació | selector ràpid del raonament, wiki |
| Importar MCP de Claude Code, Codex i OpenCode | ✗ | — | importador |
| Avís quan acaba un torn | ~ | web, escriptori, Android | campana/OSC 9 al TUI |
| Respondre en la llengua de l'usuari | ~ | `LanguageDirective` (`config.go:542`): ca o en | detecció automàtica |

### 8.4 Segona revisió (llista ampliada)

| Punt | Estat | Proves / notes |
| --- | --- | --- |
| LSP | **~+ aquesta PR** | vegeu 8.5 |
| Tree-sitter per al repomap | ✗ | Go amb `go/parser`; la resta amb regex (`repomap/extract_regex.go`). Tree-sitter vol CGO o WASM: trenca la compilació estàtica d'ara |
| PTY | ✗ | vegeu 8.2 |
| Plugins externs | ✗ | vegeu 8.2 |
| Computer use (pantalla) | ✗ | només el navegador (CDP) |
| SDK multillenguatge | ~ | Go i un Python mínim; no TS |
| Workspace remot o al núvol | ✗ | tot corre on corre el servidor; Telegram i Android en són el control remot |
| DAP, veu, notebooks, JetBrains, completat en línia | ✗ | — |
| Tasques en repòs, workflows dinàmics, plugins d'UI | ✗ | flows estàtics (`team/flow.go`) |
| Perfils/bundles de config | ✗ | — |
| ACP | ✗ | — |
| Mercat MCP | ✗ | MCP sí, sense catàleg |

### 8.5 L'LSP, amb dades

Totes dues revisions el posen primer. Abans de fer-lo s'han mirat les fonts
d'opencode (`anomalyco/opencode`, branca `dev`, 2026-10-17):

- `packages/web/src/content/docs/lsp.mdx`: l'LSP està **desactivat per
  defecte** (cal `"lsp": true`). La documentació diu que els servidors es
  desincronitzen, gasten memòria i alenteixen l'agent, i que sovint és millor
  que l'agent corri lint o typecheck per línia d'ordres. És el que Gregal ja
  fa després de cada edició.
- Entre les eines del nucli (`packages/core/src/tool/plugin/`) no hi ha cap
  eina `lsp`.

On Gregal sí que tenia un forat és la **navegació semàntica**:
- quina de diverses funcions amb el mateix nom es crida;
- on es fa servir de veritat;
- un rename que no toqui text homònim.

Això és el que s'ha fet (8.6). Els diagnòstics continuen sortint de les
comprovacions per línia d'ordres, que no necessiten cap servidor engegat.

Prova amb gopls real (instal·lat a una carpeta temporal) sobre aquest
repositori: reanomenar el mètode `Seguiment.Edit` a `EditBlock`. El nom
`Edit` també és una funció del paquet.

| | Abans (grep + edit) | Amb `lsp` (demanat al prompt) |
| --- | --- | --- |
| Resultat | correcte, build OK | correcte, build OK |
| Passos / crides | 13 / 15 | 10 / 13 |
| Crida del rename | — | 1 crida, 13 canvis en 4 fitxers, 25,8 s (càrrega inicial de gopls) |
| `references` després | — | 1,2 s |

gopls també va canviar «tools.Edit» dins d'un comentari, i el model ho va
corregir al pas següent. **Sense demanar-ho, deepseek-v4.1-flash no tria
l'eina**: en la mateixa tasca sense cap indicació va fer servir grep i edit
(11 passos, també correcte). L'eina hi és i funciona; aconseguir que el model
la triï sol queda pendent (8.7).

### 8.6 Què s'ha fet en aquesta ronda

1. **Catàleg de models** (`internal/catalog`).
   - Còpia retallada de `https://models.dev/api.json` (232 proveïdors) a
     `<data>/models-catalog.json`. Es refresca cada dia en segon pla; la
     primera vegada s'esperen com a molt 4 s.
   - El proveïdor es busca per l'adreça base: opencode zen
     (`opencode.ai/zen/go/v1`) és `opencode-go`. Si no en coincideix cap,
     el model es busca pel nom, amb la finestra més petita i sense preu.
   - **Finestres**: el catàleg va després de `/models` i abans de la taula
     escrita a mà. deepseek-v4.1-flash a zen passa de 128K a 1M.
   - **Preus**: els rols sense preu al config prenen el del catàleg. El `-p`
     ara mostra el cost (`~$0.0010` per una pregunta curta).
   - `models_catalog: off` el desactiva.
2. **Guarda de `write`** (`Seguiment.GuardaEscriptura`). Un fitxer existent
   s'ha d'haver llegit o escrit en aquesta conversa, i no pot haver canviat
   des d'aleshores. És la regla de Claude Code i el `FS_STALE_VERSION` de
   dsh. Els fitxers nous passen sempre.
3. **Eina `lsp`** (`internal/lsp`, `agent/lsp_tool.go`).
   - Operacions: definició, referències, hover, diagnòstics del fitxer i
     rename.
   - Servidors: els que hi hagi al `PATH` (gopls, pyright/basedpyright/pylsp,
     typescript-language-server, rust-analyzer, clangd) o els de
     `lsp.servers`. No es baixa res.
   - La posició és `path` + `line` + `symbol`, més fiable que una columna
     comptada pel model. Les columnes es converteixen a UTF-16.
   - El rename escriu a través del diari (desfer, `/rewind`), refusa
     fitxers de fora del workspace o canviats mentrestant, demana permís
     com `edit` i es denega en els modes de només lectura.
   - Tests amb un servidor LSP fals (el mateix binari de test) i proves a
     mà amb gopls real.

### 8.7 Full de ruta

| # | Proposta | Per què | Impacte | Esforç | Fitxers |
| --- | --- | --- | --- | --- | --- |
| 1 | Que el model triï `lsp` sol: una línia a les regles quan l'eina hi és, i `references` a l'explorador de #38 | 8.5: l'eina funciona, però flash no la tria | ●●● | S | `agent/mode.go`, `agent/explore.go` |
| 2 | Goal → bucle fins que el verificador ho confirmi amb proves | ZCode; les peces ja hi són (autònom, `--verify`) | ●●●● | M | `internal/goal`, `agent/autonomous.go`, `verify/` |
| 3 | Continuar un subagent i obrir-lo com a sessió | dsh, opencode | ●●● | M | `agent/delegate.go`, web i TUI |
| 4 | PTY (ConPTY a Windows) per als processos interactius | dsh, opencode-pty | ●●● | M-L | `internal/procs` |
| 5 | Importar MCP de Claude Code, Codex i opencode | ZCode; poca feina i molta menys fricció | ●●● | S | `internal/mcp`, `/mcp import` |
| 6 | Permisos amb glob i per subagent | opencode | ●●● | S-M | `config` (permisos), `agent/loop.go` |
| 7 | `@fitxer#L37-42` i la selecció en viu de VS Code | opencode | ●● | S | `tui/app.go`, `web/serve.go`, `vscode-gregal` |
| 8 | Avís al TUI quan acaba un torn llarg, i `lang: auto` | comparació | ●● | S | `internal/tui`, `config` |
| 9 | Fork de sessió i editar un missatge anterior | ZCode, opencode | ●● | M | `internal/session`, web i TUI |
| 10 | Tree-sitter (WASM, sense CGO) per al repomap | Aider | ●● | L | `internal/repomap` |
| 11 | Login de proveïdors amb subscripció (device code) | opencode | ●● | M | `internal/config/auth.go` |
| 12 | SDK TypeScript i OpenAPI generat a partir del contracte | opencode, dsh | ●● | M | `docs/api-contract.md`, paquet nou |

No es proposen ara el microkernel de plugins, el computer use, el DAP ni els
workspaces al núvol. Són grans, i l'evidència d'aquestes rondes no diu que
siguin el que limita la qualitat avui.

### 8.8 Resultats

Banc complet amb els tres canvis (24 tirades): **23/24**. L'única fallada
és t9 (test ocult `"2.000"`), la mateixa de sempre. Cap edició fallida per
culpa dels canvis.

La primera passada va treure a la llum un fals positiu de la guarda de
`write`. A t3 la tasca anomena `utils.py`, i el mapa del projecte n'adjunta
el contingut sencer. El model el reescrivia directament, i la guarda el
rebutjava perquè no hi havia cap `read`: 2 passos de més en 2 de les 3
tirades (mediana de 3 a 5 passos). Ara un fitxer adjuntat sencer compta com
a vist mentre no canviï (`tools.VistSencer`). t3 amb 6 tirades: 6/6, mediana
de 3 passos, cap rebuig.

Prova real del catàleg: amb una carpeta de dades nova, el primer `-p` va
baixar models.dev (232 proveïdors, 1,2 MB retallat) i va mostrar el cost de
la tirada (`~$0.0010`), que abans no sortia. `/stats` diu
«catàleg models.dev» com a origen de la finestra.
