---
title: "Codi: les eines dins de frases"
status: done
depends_on: []
---

## Why

Even with one row per call, the Codi transcript is a list of tool rows, and in the installed version each call is an open card with its JSON arguments (an empty `{}` box when there are none) and a lot of empty space. The user wants the tools hidden by default inside sentences that say what the agent is doing, so the work reads like a story.

## Acceptance criteria

- [x] Each tool call is a small chip (icon and a few words: file, `+3 −3`, command, domain) at the end of the narration sentence that precedes it; consecutive calls share the sentence. The call's row is hidden until its chip is clicked, and then it opens as a panel under the sentence.
- [x] When the model wrote nothing before a run of calls, Gregal writes a grey sentence of its own in the UI language, chosen by the most telling call of the run (editing beats reading): «Ha consultat el calendari», «Ha editat fitxers», «Ha consultat la web», «Ha executat una ordre», «Ha fet servir github»…
- [x] A failed call has a red chip (with `codi N` for a command) and its panel is open; a bash command's output reads like a terminal, not as markdown.
- [x] A read-only command (the ones the exploration fold already recognises) that exits with 1 or 2 (grep with no match, ls of a missing file) is grey with «sense resultats» / «nothing found», not red and not opened.
- [x] A call without arguments shows no empty `{}` box in its panel.
- [x] Reopened conversations, interrupted calls and the exploration fold get the same chips.

## Out of scope

- Android: task 009.
- The curl block message: task 007.
- A diff view for edits and a richer terminal block (exit code, duration): later.

## Notes

- `internal/web/app/tool-summary.js` (`sentence`, `chip`, `outcome`, `detail`), `internal/web/index.html` (`stepChips`, `toolCard`, `toolCardDone`, `stopToolAnimation`), `internal/web/app/convs.js` and `panels.js` (reopen and reload paths), `internal/web/app/refine.css`.
- Real bash failures read `ERROR: exit: exit status N` (tools.go wraps the error as `exit: …`).
- Only rows made by `toolCard` (`data-call`) and exploration folds get a chip and are hidden; thinking rows, results without a call and rows carrying the dashboard button or a bet chip stay visible.
- Tests: `sentence`, `chip`, `outcome` and `detail` have node tests. The DOM behaviour (hidden rows, chip click, failed panel open, reopen/interrupted/fold paths) has no automated test: the repo has no DOM harness (`agent-browser.test.cjs` fakes a few nodes, without `closest`/`:has`). It was checked by hand in the sandbox.
- Follow-up: the grey sentence and chip texts are not repainted when the UI language changes.
- Built on `task/001-codi-crides-d-eines-compactes` (not merged yet); checks run with `--base task/001-codi-crides-d-eines-compactes`.
