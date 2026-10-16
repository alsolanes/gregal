# Brandings

A branding is your own colours, fonts and logo. It applies to the whole app
(web, desktop, terminal) and to the documents Gregal generates: Word and
PowerPoint files, spreadsheets, HTML conversation exports and chart colours.

## Installing one

Open **Preferences → Brandings** and drop a `.gregal-brand.json` file on the
window, or click the drop area to choose it. Before anything is installed you
see a card with the colours, the fonts, the logo and a small preview of the app.
It also tells you whether the file is new, an update (`v1.0 → v1.2`), the same
version or an older one. Nothing changes until you accept. Leave **Use it now**
ticked to switch to it straight away.

An update is just a newer file with the same `id`: drop it the same way.

## Creating one

**Create a branding** opens an editor with these fields:

- name, version and author
- base: dark or light
- colour pickers
- font menus, with **Other…** to type any installed font
- a logo (PNG, JPEG, WebP or SVG, up to 512 KB)

The preview updates as you edit. **Save and use it** installs it. **Download
the file** saves it so you can share it, and **Export** does the same for any
installed branding.

## Choosing one

- **Web and desktop:** pick it in Preferences, or in the **Theme** menu, where
  installed brandings appear at the end.
- **Terminal:** use `/theme`, or `/theme <id>`.

The active branding is one per server: it is stored in the config as `brand:
<id>`, and with several users only the administrator can change it. Picking a
normal theme turns the branding off.

## The file

```json
{
  "format": "gregal-brand/1",
  "id": "acme",
  "name": "Acme",
  "version": "1.2",
  "author": "Design team",
  "mode": "dark",
  "colors": {
    "primary": "#14B8A6",
    "accent": "#F97362",
    "secondary": "#A3E635",
    "background": "#0F1E24",
    "surface": "#16282F",
    "text": "#E8F1F2",
    "muted": "#8FA5AA",
    "success": "#22C55E",
    "danger": "#EF4444",
    "chart": ["#14B8A6", "#F97362", "#A3E635", "#60A5FA"]
  },
  "fonts": { "body": "Georgia", "heading": "Trebuchet MS", "mono": "Consolas" },
  "logo": "data:image/png;base64,…"
}
```

Field rules:

- **`name` and `colors.primary`** are the only required fields.
- **`id`** defaults to a slug of the name.
- **`version`** is compared piece by piece, so `1.10` is newer than `1.9`.
- **`mode`** is `dark` (the default) or `light`.
- **Colours you leave out** come from the dark or light base theme. Hover
  states, borders and soft fills are derived from the colours you set.
- **Fonts** are names of fonts installed on the computer that shows them. They
  are not embedded.
- **`logo`** is a base64 data URL, up to 512 KB once decoded. SVG files with
  scripts, event handlers or external links are rejected. Documents only take
  PNG or JPEG.

Installed brandings live in `<data dir>/brands/<id>.json`. The data dir is
`GREGAL_DATA_DIR`, or `~/.local/share/gregal`.

## What it changes

| Where | What |
|---|---|
| Web and desktop | Every colour token, the body, heading and code fonts, the logo and name in the sidebar, the favicon and the desktop title bar |
| Terminal (TUI) | The palette and the name in the header |
| Word (`office_create`) | Heading colours, table header, accents, fonts, and the logo in the page header |
| PowerPoint | Colour and font scheme, and the logo bottom-left on every slide |
| Excel | Header, bands, totals and chart colours |
| HTML export | Accent colour, fonts, the logo above the title, and the branding name in the footer line |
| Chat charts and dashboards | Series colours (`colors.chart`, or the brand colours) |

The agent uses the active branding whenever it creates a document without
naming a theme. If you ask for a particular look, it can still use one of the
built-in themes (`default`, `modern`, `nature`, `elegant`, `mono`).
