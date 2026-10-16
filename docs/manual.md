# Gregal User Manual

[Català](manual.ca.md)

Gregal is a coding agent with terminal, desktop and web interfaces. It sends
model requests to the endpoint configured for the selected role. This guide
covers the common setup and safety boundaries; use the in-app help and the
linked references for details that depend on your build.

## Requirements

- Go 1.27.1 or newer to run from source.
- An OpenAI-compatible model endpoint and model IDs available from that endpoint.

The endpoint and model must support the capabilities you intend to use, such
as tool calling. Configuration validation does not test network access,
credentials or model availability.

## Initialize and Configure

From the repository root, create the initial configuration:

```sh
go run . init
```

On Unix-like systems, the default path is `~/.config/gregal/config.yaml` and
the file is created with mode `0600`. The command also creates an `AGENTS.md`
in the current project directory if one does not already exist. Choose a
different config path with `go run . init --config=/path/to/config.yaml`.

Set the provider URL and model IDs for the `chat`, `think`, `code` and
`reviewer` roles. The generated `local` and `local-direct` URLs refer to
loopback services that Gregal does not start. The generated `cloud` URL is an
example; replace it and the model IDs with values supported by your endpoint.
You can assign multiple roles to one provider if it supports their required
capabilities.

The example cloud provider reads its key from an environment variable:

```yaml
providers:
  cloud:
    base_url: https://api.example.org/v1 # replace with your endpoint
    api_key: ${GREGAL_CLOUD_API_KEY}
```

Set the variable in the environment used to launch Gregal, or use a secret
manager. Do not commit credentials or private configuration.

### Native provider protocols

By default every provider speaks the OpenAI-compatible Chat Completions
protocol. Two native protocols are available with `type`:

```yaml
providers:
  anthropic:
    type: anthropic            # Anthropic Messages API (POST <base_url>/messages)
    base_url: https://api.anthropic.com/v1
    api_key: ${ANTHROPIC_API_KEY}
  openai-responses:
    type: openai-responses     # OpenAI Responses API (POST <base_url>/responses)
    base_url: https://api.openai.com/v1
    api_key: ${OPENAI_API_KEY}
```

`type` accepts `openai` (or empty, the default), `anthropic` and
`openai-responses`. The engine, failover, retries (429/5xx, Anthropic 529)
and progress reporting are unchanged; only the wire format differs.

- **anthropic**: system prompt, tools, `tool_use`/`tool_result` blocks and
  streaming. Thinking blocks (with their signatures) are returned unchanged
  within a tool-call chain. Prompt caching marks the system prompt, the tool
  list and the latest message with `cache_control`; usage reports cache reads
  and cache writes. The role's `think` setting maps to adaptive thinking with a
  visible summary plus `output_config.effort` (`low`/`medium`/`high`/`max`);
  `think: no` requests `effort: low`. `temperature` is not sent, because
  current Claude models reject sampling parameters.
- **openai-responses**: messages, function calls and streaming through the
  Responses API with `store: false`. Encrypted reasoning items are requested
  and returned unchanged before the calls they produced; reasoning summaries
  stream as reasoning text. `think` maps to `reasoning.effort`.

Optional fields (thinking, effort, temperature, reasoning, include) are
dropped automatically for a model that rejects them with HTTP 400, and are
not sent to that model again during the process. The protocol is chosen by
`base_url`: if two providers share a URL with different types, the one used
by the roles decides, and the configuration is rejected if roles use both.

## Run the Terminal Interface

Check the configuration and start Gregal:

```sh
go run . --check-config
go run .
```

`--check-config` checks configuration structure and references only. In the
terminal interface, `/help` lists available commands and `/model` opens a
picker for models advertised by configured endpoints.

## Run the Local Web Interface

Start the server on loopback:

```sh
go run . --serve --addr 127.0.0.1:8097
```

Open `http://127.0.0.1:8097/` in a browser. Gregal rejects non-loopback
listeners unless a token or user authentication is configured. The server
does not provide TLS; configure HTTPS before making it reachable outside a
trusted local machine.

The server logs each turn and team run to the terminal (outcome, elapsed
time, never credentials). With `--log-file path` (or `GREGAL_LOG_FILE`) it
also appends to a file, so a stopped run leaves a trace of what happened.

In the composer, choose **Do a task** to work with tools or **Chat** to talk
without modifying files. Goal and autonomous modes are available through
the advanced modes disclosure. Permissions remain visible beside the mode
selector; changing modes does not expand them. Manual role selection is an
advanced model picker option; Gregal can select the role for the task.

## 2D Workshop

The sidebar's **2D workshop** view represents open sessions as workstations.
Each station shows its project and whether the session is working, idle,
or waiting for a response. Select a station to open its conversation;
**Who needs me?** opens the first session with a pending interaction known
to the current browser.

Stations reuse existing sessions. Attention alerts depend on questions and
approvals received by this client; they are not a global record of pending
interactions from other browsers.

The workshop also follows real delegated work: select a child agent to see
its task, model, current tool, step budget, result, or error. The animated room
and cards show observed states; step counts are not completion percentages.
Finished children remain visible until the next turn, with up to 32 recent
children retained. The room pauses when hidden and respects reduced motion.
**Roles and handoffs.** Above the bays, the room shows one character per role
for the current session: Coordinator, Researcher, Builder, Tester and
Reviewer. Each step of the coordinator is attributed to a role by the tools it
uses (planning and `todowrite` → Coordinator; `read`, `grep`, `glob`,
`web_search`, `web_fetch` and read-only shell commands → Researcher; `write`,
`edit`, `patch`, `office_edit`, `git_commit` and other shell commands →
Builder; test, build and lint commands → Tester; autonomous checkpoints and
automatic verification → Reviewer). When the role changes, an envelope flies
between desks with a short label ("2 files to edit", "Tests to run") and file
chips; the receiving character walks to its desk, its monitor lights up and a
speech bubble says what it is doing ("Reading src/stats.py", "Running tests").
Test results stay as a green or red badge on the Tester's desk. Idle roles
stand dimmed beside their desks. Real delegated children sit in a back row
under their user-defined subagent name. The attribution is visual only: the
coordinator is still one agent.

Click a character, a desk or a role in the legend to open its panel: the
role's actions in this turn, files touched (click to preview them; the Builder
also links to Changes), commands with their result and duration, the tokens
used while it held the turn and, for the Researcher, the real scout's brief
and summary. The coloured strip under the room has one segment per step;
click one to highlight that step and its character. The strip and the
handoffs also work in **Replay**.

