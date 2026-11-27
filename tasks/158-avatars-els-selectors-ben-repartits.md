---
title: Avatars: els selectors ben repartits
status: done
depends_on: []
---

## Why

A review of how the avatars show (2026-11-26) found the pickers unevenly laid out: on Android a wind with its own colour left the ninth swatch alone on a third row, the editor's avatar cards were as wide as their names, Preferences cut names short («Compass …», «Rosa de rel…»); on the web «Lliure» sat alone on a second row in a wider card; and on both the compass dial and the map, which fill their whole square, looked much bigger than the figures.

## Acceptance criteria

- [x] Android editor: the palette plus a wind's own colour is two rows of five fixed columns; the short row lines up under the full one. `WindPickerLayoutTest.a wind with its own colour gets two even rows of swatches`
- [x] Android editor: every avatar card has one width and its whole name (up to two centred lines). `WindPickerLayoutTest.the editor's avatar choices are cards of one width with whole names`
- [x] Android Preferences: every pack name shows whole in Catalan, the longest; a dial's sample is drawn smaller than a figure's. `WindPickerLayoutTest.Preferences shows every pack name whole`
- [x] Web: Preferences › Avatars is a grid of four columns (two even rows of four), the editor's packs an even grid; dial and map samples are 52 px against 64. CSS only, checked in the browser (measured card sizes, screenshots); the Node tests cannot measure layout.

## Out of scope

The faces themselves, the chat and home layouts (reviewed on the emulator: centred, no box, no clipping), the web drawing a wind with a trade kit as SVG instead of clay (by design since 112).

## Notes

- The pickers' fixed 92 dp cards and two-line names apply to every option strip of the editor (shape, eyes, mouth, barretina), not only the avatars: the barretina names are long too.
- Reviewed on emulator-5554: home strip, chat header and empty state, chat corner wind, wind profile, drawer, editor. Android Preferences › Appearance was checked through the test, not on the emulator.
