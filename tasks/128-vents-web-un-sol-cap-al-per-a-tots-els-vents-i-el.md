---
title: Vents web: un sol capçal per a tots els vents i el vent reacciona amb el cos
status: done
depends_on: ["125", "126", "127"]
---

## Why

Gregal's header differs from the other winds', and the wind does not react to what happens: errors land as red text under the composer.

## Acceptance criteria

- [x] Every wind (Gregal included) has the same header: big face with its mood, name, a role chip in the wind's colour and one «⋯» menu (Perfil, Fils, Desa a la wiki, Ordinador); tapping the face opens the profile. A node test that Gregal and another wind render the same structure.
- [x] The wind reacts with its body: a nod when you send, a lean-in with a small particle burst in its colour when a result arrives, a shake on error (reuse `vw-corner-nod`, `vw-corner-shake`, `.vw-burst`). Motion respects reduced motion (web: prefers-reduced-motion; Android: LocalWindMotion). A node test for which reaction each event triggers.
- [x] A failed turn shows as a bubble in the thread spoken by the wind («No he pogut arribar-hi · Torna-ho a provar», with a retry) instead of red text under the composer; ca+en. A node test.
- [x] The corner wind no longer gets cut at the right edge or covers the last message. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/128/`.

## Out of scope

- Android (129).

## Notes

- Files: `.vw-top` and streamChat in winds-workspace.js, winds-workspace.css, i18n.js.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
- Done (winds-workspace.js/.css, tests in winds-start.test.cjs; strings inline with `t(ca,en)` like the rest of the file, so i18n.js did not change).
- Header: `owner()` is the same for every solo wind, Gregal included: a `.vw-owner-who` button (the corner face from 108/138 plus the name) that opens the profile, the role chip (`tag`), the state mark, then the side-pane toggle and a `moreMenu()` «⋯» (role=menu, Esc/outside click close it, arrows move). Its entries: Perfil, Fils (legacy threads only), Desa a la wiki, L'ordinador de Gregal; Gregal adds «Què pot fer Gregal» at the end, because its old header button was the only way back into the guide (it still opens once on its own). Chats no longer show the «Vents /» breadcrumb. Gregal's own header (`vw-top-gregal`, `vw-gregal-*`, its glow and thread padding) and `.vw-top-action` are gone; Gregal now gets the wind wash and the corner wind like the others. While the wind sits in the corner the face slot folds to 0 so the name does not leave a hole; an empty thread (big face in the middle) keeps the wind up in the header.
- Reactions: `react(n, kind)` sets `data-react` = nod (message sent, approval allowed), lean (reply landed, with the `.vw-burst`, now smaller) or shake (error, partial failure, approval declined), for 1 s. All three are transform-only keyframes inside `prefers-reduced-motion: no-preference`. The old hop and shrug (`vw-cheer`, `vw-shrug`) are replaced by lean and shake; their two tests were changed to the new reactions and the motion test now checks nod/lean/shake instead.
- Failed turn: when the open thread's turn throws, your message stays in the thread and the wind says «No he pogut arribar-hi» / «I couldn’t get to it» in a soft bubble with the reason and «Torna-ho a provar» / «Try again» (resends the text). If a group voice had already answered, the server has the message, so there is no retry (resending would ask the whole group again). Errors before the thread exists, or in a thread you left, still go to the status line as before.
- Corner: the spot sits inside the column (`right:0`, was `-24px`, which put the wind 4 px from the window edge and pushed its props and burst outside) and the thread keeps 108 px on the right (was 88). Measured 900-1600 px: wind always ends at the thread's right edge, 12 px clear of the rows.
- Screenshots (not committed): `output/ui-review/128/` before/after `wind`, `sent`, `error`, `gregal` in light/dark at 1280 and 390, `after-menu-*` and `react-{light,dark}-{nod,lean}-*` frames; sandbox with mocked winds/threads and a scripted failing stream.
- Follow-up: the quick-notes button (`qn-launch-vw`, notes-quick.js) is appended after the «⋯» menu in every Vents header; Gregal's header used to hide it.
- Follow-up: `windShare`/`bubbleShare` (085) still only feed `--vw-wind-share`, which tints nothing (already noted in 144).
