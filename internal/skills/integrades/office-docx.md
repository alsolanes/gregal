---
name: office-docx
description: Word documents with real styles, templates, tracked changes and comments, plus visual QA. Load before creating or editing a .docx/.dotx.
---

## Creating

- Use `office_create` with `content` in **markdown**. It becomes real Word
  structure: `#`–`######` → Heading 1–6 (navigable, TOC-ready), `-` and `1.`
  → real bullet/numbered lists (nested), `| tables |` → Word tables with a
  header row, `>` → Quote, code fences → Code style, `[text](url)` → links,
  `![caption](image.png)` → embedded image with caption, `[TOC]` → table of
  contents, `\pagebreak` → page break.
- `title` (+ `subtitle`) makes a cover block and the page header; page numbers
  are on by default. `page: letter` for US, `landscape: true` for wide tables.
- Pick a `theme` that suits the audience (default, modern, nature, elegant,
  mono).
- Do not use `#` for the document title if you passed `title` (it would be
  repeated). Start sections with `##` in that case.

## Writing quality

- Structure: short intro/summary first, then sections with clear headings,
  then conclusions/next steps. Prefer tables for comparisons and numbers.
- Paragraphs of 2–5 sentences. Lists for parallel items only.
- No placeholder text, no "TODO", no empty sections.

## Templates (.dotx or a corporate .docx)

- `office_create` with `template`: the template's styles, fonts, colours,
  headers, footers, margins and logo are kept; your markdown is mapped to the
  template's own heading/list/quote/table styles.
- Templates with `{{markers}}`: they are filled from `vars` ({"client":
  "ACME", "date": "16/10/2026"}); `{{content}}` marks where the markdown goes.
  The result tells you which markers had no value.
- Check the result with `office_read detail=layout` (shows which template style
  each role used) and `office_render`.

## Editing an existing document

- Read it first with `office_read` and copy anchors **exactly**.
- `replace` keeps each run's formatting and also works in headers, footers,
  footnotes and comments; `regex: true` for patterns.
- `insert` (anchor + position before/after) and `append` take markdown and use
  the document's own styles.
- Reviews and contracts: use `track: true` so every change is a tracked
  revision (author defaults to Gregal), and `comment` to explain why.
  `accept_changes` produces a clean copy.
- Never rewrite a whole document to change a few sentences.

## Before delivering

`office_render` and look at every page: headings orphaned at the bottom of a
page, tables wider than the page, images too big, empty pages, leftover
markers. Fix and render again.
