---
title: Vents web: xat més professional, la resposta creix dins d'una sola bombolla i tipografia polida
status: done
depends_on: ["127"]
---

## Why

The wind chat does not feel professional. Found cause of the jumps: while streaming, `paintPending` (winds-workspace.js 414) removes and rebuilds the whole pending bubble on every token batch (animations restart, height jumps), and on `reply` `paintHistory` (1272) replaces every message, swapping the pending bubble for a differently shaped final one.

## Acceptance criteria

- [x] Streaming is stable: the pending reply is created once and updated in place (text appended / markdown re-rendered only inside its own body, throttled to animation frames), its face and dots never restart, nothing above it reflows, and when the reply arrives the same element becomes the final message (same layout, no swap, no full-thread repaint; paintHistory keeps existing message nodes it can reuse). A node test: during 50 token events the pending element is the same node, and after `reply` the final message is that node.
- [x] A calmer, more professional message style following the reference look in Notes: wind replies as flowing text with a small face and name at the start of a run (this replaces task 130's face-beside-bubble criterion), the person's messages as a subtle bubble, one reading width and text scale, consistent spacing for paragraphs, lists (fixes 127's list overlap if still there), code and tables; the wind's colour only on face, name and accents. Works in every theme (light and dark). Before/after screenshots, light and dark, 1280 and 390 px, in `output/ui-review/144/`, plus a short frame strip of streaming showing no jumps.
- [x] Scrolling stays pinned to the end while streaming when the person was at the end, and does not yank them down when they scrolled up. A node test.

## Out of scope

- Android (145).
- The wind's header and reactions (128).

## Notes

- Files: winds-workspace.js (`paintPending` 414, `schedulePending`, `paintHistory` 1272, streamChat 451), winds-workspace.css (`.vw-message`, `.vw-pending`), polish.css.
- Task 127 is changing winds-workspace.js; start after it is merged.
- Reference look (Claude, ChatGPT, Dots): the assistant's reply is plain flowing text on the page (no heavy bubble), with a small face and name at the start of a run; the person's message is a subtle rounded bubble aligned right; reading width about 68-72ch; one text scale (15-16 px body, line-height about 1.6, headings and lists with calm spacing); colour used sparingly (the wind's colour on the face, name and accents, not on the whole bubble); streaming text grows in place with no reflow above it.
- Person, 2026-11-25: «explora què podem fer perquè els xats es vegin una mica millor, més pro. No sé si és alguna cosa de fluïdesa, com per exemple quan genera el text que sembla que va recreant la bombolla del missatge i repintant-la perquè es veu com fa salts, o no sé si és el tema de les bombolles, el color, la tipografia... però no acaba de ser professional i no sé massa el perquè».
- Done (2026-11-25): `paintPending` keeps one row and updates it in place (body re-rendered only when its text changes, once per animation frame); the face is redrawn only when the speaker or mood changes. `paintHistory` reconciles: rows whose message is unchanged are kept (keyed on from/speaker/text/kind/id/ts…), the live row is adopted by the newest reply (`data-grown`, so the spring-in animation does not replay), and only new nodes are inserted. A turn that lands in the open chat calls `settleTurn()` (nav, thread, header mark, send button, notice, cheer) instead of `render()`; errors and stops still `render()` to restore the draft.
- While text streams, the status line, steps and tempesta box under it are hidden (`.vw-stream-text ~ *`): if they stayed below the text, removing them on landing made a thread pinned to the end drop by ~26 px.
- Style: wind replies are bubble-less text, max 70ch, 16/1.6, name tinted with the wind colour (40% into `--ink`), 24 px face beside the name at the start of a run in groups; the person's bubble is `--surface-hi`/`--ink`. Headings, lists, code, tables, quotes and hr share an em-based rhythm. Screenshots before/after (light, dark, 1280, 390) and a streaming frame strip with per-frame row tops (`*-strip-tops.txt`) are in `output/ui-review/144/` (gitignored, local only), from a sandbox with a fake provider that streams word by word.
- Follow-up: `windShare`/`bubbleShare` and `--vw-wind-share` (085) no longer tint anything; remove them with their tests in their own task.
- Follow-up: the `vw-reading` class no longer changes the look of a reply; it can go once nothing else reads it.
- Follow-up: the 390 px dark sandbox chat was routed to Ventolina instead of Gregal (router picked another wind); not related to this change.
