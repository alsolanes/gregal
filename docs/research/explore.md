# Preguntes més barates en projectes grans: l'explorador

Data: 2026-10-17. Codi de partida: `d5aace0` (v1.10.1). Model:
`deepseek-v4.1-flash` via opencode zen, Windows 11. `GREGAL_DATA_DIR`,
`GREGAL_AUTH` i `GREGAL_MEMORY` aïllats en una carpeta temporal, amb una
còpia del config de [next-improvements.md](next-improvements.md): tots els
rols al mateix model, `verify: off`, `agent.max_steps: 40`. És el candidat 5
d'aquell informe.

## 1. Com s'ha mesurat

Cinc preguntes en mode inspect sobre un clon d'aquest repositori (~1.060
fitxers), amb `gregal -p "…" --mode inspect --output json` i
`GREGAL_PERFIL=1`:

1. Com detecta Gregal un bucle de crides repetides i què passa quan salta?
2. Quins proveïdors suporta Gregal i com es tria el protocol? (en català)
3. Com decideix el scout d'inici de torn si corre, i des d'on es crida?
4. Com cau l'eina edit a les alternatives quan `old_string` no hi és exacte?
5. Quines eines pot fer servir el model en mode inspect, i on s'aplica?

Totes demanen cites `fitxer:línia`. Dues tirades de cada pregunta amb el
binari de base i dues amb el final. Per a cada tirada: passos del
coordinador, crides al model (també les dels subagents), `up_tokens` (tot
el que s'ha enviat, caché inclosa), temps de paret i cites.

Les cites es comproven de dues maneres:

- **Dins de rang**: el fitxer existeix i té aquella línia.
- **Ancorades**: un identificador entre accents greus de la mateixa línia
  de la resposta apareix a ±3 línies de la línia citada. És una mesura
  aproximada (una cita a un rang llarg o a un comentari falla encara que
  sigui bona), però és igual per a les dues versions. A més, s'han revisat
  a mà les respostes de les preguntes 2 i 3.

## 2. Què s'ha vist abans de canviar res

| Pregunta | Passos | Tokens d'entrada | Temps |
| --- | --- | --- | --- |
| 1 | 14 / 13 | 261 k / 201 k | 61 s / 50 s |
| 2 | 15 / 13 | 336 k / 282 k | 52 s / 47 s |
| 3 | 11 / 13 | 183 k / 234 k | 38 s / 57 s |
| 4 | 7 / 9 | 86 k / 164 k | 26 s / 30 s |
| 5 | 24 / 20 | 733 k / 459 k | 91 s / 78 s |

D'on surt el cost:

- **Cada pas torna a enviar-ho tot.** El prefix fix (prompt, ~20 esquemes
  d'eina, mapa del projecte) són ~5-8 k tokens, i l'historial creix ~1,3 k
  per pas. Amb 14 passos, la pregunta 1 va enviar 261 k tokens per llegir-ne
  uns 18 k.
- **Greps buits per culpa de la regex.** `grep` cercava text literal, i el
  model hi escrivia regex: `Decide\(`, `func (p \*Policy) Decide|func.*Decide(`,
  `^func `, `ProviderLlamaCpp|Provider…`. 17 dels 68 greps de la primera
  passada (25 %) van tornar «sense coincidències» per això, i cadascun en
  demanava un altre al pas següent. Un cas més: `ModeCode =` no trobava
  `ModeCode    = "code"` perquè gofmt alinea els iguals.
- **Camins absoluts.** Cada línia de grep començava per
  `C:\Users\…\Temp\gregal-explore\repo\`: ~50 caràcters per coincidència
  que tornaven a viatjar a cada pas.
- **Línies minificades.** `internal/web/app/vendor/PptxViewJS.min.js` té una
  línia de 621 k caràcters. Una coincidència hi posava tota la línia al
  prompt (és el que explica una tirada de 336 k tokens amb només 11 crides).
- **`-p` no deia els temps.** `RunAmbVerificacio` sumava les rondes i hi
  perdia `timings` (sempre zero al JSON), `interrupted` i els comptadors del
  banc.

## 3. Què s'ha fet

### 3.1 L'explorador (`internal/agent/explore.go`)

En mode inspect, una pregunta que demana mirar el codi (les mateixes regles
que el scout: no salutacions ni confirmacions) en un projecte de 40 fitxers
o més comença amb un subagent explorador:

- **Només `read`, `grep`, `glob` i `symbols`**: un prefix més curt a cada pas
  que les ~20 eines del coordinador.
- **El mapa del projecte al prompt**, perquè el primer pas ja vagi a
  fitxers concrets.
- **8 passos com a màxim**, amb l'avís de tancament i la resposta parcial
  marcada que ja tenen els subagents.
- **Instruccions per treballar en pocs passos**: totes les crides que ja es
  poden pensar en un sol pas, lectures amb `offset`/`limit` al voltant dels
  greps, parar quan ja es pot respondre.
- **Un informe amb cites `camí:línia`** de línies que ha vist, i
  `UNVERIFIED` només per al que la pregunta necessita i no ha pogut
  comprovar.

El coordinador rep l'informe dins de la tasca («EXPLORER REPORT»). Se li
demana que respongui a partir de l'informe, que no refaci les cerques i que
no rellegeixi per comprovar les cites. Si falta alguna cosa imprescindible,
té un o dos passos de lectures concretes.

Amb només aquesta nota, dues de cada deu tirades se n'anaven 10-12 passos
darrere d'un detall marcat `UNVERIFIED` (a la pregunta 5, el lloc exacte
d'on es crida `Policy.Decide`). Per això, en començar el tercer pas, el
coordinador rep un sol cop «answer the question now…»
(`OpcionsTorn.RespondreAl`, `AvisRespondre`). Les tirades finals no han
passat de 3 passos.

On corre:

- `gregal -p`: a `RunNonInteractiveOpts`, abans del primer pas.
  `explorer`, `explorer_report` i `timings.explore_ms` surten al JSON. Els
  tokens de l'explorador compten en el total.
- Web i escriptori: el mateix camí que el scout (`ScoutWantedIn`), amb el
  seu carril a la sala i un missatge d'estat propi.
- `agent.scout: off` l'apaga. `agent.scout: always` el fa córrer també en
  projectes petits. Un subagent de només lectura definit per l'usuari amb el
  nom `explorer` el substitueix.
- La pregunta es parteix per àrees com la del scout (`ScoutFocuses`), amb
  un explorador per àrea en paral·lel.

Les lectures queden dins del fill: a la web, la pregunta següent de la
mateixa conversa no les torna a enviar.

### 3.2 `grep` que entén el que escriu el model (`internal/tools/tools.go`)

Una línia casa si conté el text literal o, quan el pattern té forma de regex
i compila (RE2), si casa amb la regex. També casa en aquests casos:

- el pattern compila prenent els parèntesis com a text (`\.Decide(`);
- la línia conté el text sense les barres d'escapar (`func \(c \*Client\)`);
- la línia conté el text amb qualsevol tira d'espais (`ModeCode =`).

Si el pattern sencer no compila i té `|`, cada alternativa es prova per
separat. Un literal que no hi és continua sense casar.

Altres canvis:

- Els camins de `grep` i `glob` són relatius al projecte i porten `/`.
- Una línia de més de 300 caràcters es retalla al tros del voltant de la
  coincidència, i s'hi indica la mida («[line of 80011 characters]»).

### 3.3 Altres

- `sumaRonda` conserva `timings`, `interrupted`, els comptadors del banc i
  els camps de l'explorador.
- Amb `GREGAL_PERFIL=1`, els subagents també pinten la mida del prompt de
  cada pas a stderr.

## 4. Resultats

Mitjana de 10 tirades per versió (5 preguntes × 2):

| | Base | Final |
| --- | --- | --- |
| Tokens d'entrada per pregunta | 294 k | **65 k** (−78 %) |
| Passos del coordinador | 13,9 | **1,7** |
| Crides al model (amb l'explorador) | 14,0 | **7,1** |
| Temps (mitjana / mediana) | 53,0 / 52,3 s | **29,3 / 26,7 s** |
| Greps buits per culpa de la regex | 17 de 68 | 0 |
| Cites ancorades | 238 / 266 (89 %) | **193 / 202 (96 %)** |

Per pregunta (mitjana de 2):

| Pregunta | Base | Final |
| --- | --- | --- |
| 1 bucle de crides | 231 k, 56 s | 80 k, 34 s |
| 2 proveïdors (català) | 309 k, 50 s | 57 k, 24 s |
| 3 gate del scout | 209 k, 47 s | 43 k, 28 s |
| 4 cascada d'edit | 125 k, 28 s | 44 k, 26 s |
| 5 eines en inspect | 596 k, 85 s | 101 k, 33 s |

L'explorador fa 5-8 crides, amb prompts de 2-13 k tokens (no de 5-45 k),
en 13-32 s. El coordinador hi afegeix 1-3 passos. Les preguntes que més
baixen són les més llargues (la 5, de 24 passos a 3).

Revisió a mà: les respostes finals de les preguntes 2 i 3 citen bé les
funcions i les línies (`ScoutWantedIn` a `scout_gate.go:22`, la crida a
`motor_web.go:127`, `nativeChat` a `native.go:128-143`…). Les cites que
l'àncora no troba són sobretot rangs llargs i línies de test, i n'hi ha
tantes o més a la base.

Versions intermèdies, per a la història:

- Explorador amb la nota inicial i sense el retall de línies: 99 k tokens i
  32,9 s de mitjana. Hi ha la tirada de 336 k de la línia minificada.
- Sense el recordatori de respondre: dues tirades de 10-12 passos del
  coordinador a la pregunta 5 (196-219 k).

## 5. Què queda

- **TUI i Telegram** encara no tenen l'explorador: el seu motor és per
  esdeveniments i no té el pas previ que tenen la web i el `-p`. És on més
  hi guanyaria una conversa llarga, perquè les lectures del coordinador
  queden a l'historial de les preguntes següents.
- Un rol `small` configurat ja s'aplica a l'explorador (P12). No s'ha
  mesurat amb un model més petit que el del coordinador.
- `grep` encara busca dins de `.gregal/out` (sortides de bash desades al
  projecte) i de `vendor/`. Ara el retall de línia limita el mal que fan,
  però no fa falta buscar-hi.
- Mesurar-ho també en preguntes sobre projectes que no siguin Go, i al
  banc de tasques de codi (el mode code no canvia, però `grep` sí).
