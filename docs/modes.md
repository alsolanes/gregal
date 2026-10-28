# Modes and permissions

Gregal shows the same small set of choices in the web and desktop app, the
terminal interface (TUI) and the Android app: three modes and, for Act, one
permission. (Catalan: [modes.ca.md](modes.ca.md).)

## Modes

| Mode | Catalan | What it does | Internal id |
|---|---|---|---|
| **Ask** | Pregunta | Answers and reads the project (files, safe read-only commands, the web). Changes nothing. | `chat` |
| **Plan** | Planifica | Investigates read-only and proposes a numbered plan. Changes nothing until you approve it; running the plan switches to Act. | `plan` |
| **Act** | Actua | Makes changes, runs commands and verifies, with the permission below. | `code`, or `autonomous` with Full autonomy |

Where to pick them:

- **Web / desktop:** the mode switch in the composer (`···`). `Tab` in an empty
  composer cycles Ask → Plan → Act; `/mode` does the same, `/plan` switches
  to Plan.
- **TUI:** `Shift+Tab` cycles Ask → Plan → Act; `/mode` opens the picker;
  `/mode ask|plan|act` (also `pregunta|planifica|actua`).
- **Android:** the Ask | Plan | Act switch in the composer.

In Plan, sending a message asks for a plan (the same read-only exploration as
`/plan`); the plan card's **Run** button (TUI: `e`) switches to Act and runs
it with the Act permission you had.

## Act permission

| Permission | Catalan | What happens |
|---|---|---|
| **Ask before changes** | Demana abans de canviar | Every edit, including files inside the project, and every command that changes something asks first. |
| **Auto-approve edits** (default) | Aprova les edicions | Edits inside the project folder go ahead; commands that change things, writes outside the folder, commits, PRs and external services (MCP) still ask. |
| **Full autonomy** | Autonomia total | Works on its own until it is done and never asks (this is the autonomous mode: milestones, checks, no `question` tool). Only what is forbidden stays blocked: the shell classifier (`sudo`, piping downloads into a shell, formatting disks…) and `deny` rules. |

Where to pick it: the lock button beside the composer (web, desktop),
`/permissions` or `/permissions ask|edits|full` (TUI), **+ → Act permission**
(Android). In Ask and Plan nothing changes, so the permission only applies in
Act; picking one switches to Act. Leaving Act and coming back keeps it.

The choice is saved in the config: `mode` (`autonomous` for Full autonomy) and
`permissions.access` (`ask`, or empty / `edits` for the default). API:
`POST /api/access {access: ask|edits|full}`; `/api/state` reports `ui_mode`
and `access` (see [api-contract.md](api-contract.md)).

## Advanced, outside the main picker

- **Goal** (`/goal`, `/mode goal`) shapes a persistent goal with questions
  before running it. It is shown as Plan with a "Goal" label.
- **Debug** (`/debug`, `/mode debug`) runs Act with a debugging contract. It is
  shown as Act with a "Debug" label; Ask before changes and Auto-approve edits
  keep you in Debug.
- **Inspect** (`/mode inspect`, `--mode inspect`) is Ask with read-only tools
  and no questions, used by subagents and integrations. It is shown as Ask.
- **Permission reviewer** (Preferences in the web and Android, `/autoreview`
  in the TUI, `permissions.mode: auto-review` in the config): a fast model
  decides what would ask for permission. It does not apply to Full autonomy,
  which never asks.
- **Per-tool permissions** stay in the config file: `permissions.tools`,
  `permissions.rules`, `bash_allow`, `bash_deny` and
  `permissions.subagents` (see the manual). The TUI also has
  `/permissions tools`.

## Old names

Old configs, saved sessions and API clients keep working:

| Old | Now |
|---|---|
| Mode `code` / Act | Act with Auto-approve edits (or Ask before changes if `permissions.access: ask`) |
| Mode `autonomous` / Autonomous button | Act with Full autonomy |
| Mode `chat`, `inspect` | Ask |
| Mode `goal` | Plan (advanced Goal) |
| "Plan first" toggle | Plan mode |
| Permission Manual / Ask permission | Auto-approve edits (it already let edits inside the project through) |
| Permission Accept edits | Auto-approve edits |
| Permission Auto / Don't ask (`/permissiu on`) | Full autonomy |
| Permission Auto-review | Auto-approve edits plus the permission reviewer in Preferences |

`/api/mode` also accepts `ask`, `plan` and `act` (`act` returns to the last
Act mode); `/api/permissive` still works but no client shows it.
