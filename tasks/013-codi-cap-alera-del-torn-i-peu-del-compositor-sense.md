---
title: "Codi: capçalera del torn i peu del compositor sense soroll"
status: done
depends_on: []
---

## Why

In a finished turn the answer competes with small things around it: the composer footer keeps showing the speed of a turn that has ended, the «Go to the end» button was seen with the whole answer on screen, and a short summary with a few lists takes half the screen. The chat looks busier than the ones it is compared with.

## Acceptance criteria

- [x] The speed readout (`#speedReadout`) is visible only while a turn is running: it hides when the turn ends, and the speed events of a session that finished in the background do not bring it back when that session is opened again. The average stays in the turn header's tooltip as today. Node tests in `internal/web/turn-status.test.js` and `internal/web/indicators.test.js`.
- [x] `#toBottom` follows the content, not only the scroll: opening a short conversation that fits on screen from a long one hides it, and content growing below while the user reads at the top shows it. Node test in `internal/web/indicators.test.js`.
- [x] Assistant answers in Codi use a line height of 1.6 instead of 1.72, and a paragraph that introduces a list sits close to it instead of a full paragraph gap away. Checked in the browser only (see the notes).

## Out of scope

- The palette: accent, green and amber stay as each theme defines them.
- The top bar icons and the sidebar.
- The Vents shell (`body.shell-winds`) and the Android client.

## Notes

- **Dropped: moving Retro and Repetició out of the turn header.** The task first asked for it, but on this branch they are already invisible until the header is hovered or focused (`internal/web/app/retro.css:155`, commit 2d95e19b, not in `main` yet). The screenshot that prompted the task came from an installed version older than that commit.
- What changed: `internal/web/index.html` (`setSending` hides the readout when the turn ends and `speedAdd` only shows it while sending; the content observer that links file references now also calls `paintScrollDown`), `internal/web/app/refine.css` (line height and the paragraph-before-list gap, only for `#main > .msg.a .body` outside the Vents shell).
- A test changed because the requirement changed: «context details and speed remain visible independently of work steps» (`indicators.test.js`) now runs with a turn in progress, since the speed is no longer shown without one.
- The line height is 1.6 and not the 1.55 first written here: the Codi column is up to 820 px wide at 15.5 px, about 100 characters per line, and lines that long need a little more leading. Narrow screens go from 1.72 to 1.6 too. Plan cards, backlog cards and parallel replies are also `#main > .msg.a` and get the same values.
- The CSS criterion has no automated test: a test asserting the numbers would only repeat them. Checked in the sandbox on 2026-11-20 at 1400×900 with one sample answer (three bold lead-ins, three lists): it went from 444 px to 398 px tall, with a computed line height of 24.8 px. The paragraph before a list now has a bottom margin of 4.65 px (0.3 em); the gap seen is the larger of that and the list's own top margin (0.35 em in `calma.css:269`, about 5.4 px), down from 13.2 px.
- «Go to the end» was also reproduced by hand on the old build (a long conversation scrolled to the top, then its content replaced by a short one: the button stayed) and checked on the new one. The button is repainted on the next animation frame after the content changes, so a hidden or minimised window updates it when it is shown again.
- Follow-ups:
  - The composer footer still shows the `/ commands · @ files` hint, the context gauge and the folder at once; the model name is shown both in the composer and at the bottom of the sidebar.
  - `speedAdd` during the replay of a background session adds to whatever `turnSpeed` holds, so the average in a turn header's tooltip can mix sessions (already true before this change).
  - `doChat`, `doParallel` and `doPlan` call `setSending(false)` without checking that the visible session is the one that started them (already true before this change).
  - Android has no speed readout at all.