**Researcher scout.** In code, autonomous and debug modes, a turn whose task
needs investigation (not greetings, confirmations or tiny edits) starts with a
real read-only Researcher subagent (only in projects of 40 files or more: in a
smaller one the project map already lists every file, so the scout would only
add a sub-run before the first step; `agent.scout: always` runs it anyway):
up to 8 steps and 4 minutes, with the
`delegate` role's model when configured, otherwise the session's model (or a
user-defined read-only subagent named `researcher`). Its brief, with file
paths, is added to the task so the coordinator does not repeat the
exploration. If the scout fails or is stopped, the turn continues normally.
Its tokens count in the turn. Disable it with `agent.scout: off`
(default `auto`).

**Parallel subagents.** When the task names several independent areas (a
list such as "(1) … (2) …", numbered or bulleted lines, paths in different
folders, or a question about "the TUI, the web server and the Telegram bot"),
the scout splits: one researcher per area, up to 3, all working at the same
time, and their briefs reach the coordinator grouped by area. The coordinator
is also told to send independent investigations to `delegate` in a single
call, so they run together. At most 4 subagents run at once per turn (8 per
process); the rest show as **Queued** until a slot frees. When subagents use
a local server (Ollama, llama.cpp, LM Studio or any localhost URL) they run
one at a time, because one slot would only queue them inside the server.
`agent.parallel_subagents: N` overrides the limit. In the room, children
working together sit side by side in the back row with a "3 at once" pill;
in the bay each child card has a bar on the turn's time axis (bars that
overlap ran together), and the heading keeps "up to N at once" after they
finish.

Team runs (Team view) also appear in the workshop as live bays: coordinator,
agents with task and result, flow graph, detail, queue and stop are shared;
it is a single 2D room. The separate Team view continues to offer its
tool-free collaboration workflow.
Each bay shows the turn's real consumption (up and down tokens, cost only with
a configured price; bare call counts when the provider declares nothing) and
an inspection timeline of the turn's steps. A busy bay can be stopped in place,
and the browser tab title counts sessions waiting for a response. A merely
queued turn says so instead of faking a stop. The summary shows the total
cost of priced sessions (or "cost n/a"), the data age, and per-bay inspection
history; the attention button cycles through waiting sessions and failures
get their own section.

In Code and Autonomous modes, the coordinator is prompted to automatically
delegate useful independent investigations and reviews. Children can inspect
files and search, but cannot edit, request decisions, or create further children.
There are at most four concurrent children per web turn, each bounded to 12
steps and eight minutes. Canceling the parent stops its children. Global
ceiling: eight live subagents per process on every client; returning
summaries are capped with an explicit mark past 6000 characters.

An optional `roles.delegate` entry selects a smaller/faster model for this work;
without it, web children use the active coordinator model. For example, using
an existing provider and a model it supports:

```yaml
roles:
  delegate:
    provider: your_provider
    model: your_fast_model
    max_tokens: 2048
```

Merge this entry into the existing `roles` mapping. Delegation follows the
normal tool permissions (`permissions.tools.delegate: deny` disables it).
The model decides when delegation is worthwhile; parallel work can add model
usage and is most useful for genuinely independent investigations.

An optional `roles.small` (or `roles.fast`) entry, with the same fields,
moves the side tasks to a smaller model: compaction summaries, project-note
suggestions and the pre-turn scout. If the small model fails, the main model
does the work. Without it, nothing changes.

## Extras: Room Replay, Wrapped, #retro Tone, Weathervane, Voice and Images

All extras can be switched off in **Preferences → Extras**, and every
animation stops with the system's *reduce motion* setting.

**Room replay.** The Agents page (Live tab) has a **Replay** button, and each
finished work group in the conversation has one too. It plays a finished run
back in the 2D room from the durable event log: the coordinator thinking,
using tools (with their names) or reviewing, waiting for approvals, and
delegated children walking in, working and delivering. Play/pause (Space),
drag the scrubber, and choose 1×, 4× or 10×; long idle waits (for example an
approval left for minutes) are shortened to 15 seconds. **Download clip**
records the whole replay as an animated WebM video (browsers with
`canvas.captureStream` and `MediaRecorder`; the button is hidden elsewhere).
Work groups of conversations reopened after a reload are matched to runs by
order (best effort); the run picker in the dialog lets you choose another.

**Gregal Wrapped.** Once a week the home page shows a small card with the
highlights of the last seven days (dismiss it with **Not now** until next
week, or turn it off). **More → Stats** opens the full view for the last week
or month: turns, sessions, tool calls by kind, files changed, tests turned
green (failing → passing check sequences), the most stubborn bug (the run with
most failures before green), longest and current streak of days, favourite
model and estimated tokens (cost only when the config has prices). **Download
image** saves a shareable PNG. Everything is computed locally from your saved
conversations and the event log (`GET /api/stats`).

**#retro team review.** When you open #retro, the agents that did the work
explain how it was implemented while a team of reviewers who did not do it
looks at it in parallel, each from its own angle: **Design** (looks, UI/UX,
accessibility; only when something visible changed), **Code** (best
practices), **QA** (bugs, edge cases, missing tests), **Product** (does it do
what was asked) and **Security**. Each one posts a one-line verdict and up to
three findings with a severity (high, medium, low), receipts and a fix you
can start as a new run with **Fix it**. The bar at the top counts the
findings and filters them by severity. A reviewer with nothing to say stays
silent, and one that fails or runs out of time does not hold up the rest.

**#retro tone.** Agents in #retro can talk as *neutral*, *friendly*,
*sarcastic* or *mentor*. Pick the default in Preferences (or leave *From
config*, which uses `retro.persona` in the config file) and change it per
channel from the picker in the #retro header. Only the tone changes: the
grounding rules (receipts, no invented facts, honest failures) are the same.

```yaml
retro:
  persona: mentor   # neutral | friendly | sarcastic | mentor
```

