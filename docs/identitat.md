# Gregal Visual Identity: The Northeast Wind

This document is the source of truth for the shared palette. `internal/tui/theme.go`,
`internal/web/index.html` (`:root`) and `android/.../Screens.kt` derive their
values from it. Go and web tests compare the implementation with these tables.

The dark and light palettes use the same token names with different values.
The TUI uses the dark palette; the web and desktop interfaces provide both and
follow the system theme unless the user chooses one.

## Paleta (Palette)

The first column contains the token names used by the interfaces and, where
applicable, identifiers from `theme.go`. Hex values are uppercase; tests
compare them case-insensitively. The names are historical (the palette used
to be sea foam and sand); the roles are what matter.

**Principles.** Near-neutral cool grays (a hint of blue, like the sky when the
northeast wind clears it) carry 90% of the screen. One confident accent, the
*Gregal blue*, appears only where there is something to do: primary buttons,
focus rings, the active navigation item and links. Semantic colors (success,
warning, danger, info) are reserved for state. The playful aqua (`onada`) is
the rare fun touch: mascot, celebrations, retro reactions, autonomous mode.
In the dark theme elevation is expressed with lighter surfaces
(`abisme` < `fons` < `night` < `superficieAlta`), not heavy shadows. Body
text is at least 4.5:1 (WCAG 2.2 AA) and APCA Lc ≥ 90; even `faint` passes
4.5:1 on every surface; borders that matter (`boxIdle`) reach 3:1.

| name | hex | role |
|---|---|---|
| `escuma` | `#8193FF` | **primary accent** (Gregal blue): buttons, focus, active nav, links |
| `escumaClara` | `#A3B0FF` | primary hover and animation highlight |
| `arena` | `#F5C451` | **warning**, CODE mode |
| `onada` | `#5CE1D6` | **playful secondary** (aqua): weathervane gusts, celebrations, autonomous mode, diff hunks |
| `algua` | `#2C3240` | subdued lines, rails and decorative borders |
| `fons` | `#13151B` | main background |
| `abisme` | `#0E1015` | deeper background: sidebar, panels and code blocks |
| `night` | `#1A1D25` | raised surfaces: cards, inputs and menus |
| `superficieAlta` | `#242833` | hovered or selected surface (highest elevation) |
| `text` | `#E4E7EF` | primary text, including assistant responses |
| `ink` | `#F3F5FA` | strong text: headings and brand name |
| `slate` | `#A3AABA` | muted text: metadata and descriptions |
| `faint` | `#8890A2` | subdued text: keyboard hints and version (still ≥4.5:1) |
| `green` | `#6EDB95` | success, additions and completed tools |
| `red` | `#FF8A85` | danger: errors, deletions and denied actions |
| `lila` | `#C4A8FF` | **info**: GOAL mode text |
| `lilaFons` | `#2A2245` | GOAL mode badge background |

Web hairlines (`--line`, `--line-hi`, `--codi-vora`) are a cool gray
(`#C9D1E6`) at 10% and 20% opacity, not the accent: the accent stays scarce.
On the accent, text is `fons` (dark on the light blue, 6.6:1).

## Paleta clara (Light Palette)

The same palette in a light theme. Token roles remain the same. Accents are
darker so they read on white and white text reads on them (all ≥4.5:1).

| name | hex | role |
|---|---|---|
| `escumaClar` | `#4F5BE8` | **primary accent** on light backgrounds |
| `escumaClaraClar` | `#3D47C7` | primary hover |
| `arenaClar` | `#9A5B00` | **warning**, CODE mode |
| `onadaClar` | `#0A776F` | playful secondary (deep aqua) |
| `fonsClar` | `#F6F7FA` | main background |
| `abismeClar` | `#EEF0F5` | sidebar and panels |
| `nightClar` | `#FFFFFF` | surfaces: cards and inputs |
| `superficieAltaClar` | `#E5E8F0` | hovered surface |
| `textClar` | `#1F2433` | primary text |
| `inkClar` | `#0E1220` | strong text |
| `slateClar` | `#545D71` | muted text |
| `faintClar` | `#5F6879` | subdued text (≥4.5:1 even on `superficieAltaClar`) |
| `alguaClar` | `#D5DAE4` | hairlines, rails and borders |
| `greenClar` | `#187A41` | success and additions |
| `redClar` | `#C92A35` | danger: errors and deletions |
| `lilaClar` | `#7444D6` | info: GOAL mode text |
| `lilaFonsClar` | `#EEE8FD` | GOAL mode background |

