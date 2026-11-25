---
title: Vents web: permisos amb el color del vent i la cara a cada missatge
status: done
depends_on: ["125", "126", "128"]
---

## Why

Permission cards and messages do not carry the wind's identity.

## Acceptance criteria

- [x] Approval cards (`.vw-ask`) show the wind's face, an edge or tint in `--vw-wind`, a one-line reason, and the Allow button in the wind's colour with text contrast checked (as windShare does). A node test for the contrast pick.
- [x] Wind messages show a small face (20 px) beside the bubble, only on the first of a run; one bubble rule per speaker, not by content length. A node test. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/130/`.

## Out of scope

- Android (131).

## Notes

- Files: winds-workspace.js/.css (`.vw-ask`, `.vw-message`), wind-face helpers in winds.js.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
- Done (winds-workspace.js/.css; strings inline with `t(ca,en)`, no new strings: the reason comes from the server).
- Approval cards: every non-question card from a known wind gets `vw-ask-wind`, `data-wind` and `--vw-wind` (the asking wind's colour, also in the Work view where several winds ask), a border and 5% tint in that colour plus a 3 px inset edge. The permission card (`vw-ask-plan`) shows the wind's face instead of the shield (shield kept for an unknown wind) and `a.reason` on one line (`.vw-ask-why`, ellipsis, full text in `title`). The reason is no longer folded into `.vw-ask-sub` for `kind: rule`, so it is said once; the 102 test that read it from `.vw-ask-sub` now reads `.vw-ask-why` (requirement changed, not weakened).
- Allow in the wind's colour: `allowPaint(color, [--ink, --bg])` (exported as `gregalWindChat.allow`) picks the theme ink that keeps most of the wind's colour, mixing the fill toward the other ink only until the text reaches 4.5:1. Pastel winds keep their colour as is; Gregal's blue #3D7BFF is unchanged in dark and yields a little in light. A theme switch repaints `--vw-allow`/`--vw-allow-ink` on the open cards in place (no redraw, so a half-typed secret stays). If the theme inks are not hex, the button keeps the accent.
- Removed the dead 085 tint: `windShare`, `bubbleShare` and `--vw-wind-share` (they tinted nothing since 144); their tests went with them: the bubble-tint test in wind-chat-color.test.cjs became the Allow contrast test, and «switching between dark and light … retints its bubbles» in winds-start.test.cjs became the Allow repaint test. `mixHex`, `lum` and `contrast` are reused by `allowPaint`.
- Messages (144 had most of it): the group face is now 20 px (was 24) and is only added to the first row of a run (it used to be added to every row and hidden by CSS); the live row asks for the 20 px face too. `vw-reading` and `chatReadingSurface` (a content-length class that no longer changed the look) are gone, so a long reply has the same classes as a short one. Solo chats still show no face per message: the one face is the header/corner wind (108), as the existing test requires.
- Tests: wind-chat-color.test.cjs (Allow contrast, all winds, dark and light), winds-start.test.cjs (four tests after «130:»).
- Screenshots (not committed): `output/ui-review/130/` in this worktree, `before-`/`after-` × `ask-xaloc`, `ask-gregal`, `group` × light/dark × 1280/390; sandbox on port 18920 in %TEMP%/g130 with mocked winds, pending asks and a group history.
- Follow-up: `.vw-ask-plain` and `code.vw-ask-raw` CSS are still unused (noted in 102).