**Weathervane.** Off by default; turn it on in **Preferences → Extras →
Weathervane**. A small weathervane at the foot of the sidebar shows Gregal's
state: it points to 45° NE (the Gregal wind) with a gentle sway when idle,
spins in gusts while working, turns towards the composer with an amber tip
when an approval is waiting, swings back and settles on NE with a green
flash when a turn finishes, and droops, red and rusty, when it fails. Click
it to send a quick message to the current session (a draft in the composer
is kept); its quick message box also has **Hide**, which turns the sidebar
vane off. In the desktop app, **Floating weathervane (desktop)** opens the
same vane in a small always-on-top window: a short click returns to the app,
hold and move to drag it anywhere (it stays inside the screen and sticks to
an edge when dropped near one). Hovering shows three small controls:
**minimize** (folds it into a 32 px compass dot; click the dot to expand,
or press Esc to fold), **snap to a corner** (nearest corner, then the next
one clockwise) and **hide** (turns the preference off; turn it back on in
Preferences). Position and folded state are remembered. With *reduce
motion* the vane stays still and only its angle and colour change.

**Voice input and images.** When the browser supports the Web Speech API
(Chrome, Edge, the desktop app), the composer shows a microphone: hold it to
talk (push-to-talk) or click once to start and again to stop; interim text
appears live in the field, in the interface language. Pasted screenshots and
images dropped on the composer join the normal attachments (at most 3
images, 8 MB each). Speech recognition in Chromium-based browsers may be
processed by the browser vendor's online service.

## Data and Tool Execution

Prompts, conversation context, and any code or files included in a model
request are sent to the endpoint configured for that role. A provider named
`local` is local only if the service actually runs on your machine; a local
proxy may forward requests elsewhere. Web retrieval and configured MCP
connectors can also make network requests.

File and shell tools run on the host in the selected project. By default
(`sandbox.mode: off`) the only protections are application controls: the
shell-command classifier, the hard-deny rules (for example `sudo`, `curl`,
writes to system paths) and the confirmation prompts. They catch common
mistakes, but they are pattern checks, not isolation: a command that is
approved, or that the classifier considers safe, runs with your user's full
permissions. Review proposed changes and commands. For untrusted code, use a
disposable checkout, a container or a virtual machine.

### Shell sandbox (opt-in)

The `sandbox` section confines the `bash` and `bash_background` tools (and
`!` commands typed in the terminal interface) at the operating-system level.
The classifier, hard-deny rules and approvals still apply on top of it. File
tools (`write`, `edit`, `patch`), hooks, MCP servers and the browser are not
sandboxed.

```yaml
sandbox:
  mode: workspace      # off (default) | workspace | strict
  network: "off"       # on (default) | off
  allow_write: []      # extra writable paths (~ and $VARS are expanded)
  backend: auto        # auto | bwrap | landlock | sandbox-exec | docker
  image: alpine:3.20   # docker backend only
```

- `workspace`: writes are allowed only in the command's workspace, the
  temporary directory, your user cache directory (so builds and package
  managers keep working) and `allow_write`. If no backend is available,
  Gregal warns once (in the tool output and in `gregal --doctor`) and runs
  commands **unsandboxed**.
- `strict`: like `workspace`, without the user cache directory and with a
  private `/tmp` on Linux. If no backend is available, or the backend cannot
  cut the network when `network: off` is requested, shell commands are
  refused.

What each backend guarantees:

| Backend | Platform | Filesystem | Network off | Notes |
|---|---|---|---|---|
| `bwrap` (bubblewrap) | Linux, first choice of `auto` | whole filesystem read-only, writable binds for the allowed paths, new `/dev` and `/proc`, separate PID namespace | yes (`--unshare-net`) | needs the `bwrap` binary and unprivileged user namespaces |
| `landlock` | Linux 5.13+, used by `auto` when `bwrap` is missing | writes restricted to the allowed paths; reads are not restricted | TCP only, and only with Landlock ABI 4 (Linux 6.7+); UDP is not covered | applied by a small `gregal __sandbox-exec` helper process |
| `sandbox-exec` | macOS, chosen by `auto` | file writes restricted to the allowed paths; reads are not restricted | yes (Unix sockets still allowed) | deprecated by Apple but still shipped |
| `docker` | any OS; the only option on Windows; never chosen automatically | only the workspace and `allow_write` are mounted from the host (same path; `C:\x` becomes `/c/x`) | yes (`--network none`) | commands run in `image` with its tools, not the host's; `strict` adds a read-only root, a tmpfs `/tmp` and drops all capabilities |

On Windows there is no automatic backend: set `backend: docker` with Docker
running. Without it, `workspace` runs unsandboxed with a warning and `strict`
refuses shell commands. No backend limits CPU, memory or time beyond the tool
timeouts, and none hides secrets that are readable inside the workspace or
(except docker) anywhere your user can read. The active backend is shown by
`gregal --doctor`, in `/api/state` (`sandbox`) and at the bottom of the web
composer's permission menu.

### Agent browser

The `browser` tool drives a real Chromium page (JavaScript, local apps on
`localhost`, sites where you are signed in). Where it runs is set by
`browser.mode`:

```yaml
browser:
  mode: auto   # auto (default) | app | headless | window
```

| Mode | Behaviour |
| --- | --- |
| `auto` | `app` when the desktop app started this backend and registered its browser bridge; otherwise `headless` |
| `app` | the desktop app's own browser pane opens next to the conversation and the agent drives it; you watch live and can click or type at any time. Without a bridge it falls back to `headless` and the tool result says so |
| `headless` | the machine's Chrome or Edge without a window, using Gregal's own profile (`~/.config/gregal/browser`) |
| `window` | the same browser with a visible window; useful to sign in to a site once in Gregal's profile |

`GREGAL_BROWSER_HEADLESS=1` forces `headless` whatever the config says.
In the desktop app the bridge is a loopback-only HTTP endpoint served by the
app with a random port and a random token, passed to the backend through its
environment when the app starts it; it only accepts page-level DevTools
commands and never navigates to the backend's own address. A desktop app
attached to a backend it did not start (`GREGAL_URL` or an already running
service) has no bridge, so the agent uses `headless`. The agent pane shares
cookies with the app's Browser page. Reading actions (`open`, `read`,
`screenshot`, `back`) run directly; acting on the page (`click`, `type`,
`press`, `eval`) asks for permission and is never allowed in chat or inspect
mode. Screenshots also appear in the web tool card.

### Auto-review permission mode

In auto-review mode, when a tool call needs approval, a fast reviewer model
decides instead of prompting you. It uses the `reviewer` role, or `chat` if
there is no reviewer role. It receives the task, recent context, the project
rules (`AGENTS.md`, `.gregal/memory.md`) and the exact call: the diff for an
edit or the command for a shell call. It answers `allow`, `deny` or `ask`:

