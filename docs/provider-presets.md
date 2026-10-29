# Provider Presets

New configurations include public OpenAI-compatible endpoints and two native
protocol presets (Anthropic Messages and OpenAI Responses). No API keys,
private endpoints or account-specific configuration are bundled. Existing
configurations are never overwritten on startup or update.

In the provider settings, select a preset and use **Use for all roles**, then
set that provider's API key. This selects compatible role defaults, resets
provider-specific context/reasoning overrides and removes previous fallbacks.
Custom endpoints are preserved and require manual role assignment. Explicit
per-session model choices remain independent of global role defaults.

| Provider | Base URL | Default model | Key environment variable |
|---|---|---|---|
| OpenAI | `https://api.openai.com/v1` | `gpt-4.1-mini` | `OPENAI_API_KEY` |
| OpenRouter | `https://openrouter.ai/api/v1` | `openai/gpt-4.1-mini` | `OPENROUTER_API_KEY` |
| Groq | `https://api.groq.com/openai/v1` | `openai/gpt-oss-120b` | `GROQ_API_KEY` |
| DeepSeek | `https://api.deepseek.com` | `deepseek-flash` | `DEEPSEEK_API_KEY` |
| Mistral | `https://api.mistral.ai/v1` | `mistral-small-latest` | `MISTRAL_API_KEY` |
| Together AI | `https://api.together.xyz/v1` | `Qwen/Qwen3-Coder-480B-A35B-Instruct-FP8` | `TOGETHER_API_KEY` |
| Cerebras | `https://api.cerebras.ai/v1` | `gpt-oss-120b` | `CEREBRAS_API_KEY` |
| OpenCode Go | `https://opencode.ai/zen/go/v1` | `glm-5.3-flash` | `ZEN_API_KEY` |
| Anthropic (`type: anthropic`) | `https://api.anthropic.com/v1` | `claude-opus-5-5` | `ANTHROPIC_API_KEY` |
| OpenAI Responses (`type: openai-responses`) | `https://api.openai.com/v1` | `gpt-4.1-mini` | `OPENAI_API_KEY` |

