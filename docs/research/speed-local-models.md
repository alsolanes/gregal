# Speed with a slow local model: where 70 minutes go

Date: 2026-10-28. Trigger: on 2026-10-27, a task to fill in a PDF with data from
another document plus web research ran for over 70 minutes without answering.
It used `halogen-qwen3.8-flash-next` served from `llm.solanes.xyz`, before
`pdf_read`/`pdf_edit` existed. A day later, a headless run sat 12 minutes
without output or CPU and the same command worked on the second try.

The original run left no trace on this PC. The desktop backend log
(`gregal-backend.log`) has no turn or `vents xat` line after 2026-10-27 15:25,
and no PDF session exists under `~/.local/share/gregal/sessions` or in
`events.jsonl`. The timeline below therefore comes from reproducing the task
in an isolated sandbox: `GREGAL_DATA_DIR`, `GREGAL_AUTH`, `GREGAL_MEMORY`,
`HOME` and `USERPROFILE` in Temp, the same provider and model, and
`mode: autonomous` with the user's autonomous limits (600 min, 2000 tools).

## The model and the server

Measured directly against `llm.solanes.xyz`:

| | |
| --- | --- |
| Decode | ~38 tokens/s (qwen flash thinks before answering: ~1/3 of the tokens are reasoning) |
| Prefill, uncached | ~1,400 tokens/s (32k prompt: 22.5 s before the first token) |
| Prefill, cached prefix | 41k-token prompt plus one appended turn: 2.2 s |
| **Non-streamed request > ~100 s** | **HTTP 524 from Cloudflare after 125 s**, every time |

The last row is the main finding. `llm.solanes.xyz` sits behind Cloudflare,
which returns 524 when the origin sends nothing for ~100 s. A non-streamed
call sends nothing until the model has finished, so any answer over ~3,800
tokens hits the limit. A smaller answer on top of a large prompt hits it too.

## The task

`task-base/`: a Word note (`nota-projecte.docx`, entity, budget, schedule,
impact) and a reportlab AcroForm (`sollicitud.pdf`, 10 text fields, `/Helv`,
`MaxLen 100`). Prompt: fill the form from the note; complete the population,
EU and Spanish regulations and two reference projects from the web; save it as
`sollicitud-omplerta.pdf` and say what was used and where it came from.

## Before

### Build from before PDF support (`34ff5a8`, the user's situation): 39.7 min

31 steps, 38 model calls, 43 tools, 1.51 M prompt tokens uploaded (21% served
from the prefix cache). Model time 30.7 min, of which 4.8 min were
"continue?" decisions; tools 1.9 min; checkpoint reviews 0.7 min.

| Steps | What | Time |
| --- | --- | --- |
| 1-6 | Read the docx; probe the PDF with python/bash | 1.0 min |
| 7-10 | Web research (4 searches, 3 fetches) | 3.0 min |
| 11 | `todowrite`: non-streamed → **524** after ~125 s → resent | 3.8 min |
| 12 | `write` of a hand-made PDF filler script: **524** → resent | 4.0 min |
| 13-29 | Hand-written AcroForm filler, then render/verify loop (LibreOffice, headless Chrome, an `http.server` it left running, `read_image`) | 17.5 min |
| 30-31 | Answer | 2.9 min |
| between | 5 "continue?" decisions, 4 checkpoints | 5.5 min |

The steps were slow (55-90 s each from step 13) because every step resent
25-30k tokens of prompt with a poor cache hit rate, then reasoned before
writing the tool call. But most of the time went to work that should not
exist: building PDF filling by hand and verifying it by rendering. The
user's run went the same way, with more research and steps lost to 524
loops. Each 524 costs up to six full attempts (~11 silent minutes, the
"call that never returns") while the server keeps generating the abandoned
answers. The engine then retries twice more, each with six attempts.

### Current `main` (with `pdf_read`/`pdf_edit`): 7.6 min, no PDF written

