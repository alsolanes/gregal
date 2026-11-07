---
name: office-xlsx
description: Correct Excel workbooks: formulas not values, zero errors, formats, roles, charts, QA. Load before creating or editing a .xlsx/.xltx.
---

## Golden rules

1. **Formulas, not hard-coded results.** Any number derived from other cells
   must be a formula (`"=B2*(1+$B$7)"`). Only inputs are typed values.
2. **Zero formula errors.** Check the formula report from `office_create` or
   the QA of the final `office_render` and fix every `#DIV/0!`, `#REF!`,
   `#VALUE!`, `#NAME?`, `#N/A`.
3. **Assumptions in their own labelled cells**, referenced with absolute
   references (`$B$7`), never typed inside formulas.
4. Use functions every Excel understands: SUM, AVERAGE, IF, IFERROR,
   SUMIFS, COUNTIFS, INDEX/MATCH, VLOOKUP, ROUND… Avoid XLOOKUP, FILTER, SORT,
   UNIQUE, SEQUENCE and LET (they break in older Excel and LibreOffice).
5. Guard divisions: `=IFERROR(B4/B2,0)` or `=IF(B2=0,0,B4/B2)`.

## Creating (`office_create` with `spec`)

```json
{"sheets":[{"name":"Model",
  "rows":[["Concepte","2025","2026"],["Vendes",1000,"=B2*(1+$B$6)"],["Costos",600,"=B3*1.05"],["Marge","=B2-B3","=C2-C3"],[null],["Creixement",0.1]],
  "number_formats":{"B2:C4":"#,##0 €","B6":"0.0%"},
  "roles":{"B2:B3":"input","B6":"key","A4:C4":"total"},
  "comments":{"B6":"Font: pla comercial 2026"},
  "charts":[{"type":"col","title":"Vendes","categories":"Model!$B$1:$C$1","series":[{"name":"Model!$A$2","values":"Model!$B$2:$C$2"}]}]}]}
```

- First row is styled as a header and frozen automatically; column widths
  fit the content. Override them only when needed with an array of widths in
  column order, for example `"widths":[24,14,14]`.
- **Colour roles** (financial convention): `input` blue text and light blue fill so blank inputs are visible, `formula`
  black, `link` green (from another sheet), `key` yellow fill (key
  assumption), `total` bold with top border, `note` grey italic.
- **Number formats**: currency `#,##0 €` / `$#,##0`, thousands `#,##0`,
  percent `0.0%` (store 0.1, not 10), negatives `#,##0;(#,##0)`, dates
  `dd/mm/yyyy`. Never leave long decimals unformatted.
- `table: true` for filterable data tables, `conditional` for data bars /
  colour scales / highlight rules, `validation` for drop-down lists.
- For clickable web sources, put the display text in `rows` and use
  `"hyperlinks":{"D2":"https://example.com/source"}` in the same sheet spec.
- Charts: `col`, `bar`, `line`, `pie`, `doughnut`, `area`, `scatter`. Without
  `series`, the first column is used as categories and the rest as series.
- Several sheets: inputs/assumptions, calculations, summary. Link with
  `'Sheet name'!B2`.

## Editing

- Read first (`office_read`): it shows formulas and their values.
- Group edits with `set_cells` / `set_range`. They keep existing styles; text starting
  with `=` becomes a formula, `12%` becomes 0.12 with a % format, `1.234,5`
  is read as a number.
- `format` applies number formats, roles, widths, conditional formats or
  charts to an existing sheet (spec = one sheet object).
- Match the workbook's existing conventions; do not restyle what you did not
  touch, and never overwrite formulas with values.

## Before delivering

Run `office_render qa_only=true` after the last batch of edits when a formula
check is still needed. Its QA includes formula
recalculation, so use `office_edit op=recalc` separately only if you need an
earlier formula check. Render visually when the layout matters and check for
`####` (column too narrow), readable headers, number formats and the right chart series.
