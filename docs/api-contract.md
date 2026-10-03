# HTTP Backend Contract

All clients (web, desktop, Android and VS Code) use `gregal serve`. Responses
are JSON unless marked **SSE** (`event:` plus `data:` containing JSON). When a
server token is configured, API routes require
`Authorization: Bearer <token>`, except `/api/login`; `/` and `/app/*` (the UI)
do not require it.

This document is executable: `TestContracteAPIDocumentat`
(`internal/web/contract_test.go`) fails if a registered `/api/*` route is not
documented here.

## Team Collaboration

`GET /api/providers` includes a `catalog` of public provider presets with
`name`, `label`, `url`, `env_var`, `default_model`, `local` and `note`.
`POST /api/providers` with `{action:"preset",name}` selects that hosted
provider/model for all role defaults without changing existing custom URLs
or exposing API keys. Literal keys remain memory-only; environment references
are persisted. See [provider presets](provider-presets.md).

`POST /api/v2/team/run` accepts `{task,lang?}` (`en` by default, or `ca`).
It uses the selected session and active model, with four tool-free roles.
The coordinator plans first; researcher analysis and an independent builder
draft run in parallel from that plan. The reviewer receives both deliverables
in deterministic order. A structured rejection can request one builder revision
and one final review, never an unbounded debate. Roles share public
deliverables, not hidden reasoning. This implementation cannot browse,
read files, execute code or create plots. It shares the session busy guard.
The response is SSE: `event: team`, JSON
`{type,agent?,to?,output?,activity?,message?,model?}`. Before the first response,
`model` names the configured model; completed events identify the model used,
including fallback routing. This is not an intelligence score.
Types: `started`, `working`, `completed`, `handoff`, `discussion`, `done`, `failed`, `cancelled`.
Only `completed` and `done` include deliverables. Closing/aborting the HTTP
request cancels work; there is a ten-minute timeout. Tasks are limited to 16 KB
and request bodies to 64 KB. Runs are not persisted or resumable yet.

The browser's local demo uses scripted events, including simulated tools and
delegated characters. It makes no model or web-search requests. Real Team runs
do not yet connect to the main agent's tool or delegation loop.

## Language

| Route | Method | Request and response |
|---|---|---|
| `/api/lang` | POST | `{lang}` (`ca` or `en`): sets the UI and agent-response language. Stored in config; application-wide, not per session. |

## Sessions

Before version 1.1, the server had one conversation and one active turn; a
second `POST /api/agent` returned 409. Since 1.1, each client can open multiple
sessions and work in them in parallel.

- Select a session with the `X-Gregal-Session: <id>` header or `?session=<id>`.
  An ID must match `[A-Za-z0-9_-]{1,64}`.
- Without an ID, requests use the `default` session, preserving the previous
  behavior for older clients.
- Each session has its own conversation, mode, role, permissions, workspace
  and processes. Providers, models, jobs and Office state are application-wide
  and shared.

| Route | Method | Behavior |
|---|---|---|
| `/api/sessions/live` | GET | `{sessions:[{id,title,msgs,busy,mode,role,cwd,project,created_at}], current}` |
| `/api/sessions/open` | POST | `{id?,title?,cwd?}` → `{id,title,cwd,project}`; generates an ID when omitted. |
| `/api/sessions/close` | POST | `{id}` → `{ok,closed}`; stops that session's processes. The `default` session cannot be closed (400). |
| `/api/agent/cancel` | POST | Stops the active turn in the selected session → `{ok,cancelled}`. |
| `/api/md` | POST | `{text,tema}` → `{html}`: renders response Markdown on the server (goldmark + Chroma with the `internal/tema` palette) and sanitizes it with bluemonday. The client requests it after a turn; while text is arriving, `app/md.js` renders it without a request per fragment. |

## Users and Authentication

Without user accounts, the server uses a single token and its holder has
administrator access. With `users:` in the config, each user has an account
(passwords use PBKDF2-HMAC-SHA256 and are never stored in plaintext) and
workspace roots.

- A client obtains a token from `/api/login` and sends it in
  `Authorization: Bearer <token>`. The existing admin token (`--token` /
  `GREGAL_API_TOKEN`) remains supported for older clients.
