---
name: configuracio
description: Gregal's config.yaml (roles, providers, permissions, verification, step budget) and where the API keys live
---

## The two files

- `~/.config/gregal/config.yaml` — everything except secrets. Safe to
  show and to keep under version control.
- `~/.config/gregal/auth.yaml` — the API keys, kept separate. Do not open
  it to show its contents and never copy a key into the conversation.

`gregal --config <path>` uses a different config. `gregal --check-config`
validates it and exits; `gregal --doctor` also checks that the providers
respond.

## The keys

- `providers:` — one per endpoint, with `base_url` and `api_key`.
  `api_key` accepts `${VARIABLE}` to read it from the environment.
- `roles:` — which model does which job. The required roles are `chat`,
  `think`, `code` and `reviewer`; each has `provider`, `model`,
  `temperature`, `max_tokens` and, optionally, `context_window` (0 = the
  one the provider declares) and `prompt: compact|full` (empty = automatic:
  local models of 14B or less, or with a window under 32k, get a short
  prompt and only the core tools).
- `mode:` — the startup mode (`code`, `chat`, `inspect`, `goal`,
  `autonomous`). See the `modes` skill.
- `permissions:` — `tools:` with `allow`, `ask` or `deny` per tool, plus
  `bash_allow` and `bash_deny` to restrict the shell. Tools not listed
  follow the default: reading runs freely, writing and the shell ask.
- `verify:` — `mode: off|manual|auto|both|strict`: whether a reviewer
  checks the work at the end of the turn.
- `agent.max_steps` — the step budget for a turn. When it runs out, the
  user is asked whether it is worth continuing. Turns that stop halfway
  with "the N available steps are used up" (Catalan: «s'han acabat els N
  passos») usually mean this number is too low for the task.
- `agent.autonomous` — `checkpoint_every`, `review_every`, `max_minutes`
  for autonomous mode.
- `router:` — automatic role selection by task (`mode: auto|off`).
- `hooks:` — `post_edit` runs a command with `{file}` after every
  successful write (e.g. `gofmt -w {file}`); `diag` controls whether
  syntax checking runs.
- `system:` — the identity prompt. Empty = Gregal's default.
- `theme:` (`fosc`|`clar`), `lang:` (`en`, the default, | `ca` for
  Catalan), `mouse:` (`on`|`off`), `animacions:` (`on`|`off`, off by
  default) and `cockpit:` (`on`|`off`, the TUI's right-hand column).
- `mcp:` — MCP servers; their tools appear with the `mcp_` prefix.
- `telegram:` — the bot token and authorized users.
- `cost:` — prices per million tokens, used to compute the session cost.

## Changing it without editing the YAML

In the TUI, `/settings` (or Ctrl+,) covers everything: mode, role, model,
API key, theme, reviewer, permissions and providers. There are also
`/permissions`, `/model`, `/role`, `/reviewer`, `/provider`, `/key`,
`/theme` and `/mouse`. Everything chosen there is saved to the config.

When asked for a configuration change, the clean way is to tell the user
which TUI command does it. Editing the YAML with `edit` also works, but
Gregal must then be restarted to read it.
