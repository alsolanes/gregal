# Gregal Evaluations

Configure your own OpenAI-compatible provider and model in `evals/example.yaml` before running evaluations. The example reads an optional key from `GREGAL_EVAL_API_KEY`.

```sh
./gregal --config evals/example.yaml --eval evals
./gregal --config evals/example.yaml --eval evals --tasks t1,t6
./gregal --config evals/example.yaml --eval evals --eval-baseline
./gregal --config evals/example.yaml --eval bench --repeat 10
```

The runner copies task fixtures into `results/`, reports verdicts, steps, tokens, elapsed time and tools, and writes `results/summary.json`. A saved baseline helps detect regressions. A failing assertion or regression returns a nonzero exit status. Assertion scripts require their respective shell/runtime dependencies.

Evaluation tasks execute code and may modify files in their temporary workspace. Review the tasks and provider costs before running them. Do not publish logs containing model prompts or private source. Results are provider/model/configuration dependent and do not establish general superiority over other products.

## Task format

A suite directory holds `tasks/<t>.prompt` (the instruction) and, optionally:

- a fixture, either `tasks/<t>/` or `fixtures/<t>/`, copied into `results/<t>` before the run;
- `tasks/<t>.assert`, an `sh` script run inside the copy with the JSON log and `tasks/<t>.expected` as arguments; exit code 0 is PASS;
- `tasks/<t>.expected`.

Assertions also receive `GREGAL_EVAL_DIR` (the suite root) and `GREGAL_EVAL_FIXTURE` (the pristine fixture). The `bench/` asserts use them to restore the original tests before checking, so an edited test cannot make a task pass, and t9 also runs the hidden test `bench/hidden/t9_hidden.py`, which the agent never sees. A run that modifies the original fixture is reported as ERROR.

Two suites ship with the repository:

| Suite | Tasks | Checks |
| --- | --- | --- |
| `evals/` | t1–t6 | pytest, exact file contents, answer text, no writes for a read-only question |
| `bench/` | t1, t3–t9 | Python, Go (`go test ./...`), HTML/CSS/JS static checks, t9 with a hidden test |

The `bench/` tasks need `python3` (or `python` on Windows; the runner adds a shim) and Go.

## Office tasks

`t10`–`t13` exercise the office tools: a Word report with headings, a list and
a table (`t10`), a five-slide deck with a native chart (`t11`), a workbook whose
totals must be formulas (`t12`) and a tracked-changes edit of a contract
(`t13`, fixture in `tasks/t13/`). Their asserts call `office_check.py`, an
independent stdlib-only checker: the package must be well-formed (every XML
part parses, every internal relationship target exists) and meet the task's
structure (real heading styles and `w:tbl`/`w:numPr`, slide count and a chart
part, formula count and no error values, `w:ins`/`w:delText` with the expected
text). It needs `python3`.

## Repeats

Models are stochastic: a single PASS/FAIL per task cannot separate an improvement from noise. `--repeat N` runs every task N times, each in its own copy (`results/<t>.r1`, `results/<t>.r2`, …, with matching `.log` and `.assert.log`), and adds a per-task table:

- success rate (`passes/runs`);
- `pass@k`, the unbiased estimator `1 - C(n-c, k) / C(n, k)` for k = 1, 3, 5, 10, 20, 50, 100 (those below N) and N;
- median and interquartile range (linear interpolation, as numpy) of steps, seconds, real prompt tokens and LLM calls.

With repeats, a task's verdict is PASS when at least half of its runs pass. That aggregated verdict is what `--eval-baseline` saves and what regressions are checked against; with a single run it is the usual verdict. The exit status is still nonzero if any run fails.

## Metrics

Every run records, from the agent's own counters (`gregal -p … --output json` returns the same fields):

| Metric | Meaning |
| --- | --- |
| `prompt_tokens` | input tokens declared by the provider, summed over every call of the run (0 when only an estimate is available; see `tokens_source`) |
| `cached_tokens` | the part of `prompt_tokens` the provider served from its prompt cache |
| `cache_hit_rate` | `cached_tokens / prompt_tokens` |
| `llm_calls` | model requests, including compaction, extension and synthesis calls |
| `edit_calls`, `edit_failed`, `edit_fail_rate` | executed `edit`/`patch` calls, those that returned an error, and the ratio |
| `tool_call_invalid` (and `tool_call_recovered`, calls written as text that were recovered) | calls to tools that were not offered, arguments that are not valid JSON, and tool calls written as text instead of structured calls |
| `doom_hits` | times the repeated-call detector fired |
| `compactions` | times the history was pruned, summarized or trimmed to fit the window |

Rates are computed over sums, not as an average of per-run rates. A rate with a zero denominator is 0 in JSON and `-` in the table.

`cache_hit_rate` relies on `prompt_tokens_details.cached_tokens` as reported over the OpenAI-compatible API (and Anthropic's cache read tokens). Providers that do not report it, including llama.cpp's `timings.cache_n`, show 0 until the client reads those fields.

## `results/summary.json`

```json
{
  "repeat": 10,
  "totals": {"runs": 80, "passes": 61, "success_rate": 0.7625, "prompt_tokens": 0, "cache_hit_rate": 0, "edit_fail_rate": 0, "...": 0},
  "tasks": [
    {"task": "t1", "runs": 10, "passes": 9, "success_rate": 0.9, "verdict": "PASS",
     "pass_at_k": {"1": 0.9, "3": 1, "5": 1, "10": 1},
     "steps": {"median": 4, "q1": 3, "q3": 5, "iqr": 2},
     "secs": {"median": 11, "q1": 9, "q3": 14, "iqr": 5},
     "prompt_tokens_dist": {"median": 21000, "q1": 19000, "q3": 24000, "iqr": 5000},
     "llm_calls_dist": {"median": 4, "q1": 4, "q3": 5, "iqr": 1},
     "prompt_tokens": 0, "cached_tokens": 0, "cache_hit_rate": 0, "edit_fail_rate": 0, "...": 0}
  ],
  "runs": [{"task": "t1", "run": 1, "verdict": "PASS", "steps": 4, "...": 0}]
}
```

Earlier versions wrote only the `runs` array. `baseline.json` keeps its format (task → verdict).