10 steps: 2 reading, **8 web steps (23 searches and fetches) chasing exact
details** (the population from the municipal register to the day, a directive's
article number), then the "continue?" decision (72 s) and the summary (112 s).
It stopped at the step limit without ever calling `pdf_edit`. The answer says
so: "I could not write the file". In autonomous mode on the desktop, the
"continue?" decision keeps granting more steps, so the same behaviour simply
goes on.

## Causes, by weight

1. **No research budget.** Nothing in the loop says "enough": a slow local
   model with web tools keeps refining details, and every extra step resends
   the whole growing prompt (15-40 s per step here). The deliverable comes
   last or never.
2. **HTTP 524 treated as a transient 5xx.** Any non-streamed call longer
   than ~100 s is resent identically 6 times, plus 2 engine rounds. That
   covers the headless main loop, subagents, "continue?", summary,
   compaction and side calls. It explains the 12-minute "call that never
   returns" (6 × ~120 s) and minutes lost in the 70-minute run.
3. **A stall costs 20 minutes in autonomous mode.** The per-step inactivity
   limit was 20 minutes with `mode: autonomous`, the user's default mode.
   Headless did not stream, so the limit was really a cap on the whole step.
   Subagent steps had no limit at all (only the HTTP client's 20 minutes),
   and a stalled step was never retried (a rerun worked).
4. **Without `pdf_edit`, PDF work is a loop** (writing a filler, rendering,
   checking). Now the tools exist, but `pdf_edit` failed on any accented text
   in forms that use `/Helv` ("font Roboto-Regular not available"). pdfcpu's
   Unicode form font was never installed, because Gregal disables pdfcpu's
   config folder. The model then fell back to `pip install pypdf` into the
   system Python. `pdf_read` also did not show `MaxLen`, so the model found it
   with grep and counted characters with python (6 steps), and `pdf_edit`
   without `op` failed even when only `fields` was given.
5. Smaller costs: the "continue?" decision with full history and thinking
   (70-100 s each on this model) and a low prefix-cache hit rate (21% in the
   long run).

Ruled out as main causes: web search pauses (blocked engines are skipped,
not waited for; one DuckDuckGo captcha cost nothing), compaction (never
triggered: prompts stayed under 30k tokens), tool time (2-5% of the total),
and prompt size at the start (2.3k tokens of system and tools).

## Changes

| Change | Where |
| --- | --- |
| HTTP 524 is not retried with the same request: the endpoint is marked and every non-streamed call to it is made in streaming from then on (also the call that got the 524) | `internal/llm/origin_timeout.go`, `retry.go`, `client.go` |
| Web research budget per turn: at 8 web calls a one-time guide ("stop researching, do the deliverable now, mark what is unconfirmed"); from 14 on, web calls are answered without running | `internal/agent/research_budget.go`, `motor.go` |
| Silence limit for streamed steps: 5 min autonomous (was 20), 4 interactive; `ModelTimeout` (whole step, non-streamed paths) unchanged | `internal/config/config.go`, `internal/agent/model_context.go` |
| A step cut for silence is asked again once (`ModelStallError`) | `internal/agent/motor.go` |
| Subagent steps get the same silence limit and single retry, and stream | `internal/agent/delegate.go` |
| The headless main loop streams (without printing) so the silence limit measures silence | `internal/agent/run.go` |
| `pdf_edit fill_form` installs pdfcpu's Unicode font on first need, in Gregal's own folder (`<GREGAL_DATA_DIR or user cache>/gregal/pdf`), never `~/.config/pdfcpu` | `internal/pdfdoc/pdfdoc.go` |
| `pdf_read detail=info` shows `text, max N chars`; `fill_form` rejects all over-long values at once before writing; `pdf_edit` with only `fields` means `fill_form` | `internal/pdfdoc/pdfdoc.go`, `internal/tools/pdf.go` |

Tests: `origin_timeout_test.go` (524 switches to streaming and is
remembered; 524 on a stream is not retried), `research_budget_test.go`
(nudge once at 8, cut from 14, other tools untouched),
`stall_retry_test.go` (a stuck subagent request is cut at the limit and
retried, the child answers), `model_context_test.go` (stall retried once,
second ends the turn), `pdfdoc_test.go` (accents in a `/Helv` form,
`MaxLen`), `pdf_test.go` (implicit `fill_form`).

## After

Same task, same sandbox and model:

| Run | Time | Steps | Result |
| --- | --- | --- | --- |
| Before PDF support (`34ff5a8`) | 39.7 min | 31 | PDF filled by a hand-made script |
| `main` before these changes | 7.6 min | 10 (limit) | **no PDF**, research only |
| With the 524, budget and stall changes (PDF font bug still there) | stopped at 7 min | 18+ | budget worked (`pdf_edit` at step 7); then the accent bug sent it to pip |
| + PDF font fix | **6.6 min** | 14 | PDF filled, accents correct; 6 steps spent discovering `MaxLen` |
| + `MaxLen` visible and implicit `fill_form` | **PDF filled at step 6, ~2.6 min**; turn ended at 14.5 min | 16 | see below |

| + appearance fix for accents (below) | **3.9 min** | 10 | **PDF filled, accents drawn correctly, answered on its own** |

In the previous run, research stopped after 4 steps and `pdf_edit` filled all
10 fields at step 6 with correct values (`/V`). The model then checked the
page image and saw blanks instead of accented letters ("Energ tica"). It spent
the remaining 12 minutes writing a script to redraw the appearances. The
cause is upstream in pdfcpu v0.16.3: for **multiline** fields with a standard
font, `textFieldLines` turns the text into Latin-1 bytes, and `wrap()` then
loops over it as UTF-8 (`for _, char := range s`). Every accented byte
becomes U+FFFD and is written as a space.

`internal/pdfdoc/aparences.go` repairs what pdfcpu writes. For each widget
whose value is not ASCII, it lines up the `( ) Tj` strings of the appearance
with the value in WinAnsi bytes and puts back each accented byte that became
a space. It also switches that stream to a Helvetica font with
`WinAnsiEncoding`, so the bytes draw as Latin-1 whatever the form font's own
encoding is. If the alignment does not match (comb fields, different
clipping), the appearance is left as it was. Tests (`aparences_test.go`) read
back what is drawn, for one-line values and for a narrow field where pdfcpu
wraps the value and an accent can start a line.

With that fix, the last run took 3.9 min and 10 steps: 5 steps of research,
`pdf_edit` at step 7, two `pdf_read` checks (the page image now looks right),
and an answer of its own, with no step limit, "continue?" decision or
summary. The model still made an arithmetic slip (40% of 512,000 written as
204,000), which is model quality, not speed.

Each configuration ran once on a shared server, so single-step times vary
(15-90 s at similar prompt sizes). The step counts and what each run did
are the robust part of the comparison.

## Left for later

- **Prefix cache on the shared server**: 21% hits in the long run and 64%
  after, against near-100% for a single conversation measured directly. If
  the halogen server supports `prompt_cache_key` or slot affinity, setting it
  for `strix` would cut prefill from ~15-20 s to ~2 s per step on long turns.
- **"Continue?" decision**: 70-100 s with the full history on this model. A
  short answer budget, or thinking off for that call only, would save a
  minute per extension.
- `gregal -p` tests and runs write failed-request dumps to the real
  `~/.local/share/gregal/req-dumps` (it uses `HOME`, not `GREGAL_DATA_DIR`),
  and rotation keeps only 20.
- Turns on the desktop backend leave no per-step timing in
  `gregal-backend.log`. A one-line summary per turn (steps, model, tools,
  retries) like the winds' `vents xat` line would have made the 70-minute
  run diagnosable from logs.
