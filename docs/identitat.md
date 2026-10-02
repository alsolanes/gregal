# Gregal Visual Identity: The Tide

This document is the source of truth for the shared palette. `internal/tui/theme.go`,
`internal/web/index.html` (`:root`) and `android/.../Screens.kt` derive their
values from it. Go and web tests compare the implementation with these tables.

The dark and light palettes use the same token names with different values.
The TUI uses the dark palette; the web and desktop interfaces provide both and
follow the system theme unless the user chooses one.

## Paleta (Palette)

The first column contains the token names used by the interfaces and, where
applicable, identifiers from `theme.go`. Hex values are uppercase; tests
compare them case-insensitively.

| name | hex | role |
|---|---|---|
| `escuma` | `#9FE0C4` | **primary**: brand, actions, cursor |
| `escumaClara` | `#BCEBD4` | primary hover and animation highlight |
| `arena` | `#DEC899` | **accent**: warnings and CODE mode |
| `onada` | `#8FB9A5` | secondary: project, roles, INSPECT mode and diff hunks |
| `algua` | `#264237` | subdued wave lines, rails and borders |
| `fons` | `#171A19` | main background |
| `abisme` | `#101312` | deeper background: sidebar, panels and code blocks |
| `night` | `#1E2321` | surfaces: cards, inputs and menus |
| `superficieAlta` | `#29302D` | hovered or selected surface |
| `text` | `#ECF0EA` | primary text, including assistant responses |
| `ink` | `#F4F4EF` | strong text: headings and brand name |
| `slate` | `#AEB9B1` | muted text: metadata and descriptions |
| `faint` | `#87978B` | subdued text: keyboard hints and version |
| `green` | `#7DE2A5` | success, additions and completed tools |
| `red` | `#EF8D82` | errors, deletions and denied actions |
| `lila` | `#C7B6D9` | GOAL mode text |
| `lilaFons` | `#30283A` | GOAL mode badge background |

Web hairlines (`--line`, `--line-hi`) use `escuma` at 10% and 20% opacity:
`rgba(159,224,196,.10)` and `rgba(159,224,196,.20)`. Text contrast is not
uniformly WCAG AA: `faint` is intentionally subdued, and the automated
contrast test tracks minimum ratios for `--faint`, `--muted` and `--text`.

## Paleta clara (Light Palette)

The same palette in a light theme. Token roles remain the same. Accent colors
are darker because the dark-theme values do not provide enough contrast on
white backgrounds.

| name | hex | role |
|---|---|---|
| `escumaClar` | `#0E8C74` | **primary** on light backgrounds |
| `escumaClaraClar` | `#0A6F5C` | primary hover |
| `arenaClar` | `#9A6B12` | **accent**: warnings and CODE mode |
| `onadaClar` | `#0E7490` | secondary |
| `fonsClar` | `#F5F7F8` | main background |
| `abismeClar` | `#EBEFF1` | sidebar and panels |
| `nightClar` | `#FFFFFF` | surfaces: cards and inputs |
| `superficieAltaClar` | `#E2E9EC` | hovered surface |
| `textClar` | `#1E293B` | primary text |
| `inkClar` | `#0B1220` | strong text |
| `slateClar` | `#52616B` | muted text |
| `faintClar` | `#5F6E78` | subdued text (replaces `#7C8B95`, which measured 3.2:1 on `abismeClar`) |
| `alguaClar` | `#CBD5DC` | hairlines, rails and borders |
| `greenClar` | `#15803D` | success and additions |
| `redClar` | `#DC2626` | errors and deletions |
| `lilaClar` | `#6D28D9` | GOAL mode text |
| `lilaFonsClar` | `#EDE9FE` | GOAL mode background |

The light theme's hairlines use `escumaClar` at 14% and 28% opacity. Contrast
ratios are tracked by the automated test; the subdued `faint` token is not
intended for small text that must meet WCAG AA.

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
| paraula clau | `#C7B6D9` | `#6D28D9` | `lila` |
| cadena | `#9FE0C4` | `#0A6F5C` | `escuma` |
| número | `#DEC899` | `#9A6B12` | `arena` |
| comentari | `#87978B` | `#6B7A85` | entre `slate` i `faint` |
| funció | `#8FB9A5` | `#0E7490` | `onada` |
| tipus | `#BCEBD4` | `#0E8C74` | `escumaClara` |
| operador | `#AEB9B1` | `#52616B` | `slate` |
| nom | `#ECF0EA` | `#1E293B` | `text` |

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
(`#3C5750`) és la vora del composer en repòs i és `algua` aclarida; s'hi
tolera com a únic to derivat.

**Web i escriptori** (`index.html :root`):

| token | valor |
|---|---|
| `--bg` | `fons` |
| `--sidebar`, `--panel` | `abisme` |
| `--surface` | `night` |
| `--surface-hi` | `superficieAlta` |
| `--line` / `--line-hi` | `escuma` al 10% / 20% |
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