- `allow`: the call runs. The activity log and the tool card show "Reviewer
  approved: reason".
- `deny`: the call is blocked and the model receives the reason.
- `ask`, a timeout (`permissions.review_timeout_s`, default 30 s), an error
  or an unreadable reply: the normal approval prompt appears with the
  reviewer's reason.

The reviewer only sees calls that would otherwise ask. Hard-denied calls,
classifier denials and `permissions.tools: deny` never reach it, so it cannot
approve them. It plays no part in autonomous mode, which never asks. The reviewer is itself a
model and can be wrong or be manipulated by content in the context: it reduces
prompts, it is not a security boundary. Combine it with the sandbox for
unattended work.

Enable it with `permissions.mode: auto-review` in the config (web, terminal,
non-interactive runs and Telegram), from the web permission menu
(**Auto-review**, this session only) or with `/autoreview` in the terminal
interface.

Autonomous mode never asks for permission: it decides and does everything on
its own. Whatever other modes would ask about runs without waiting for
anyone. Only what is forbidden stays blocked: commands the classifier denies
(`sudo`, piping downloads into a shell, formatting disks, deleting the root…)
and `deny` rules in `permissions.tools`. To keep autonomous mode away from
something, set it to `deny`; an `ask` in the config does not make it ask.

### Non-interactive runs (`-p`)

`gregal -p "<task>"` runs one turn without a user interface:

- `--dir <path>` sets the workspace (tools, project map, verification). Without
  it, the current directory is used. In `code` mode, writes inside the
  workspace are allowed without `--auto-approve`; other actions that ask still
  need `--auto-approve`.
- `--max-steps N` (1-200) is a hard limit: when it is reached the agent writes
  a summary of what it did and what is left, and the JSON output has
  `"exhausted": true`. Without the flag, the configured `agent.max_steps`
  applies and the agent may ask for more steps while it makes progress.
- Autonomous mode works on its own and asks nothing: it does whatever the task
  needs, and `question` is disabled (the agent states its assumptions
  instead). Only what is forbidden (classifier and `deny` rules) stays
  blocked. `--mode autonomous` does not need `--auto-approve` and cannot be
  combined with it.
- Tools that need a person (`question`, `preview`, `office_open`, `browser`)
  are not offered. If the agent needs a decision, it stops and asks in its
  final answer instead of guessing.
- `--output json` always prints one JSON object, including `"error"` when the
  run fails. With `--verify`, `"verified"` is always present and the steps,
  tools and trace of every repair round are included.

Exit codes: `0` success, `1` the run failed (provider, configuration or
cancellation), `2` invalid flags (checked before any model call), `3` the
`--verify` command still fails after the last round.

Delegated agents use the active workspace and session, inherit configured
restrictions, and stop when the parent run is canceled. Background commands
started by agent tools belong to the run and stop when it ends or is canceled;
manually started terminal processes remain independent.

Approving a shell command or explicitly allowing a tool grants real access
with the backend user's privileges. Project-local write permissions do not
restrict shell commands, network access, or file reads to that project. Do not
run untrusted repositories or unattended tasks with broad allow overrides on
a machine containing sensitive data; use an isolated environment instead.

## Git: Checkpoints, Worktrees, Commits and Pull Requests

**Checkpoints.** In a git repository, gregal snapshots the whole working tree
(tracked files plus new files that are not ignored) at the start of each turn
and before each shell command that is not read-only. Snapshots are commits on
hidden refs (`refs/gregal/checkpoints/<session>/<n>`) written with a temporary
index: HEAD, your index, branches and the stash are never touched. Undo
(`/rewind`, the web Undo button) now also restores what shell commands changed;
`/checkpoints` lists git snapshots as `gN`, and `/rewind gN` (or
`/api/rewind-to {"git": N}`) goes back to one: files that differ are restored
and files created afterwards that are not ignored are deleted. Ignored files
are never touched. Outside git the change journal works as before. Settings:
`git.checkpoints: on|off` (default on) and `git.checkpoint_keep` (refs kept per
repository, default 50). Snapshots skip huge sets of new files (more than 5000,
or a new file over 50 MB): add them to `.gitignore`.

**Isolated sessions (worktrees).** In the web session tabs, `⎇＋` opens a new
session in `<repo>/.gregal/worktrees/<name>` on a new branch `gregal/<name>`
from HEAD. Files listed in a `.worktreeinclude` file at the repository root
(one glob per line, e.g. `.env`) are copied in. The tab shows a `⎇ name`
badge. On the Changes page, **Bring to main repo…** shows the worktree's diff
and either applies it as a patch to the main working tree (no commit) or merges
the branch (only with a clean main tree and the work committed; conflicts are
aborted). Nothing is forced; removing a worktree with uncommitted changes is
refused. In the terminal: `/worktree new <name>`, `/worktree list`,
`/worktree diff|apply|remove <name>`, `/worktree exit`. Headless and TUI:
`--worktree <name>` (created if needed, reused otherwise).

**Commits and pull requests.** The agent has two tools that always ask for
approval and are denied in chat, inspect and goal modes: `git_commit` (by
default commits only the files changed in this session, shell changes included;
other staged files are left out) and `gh_pr_create` (uses `gh`; on the default
branch it first creates a branch `gregal/<title>`, pushes without force and
opens the PR). The Changes page has **Commit…** and **Create PR…** dialogs with
a **Suggest** button that drafts the message from the diff with the chat role.
A reusable GitHub Action for PR reviews is described in
[github-action.md](github-action.md).

## Debug Mode, Try N Ways, Plan Critic and Confidence Bets

**Debug mode.** Choose **Debug** in the composer's advanced modes, `/mode debug`
in the terminal or `--mode debug` with `-p`. The agent works like code mode
(same write policy) with a debugging contract: it writes 2-4 ranked
hypotheses, adds minimal temporary instrumentation where every line carries
the marker `GREGAL-DEBUG`, runs the reproduction itself when you give a
command or one is obvious (otherwise it asks you to reproduce and stops),
reads the evidence, fixes the root cause, removes all instrumentation and runs
the checks. The final answer lists each hypothesis as confirmed or refuted
with the evidence that decided it. Gregal enforces the clean-up: before the
turn closes it searches the workspace for new `GREGAL-DEBUG` lines and sends
the agent back to remove them (twice at most; then it closes with a warning).
Projects that already contain the marker are not affected: only lines added
during the turn count.