- Live and saved conversations are scoped to the authenticated user
  (`sessions/<user>/`); users cannot view or resume another user's sessions.
- File routes (tree, files, diffs, directories, execution and Office) are
  restricted to the user's configured `roots`; paths outside them return 403.
- With no users and no token configured, the server remains open for local use.

| Route | Method | Request and response |
|---|---|---|
| `/api/login` | POST | `{user,password}` → `{token,user,roots,admin}`. The only `/api` route that does not require a token. Limited to 8 failed attempts per IP address every 5 minutes. |
| `/api/logout` | POST | Invalidates the caller's token → `{ok}`. |
| `/api/me` | GET | `{user,roots,admin,usuaris,local}`: identity and allowed workspaces. |

## State and Discovery

| Route | Method | Response |
|---|---|---|
| `/api/state` | GET | `{role:[], current_role, mode, model, provider, verify, agent_busy, cwd, project, branch, permissive, used_tokens, context_window}`. `used_tokens` estimates conversation context usage; `context_window` is 0 when the role does not declare it. |
| `/api/health` | GET | `{status, protocol, instance_id, capabilities}` for discovering and validating the service instance. `capabilities.interactive_events` is `true` only when durable storage and payload serialization are available. |
| `/api/active` | GET | `{active:{id,task,started_at,role,mode,project,events[]}}` for the active turn. |
| `/api/v2/events` | GET | `?after=<cursor>&limit=1..500&session=<id>` → `{events:[{id,at,session_id,run_id,kind,text,payload?}],cursor,next}`; durable reads can resume after a disconnect. |
| `/api/v2/events/stream` | GET **SSE** | `?after=<cursor>&limit=1..500&session=<id>` → streams subsequent events and stays open; also accepts `Last-Event-ID`. Each message has `id: <cursor>`, `event: <kind>` and the same event JSON (including `payload` when present); sends `: heartbeat` every 15 seconds. |
| `/api/v2/runs` | POST | `{task, images?, mode?, idempotency_key?, workspace?}` → 202 `{run:{id,state,session,workspace,...},cursor}` when queued. `workspace` is the absolute directory the turn runs in (tools, map, verify); empty = the session cwd. It must exist and pass the user's guard (400 if relative/missing, 403 outside their roots). `cursor` is the last event before this run, so a client can follow its stream without replaying older history. Returns 200 with an existing run if the idempotency key was already used; 423 if strict review has blocked; 429 if the session queue is full. |
| `/api/v2/runs` | GET | `{runs:[...]}`: runs in the session, newest first. |
| `/api/v2/runs/{id}` | GET | `{run}` with state and timestamps; 404 if it does not exist or belongs to another session. |
| `/api/v2/runs/{id}/cancel` | POST | Cancels the identified turn: a queued run will not execute; a running turn has its context cancelled → `{ok,cancelled}`. Returns 404 outside the session. |
| `/api/models` | GET | `{models:{provider:[]}, errors:{}, current, role}`. |
| `/api/sessions` | GET/DELETE | GET: saved conversations on disk, newest first: `[{name,title,msgs,when,at,current,pinned,workspace}]` (`title` comes from the first message; `at` is RFC3339 for date grouping; `current` marks the conversation currently being written). DELETE `?name=` removes one. Saved automatically after each turn. |
| `/api/sessions/pin` | POST | `{name,pinned}` updates the navigation pin without cloning the conversation and returns `{name,pinned}`. |
| `/api/goal` | GET | `{goals:[{id,title,body,status,project}]}`. |
| `/api/checkpoints` | GET | `[{seq,op,path,at}]` (filtered to the workspace when multiple sessions are live). |
| `/api/providers` | GET | `{providers:[{name,url,key,used_by}], roles}`. |
| `/api/github` | GET | `{available, cwd}`. |
| `/api/jobs` | GET | Scheduled jobs. |
| `/api/jobs/runs` | GET | `?id=` returns run history. |
| `/api/mcp` | GET | `{summary,servers:[{name,status,error?}],tools:[name]}`: safe diagnostics for MCP connectors; does not expose commands or secrets. |

## Turn Execution