The light theme's hairlines use a deep navy (`#1E2A4A`) at 14% and 28%
opacity. On the accent, text is white or `fonsClar` (5.3:1 / 4.9:1).

## Temes inspirats

A més del fosc i el clar de casa, la finestra (web i escriptori) té tres
temes inspirats en altres eines. Són el mateix joc de tokens amb altres
valors: viuen a `internal/tema/tema.go` (`Gpt()`, `Claude()`, `Opencode()`),
el CSS es genera igual que els altres i el test d'identitat els exigeix els
mateixos tokens. El TUI continua sempre fosc.

| tema | fons | text | accent | aire |
|---|---|---|---|---|
| `gpt` (ChatGPT) | `#FFFFFF` blanc | `#1A1A1A` | `#10A37F` verd | clar neutre, grisos càlids |
| `claude` (Claude) | `#F0EEE6` closca | `#262522` | `#C15F3C` corall | crema càlid |
| `opencode` (OpenCode) | `#0A0A0C` zinc | `#E4E4E7` | `#E8833A` taronja | fosc neutre |
| `mistral` (Le Chat) | `#FFFBF4` blanc càlid | `#2A231A` | `#CE4E00` taronja | clar càlid |
| `gemini` (Gemini) | `#F8FAFF` blanc blavós | `#17294D` | `#1A73E8` blau | clar fred |
| `grok` (Grok) | `#000000` negre pur | `#EDEDEF` | `#F5F5F5` blanc | fosc mínim, monocrom |

Tot el text passa AA sobre el seu fons; els accents són només UI (botons,
vores), com al clar de casa.

## Colors del codi

Vuit categories i prou. Chroma en distingeix desenes, però amb més de vuit
colors un bloc de codi deixa de llegir-se. Els valors són els mateixos al
TUI i a la web: surten d'`internal/tema`, que és qui construeix l'estil de
chroma i el de glamour.

| categoria | fosc | clar | ve de |
|---|---|---|---|
| paraula clau | `#C4A8FF` | `#7444D6` | `lila` |
| cadena | `#6EDB95` | `#187A41` | `green` |
| número | `#F5C451` | `#9A5B00` | `arena` |
| comentari | `#7D869A` | `#5F6879` | entre `slate` i `faint` (clar: `faint`) |
| funció | `#A3B0FF` | `#3D47C7` | `escumaClara` |
| tipus | `#5CE1D6` | `#0A776F` | `onada` |
| operador | `#A3AABA` | `#545D71` | `slate` |
| nom | `#E4E7EF` | `#1F2433` | `text` |

## Insígnies de mode

Les quatre, sempre amb `Padding(0,1)` / `padding: 0 6px`, negreta, i el mateix
color a tots els fronts. El text de la insígnia és `fons` (fosc sobre clar)
llevat d'OBJECTIU.

| mode | etiqueta | fons | text |
|---|---|---|---|
| code | `CODE` | `arena` | `fons` |
| chat | `XAT` | `escuma` | `fons` |
| inspect | `CONSULTA` | `onada` | `fons` |
| goal | `OBJECTIU` | `lilaFons` | `lila` |

## Correspondència per front

**TUI** (`theme.go`): els noms de la taula són els identificadors. `boxIdle`
(`#626B80`, clar `#7E879B`) és la vora del composer en repòs: un gris entre
`algua` i `faint` que arriba a 3:1 sobre el fons; s'hi tolera com a únic to
derivat.

**Web i escriptori** (`index.html :root`):

| token | valor |
|---|---|
| `--bg` | `fons` |
| `--sidebar`, `--panel` | `abisme` |
| `--surface` | `night` |
| `--surface-hi` | `superficieAlta` |
| `--line` / `--line-hi` | gris fred (`Vora`) al 10% / 20% (clar: 14% / 28%) |
| `--ink` | `ink` |
| `--text` | `text` |
| `--muted` | `slate` |
| `--faint` | `faint` |
| `--accent` / `--accent-hi` | `escuma` / `escumaClara` |
| `--amber` | `arena` |
| `--green` / `--red` | `green` / `red` |

