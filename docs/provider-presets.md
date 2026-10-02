# Provider Presets

New configurations include public OpenAI-compatible endpoints. No API keys,
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

The historical provider name `zen` is retained for OpenCode Go compatibility.
Go subscription and pay-as-you-go Zen are not interchangeable endpoints.
These presets use Chat Completions, not Responses-only or Anthropic-only models.
Model availability depends on the account, credits, region and provider policy.
Defaults are editable; they are not a guarantee of future model availability.

Local Ollama (`localhost:11434/v1`), LM Studio (`localhost:1234/v1`) and
llama.cpp (`localhost:8080/v1`) are also configured. They need a running local
server and an installed model, rather than a cloud API key.

Literal API keys entered in the UI are active in memory only. For persistence,
use an environment variable reference such as `${ZEN_API_KEY}`. Hosted presets
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