The historical provider name `zen` is retained for OpenCode Go compatibility.
Go subscription and pay-as-you-go Zen are not interchangeable endpoints.
All presets except Anthropic and OpenAI Responses use Chat Completions. The
two native presets set `type` on the provider so the client speaks the
Anthropic Messages API or the OpenAI Responses API (see the
[manual](manual.md#native-provider-protocols)). The `openai` and
`openai-responses` presets share a base URL; the provider used by the roles
decides which protocol that URL uses.
Model availability depends on the account, credits, region and provider policy.
Defaults are editable; they are not a guarantee of future model availability.

Local Ollama (`localhost:11434/v1`), LM Studio (`localhost:1234/v1`) and
llama.cpp (`localhost:8080/v1`) are also configured. They need a running local
server and an installed model, rather than a cloud API key.

| Local server | `type` | What Gregal sends | What Gregal probes |
|---|---|---|---|
| Ollama | `ollama` | native `/api/chat`, fixed `options.num_ctx`, `keep_alive: 30m` | `/api/show` (tools, context length), `/api/ps` (loaded context) |
| llama.cpp | `llamacpp` | Chat Completions plus `cache_prompt: true` and a fixed `id_slot` | `/props` (`n_ctx`, `total_slots`, chat template), `timings` on every reply |
| LM Studio | (none) | Chat Completions | `/api/v0/models` (`max_context_length`, `loaded_context_length`, `state`) |

Existing configurations keep their protocol: an Ollama provider without
`type: ollama` still uses `/v1`, where Ollama's default `num_ctx` (2-4k)
silently drops the start of the prompt. `gregal doctor` flags it. The
options `num_ctx`, `keep_alive`, `slot` and `side_slot`, and the server
flags worth setting, are in [local-models.md](local-models.md).

Literal API keys entered in the desktop or Android app are saved to the key
store (`auth.yaml`, 0600) and stay until you clear them (save the key empty).
A plain `gregal --serve` keeps them in memory only unless it runs with
`GREGAL_PERSIST_KEYS=1`; there, use an environment variable reference such as
`${ZEN_API_KEY}` for persistence. Hosted presets
without keys are not probed for models. No paid inference is performed by setup.

Defaults were checked against official documentation on 2026-10-02:
[OpenAI](https://developers.openai.com/api/docs/models/gpt-4.1-mini),
[OpenRouter](https://openrouter.ai/openai/gpt-4.1-mini),
[Groq](https://console.groq.com/docs/models),
[DeepSeek](https://api-docs.deepseek.com/),
[Mistral](https://docs.mistral.ai/resources/migration-guides),
[Together AI](https://www.together.ai/blog/qwen-3-coder),
[Cerebras](https://inference-docs.cerebras.ai/api-reference/models/public-models),
[OpenCode Go](https://dev.opencode.ai/docs/go/).

## Azure OpenAI and Microsoft Foundry

Use the **v1 endpoint** and put the *deployment* name in `model`:

```yaml
providers:
  azure:
    base_url: https://YOUR-RESOURCE.openai.azure.com/openai/v1   # or https://YOUR-RESOURCE.services.ai.azure.com/openai/v1
    api_key: ${AZURE_OPENAI_API_KEY}
  # Same resource through the Responses API: needed to keep reasoning with
  # tools on GPT-5.6 (see below) and to use reasoning `max`.
  azure-responses:
    type: openai-responses
    base_url: https://YOUR-RESOURCE.openai.azure.com/openai/v1
    api_key: ${AZURE_OPENAI_API_KEY}
  # Foundry Model Inference API (non-OpenAI models such as DeepSeek, Llama, Mistral…)
  foundry:
    base_url: https://YOUR-RESOURCE.services.ai.azure.com/models
    api_key: ${AZURE_AI_API_KEY}
roles:
  code:
    provider: azure
    model: my-gpt-5-deployment   # deployment name, not the model family
    think: medium                # sent as reasoning_effort
```

Classic deployment URLs also work, with the query kept at the end:
`https://YOUR-RESOURCE.openai.azure.com/openai/deployments/DEPLOYMENT?api-version=2025-04-01-preview`.

What Gregal handles on its own for these endpoints:

- **Authentication**: an API key is sent in the `api-key` header, as the
  Azure REST API expects. A Microsoft Entra ID token (a JWT, starting with
  `eyJ`) is sent as `Authorization: Bearer`.
- **Foundry Model Inference** (`…services.ai.azure.com/models`,
  `*.models.ai.azure.com`, `*.inference.ai.azure.com`): requests carry
  `extra-parameters: pass-through`, so `reasoning_effort` reaches models
  that support it instead of being rejected by the API.
- **`max_tokens`**: reasoning models only accept `max_completion_tokens`;
  after the first `400` Gregal resends with it and remembers it for that
  model. The same happens with a non-default `temperature`.
- **`reasoning_effort`** (from the role's `think`: `no` → `none`, `low`,
  `medium`, `high`, `max`) depends on the model: `none` exists from
  gpt-5.1, `minimal` only on the original gpt-5, `xhigh` on gpt-5.4+ and
  codex-max, `max` only on GPT-6/5.6 through the Responses API, and o1-mini
  takes none at all. If a model rejects the value, Gregal reads the
  supported values from the error, uses the nearest one (`max` → `high`) and
  remembers it; if the model does not know the parameter, it is dropped for
  that model only (other models on the same endpoint keep it).
- **GPT-5.6 on Chat Completions cannot reason and call tools in the same
  request** (it fails even without `reasoning_effort`, because its default is
  `medium`). Gregal then sends `reasoning_effort: none` whenever tools are
  present, so the agent keeps working without reasoning. To keep reasoning
  with tools, point the role at the `openai-responses` provider above.

Checked against Microsoft's documentation on 2026-10-14:
[v1 API](https://learn.microsoft.com/en-us/azure/foundry/openai/api-version-lifecycle),
[reasoning models](https://learn.microsoft.com/en-us/azure/foundry/openai/how-to/reasoning),
[Model Inference API](https://learn.microsoft.com/en-us/rest/api/microsoft-foundry/modelinference/).