| Route | Method | Notes |
|---|---|---|
| `/api/chat` | POST **SSE** | `{message}` → `token`, `done`, `verify`, `error`. |
| `/api/agent` | POST **SSE** | `{task, images?, mode?}` → see the event table below. `mode: chat|inspect` only restricts the turn (the Office add-in uses `chat` for read-only requests). If the session is already working, the request waits in the shared turn queue with SSE heartbeats; it does not return 409. |
| `/api/parallel` | POST | `{message}` → `{results:[{role,provider,model,reply?,error?}]}`, up to 4 roles. |
| `/api/plan` | POST | Read-only exploration that proposes a plan. |
| `/api/goal` | POST | Creates, runs or deletes goals. |
| `/api/approve` | POST | `{key, approve, remember?}` → `{ok, note?}`. |
| `/api/question` | POST | Answers a selectable question: `{key, answer}` (label or `text:<free text>`) → `{ok}`. |
| `/api/verify` | POST | `{mode}`: `off`, `manual`, `auto`, `both` or `strict`. |
| `/api/verify-approve` | POST | Clears a `strict` review block. |
| `/api/permissive` | GET/POST | `{on}`: lets the agent proceed without asking; `deny` rules still block. |
| `/api/mode`, `/api/role`, `/api/model` | POST | Changes the active mode, role or model (per session). |
| `/api/save`, `/api/new`, `/api/resume` | POST | Operates on conversations saved to disk. |
| `/api/rewind`, `/api/rewind-to` | POST | Reverts file changes → `{summary}`. |
| `/api/providers`, `/api/github` | POST | Adds/tests providers; `gh` remains read-only. |
| `/api/office/upload` | POST | Uploads a docx/xlsx/pptx as base64 → `{id, kind, name, size, path}`; `path` is the disk path for giving the document to the agent with `@path`. |
| `/api/office/read` | POST | Reads text or data from an uploaded document. |
| `/api/office/edit` | POST | `set_cell` (xlsx) or `replace` (docx/pptx); records the change in the journal. |
| `/api/office/download` | POST | Returns the edited file. |
| `/api/office/open` | POST | `{id}` → opens the document with the server's associated application (local/desktop only). |
| `/api/jobs/save` | POST | Creates or updates a scheduled job. |
| `/api/jobs/delete` | POST | Deletes a scheduled job. |
| `/api/jobs/run` | POST | Runs a job immediately. |

### `/api/agent` SSE Events

`route` (the router changed roles) · `thinking` (reasoning accompanying a
tool call) · `tool_call` · `approve_request` · `approve_timeout` (no response
within 120 seconds means denied) · `question_request` (selectable question:
`{key, query, options:[{label, description}]}`; answer with `/api/question`) ·
`question_timeout` (no response within 120 seconds means the agent continues
without it) · `tool_result` · `blocked` · `assistant` · `fallback` (the
secondary model responded) · `escalate` · `budget` · `gated` (strict review
required) · `steps_exhausted` (the `agent.max_steps` limit was reached; the
following response is a summary, not completed work) · `verify_start` ·
`verify` · `done` · `error`. `/api/chat` emits a subset (`token`, `done`,
`verify`, `error`).

Durable interactive events `approve_request` and `question_request` include an
optional JSON `payload` so a reconnecting client can restore the interaction.
The approval payload contains only `{key,call_id,name,args}`; the question
payload contains `{key,call_id,query,options}`. Older events have only `text`
and no `payload`.

## Shared Run Queue

All turns (web, delegated TUI and API v2) use the same service queue. A session
can run at most one turn at a time; additional messages are queued (up to 8
live entries per session). Turns sharing a workspace run serially. Interactive
work takes priority over unattended work. Run states are `queued`, `running`,
`completed`, `failed` and `cancelled`.

The lifecycle is recorded durably with `run_queued`, `run_started`,
`run_completed`, `run_failed` and `run_cancelled` events, each with its
`run_id`. A client can follow them through `/api/v2/events` without duplicates
or reruns. Idempotency keys prevent retrying the same message from creating a
second run while the first is active. Closing a client does not stop the run;
it continues on the service and can be observed or cancelled by ID. The TUI
can delegate turns here with `/connect` and display their events.

## Workspaces and Files

The working directory is selected per session rather than fixed at startup.