**Try N ways (best-of-N).** In the web `⋯` menu, **Try N ways** takes the task
in the composer, asks for N (2-4) and optionally one model per attempt (a role
name or `provider/model`). Gregal creates N worktrees from HEAD
(`.gregal/worktrees/bestof-<id>-<k>`), runs the same task in each through the
shared run queue, runs the configured verification (`agent.autonomous.verify`)
in each worktree and asks the reviewer role for a 0-10 rubric score. The
comparison card shows files and lines changed, checks, score and cost per
attempt and marks the recommended one. **Apply** brings the chosen attempt to
the main working tree as a patch (no commit) and removes the other worktrees;
**Discard all** removes every attempt. Attempts run non-interactively: actions
that need approval are allowed only while the session is permissive. Headless:
`gregal -p "task" --best-of 3 [--best-of-models code,openai/gpt-x] [--verify "go test ./..."]`
prints the comparison and leaves the worktrees for you to apply or remove;
add `--best-of-apply` to apply the recommended attempt directly. The
recommendation first prefers passing checks, then **cross-tests** (the tests
each attempt wrote are run against the other attempts' code, so a fix that
also passes its rivals' tests wins), then **votes** (attempts that reach the
same patch reinforce each other), then the score, the smaller diff and the
lower cost.

**Plan critic.** After **Plan** proposes a plan, the reviewer role checks it
against the task and the project map and lists missing steps, risky steps and
untested assumptions. The plan card shows **Critic: N concerns** (expandable);
running the plan passes the concerns to the agent, and in autonomous mode the
agent revises the plan once with them before executing. Autonomous runs also
send their first milestone checklist through the critic. Set
`plan.critic: off` to disable it.

**Confidence bets.** Before running a verification command (tests, build,
vet, lint) the agent states the probability that it passes (the `bet`
parameter of `bash`). Gregal compares it with the result, shows a chip on the
tool row (`bet 80% ✓`), adds it to the #retro evidence and stores it in
`<data>/bets.jsonl`. `GET /api/calibration?scope=session|project|all` returns
the calibration summary (Brier score, mean predicted probability, pass rate,
buckets). `agent.bets: off` disables bets; `agent.bets: side` also asks the
chat role for a probability when the model did not give one (one small extra
call per check).

## Lifecycle Hooks

Besides `hooks.post_edit` (a command run after every successful write/edit,
with `{file}` replaced) and `hooks.diag`, the `hooks:` section accepts commands
for six lifecycle events. They run in every client (terminal, web, desktop,
headless, Telegram) because they live in the shared agent engine:

```yaml
hooks:
  post_edit: "gofmt -w {file}"
  session_start:
    - command: ./scripts/context.sh        # stdout is added to the first prompt
  user_prompt:
    - command: ./scripts/check-prompt.sh   # exit 2 blocks the turn
  pre_tool:
    - command: ./scripts/guard.sh
      matcher: "bash|write|edit|patch"    # anchored regex on the tool name
      timeout_s: 10
  post_tool:
    - command: ./scripts/lint-note.sh
      matcher: "write|edit"
  stop:
    - command: ./scripts/tests-green.sh    # can ask the agent to keep working
  pre_compact:
    - command: ./scripts/backup-transcript.sh
  stop_max_continues: 3
```

Each command runs through the system shell in the session workspace, with
`GREGAL_EVENT`, `GREGAL_SESSION`, `GREGAL_WORKSPACE` and `GREGAL_TOOL` in the
environment, and receives a JSON object on stdin: `event`, `session`,
`workspace`, `mode`, and per event `tool` + `args` (pre_tool, post_tool),
`result`/`error` (post_tool), `prompt` (user_prompt, stop), `answer` and
`stop_count` (stop), `messages` and `tokens` (pre_compact).

| Exit / output | Meaning |
|---|---|
| exit 0, plain stdout | Context for the model: appended to the prompt (session_start, user_prompt) or to the tool result as `[hook] …` (post_tool). |
| exit 0, JSON stdout | `{"decision":"allow\|deny\|ask","reason":"…","continue":true,"context":"…"}`. |
| exit 2 | Block: stderr (or stdout) is the reason. For `stop`, exit 2 means "continue". |
| other exit, timeout | Non-blocking warning shown in the activity; the action goes on. |

- `session_start` runs on the first turn of a new conversation.
- `user_prompt` can add context or block the turn (the reason is shown as the
  turn error).
- `pre_tool` runs after the permission policy: `deny` blocks the call (the UI
  shows it as a blocked tool and the model receives the reason), `ask` forces
  an approval even for tools that would run directly (a remembered approval
  does not skip it), and `allow` skips the policy's confirmation. Policy
  denials cannot be overridden by a hook. Subagents apply pre_tool too; there,
  `ask` blocks because nobody can answer.
- `post_tool` can append a note for the model to the tool result.
- `stop` runs before the turn ends with an answer; `{"continue":true,"reason":…}`
  sends the reason back to the agent as a new instruction, at most
  `stop_max_continues` times per turn (default 3). It does not run on forced
  summaries (step budget exhausted).
- `pre_compact` runs before the conversation history is summarized. It cannot
  stop the compaction.

The default timeout is 10 s (`timeout_s`, up to 600). Cancelling the turn (Esc,
Stop) kills the hook and its child processes. Hooks are your own trusted
commands, like `post_edit`: they run with your privileges. In the terminal UI,
`session_start`, `user_prompt`, `pre_tool` and `stop` run synchronously in the
engine, so a slow hook pauses the screen until it ends or times out.

## MCP Connectors

Gregal connects to Model Context Protocol servers in two ways:

```yaml
mcp:
  files:                         # local server over stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/home/me/notes"]
  linear:                        # remote server: Streamable HTTP
    url: https://mcp.linear.app/mcp
  internal:
    url: https://mcp.example.com/sse
    transport: sse               # legacy HTTP+SSE (2024-11-05)
    headers: {X-Api-Key: "${INTERNAL_MCP_KEY}"}
  github:
    url: https://api.example.com/mcp
    oauth: {client_id: my-client-id, scopes: [repo]}
```

- `transport` is optional: a `command` means stdio and a `url` means Streamable
  HTTP; if a server answers the first POST with 400/404/405 Gregal falls back to
  the legacy SSE transport. Header values may use `${VAR}`; they are expanded
  when connecting and saved unexpanded.
