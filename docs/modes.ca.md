# Modes i permisos

Gregal mostra les mateixes poques opcions a la web i l'app d'escriptori, la
interfície de terminal (TUI) i l'app d'Android: tres modes i, per a Actua, un
permís. (Anglès: [modes.md](modes.md).)

## Modes

| Mode | Què fa | Id intern |
|---|---|---|
| **Pregunta** | Respon i llegeix el projecte (fitxers, ordres segures de lectura, la web). No canvia res. | `chat` |
| **Planifica** | Investiga sense canviar res i proposa un pla numerat. No toca res fins que l'aproves; executar el pla passa a Actua. | `plan` |
| **Actua** | Fa canvis, executa ordres i verifica, amb el permís de sota. | `code`, o `autonomous` amb Autonomia total |

On es trien:

- **Web / escriptori:** el commutador de mode del compositor (`···`). `Tab`
  amb el camp buit fa Pregunta → Planifica → Actua; `/mode` igual, `/pla`
  passa a Planifica.
- **TUI:** `Shift+Tab` fa el cicle; `/mode` obre el selector;
  `/mode pregunta|planifica|actua` (també `ask|plan|act`).
- **Android:** el commutador Pregunta | Planifica | Actua del compositor.

A Planifica, enviar un missatge demana un pla (la mateixa exploració de
només lectura que `/plan`); el botó **Executa** de la targeta (TUI: `e`)
passa a Actua i l'executa amb el permís d'Actua que tenies.

## Permís d'Actua

| Permís | Què passa |
|---|---|
| **Demana abans de canviar** | Cada edició, també dins del projecte, i cada ordre que canviï res pregunta abans. |
| **Aprova les edicions** (per defecte) | La feina dins de la carpeta del projecte passa sola: les edicions, les ordres de desenvolupament (`npm install`, `pytest`, `make`, `cargo`, `git add`…), engegar una app del projecte, les notes de coneixement del projecte i un commit local. Les ordres perilloses (`git push`, instal·lacions globals, `docker`, aturar processos, esborrats grans), escriure fora de la carpeta, PRs i serveis externs (MCP) continuen preguntant. |
| **Autonomia total** | Treballa sol fins que acaba i no pregunta res (és el mode autònom: fites, comprovacions, sense l'eina `question`). Només es bloqueja el prohibit: el classificador de shell (`sudo`, baixades passades a un shell, formatar discos…) i les regles `deny`. |

El mateix permís val per a Codi i per als Vents: la mateixa acció rep la mateixa
resposta als dos. Amb «Demana abans de canviar» i amb «Aprova les edicions», les
portes dures pregunten sempre, encara que el config permeti l'ordre: instal·lar
programari, baixar paquets o binaris, esborrar fora del projecte i tocar serveis
del sistema; i una web es fa pública per primer cop només amb el teu sí (a Codi
l'eina `publish` la deixa privada fins que ho confirmes). El que la política
denega continua denegat. Els vents mantenen els seus límits propis: a la
rotllana només pot canviar coses el primer que parla (els altres, i una petició
de revisió, són de només lectura), els xats compartits només cerquen a la web, i
un estudi només escriu a la seva carpeta de pràctiques.

On es tria: el botó del cadenat al costat del compositor (web, escriptori),
`/permissions` o `/permissions ask|edits|full` (TUI), **+ → Permís d'Actua**
(Android). A Pregunta i Planifica no es canvia res, així que el permís només
compta a Actua; triar-ne un hi passa. Sortir d'Actua i tornar-hi el conserva.

Queda desat al config: `mode` (`autonomous` per a l'Autonomia total) i
`permissions.access` (`ask`, o buit / `edits` per defecte).

## Avançat, fora del selector principal

- **Objectiu** (`/goal`, `/mode goal`): concreta un objectiu persistent amb
  preguntes. Es mostra com a Planifica amb l'etiqueta «Objectiu».
- **Depuració** (`/debug`, `/mode debug`): Actua amb un contracte de
  depuració. Es mostra com a Actua amb l'etiqueta «Depuració».
- **Consulta** (`/mode inspect`): Pregunta amb eines de lectura i sense
  preguntes, per a subagents i integracions. Es mostra com a Pregunta.
- **Revisor de permisos** (Preferències a la web i l'Android, `/autoreview` al
  TUI, `permissions.mode: auto-review` al config): un model ràpid decideix el
  que demanaria permís. No compta amb l'Autonomia total, que no pregunta mai.
- **Permisos per eina**: al fitxer de config (`permissions.tools`,
  `permissions.rules`, `bash_allow`, `bash_deny`, `permissions.subagents`). El
  TUI també té `/permissions tools`.

## Noms antics

Els configs antics, les sessions desades i els clients de l'API continuen
funcionant: `code` és Actua, `autonomous` és Actua amb Autonomia total, `chat`
i `inspect` són Pregunta, `goal` és Planifica (Objectiu), «Primer el pla» és
ara el mode Planifica, els permisos Manual i Accepta edicions són Aprova les
edicions, Auto (`/permissiu on`) és Autonomia total i el Revisor automàtic
passa a Preferències. `/api/mode` accepta també `ask`, `plan` i `act`.
