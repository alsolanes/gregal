---
title: "Tema fosc: negre de debò per a pantalles AMOLED"
status: done
depends_on: []
---

## Why

The user asked for the dark mode to be really dark so it suits AMOLED screens.
Today the dark theme is graphite (`#131518`), which lights every pixel. This
replaces the default dark palette rather than adding a new option: the request
was for "the dark mode" itself.

## Acceptance criteria

- [x] `tema.Fosc()` has a pure black background and sidebar (`Fons` and
  `Abisme` are `#000000`). `Night` and `SuperficieAlta` stay close to black but
  are lifted in order (contrast with black >1.05 and <1.4, strictly rising).
  `Text`, `Slate` and `Faint` reach 4.5:1 on every surface, and `BoxIdle`
  reaches 3:1 on `Night`. Checked by a test in `internal/tema`.
- [x] Web and desktop: the generated `:root` in `index.html` and the hard-coded
  dark block in `calma.css` (`--bg`, `--sidebar`, `--panel`, `--surface`,
  `--surface-hi`, `--codi-fons`, `--composer`) match `tema.Fosc()`. Checked by
  `TestCSSAlDia` and a new test in `internal/web`.
- [x] Desktop: the window's colours before the page loads (`desktop/startup.js`
  `FALLBACK.dark`) are black too, so there is no grey flash. Checked by
  `desktop/startup.test.js`.
- [x] Android: `DarkPal` uses the same hex values as `tema.Fosc()`
  (`PaletaTest`), and still passes the contrast checks in `TemesTest`.
- [x] Android: full-screen pages that painted `Superficie` (Winds and its
  chats, wind profile and creator, guide, preview), their top bars, tab rows,
  the chat composer dock and the status bar use the palette's `page`. That is the black background in dark
  themes, and the same surface as before in light themes and with Vidre
  (`PaletaTest.lesPaginesSonNegresAlFosc`).
- [x] `docs/identitat.md` lists the new values (`identitat_test.go`).

## Out of scope

- The other themes (Calma, light, the brand themes, Android's Nit, Vidre).
- A separate "black / AMOLED" option next to the graphite one.

## Notes

- Source of truth is `internal/tema/tema.go`; the TUI follows it as well
  (its dark golden screens were regenerated: only the `night` background and
  the text on the accent change).
- `page` applies to every dark Android palette, so the other dark themes
  (Nit, ...) also paint those pages with their own background, as the web does.
- Android's window and splash background (`themes.xml`) and the Vents web
  manifest colours are black too, so the app does not open on a grey flash.
- The Android screens are only checked by eye on the emulator: the test
  covers the `page` colour, not each screen using it.
- Follow-up: `desktop/loading.html` and `error.html` keep an old graphite
  default (`#14161B`) used only when `startup.js` passes no colours.