- **OAuth 2.1.** When a remote server answers 401, Gregal discovers its
  authorization server (protected resource metadata, then authorization server
  metadata), registers itself dynamically if the server offers it (otherwise set
  `oauth.client_id`), and uses the authorization code flow with PKCE (S256) and
  the `resource` parameter. Tokens are stored in
  `~/.local/share/gregal/mcp-oauth.json` (or `$GREGAL_DATA_DIR`) with 0600
  permissions and are refreshed automatically. Startup never opens a browser:
  a server that needs authorization shows **Needs authorization**.
  - Web and desktop: **Connectors / MCP** page → **Connect**. The authorization
    page opens in your browser and returns to `/api/mcp/oauth/callback` on the
    local server; the page refreshes when the server is connected.
  - Terminal: `/mcp connect <name>` prints the URL to open; a temporary
    listener on `127.0.0.1` receives the callback. `/mcp disconnect <name>`
    forgets the token.
- **Tools** appear as `mcp_<server>_<tool>` and ask for confirmation by default
  (`permissions.tools` can change that). A server marked `readonly: true`
  declares that its tools only read: they run without a prompt, also in the
  read-only modes (chat, goal, inspect) and in autonomous mode. An explicit
  `permissions.tools` entry still wins. Only set it for servers you trust to
  be read-only.
- **Resources.** If a connected server exposes resources, the agent gets
  `mcp_resource_list` (optional `server`) and `mcp_resource_read`
  (`server`, `uri`).
- **Prompts** become slash commands `/mcp__<server>__<prompt>` in the terminal
  (with completion) and in the web command palette. In the terminal, arguments
  go after the command, positionally or as `key=value`; the web palette asks for
  each argument and puts the expanded prompt in the composer for review.
- The **Connectors / MCP** page shows each server's transport, status,
  authorization state and number of tools and prompts, with Connect/Reconnect,
  Disconnect and Remove buttons and a form to add a server (URL or local
  command). Adding and removing write `config.yaml` and are restricted to the
  administrator when the server has users.

## Gregal as an MCP Server

`gregal mcp serve` runs Gregal as an MCP server over stdio, so another MCP
client can hand it work. It exposes three tools:

- `gregal_run` — `task` (required), `mode`, `workspace` (absolute path; default
  the directory where the server started or `--workspace`), `auto_approve`,
  `max_steps`, `verify`. It runs a headless turn (like `gregal -p`) and returns
  JSON with `answer`, `changed_files` (from `git status` before and after; from
  the write tools outside git), `steps`, `tools_used`, `exhausted`, `verified`
  and `session` (the run is saved as a conversation). Runs are serialized; an
  MCP cancellation stops the turn.
- `gregal_sessions_list` — saved conversations (`limit`, `workspace`).
- `gregal_session_get` — one conversation (`name`, `max_messages`).

Add it to Claude Code:

```sh
claude mcp add gregal -- gregal mcp serve --workspace /path/to/project
```

Other clients (Cursor, Claude Desktop, another Gregal) use the usual JSON:

```json
{"mcpServers": {"gregal": {"command": "gregal", "args": ["mcp", "serve", "--config", "/home/me/.config/gregal/config.yaml"]}}}
```

or, in Gregal's own `config.yaml`, `mcp: {gregal: {command: gregal, args: [mcp, serve]}}`.
Without `auto_approve`, actions that need approval are denied, exactly as in
`gregal -p`; `autonomous` mode cannot be combined with `auto_approve`. The
protocol uses stdout; progress goes to stderr.

## Demo database

Gregal ships a tiny, deterministic demo database so you can try MCP tools,
queries and dashboards without connecting anything of your own. It describes a
fictional European retailer, Northwind Outfitters, with dates anchored to
2026-10-01 (orders cover the previous 24 months), so results are always the
same:

| Table | Rows | Contents |
|---|---|---|
| `regions` | 8 | name, country, manager |
| `customers` | 800 | segment (Consumer, SMB, Enterprise), region, signup date |
| `products` | 60 | category, price, cost, launch date |
| `orders` | 6000 | date, customer, product, channel, status, promo, quantity, discount, revenue, cost, profit (seasonality, growth and promotions included) |
| `marketing_spend` | 120 | monthly spend, impressions, clicks and conversions per channel |
| `support_tickets` | 1500 | priority, status, category, resolution hours, CSAT |

`gregal mcp demo` serves it as a read-only MCP server over stdio. It is
generated in memory at startup (a few MB, a few milliseconds) and touches no
files. Tools:

- `list_tables` — tables with row counts, descriptions and joins.
- `describe_table` (`table`) — columns, types, joinable tables and five
  sample rows.
- `query` — a structured query, not SQL: `table`; `select` with columns or
  aggregates `sum`, `avg`, `min`, `max`, `count(*)`, `count(col)`,
  `count_distinct(col)`, each optionally `… as name`; `filters`
  `[{col, op, value}]` with `=`, `!=`, `>`, `>=`, `<`, `<=`, `in`, `not_in`,
  `between`, `contains`; `group_by` with columns or date buckets `day()`,
  `week()`, `month()`, `quarter()`, `year()`; `order_by` like `"revenue desc"`;
  `limit` (default 100, max 200). Columns of related tables use dots
  (`customers.segment`, `products.category`, `regions.name`) along a fixed set
  of joins: orders and support tickets → customers and products, customers →
  regions. Results are JSON `{columns, rows, total_rows, truncated}`.

It also exposes the resource `demo://schema` (the schema in Markdown) and the
prompts `/mcp__demo__sales_dashboard` and `/mcp__demo__support_kpis`.

In the web and desktop app, **Connectors / MCP → Add demo database** adds it in
one click (command: the running Gregal executable with `mcp demo`,
`readonly: true`), and the home page card **Try the demo data** (also offered
in an empty **Plots & dashboard** library) adds it if needed and asks the agent
for a sales dashboard: monthly revenue, revenue by category and by region, and
support tickets by priority as `gregal-plot` charts. Save them with **Add to
dashboard**; a dashboard named "Demo sales" keeps them together. Remove the
connector like any other. In `config.yaml` it looks like:

```yaml
mcp:
  demo:
    command: /path/to/gregal
    args: [mcp, demo]
    readonly: true
```

## Subagents, Team Tools, Side Questions and Memory

### User-defined subagents

Put Markdown files with YAML front matter in `.gregal/agents/*.md` (project)
or `~/.config/gregal/agents/*.md` (user; the project wins on name clashes):

