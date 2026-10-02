# Gregal Evaluations

Configure your own OpenAI-compatible provider and model in `evals/example.yaml` before running evaluations. The example reads an optional key from `GREGAL_EVAL_API_KEY`.

```sh
./gregal --config evals/example.yaml --eval evals
./gregal --config evals/example.yaml --eval evals --tasks t1,t6
./gregal --config evals/example.yaml --eval evals --eval-baseline
```

The runner copies task fixtures into `results/`, reports verdicts, steps, tokens, elapsed time and tools, and writes `results/summary.json`. A saved baseline helps detect regressions. A failing assertion or regression returns a nonzero exit status. Assertion scripts require their respective shell/runtime dependencies.

Evaluation tasks execute code and may modify files in their temporary workspace. Review the tasks and provider costs before running them. Do not publish logs containing model prompts or private source. Results are provider/model/configuration dependent and do not establish general superiority over other products.
