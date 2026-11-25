---
title: Vents web: Avui com un resum del dia, una targeta per vent
status: done
depends_on: ["125", "126", "127"]
---

## Why

Avui should feel like a daily brief, like Duet's, not a set of empty boxes.

## Acceptance criteria

- [x] Avui leads with one swipeable card per wind that did or proposes something today (what it did, what it proposes, a button to act), reusing the comparison pager; winds with nothing are skipped. A node test with two winds, one idle.
- [x] Weather and match cards get small illustrated art (inline SVG in the wind's or theme colour, a few paths, no new image assets) instead of the empty smudge. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/134/`.

## Out of scope

- Android (135).
- New backend data: use what /api/winds/avui and novetats return; if a field is missing, note it.

## Notes

- Files: vents-avui-glance.js, the comparison pager module, winds-workspace.css.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
- Rule (same as Android 135, `AvuiBriefLogic.briefs`): `briefs(data, winds)` in vents-avui-glance.js. «Ha fet» = the wind's first card with source `feina:*` or `job` (open or done today); «Proposa» = its first other open card; the button is the first action of the proposal, else of the finished work, never of a `pausa` card (that one resumes with «Torna-hi» on its own card). Winds with neither are skipped. Order: tray order (now, fyi, ahead), then winds that only did work.
- Rendering: `renderBrief(box, opts)` (exposed on `window.gregalAvuiGlance`), called at the top of `paintAvui` before the glance. The button calls the new `runAvui(w, a)` in winds-workspace.js, which `avuiCard` now also uses (same code moved out, so `thread` actions open that conversation and the rest start a chat with the prompt). Only that and one line in `paintAvui` changed in winds-workspace.js.
- Pager: it reuses the comparison pager of md.js (`.gregal-compare .gx-carousel/.gx-track`, `data-gx-step`, `data-gx-dot`, the document-level listeners and CSS), in row mode (not `gx-paged`), so on desktop the next card peeks and the arrows move ~one card. The arrows are always rendered because md.js's `gxSync` reads `.gx-prev`/`.gx-next` on hover.
- Art: inline SVG, no assets. The weather wind (`kit` or `specialty` = `weather`, as Android) gets the glance's sky for today's code when `glance.weather` is there; a wind that a team follow points to (`follows[].wind`) gets `matchArt()` (pitch + bouncing ball, `var(--wind)` with the theme tint as fallback). In the glance, the team follow badge is now the pitch, the place and team invitations show a small sky / pitch, and the weather and team cards lost the radial «smudge». The ball bounce and the card entry only run under `prefers-reduced-motion: no-preference` (transform only).
- Tests: vents-avui-glance.test.cjs «el resum té una targeta per vent…» (two winds, one idle), «el resum: el botó salta la vigilància pausada…», «el temps i els partits porten el seu dibuix…».
- Screenshots (output/ is git-ignored): output/ui-review/134/{before,after}-{light,dark}-{1280,390}.png plus after-*-390-swiped.png, from a sandbox (%TEMP%/g134, port 18910) built from this branch; `/api/winds/avui` GET was answered with synthetic data by Playwright and the before shots served the three changed assets from b94a4db0.
- Data: nothing missing from /api/winds/avui; novetats were not needed. As on Android, «did vs proposes» is inferred from `source`.
- Follow-up: md.js's pager listener assumes every `.gx-carousel` has both step buttons (`car.querySelector(".gx-prev").disabled` throws otherwise); a null check there would make the pager safer to reuse.
