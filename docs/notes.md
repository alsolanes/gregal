# Notes per camí

Context que arriba **tard i sense cap crida al model**: quan el model toca un
fitxer, el resultat de l'eina porta a sobre el que ja sabem d'aquell fitxer.

És la versió barata del que fan OpenHands amb les seves regles per ruta: en
comptes de fer un torn de model sencer per investigar abans de començar (el
que fa el scout), la nota apareix **just** quan el model ja ha demanat el
fitxer. Zero latència, zero tokens de més.

## Per què existeix

Mesurat en aquest repo: el scout s'enduava 45-57 s per torn, el 71-87% del
temps total, abans del primer pas del coordinador. I en un servidor d'una sola
ranura (llama.cpp local) fer-lo córrer en paral·lel **empitjora** el torn
(58 s → 2m18s), perquè el scout i el coordinador es disputen el mateix motlle.

Una nota enganxada al resultat no demana ranura: és text que ja tenim.

## On es declaren

| Fitxer | Què hi va | Es versiona |
|---|---|---|
| `.gregal/notes.yaml` | coses del projecte (arquitectura, convencions, paranys) | sí |
| `.gregal/notes.local.yaml` | coses d'aquesta màquina (ports, rutes, entorn) | no |

Els dos s'ajunten, en aquest ordre.

```yaml
notes:
  - match: "internal/web/*"
    note: "Les rutes es declaren a la llista de handle(), no a extras.go."
  - match: "*.kt"
    note: "L'app és bilingüe: tota cadena va a AppStrings.kt en els dos idiomes."
```

### Com encaixa el `match`

- **Amb barra** → es prova contra el camí relatiu al workspace, i també com a
  prefix de carpeta: `internal/web/*` cobreix `internal/web/x/y.go`.
- **Sense barra** → es prova contra el nom del fitxer: `*.kt`, `serve.go`.
- Els globs són els de Go (`path.Match`): `*` no travessa barres.

## Quan s'injecta

- Només eines que toquen **un fitxer concret**: `read`, `edit`, `patch`,
  `write`, `read_image`, `office_read`, `pdf_read`. `grep`, `glob`, `bash` o
  `delegate` no en porten.
- Només si l'eina ha anat bé i ha tornat text.
- **Un cop per sessió**: si el model rellegeix el mateix fitxer, la nota no es
  repeteix (si no, el resultat es tornaria sopa).
- Topalls: **3 notes** per injecció i **700 caràcters** el bloc sencer.

El bloc que veu el model:

```
[notes] internal/web/serve.go
- Les rutes es declaren a la llista de handle() del muntatge, no a extras.go.
- El codi va amb gofmt i els tests al costat del fitxer.
```

## On viu al codi

- `internal/agent/pathnotes.go` — càrrega, encaix, topalls i memòria de què ja
  s'ha injectat.
- `internal/agent/lifecycle.go` (`ExecCtx`) — el punt únic per on passen totes
  les eines (TUI, web, headless i Telegram), just abans dels hooks `post_tool`.
- `internal/agent/pathnotes_test.go` — encaixos, un-cop-per-sessió, topalls i
  el `.local.yaml`.

## Com es mesura

Cada injecció deixa una línia al registre:

```
notes: 2 injectades a internal/web/serve.go
```

Si no en surt cap quan el model llegeix un fitxer que té nota, el primer que
cal mirar és el `match` (barra sí/barra no) i que el workspace sigui
realment el directori del projecte.

## Límit honest

Les notes són **estàtiques**: les escriu una persona. No descobreixen res. Per
a «investiga com està muntat X» sense tenir-ho apuntat, el que cal continua
sent el scout — o una cerca del propi model dins del bucle. El que fa això és
treure el scout del camí quan la informació ja la sabíem.