| Route | Method | Notes |
|---|---|---|
| `/api/workspaces` | GET/POST | GET: `{current, project, branch, workspaces:[{path,name,last}]}`. POST `{path}` changes it for the session. |
| `/api/dirs` | GET | `?path=` (defaults to the session workspace) → `{path, name, git, parent, home, roots, dirs:[{name,path,git}], hidden, truncat}`; directory picker for project selection, names only. |
| `/api/tree` | GET | `?path=&depth=1..6` → `{path, root, project, entries:[{name,path,dir,size,status,children}], truncated}`. Skips `.git`, `node_modules`, `dist`, `target`, etc. |
| `/api/file` | GET | `?path=` → `{path,size,content,lines}`; returns `{binary:true}` or `{too_big:true}` when it cannot display the file. Maximum 2 MB. |
| `/api/diff` | GET | Optional `?path=` → `{files:[{path,status,binary,untracked,added,removed,hunks:[{index,header,lines:[{kind,text,old,new}],added,removed,patch}]}], added, removed, branch, project}`. |
| `/api/diff/discard` | POST | `{path, hunk?}`: without `hunk`, discards the whole file; a new file is deleted. Returns 409 if the patch no longer applies. |

All file routes are relative to the workspace and reject `..`, absolute paths
and `~` (400).

## Background Processes

The agent's `bash` command has a two-minute limit. Longer processes (such as a
server, build or test suite) run here and can outlive the turn; the agent uses
`bash_background`, `bash_output` and `bash_kill` for them.

| Route | Method | Notes |
|---|---|---|
| `/api/exec` | GET | `{procs:[{id,cmd,dir,session,started,running,exit,lines}]}` for the session. |
| `/api/exec` | POST | `{cmd}` → `{id,cmd,dir,running}`. Applies the same policy as the agent's `bash`; a denied command does not start (403). |
| `/api/exec/output` | GET | `?id=&from=` → `{lines:[{n,stream,text}], done, exit, running}`. |
| `/api/exec/stream` | GET **SSE** | `?id=&from=` → `line`, `end`. |
| `/api/exec/kill` | POST | `{id}` → `{ok,killed}`; stops the entire process group. |

## Compatibility

Clients may ignore new fields; existing required fields are not reused with a
different type. Images are `image/png`, `image/jpeg`, `image/webp` or
`image/gif` data URLs, up to 3 per turn and 8 MB each.

### Known Limits

- The change journal (`/rewind`) is process-wide, not session-scoped. With
  multiple live sessions, `rewind`, `rewind-to` and `checkpoints` are filtered
  to the requesting session's workspace; with one session, behavior is
  unchanged.
- `/api/office/*` and `/api/jobs/*` are application-wide and are not scoped by
  the session ID.

## Graphs

A **graph** is a saved procedure: its steps, their order, and the branch taken
based on each result. Graphs are stored under `.gregal/flows/*.json` in the
project, not per user, because procedures belong to the repository. These
routes are scoped to the session, so changing workspaces changes the available
graphs.

| Route | Method | Notes |
|---|---|---|
| `/api/flows` | GET | `{flows:[{name,slug,desc,steps}], dir}`, alphabetically ordered. |
| `/api/flows/load` | GET | `?name=` → `{flow}`; 404 when absent. |
| `/api/flows/save` | POST | `{flow, rename?}` → `{ok,path,slug}`. Validates before writing; an invalid graph returns 400 with the reason. `rename` is the previous name and is removed when the name changes. |
| `/api/flows/delete` | POST | `{name}` → `{ok}`. |
| `/api/flows/run` | GET **SSE** | `?name=&auto=1&input=` → `start`, `step`, `done`. Returns 409 if an agent turn, job or another graph is already running; they share a worker. |

Events from `/api/flows/run`:

- `start` → `{flow, steps, auto}`
- `step` → `{node, kind, title, input, output, error, ms}` after each step
- `done` → `{steps, stopped, error?, answer?}`

`auto=1` allows tools marked as requiring permission to proceed; a graph then
runs unattended and no one is available to answer the prompt. Explicitly
denied tools remain blocked. Closing the `EventSource` stops the graph; no
separate stop route is needed.

When a graph finishes, its summary is added to the session conversation so it
can be discussed in a follow-up turn.