La marca (`.brand-mark`) és `≋` en `fons` sobre `escuma`, sense degradat: el
degradat blau-lila d'abans era d'una altra app.

**Android** (`Screens.kt`): `Sea`=`escuma`, `Sand`=`arena`, `DeepTop`=`fons`,
`DeepBottom`=`abisme`, `Ink`=`text`, `Muted`=`slate`. Els noms en anglès es
poden quedar; els valors no.

**VS Code**: l'extensió no té UI pròpia i hereta el tema de l'editor. Només
porta la marca a la icona i la descripció del `package.json`.

**Telegram**: text pla. La identitat és el to i el vocabulari de glifs.

## Vocabulari de glifs

Els mateixos a tots els fronts que pintin text monoespaiat.

| glif | vol dir |
|---|---|
| `≋` | Gregal (la marca, el logo, els títols d'ajuda) |
| `↗ ⇗` | Gregal: rumb 45° Nord-Est (el vent a la rosa dels vents) |
| `彡 ༄` | ràfega de vent / corrent aerodinàmic |
| `(≋ ↗)` | mascota del Gregal (el vent que guia el mar) |
| `≋ ≈ ~ ·` | l'onada, de cim a calma; també els passos de la línia d'horitzó |
| `▣` | projecte / workspace |
| `■ □` | permisos: ple = cada eina delicada demana permís; buit = permissiu |
| `•` | un caràcter d'una clau d'API mentre s'escriu |
| `◆` | acció que canvia coses (CODE, confirma) |
| `◇` | acció que no en canvia (XAT, selector, pensament) |
| `◌` | esperant (aprovació pendent) |
| `┃` | rail d'eina o de bloc |
| `↑ ↓` | tokens amunt / avall |
| `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏` | spinner de feina (braille) |

Emoji només a Telegram i al mòbil, i pocs.

## Moviment: les tres animacions de la marea

Definides a `internal/tui/anim.go` i reproduïbles en CSS i Compose. **Regla
única: es mou només mentre hi ha feina.** En repòs tot està quiet; l'única
excepció és la mascota del logo del TUI, que respira a mig segon.

| nom | què és | on | quan |
|---|---|---|---|
| **l'onada que passa** (`tideLine`) | una sola cresta `·~≈≋≈~·` que travessa un buit d'esquerra a dreta, amb mar plana entre onada i onada | capçalera, entre el logo i la versió | torn en curs |
| **la cresta que llisca** (`shimmer`) | una banda de `escumaClara` de 3 columnes que recorre una etiqueta i fa una pausa abans de tornar | l'etiqueta "TREBALLANT" del composer | torn en curs |
| **el mesurador que batega** (`tideGauge`) | barra de context amb resolució de vuitens (`▏▎▍▌▋▊▉█`); per sobre del 80% l'última cel·la plena alterna `█`/`▓` | barra d'estat | sempre visible; batega només ≥80% |

Cadència: 60 ms per frame amb feina (el tick del TUI). En CSS, l'onada dura
~4 s per travessar 60 columnes i la cresta ~1,2 s per etiqueta amb 0,6 s de
pausa. El rellotge del torn (`elapsedShort`: `7s`, `2m04s`, `1h05m`) acompanya
la cresta: que es vegi quant fa que l'agent hi és treu l'angoixa de no saber si
s'ha penjat.

## Tipografia

- Terminal i codi: monoespaiada del sistema (`ui-monospace, SFMono-Regular,
  Menlo, Consolas, "Liberation Mono"`).
- UI de web, escriptori i mòbil: sans del sistema. Sense tipografies
  descarregades: no hi ha xarxa garantida i el TUI no en pot fer servir.
- Títols en negreta i sense majúscules forçades llevat de les insígnies i les
  etiquetes d'estat (`TREBALLANT`, `CONFIRMA L'ACCIÓ`).

## Línia d'horitzó

`waveLine(width)`: l'onada s'esmorteeix cap als extrems —cim `≋` al centre,
`≈`, `~`, `·` i calma a les vores— en comptes d'una filera massissa. Separa la
capçalera del contingut i encapçala la benvinguda (màxim 48 columnes).
