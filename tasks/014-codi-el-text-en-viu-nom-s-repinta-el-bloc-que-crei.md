---
title: "Codi: el text en viu només repinta el bloc que creix"
status: done
depends_on: []
---

## Why

While an answer streams, the web client converts the whole accumulated markdown and replaces the bubble every 120 ms (`paintLive` in `internal/web/index.html`). Long answers arrive in jumps, the cost grows with the length, and a text selection or an open code block is lost on every repaint. It is one reason Codi feels slower than other agents with the same model.

## Acceptance criteria

- [x] A function in `internal/web/app/md.js` splits streaming markdown into finished blocks and the block still being written; a node test shows that a complete paragraph, list and fenced code block do not change when more text arrives after them, and that an unclosed fence stays in the open part.
- [x] During a turn, `paintLive` writes the DOM of a finished block once and afterwards only replaces the open block: a finished block's element is the same node before and after later tokens arrive.
- [x] The bubble after the turn ends has the same HTML as today (`md(text)` of the full answer, then `mdBo`): the existing md tests pass unchanged.
- [x] `steer_restart` and a tool call in the middle of the text still discard or close the live bubble as they do now.

## Out of scope

- The Android and terminal clients: they render the stream on their own.
- A typewriter effect or any animation; the 120 ms cadence stays.
- The output of `md.js` for finished text.

## Notes

- `internal/web/index.html`: `paintLive`, `paintLiveNow`, `settleLiveEl` (around line 4086) and the `/api/chat` path (`paint()`, around line 3956).
- `internal/web/app/md.js` and its node tests (`md-*.test.cjs`).
- The repo has no DOM harness (see the notes of task 008): criterion 2 needs a small fake-node test like `agent-browser.test.cjs` or a check by hand in the sandbox, and the task must say which it did.
- Android: not looked at; whether its streaming has the same cost is unknown.
- How it was checked: `liveCut` and `paintLive` have node tests (`md-live.test.cjs`, with a fake body). Criterion 2 was also checked by hand in the sandbox with the real DOM: a finished block keeps its node while later text arrives. Criterion 4 has no test: the `steer_restart`, `tool_call` and `thinking` handlers are unchanged apart from calling `mdLive`, and were checked by reading, not by running them.
- The open part lives in a `<div class="md-live">` (`display:contents`) at the end of the body, so that what other scripts add next to a finished block (the chart card of `plots.js`) is not removed. Narration bubbles closed by a tool call keep that wrapper; final answers are repainted whole as before.
- **Agent turns do not stream text at all today.** `internal/web/motor_web.go:87` sets `llm.WithRecoverTruncation` for every mode but chat, and `internal/llm/client.go` then holds the text and the reasoning until the model call ends, so a truncated answer can be retried. In the sandbox a 1432-character answer arrived as a single `token` event 100 ms before `run_completed`. This task only shows in chat mode until that changes; it needs its own task (stream live and discard the live bubble on a retry, as `steer_restart` already does).
- Follow-ups found by the review, all present before this change: a table header followed by a `---` line without a leading `|` makes `render` throw (md.js `table`, `rows[1]` undefined); a fenced block inside a blockquote prints the literal `<<B0<<B`; the `error` handler of `doChat` does not clear `paintTimer`; `` ```inline``` `` followed by more lines is treated as a code block open to the end of the text.