```markdown
---
name: test-runner
description: Runs the test suite and reports failures with file and line
tools: [read, grep, glob, bash]
mode: inspect        # inspect (default, read-only) or code (may write)
max_steps: 15        # default 12, max 40
model: openrouter/qwen3-coder   # optional: provider/model or just a model
# role: chat         # optional: use a configured role instead
---
Run the narrowest test command first. Report only failures.
```

The agent sees the list (name and description) in its system prompt and can
pass `agent: test-runner` in a `delegate` task. Code-mode subagents write only
when the session works in an isolated worktree (`--worktree`, Changes page) or
when you approve each change (the usual approval card in the web); otherwise
they stay read-only. In the terminal there is no approval for subagents, so
outside a worktree they are read-only. Caps stay at 4 per call and 8 alive.
`/agents` in the terminal and the Agents page (Team run tab, "Defined agents")
list them; the workshop bays show the subagent name.

### Team runs with tools

On the Agents page, Team run tab, "Roles use tools" (on by default, remembered
by the browser) lets the roles work on the real project: the researcher reads
files and the web, the reviewer also runs read-only checks (inspect-mode
`bash`), and the builder edits code with your session permission policy.
Actions that need permission appear as an approval card on the Team page.
The coordinator stays tool-free. A flow node can name a defined subagent
("Subagent" field in the flow editor) to use its model, tools and prompt.

### Side questions and recaps

While a turn runs, type `/btw <question>` in the composer (web or terminal):
the chat role answers from a copy of the transcript and live trace in a small
aside card. The main run is not interrupted and the question is not added to
the history. `/recap [N]` summarizes the last N requests (default 3). In the
web, when you come back to a session with activity you have not seen, a
"Recap" button summarizes what happened since you left.

### Project memory

After a code turn that applied edits, Gregal may propose up to two durable
project notes ("Remember this?" card in the web; `/keep [n|all|none]` in the
terminal). Only accepted notes are written to `.gregal/memory.md`.
Preferences > Project memory lists the notes and "Tidy notes" proposes a
consolidated list (merged duplicates, stale notes dropped) and shows the diff
before writing. From the command line: `gregal memory tidy [--dry-run]
[--yes] [--dir path]` shows the diff and asks before writing.

## Office Documents

Gregal creates, edits and reviews Word, PowerPoint and Excel files
(`.docx`, `.pptx`, `.xlsx` and the templates `.dotx`, `.potx`, `.xltx`)
without Python or Office installed:

- **Word**: markdown becomes real Word structure (Heading styles, numbered
  and bullet lists, tables with header rows, quotes, code, links, images,
  table of contents, page breaks), with a cover, header and page numbers.
  Editing keeps the formatting of every fragment, also in headers, footers,
  footnotes and comments. Changes can be **tracked** (`track: true`; only
  the words that change are marked) and annotated with **comments**.
- **PowerPoint**: a built-in 16:9 theme with nine layouts (title, section,
  content, two columns, comparison, big number, picture, title only, blank),
  native editable charts, styled tables and speaker notes. Five themes
  (`default`, `modern`, `nature`, `elegant`, `mono`) with colour overrides.
- **Excel** (through excelize): formulas rather than values, number formats,
  colour roles (input, formula, link, key assumption, total), frozen headers,
  tables, conditional formats, drop-down validation, comments and native
  charts. `recalc` lists every formula error.
- **Templates**: a corporate `.dotx`/`.docx` keeps its styles, header, footer
  and margins, and `{{markers}}` are filled from `vars`; a `.potx`/`.pptx`
  lends its layouts and colours (`office_read detail=layout` lists them).
