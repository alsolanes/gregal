# Agent performance

Measure time to a correct, verified result, rather than only the first token.
Gregal already executes independent read tools concurrently, adds a project map
at the start of a code turn, and supports prompt caching for compatible providers.

## Startup discovery

Web/desktop and headless turns now discover context windows only for the active
role and its fallback. Explicit or already-known windows skip unnecessary
requests. An unrelated slow provider no longer holds up a turn. Global discovery
remains available to the TUI at startup.

The following benchmark isolates discovery with a synthetic 25 ms delay on an
unrelated provider; it makes no generation requests:

```sh
go test ./internal/agent -run '^$' -bench BenchmarkWindowDiscoveryScope -benchtime=5x -count=3
```

On the development Windows machine, all-role discovery took 25.5–25.9 ms and
active-role discovery took 0.15–0.21 ms. This demonstrates removal of that wait,
not a speedup factor for completing real coding tasks. Savings are negligible
when all configured providers are fast or discovery is already cached.

## Reasoning and output budgets

`think: auto` retains the model's default reasoning for decisions and requests
less reasoning after successful mechanical work and for the final synthesis.
For GLM-5.3, GLM-5.3-Flash and GLM-5.3-FlashX, which require reasoning, Gregal
translates that request to `reasoning_effort: low` instead of disabling thinking.
Other models keep the existing `think: no` behavior.

Explicit `think: low`, `medium`, `high` or `max` forwards the corresponding
reasoning effort; the provider must support the selected level. GLM-5.3 supports
`low`, `high` and `max`, not `medium`. Unknown optional fields still use the
existing compatibility negotiation. No effort is added when `think` is unset.
See [Z.ai's model documentation](https://docs.z.ai/guides/llm/glm-5.3) and
[GLM-5.3-Flash parameters](https://docs.z.ai/guides/vlm/glm-5.3-flash).

Too small an output budget can force truncation retries that regenerate the
same work. A larger `max_tokens` cap allows a longer answer; it does not require
the model to generate that many tokens. Choose a cap that fits the task and the
model's context. Keep full diagnostics and verification when comparing speed.

For a trial with an existing coding role, retain its provider/model and adjust
only these fields in a separate private benchmark configuration:

```yaml
roles:
  code:
    # Keep the existing provider, model and other role fields.
    max_tokens: 8192
    think: auto
```

Use a build containing the reasoning compatibility changes for GLM-5.3.
Compare this profile against the original; lower effort is not a guarantee
of the same reasoning quality, and a larger cap does not guarantee no retries.

## Comparison with OpenCode

OpenCode documents [parallel subagents and specialized exploration agents](https://opencode.ai/docs/agents/).
Those are useful design references, but documentation alone does not establish
which implementation completes a task faster.

For an A/B comparison, use separate clean workspaces, identical task text,
provider endpoint, model, reasoning effort, output budget, tool permissions and
verification commands. Alternate run order, distinguish cold and warm caches,
and repeat each task at least three times. Report median total time, success
rate, model requests, tool requests and tokens. Do not count an incomplete or
unverified result as a faster success.

Gregal's existing `--eval` runner records correctness, elapsed time, steps,
tokens and tools; see [evaluation instructions](../evals/README.md).
`GREGAL_PERFIL=1` adds per-model-call timings to headless runs. Live model runs
consume the configured provider's quota; the startup benchmark above does not.
