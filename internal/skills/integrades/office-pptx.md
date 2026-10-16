---
name: office-pptx
description: Professional PowerPoint decks: layouts, design rules, templates and visual QA. Load before creating or editing a .pptx/.potx.
---

## Workflow

1. **Plan the story first**: one message per slide. 6–12 slides for most decks.
   Title → agenda/context → sections → conclusion/next steps → closing.
2. **Create** with `office_create` and a `spec` (JSON). Prefer the spec over
   markdown: it gives every slide a real layout.
3. **Render** with `office_render`. Look at every thumbnail, then at full size
   (`pages="N"`) any slide that looks dense, empty or odd.
4. **Fix** with `office_edit` (fill, add_slide, delete_slide, move_slide,
   replace) and render again only the slides you changed.
5. Deliver only when the automatic QA is clean and the slides look right.

## Layouts (own theme)

- `title`, `closing`: dark background, title + subtitle.
- `section`: dark divider between parts.
- `content`: title + bullets/body, or + `table`/`chart`/`image`.
- `two_columns`: `left`/`right` (markdown); with a `chart`/`table`/`image` and
  no `right`, the visual goes on the right.
- `comparison`: `left_title`/`left` vs `right_title`/`right`.
- `big_number`: one key figure (`number`) + `caption`.
- `image`: `image` on the left, `body` on the right.
- `title_only`, `blank`.

Leave `theme` out unless the user asks for a look: without it the deck uses
the user's active branding (colours, fonts, logo on every slide) or the
default. Themes: `default` (navy/teal), `modern` (charcoal/coral), `nature`
(greens), `elegant` (burgundy/gold, serif titles), `mono`. Override colours
with `"theme": "default:primary=0B5394,accent=E69138"`.

## Design rules

- **One idea per slide.** At most 5–6 bullets, ~12 words each. If the QA says a
  shape overflows, split the slide; do not shrink the text.
- **Vary layouts.** Never three bullet slides in a row: use a big number, a
  comparison, a chart or a table to break the rhythm.
- **Every slide needs a visual anchor** (chart, table, image, big number,
  strong two-column contrast). Data → native `chart`, not a bullet list.
- **Charts**: pick the right type (column = compare categories, line = trend,
  pie/doughnut = parts of a whole with ≤5 slices). Give a title through the
  slide title, keep series ≤4, units in the category or series name.
- **Titles say the conclusion** ("Option B saves 1.2 M€ a year"), not the topic
  ("Costs").
- Left-aligned body text; never centre long text.
- Avoid: decorative lines under titles, walls of text, clip-art, more than two
  font sizes in a text box, leftover sample text ("Lorem", "Click to add").
- Put speaker detail in `notes`, not on the slide.

## Templates (.potx / corporate .pptx)

1. `office_read path=template.potx detail=layout` → lists layouts (name, type,
   placeholders) and the shapes of each sample slide.
2. Either `office_create` with `template` + `spec` (layouts are matched by kind:
   title, content, two_columns, comparison, section, image, title_only, blank;
   you may also give the exact layout name or number), or keep the sample
   slides (`keep_template_slides: true`) and edit them:
   `duplicate_slide`, `delete_slide`, `move_slide`, then `fill` with
   `shapes: {"title": "…", "body": "- point\n- point"}`.
3. **Template slots ≠ your items**: if the template has 4 boxes and you have 3
   items, delete the unused box's slide or pick another layout; never leave an
   empty placeholder.
4. Keep the template's fonts and colours: do not hard-code your own.

## Editing an existing deck

- Read it first (`office_read`, `detail=layout` for positions).
- `replace` keeps formatting; `fill` keeps the first run's font/size/colour.
- Always finish with `office_render`.