- **Visual review**: `office_render` turns the document into images
  (LibreOffice, or Word/PowerPoint on Windows) and runs automatic checks for
  overflowing text, overlaps, shapes off the slide, leftover sample text and
  formula errors. The agent is told to render and fix before delivering.
  Without LibreOffice or Microsoft Office only the structural checks run;
  install [LibreOffice](https://www.libreoffice.org) for the visual review.
- **Safety**: every write goes to the journal (`/rewind` undoes it) and an
  edit that would leave a package structurally broken is refused.

Built-in skills `office-docx`, `office-pptx` and `office-xlsx` carry the
formatting and design rules the agent follows.

## Knowledge Collections

The **Knowledge** page (sidebar, after Files) holds searchable collections of
documents and notes that the agent consults with the `knowledge_search` tool
and cites as `path · location`. Each collection has a scope:

- **Global**: available in every project. Stored under
  `~/.local/share/gregal/knowledge/` (or `GREGAL_DATA_DIR`).
- **This project**: stored in `.gregal/knowledge/` inside the project, so the
  definition (name, description, sources, notes) travels with the repository.
  Sources inside the project are saved as relative paths; the index files are
  git-ignored automatically and rebuilt on each machine.
- **This session**: only for the current conversation (in the terminal UI,
  for the terminal session).

Create a collection with a name, a description (it helps the agent decide when
to search it) and a scope, then link folders or files (Browse uses the system
dialog in the desktop app and the folder picker in the browser) with optional
include/exclude globs such as `*.md, docs/**` or `**/*.pdf`. Gregal reads
Markdown, plain text and code, HTML (main content), PDF (text layer),
`.docx`, `.xlsx` and `.pptx`; images, archives and binaries are skipped, and
legacy `.doc/.xls/.ppt`, encrypted PDFs and scanned PDFs without a text layer
are listed as unsupported or without text. Folders such as `.git`,
`node_modules`, `vendor`, `dist` and hidden folders are not indexed.

Documents are split into overlapping chunks of about 1,500 characters that
remember their location: heading path and line range, PDF page, sheet or
slide. Indexing runs in the background with a progress bar and is incremental:
**Reindex** only re-reads new or changed files (by size and modification time)
and drops deleted ones; **Full reindex** re-reads everything. Each collection
lists its documents with their status, chunk count and last indexing time, and
can be disabled (kept, but neither searched nor shown to the agent) or
deleted (the index goes; source files are never touched). You can also add
short notes to any collection.

The search box at the top tests retrieval across all enabled collections or
one of them. Ranking is BM25 over words, accent- and case-insensitive, and
works offline. For semantic search, configure an OpenAI-compatible embeddings
model; vectors are stored with the index and results combine both rankings:

```yaml
knowledge:
  embeddings: {provider: openai, model: text-embedding-3-small}
  max_files: 2000     # per collection
  max_mb: 200         # read per collection
  max_file_mb: 25     # larger files are skipped
```

The system prompt lists the enabled collections (name, scope and description),
so the agent knows when to search them and is asked to cite the passages it
uses. `knowledge_search` is allowed in every mode. `knowledge_note` (the agent
saving a note) asks for approval, is denied in Chat, Inspect and Goal modes,
and runs without asking in Autonomous mode (like everything else there);
without a collection name it goes to the session's "Session
notes".

The page also shows the built-in memories: the **user memory**
(`~/.config/gregal/memory.md`, `/remember`) and the **project notes**
(`.gregal/memory.md`, `/nota`) can be edited and saved; if the file changed
meanwhile, Gregal reloads it and shows your version instead of overwriting.
`AGENTS.md`/`CLAUDE.md` are shown read-only. In the terminal UI,
`/knowledge` lists the collections and `/knowledge search <query>` tests a
search. On servers with configured users, only administrators change global
collections and the user memory, and source folders must be inside the user's
roots. The HTTP API is documented in `docs/api-contract.md`.

## References

### Website Prototypes


In Team, choose Create a website, adapt the task, configure an active model,
then start the team. Roles collaborate on text and code without tool access.
Select the builder or reviewer to open a complete HTML deliverable in the side
preview or download it as `website.html`. Partial documents are not offered
as previews. The local demo is scripted and does not use a provider.

The HTML preview runs in a sandboxed frame without same-origin access, with
external resources, fetch requests and form submissions restricted by CSP.
It is not a full browser sandbox or a deployment. Review generated code before
opening downloaded HTML outside the preview or publishing it.

- [Getting Started](getting-started.md)
- [HTTP API contract](api-contract.md)
- [Compatibility notes](compatibility.md)
- [Security policy](../SECURITY.md)
- [Contributing](../CONTRIBUTING.md)
## Model Request Time Limits

Autonomous model steps default to a 20-minute time limit; interactive steps
default to four minutes. Set `agent.model_timeout_s` to a value from 1 to 1200
seconds to override either default, or use `0` for automatic selection.
Retries share the same step deadline. Cancellation still stops the request.
Autonomous requests are also bounded by the remaining `max_minutes` budget;
when that budget expires, pending tools are discarded and a final summary is
requested separately, with its existing bounded time limit. A timed-out step
is not automatically replayed.

## Chat plots and dashboard

Ask Gregal for a chart from your project data. Fenced `gregal-plot` blocks
render inline as line, bar, or scatter charts with a data table and PNG/JSON
downloads. Use **Save plot** or **Add to dashboard** to keep a snapshot.

Open **Plots & dashboard** in the sidebar to browse the project's library
or dashboard. Import JSON, edit data and titles, add notes, change card width,
reorder cards, and delete plots. Libraries persist on the server per user
and project across restarts. Saved snapshots do not refresh automatically.

Create and name multiple dashboards with **New dashboard**, then use each
plot's **Move to dashboard** selector to assign it. Deleting a dashboard keeps
its plots in the library. Search by title, note or source session and use
**Export collection JSON** to download a copy of all dashboards and plots.

## Project Map and Symbol Search

In code, autonomous, inspect and goal modes, each turn starts with a project
map attached to your message: the file list, the git status and any small file
the task names. For Go, JavaScript/TypeScript, Python, Java, C# and Rust
projects the map also carries a compact **symbol section**: the top-level
functions, methods, types, classes, interfaces, enums, traits and constants of
the files most relevant to the task, one line per file
(`path: Name(f) Type.Method(m) …`). Relevance comes from the identifiers in the
task (split on camelCase and snake_case) matched against symbol names and file
paths, plus a boost for files you are changing (git status) or changed
recently. The section stays within about 1,200 tokens; set
`GREGAL_REPOMAP_TOKENS` to change the budget or to `0` to turn it off.

The symbol index is built with Go's own parser for Go and with lightweight
pattern matching for the other languages. It follows the same file rules as
the map (git's file list when available, no `vendor`/`node_modules`, files up to
256 KB), lives in memory per workspace and only re-reads files whose size or
modification time changed. On very large repositories the first build stops
after 300 ms and the map says the index is incomplete; later turns continue
where it stopped. The map is only repeated when the file list or git status
changes, or when the task names a symbol literally (for example
`MapaProjecte`): then only the relevant symbols are attached.

The agent also has a read-only `symbols` tool, allowed in every mode like
`grep` and `glob`. It searches the same index by name (`query`, a
case-insensitive substring such as `Decide` or `Policy.Decide`, or a regular
expression with `regex: true`), `kind` (`func`, `method`, `type`, `struct`,
`interface`, `class`, `enum`, `trait`, `impl`, `const`, `var`), `file` (a glob
such as `internal/**/*.go`), `exported` and `limit` (default 50, maximum
200), and returns `path:line kind name — signature`.

## Export and Share Conversations

In the web and desktop apps, **⋯ → Export** (current conversation) or the
**⤓** button on a row of the conversation list (saved conversation) opens the
export menu:

- **Markdown (.md)** or **Self-contained HTML (.html)**: user messages, the
  assistant's work groups (collapsed, with each tool call's name, arguments and
  truncated result) and the final answers. The project map and other internal
  context (system prompt, hook context, compaction summaries, internal
  guidance) are never included. The HTML file has inline CSS and no scripts or
  external assets.
- **Hide keys, secrets and home paths** (on by default) replaces API keys and
  tokens (`sk-…`, `ghp_…`, `xoxb-…`, `AKIA…`, JWTs, `Bearer …`, `api_key=…`,
  PEM private keys), the secret values from your config (provider keys, the
  Telegram token, MCP env/headers) and secret-named environment variables with
  `[REDACTED]`, and home directories with `~`. **Also hide e-mail addresses**
  replaces addresses with `[email]`.
- **Create share link** stores a read-only, always-sanitized HTML snapshot
  under `<data dir>/shares/` and copies its `/share/<id>` URL. The id is
  random (160 bits) and is the only key: anyone who has the link and can reach
  your Gregal server can open it, without the API token. Pages are served with
  a script-free Content-Security-Policy. **Preferences → Shared links** lists
  your links and revokes them (a revoked link returns 404).

In the terminal, `/export` writes the current conversation to a Markdown file
in the working directory and prints the path. `/export html` writes HTML,
`/export md notes/chat.md` chooses the path, `--redact` sanitizes as above and
`--emails` also hides e-mail addresses.
