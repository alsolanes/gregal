# Local models: Ollama, llama.cpp and LM Studio

*[Català](local-models.ca.md)*

On a local server, **prefill** (reading the prompt) takes most of the time.
An agent resends the whole conversation on every step. When the server's KV
cache keeps the prefix, each step only processes the new tokens. When the
cache misses, a 30k-token conversation can take tens of seconds on a laptop
before the first token. So the cache hit rate matters more than almost
anything else. Two failure modes are silent:

- **Ollama truncates the context.** Its default `num_ctx` is 2048-4096
  tokens. A longer prompt loses its beginning, which is the system prompt and
  the tool definitions, and Ollama gives no error.
- **llama.cpp shifts the context** when `--context-shift` is on. It drops old
  tokens when the window fills, also without an error.

Gregal handles the client side of both. You can check the server side with
`gregal doctor`.

## Ollama: `type: ollama`

```yaml
providers:
  ollama:
    type: ollama
    base_url: http://localhost:11434/v1   # /v1 or the bare root both work
    api_key: ""
    # num_ctx: 65536     # optional; see below
    # keep_alive: 30m    # optional; "-1" keeps the model loaded forever
```

With `type: ollama` Gregal uses the native `/api/chat` API, with streaming,
tools, images and thinking. It also does the following:

- **It sends a fixed `options.num_ctx`.** The value is chosen once per model
  and process, and never changes between requests, because a different
  `num_ctx` makes Ollama reload the model. Gregal picks the first of these
  that applies:
  1. `num_ctx` from the provider config;
  2. the context the model is already loaded with (`/api/ps`), if it is at
     least 16k, so the first call does not reload it;
  3. 32k, or the model's maximum (`/api/show`) if that is smaller.
  
  Ollama recommends 64k or more for agents. Set `num_ctx: 65536` if the
  model and your memory allow it.
- **It sends `keep_alive: 30m`.** With Ollama's default of 5 minutes, the
  model unloads between turns and the cache is lost.
- **It checks capabilities with `/api/show`.** A model without `tools` gives
  a clear error instead of a failed request. `think` is only sent to models
  with `thinking`. gpt-oss gets `low`/`medium`/`high`.
- **It uses that `num_ctx` as the context window**, so compaction starts
  before Ollama would truncate.

Ollama does not report cached tokens. When `prompt_eval_count` is well below
the prompt size, the difference is counted as cached (an estimate).
`load_duration` should be close to 0 from the second turn on. If it is not,
the model is being reloaded.

Server environment (for `ollama serve`):

| Variable | Value | Why |
|---|---|---|
| `OLLAMA_FLASH_ATTENTION` | `1` | Required for a quantized KV cache; faster prefill |
| `OLLAMA_KV_CACHE_TYPE` | `q8_0` | Half the KV memory, negligible quality loss |
| `OLLAMA_CONTEXT_LENGTH` | `32768` or more | Default for clients that do not send `num_ctx` |
| `OLLAMA_KEEP_ALIVE` | `30m` or `-1` | Default for clients that do not send `keep_alive` |

Sources: [context length](https://github.com/ollama/ollama/blob/main/docs/context-length.mdx),
[FAQ](https://github.com/ollama/ollama/blob/main/docs/faq.mdx),
[API](https://github.com/ollama/ollama/blob/main/docs/api.md),
[reload when num_ctx changes](https://github.com/ollama/ollama/issues/9749).

## llama.cpp: `type: llamacpp`

```yaml
providers:
  llamacpp:
    type: llamacpp
    base_url: http://localhost:8080/v1
    api_key: ""
    # slot: 0          # id_slot of the session (-1 = let the server choose)
    # side_slot: 3     # id_slot of side calls (default: the last slot)
```

With `type: llamacpp` Gregal keeps Chat Completions and adds the following:

- **`cache_prompt: true`** on every request.
- **A fixed `id_slot`** when the server has more than one slot
  (`--parallel N`, read from `/props`). The session uses slot 0. Side calls
  (plan critic, automatic review, commit messages) use the last slot, so they
  do not evict the session prefix. If the slot already has a request in
  flight from this process (parallel subagents), the request goes without
  `id_slot` and the server picks a free slot. Otherwise it would wait in a
  queue.
- **It reads `timings`.** `cache_n` becomes `cached_tokens` in the usage
  meter. `prompt_n` and `prompt_per_second` give the real prefill speed.
- **It reads the context window from `/props`** (`n_ctx` per slot). The
  `/v1/models` endpoint only declares the training context, which is usually
  much larger than what the server allocated.

Recommended `llama-server` flags:

| Flag | Why |
|---|---|
| `--jinja` (default in current builds) | Native tool calls with the model's template. Without it, or with a template llama.cpp does not recognise, tools fall back to the **Generic** format: less reliable and more tokens |
| `--cache-reuse 256` | Reuses cached chunks after a change in the middle of the prompt ([PR #9866](https://github.com/ggml-org/llama.cpp/pull/9866)) |
| `-ctk q8_0 -ctv q8_0` | Half the KV memory. q8_0 on K changes perplexity by ~0.03%; **q4_0 on K by ~3.3%** and hurts tool calls ([PR #7412](https://github.com/ggml-org/llama.cpp/pull/7412)) |
| `-fa on` | Flash attention, needed for a quantized V cache |
| `-ub 1024` (to 2048) | Larger physical batch, faster prefill |
| `--spec-type ngram-mod` | Speculative decoding from n-grams in the prompt (prompt lookup), with no draft model. Edits copy a lot of their input: 2-4x faster ([prompt lookup](https://github.com/apoorvumang/prompt-lookup-decoding)) |
| `-md draft.gguf --spec-draft-n-max 16` | Draft-model speculation (the old `--draft-max`/`--draft-min` flags are gone) |
| `--no-context-shift` | Never drop old tokens silently; Gregal compacts before the window fills |
| `--parallel 2` (or more) | One slot for the session and one for side calls |
| `--slot-save-path DIR` | Lets you save and restore a warm prefix |

Hybrid and sliding-window models (Gemma 3/4, Qwen3-Next) reuse the cache
less well. See [issue #19794](https://github.com/ggml-org/llama.cpp/issues/19794).

Sources: [server README](https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md),
[function calling](https://github.com/ggml-org/llama.cpp/blob/master/docs/function-calling.md).

## LM Studio

LM Studio uses Chat Completions (no `type`). At startup Gregal reads
`/api/v0/models`. If the model is loaded, the context window is
`loaded_context_length`. Otherwise it is `max_context_length`. Raise the
idle TTL in LM Studio so the model stays loaded between turns.

## vLLM and others

vLLM declares `max_model_len` in `/v1/models`, and Gregal already reads it.
Prefix caching is on by default. Tool calls need
`--enable-auto-tool-choice --tool-call-parser <family>`.

## `gregal doctor`

For every local provider used by a role, `gregal doctor` prints:

- **llama.cpp**: `n_ctx` and slots, plus a warning if `--context-shift` is
  on, if the template cannot render tools (Generic format), if the context is
  below 16k, or if the provider lacks `type: llamacpp`. Then a tip line with
  the flags it cannot confirm (`--cache-reuse 256`, `-ctk q8_0 -ctv q8_0`,
  `-fa on`, `-ub 1024`, `--spec-type ngram-mod`).
- **Ollama**: the `num_ctx` Gregal will send and where it comes from, plus a
  warning if it is below 16k, if the model has no `tools` capability, or if
  the provider still uses `/v1` without `type: ollama`.
- **LM Studio**: the context of each role's model and whether it is loaded.

Warnings (`WARN`) and tips (`INFO`) do not make `doctor` fail.
