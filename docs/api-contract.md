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
`name`, `label`, `url`, `env_var`, `default_model`, `local`, `note` and,
for native protocols, `type` (`anthropic` or `openai-responses`).
`POST /api/providers` with `{action:"preset",name}` selects that hosted
provider/model for all role defaults without changing existing custom URLs
or exposing API keys. Literal keys remain memory-only; environment references
are persisted. See [provider presets](provider-presets.md).

`POST /api/v2/team/run` accepts `{task,lang?,flow?,tools?}` (`en` by default, or `ca`).
It uses the selected session and active model, with four roles. Without
`tools` (or `tools:false`) the roles are tool-free, as before.
The coordinator plans first; researcher analysis and an independent builder
draft run in parallel from that plan. The reviewer receives both deliverables
in deterministic order. A structured rejection can request one builder revision
and one final review, never an unbounded debate. Roles share public
deliverables, not hidden reasoning. This implementation cannot browse,
read files, execute code or create plots unless `tools:true`. With
`tools:true` each role runs on the shared agent engine in the session
workspace: the coordinator stays tool-free; the researcher gets read tools
(read, grep, glob, read_image, web_search, web_fetch, office_read, skill);
the reviewer gets the same plus `bash` limited to read-only commands
(inspect mode); the builder gets code tools under the session permission
policy (writes inside the project pass, anything that asks emits an
`approve_request` event on the same stream, answered with `POST /api/approve`;
permissive mode and remembered approvals apply). Tool use streams as team
events of type `tool` (`activity` = tool name, `message` = argument preview)
and `approval` (just before the matching `approve_request`). It shares the
session busy guard.
The response is SSE: `event: team`, JSON
`{type,agent?,to?,output?,activity?,message?,model?}`. Before the first response,
`model` names the configured model; completed events identify the model used,
including fallback routing. This is not an intelligence score.
Types: `started`, `working`, `completed`, `handoff`, `discussion`, `done`, `failed`, `cancelled`,
and with tools also `tool` and `approval`.
Only `completed` and `done` include deliverables. Closing/aborting the HTTP
request cancels work; there is a ten-minute timeout. Tasks are limited to 16 KB
and request bodies to 64 KB. Runs are not persisted or resumable yet.
While running, the pipeline also appears as a live bay in the workshop
2D room (same states, agents, flow graph and detail as sessions).
An optional `flow` object replaces the default pipeline with a custom one:
`{name?,nodes:[{id,role,mode?,label?,instruction?,agent?}],edges:[{from,to,kind}],
output,max_rounds?}`. `agent` names a user-defined subagent (see below): that
node runs with the subagent's model, tools, mode and prompt even without
`tools:true`; an unknown name is a 400. Roles are coordinator|researcher|builder|reviewer;
modes are task (default) or review (strict approved/feedback/deliverable
verdict); kinds are data or revision. Validation rejects unknown roles,
dangling edges, self loops, data cycles, orphan nodes, revision edges from
non-review nodes or to work that never reaches the review, an unknown output,
and negative rounds (400, before any model call). Classic role/mode pairs
may omit the instruction; other combinations must spell it out. Every node
must reach the output, which feeds nothing further. MaxRounds bounds each
revision edge (0 = review as gate). The same SSE vocabulary streams back,
with node IDs as agents.

The browser's local demo uses scripted events, including simulated tools and
delegated characters. It makes no model or web-search requests.

## User-defined subagents

Markdown files with YAML front matter in `.gregal/agents/*.md` (project) and
`~/.config/gregal/agents/*.md` (user; a project file with the same name wins).
Front matter: `name` (lowercase, `[a-z0-9_-]`), `description` (when to use
it), optional `model` (`provider/model` or a model of the role's provider),
`role` (a configured role), `tools` (allowlist of native tools; `delegate`,
`question`, `todowrite` and `preview` are never available), `mode`
(`inspect`, default, or `code`) and `max_steps` (default 12, max 40). The body
is the subagent's prompt. The agent's system prompt lists name and
description; the `delegate` tool accepts `{tasks:[{prompt,agent?}]}`.
Code-mode subagents write only inside an isolated gregal worktree
(`.gregal/worktrees/<name>`) or with user approval through the same
`approve_request` flow as the parent turn; without either they stay
read-only. Limits stay at 4 per call and 8 alive per process. Delegate SSE
events carry `agent` with the subagent name.

`GET /api/agents/defined` returns `{agents:[{name,description,model?,role?,
tools?,mode,max_steps,source,path,warnings?}],dirs,isolated}` for the session
workspace (`isolated` = it is a gregal worktree).

## Side questions and recaps

`POST /api/btw` `{question}` answers a side question with the chat role from a
copy of the session transcript plus the live trace of the running turn. It
never waits for, interrupts or alters the main run, and nothing is added to
the history. Response `{answer,model,busy}`; 400 without a question or model,
502 when the model call fails.

`POST /api/recap` `{since?,turns?,conversation?}` summarizes session activity
with the chat role: `since` (RFC 3339) reads the durable event log of the
session (last 5000 events of the process at most), `turns` (0-50, default 3)
uses the last N user requests of the transcript, and `conversation` names a
saved conversation to summarize instead. Response
`{summary,empty,source,count,model?}` (`source` = `events`, `turns` or
`conversation`; `empty:true` when there is nothing to summarize).

## Project memory hygiene

After a successful code/autonomous turn with at least three tool executions
and one applied edit, the server asks the chat role in the background for at
most two durable project notes. Nothing is written automatically.
`GET /api/memory/suggest` returns `{state,notes,at}` (`state` = `pending`,
`ready` or `none`). `POST /api/memory/suggest` `{action:"accept"|"dismiss",
note?}` accepts a suggested note (appended to `.gregal/memory.md` with the
usual date prefix) or dismisses it; `dismiss` without `note` clears all. A note
that is not in the current suggestion is a 404.

`GET /api/memory/tidy` returns the current notes `{notes,hash,path}`.
`POST /api/memory/tidy` asks the chat role for a consolidated list (merge
duplicates, drop stale notes, never add) and returns
`{before,after,diff,hash,changed}` without writing. `POST /api/memory/tidy/apply`
`{after,hash}` writes that list only if the file still has `hash` (409
otherwise) and `after` has no more lines than the current notes.

## Companion (personal memory + check-ins)

These routes use the verified authenticated identity, not a client-supplied
user name. Personal memory for named users lives at `users/<name>/memory.md`
beside the config, shared across that user's sessions only. No fallback to or
migration from another user's/global memory occurs. This applies consistently
to memory read/accept/forget/extraction, check-ins, pet/wind/group chats and wind
heartbeats. Legacy no-user and `Tothom` single-token mode keep the global
`GREGAL_MEMORY`/default memory behavior. Wind profiles/knowledge and catalogs
remain in their existing authenticated scope. Existing conversations stay in
their current locations; this change does not migrate chat files.

| Route | Method | Request and response |
|---|---|---|
| `/api/companion/memory` | GET | `{state,facts,notes}`: `facts` are the currently proposed personal facts (`state` = `pending`, `ready` or `none`), `notes` is the full user memory (the audit view: "what do you know about me"). |
| `/api/companion/memory` | POST | `{action:"accept",fact}` appends an accepted fact to the authenticated user's memory (date-prefixed) and removes it from the proposal → `{ok,total,notes}`; a fact not in the current proposal is 404. `{action:"dismiss"}` clears the proposal → `{ok}`. `{action:"forget",term}` deletes that user's notes containing `term` → `{ok,forgotten,notes}`. |
| `/api/companion/checkin` | POST | Generates a proactive check-in (1-2 sentences) grounded in the user memory and the last session digest → `{checkin,at}`. Read-only side call; nothing is written. With `{pet}` (one of the pet ids below) the check-in is written in that pet's voice and the response adds `pet`. |
| `/api/companion/pets` | GET | The companion pet catalog → `{pets:[{id,name,role}]}`. Four specialized pets (music, news, code, system); each has its own voice server-side. |
| `/api/companion/chat` | GET | `?pet=<id>` → `{pet,history:[{from,text}]}`; returns the authenticated user's durable conversation for that pet. |
| `/api/companion/chat` | POST | Chat with one pet in its voice → `{reply,pet,history:[{from,text}]}`. Body: `{pet,message}`. Histories are separate per pet, persisted in the authenticated user's saved-conversation directory, and the model context is capped at the last 16 turns. The user memory is injected as what the pet knows. The pet has no access to external data and says so instead of inventing. |
| `/api/winds` | GET | → `{winds:[{id,name,color,role,style}]}`. The wind catalog omits persona; use the authorized profile endpoint to read/edit it. |
| `/api/winds` | POST | Create/update a wind → `{wind}` (full object, including persona). Body: `{id?,name,color,role,persona?,style?}`. Explicit `id` edits only that existing wind and stays stable on rename; unknown explicit id is 400. Without id, the name slug creates/updates the legacy same-name entry. Blank/omitted persona preserves the existing voice, or fills a default for a new wind. Invalid color falls back to `#FF9F00`. |
| `/api/winds` | DELETE | Archive a wind, preserving identity, memory, knowledge and documents in the user library. Body: `{id}`. |
| `/api/winds/library` | GET, POST | GET returns `{items}` with active, archived, recovered and pre-transfer versions, including per-wind memory, knowledge and full skills. `?export=<item-id>` exports one JSON snapshot. POST `{action:"restore",source}` restores an archived identity; `{action:"assign",source,target,memory,knowledge}` merges selected information into an active wind, saving a previous version and rejecting conflicting documents or size overflow. |
| `/api/winds/squads` | GET, POST | GET `{squads}` lists predefined everyday, data and software/security specialists. POST `{id,lang:"ca"\|"en"}` activates missing specialists without changing existing winds. Stable independent identities, idempotent activation. |
| `/api/winds/crew/propose` | POST | `{task,lang}` → `{plan,reuse:[{id,name,role,why}],create:[{name,role,kit,persona,why,color,style}]}`. Reuses existing winds that fit first (they keep memory and knowledge) and proposes up to 3 new specialists only for missing expertise, each with an unused colour, name and face. Falls back to word matching without a model. |
| `/api/winds/crew/start` | POST | `{task,lang,reuse:[id],create:[…]}` → `{rotllana,created}`. Creates the new winds permanently, opens a rotllana in the current project and runs a server-side job: a design discussion round, then a **coordinator** that decides each next step: `work` (1-4 members build their parts at the same time, each with its own files, and another member reviews it all; another round only if the review leaves something open, 3 rounds at most), `ask` (one question to the user: the job waits, its state is saved, and the user's next message in that rotllana resumes it instead of going to the normal chat; 2 questions at most), `finish` (a result message with the conclusion, the files, what was checked and what was not) or `giveup`. Thread messages carry `kind`: `debate` (discussion and progress, which apps fold), `question` and `result`. |
| `/api/winds/crew/status` | GET | `?rotllana=<id>` → `{running,step}` with step `discussing`, `working`, `waiting` (the crew asked the user and waits for the answer; kept across restarts), `done` or `failed`. |
| `/api/winds/doc` | GET | `?name=<file>` → the document a wind created in its chat (`pdf_create`, `office_create`), as an attachment. Winds write documents only in their own folder (per user) or edit documents the user attached; replies carry a `[name](/api/winds/doc?name=…)` link (or `/api/office/download?id=…` for an edited attachment), which the apps download with their own authentication. |
| `/api/winds/cap` | GET | `?lang=ca` → `{name,color,style,history:[{from,text,action,winds}],brief:{done,failed,running,news,titles,text}}`. The mascot (chief of staff) of the user: its look, the short thread with it, and a summary of what happened since the last visit (tasks finished or stuck, tasks running, new notes from the winds), worded without a model, and `pet` (see `/api/winds/cap/play`). Each GET marks the visit. |
| `/api/winds/cap` | POST | `{message,lang,force}` → `{action,reply,winds,task,rotllana,history,confirm,message}`. The mascot decides who handles the message: `self` (it answers, `reply` is the answer), `wind` (one wind, named in the message or chosen by the model), `rotllana` (2-3 winds; `rotllana` is an existing group with exactly those winds, if any) or `crew` (long work; `task` is the brief). For real questions it prefers gathering the specialists of the subject; `create` lists new specialists it proposes when none exist, and `novices` the chosen winds that know nothing yet (no knowledge, no memory). Nothing runs: the app shows the decision and acts on "go ahead". `force` is the user's choice for the same message (`self`, `wind:<id>`, `rotllana`, `crew`) and replaces the previous decision in the thread. A short "go ahead" ("endavant", "sí", "som-hi", "yes") right after a proposal returns that proposal again with `confirm:true` and `message` = the original request, without calling the model; the app carries it out at once (a crew is proposed and started). |
| `/api/winds/cap/go` | POST | `{message,lang,winds:[id],create:[{name,role,persona,kit}],course}` → `{wind}` or `{rotllana}`, plus `{created,studying}`. Carries out the mascot's decision: creates the proposed specialists (names, colours and faces kept distinct, an existing wind of the same trade reused), opens the chat (one wind, or the group with exactly those winds, created if missing) and, with `course`, starts a crash course (`/api/winds/expert` on its role) for every member that knows nothing yet. |
| `/api/winds/cap/link` | POST | `{link,text}` → `{history}`. Notes in Ventet's thread where a delegation ended up (`link` is `wind:<id>` or `rot:<id>`); `/api/winds/cap/go` does it by itself, the app uses this for crews started from the crew sheet. Thread messages carry `action` (`self`, `wind`, `rotllana`, `crew`, `link`), `winds`, `task`, `create`, `novices` and `link`, so a pending delegation can be shown again after reopening the app. The thread keeps up to 1000 messages. |
| `/api/winds/summaries` | GET | `{items:[{key,from,speaker,text,ts,count}]}`: the last message and its time (seconds) for every wind (`wind:<id>`), group (`rot:<id>`) and Ventet (`cap`), for the conversation list without loading every thread. |
| `/api/winds/cap/play` | POST | `{act,lang}` → `{pet,say,level_up,unlocked}`. Looking after the mascot: `pet`, `feed:galeta|poma|cafe`, `game:<id>:<score>`, `sleep`, `wear:<outfit>` (`wear:` takes it off). `pet` is `{happy,energy,full,xp,level,level_xp,level_need,outfit,outfits,played}`; time makes energy recover, hunger grow and happiness drop slowly, never below 25. Outfits unlock with level and by season. |
| `/api/winds/cap/profile` | POST | `{name,color,style}` → `{name,color,style}`. Renames or restyles the mascot. |
| `/api/winds/tasks` | GET | `{tasks}` returns up to 100 recent delegated tasks for the authenticated user with folder, status and persisted public events/results (bounded to 160 events per task). No tool arguments or hidden reasoning. Execution still requires Gregal to remain open. With `?summary=1`, each task is a light summary (`status` running/done/failed/cancelled/interrupted, `agents`, `files`, `result`, `error`, `updated`) and crew tasks still in their design discussion are included. |
| `/api/winds/chat` | GET | `?wind=<id>` → `{wind,history:[{from,text}]}`; the durable conversation with a wind, persisted as `company-<wind>`. |
| `/api/winds/chat/stream` | POST | Same turn as `/api/winds/chat` but as SSE: `phase` (`thinking|searching|scheduling|working`), `token` (reply-text fragments; speculative tokens before a phase must be discarded), `done` (same payload as the classic POST plus `actions`), `error`. Body: `{wind,message}`. |
| `/api/winds/rotllana/stream` | POST | Same group turn as `/api/winds/rotllana` but as SSE with the same event protocol; `done` carries `{reply:{wind,text},history,actions}`. Between speakers the server emits `phase: speaker:<windID>` so the client can start a new bubble. Body: `{rotllana,message,wind?}`. |
| `/api/winds/memoria` | GET | `?wind=<id>` → `{wind,memoria}`. What the wind remembers about the user (markdown, user facts only, no citations). Unknown wind is 400. |
| `/api/winds/memoria` | PUT | Replace the wind's memory → `{ok:true}`. Body: `{wind,memoria}`. Atomic write, max 8,192 bytes; bumps the revision so stale background extractions cannot restore edited-out facts. The wind learns on its own after each conversation; the user can read/edit here, on mobile, on desktop, or by hand (`wind-memory/<id>.md`). |
| `/api/winds/coneixements` | GET | `?wind=<id>` → `{wind,coneixements:[{nom,titol,descripcio}]}` (index only: descriptions go in the prompt, bodies load on demand via the `skill` tool). Unknown wind is 400. |
| `/api/winds/coneixements` | PUT | Create/update a wind knowledge entry → `{ok:true,nom}`. Body: `{wind,nom,titol?,descripcio,cos}` (markdown `SKILL.md` with frontmatter, stored at `wind-skills/<wind>/<nom>.md`). Incomplete is 400. |
| `/api/winds/coneixements` | DELETE | Delete a wind knowledge entry. Body: `{wind,nom}`. Missing is 404. |
| `/api/winds/coneixement` | GET | `?wind=<id>&nom=<nom>` → `{wind,nom,titol,descripcio,cos}` (full body for the editor). Missing is 404. |
| `/api/winds/resum` | GET | → `{vents:[{id,nom,rol,coneixements:[{nom,titol,descripcio}]}]}`. All winds with role and knowledge index in one call, for the member picker (name + what each one is expert in). |
| `/api/winds/novetats` | GET | `?des_de=<unix>` → `{novetats:[{wind,source,text,ts}]}` (max 50, best-effort). What the winds said on their own (heartbeats, reminders) since the timestamp, for the app's local notifications. No auth beyond the user session; empty when nothing is new. |
| `/api/winds/programats` | GET | `{reminders:[{id,wind,wind_name,at,text}], hearts:[{wind,wind_name,next_at,last_at,last_status}], stats:[{wind,wind_name,beats,spoke,silent,reminders,last_at}]}`. The winds' proactive schedule for the app's "Programats" view: pending reminders (what each wind will say and when), hearts on watch (next look) and per-wind alert stats derived from real runs (beats, spoken, silent). |
| `/api/winds/wiki` | GET, POST, DELETE | GET → `{pages:[{nom,titol,descripcio,updated,font,autor}]}` (cos no inclòs; les més recents primer); POST desa `{nom,titol,descripcio,cos}` → `{page}`; DELETE `?nom=` o `/api/winds/wiki/{nom}` esborra. Pàgines de resum de l'espai vents, markdown amb frontmatter a `wind-wiki/`. |
| `/api/winds/wiki/{nom}` | GET | `{page:{nom,titol,descripcio,cos,updated,font,autor}}` — la pàgina sencera amb el cos. `font` és la conversa d'origen (`wind:<id>` o `rotllana:<id>`) i `autor`, qui l'ha escrita. |
| `/api/winds/wiki/summary` | POST | `{wind}` o `{rotllana}` (+ `titol` opcional) → `{page, updated}`. Resumeix la conversa desada en una pàgina amb títol, descripció, gràfics ```chart amb els números reals, imatges i fonts. Si la conversa ja tenia pàgina (`font`), l'actualitza amb el que és nou (`updated:true`); si no, en crea una de nova sense trepitjar-ne cap amb el mateix títol. 400 si el vent/rotllana no existeix o la conversa és buida; 502 si el proveïdor falla o el resum surt buit. |
| `/api/winds/rutina` | POST | `{wind, kind: "daily"|"interval", time: "HH:MM" | interval_h, prompt, name?}` → la rutina creada. El vent la fa de veres: model i eines, i decideix si cal avisar. |
| `/api/winds/rutines` | GET | `{rutines:[…]}`: les rutines dels vents visibles per a l'usuari. |
| `/api/winds/rutines/delete` | POST | `{id}` → `{ok:true}`. Esborra una rutina de vent. |
| `/api/winds/meta` | GET / POST | `{meta:{favorites:["wind:<id>"\|"rot:<id>"], sections:[{id,name,items:[…]}]}}`. Preferències d'organització de la llista (favorits i seccions); POST desa l'estat complet. |
| `/api/winds/expert` | GET/POST | POST `{wind,tema}` → `{ok,wind,tema}` starts a study session (409 if one is already running); GET `?wind=` → `{wind,tema,fase,pas,passos,iniciat,acabat}` with fase "" (never) / `cerca` (searching the web) / `apren` (writing the knowledge) / `fet` (done) / `error`. The wind researches the topic with the heartbeat's web tools and saves what it learns as one of its own knowledge files; the app animates the face with the live fase and records a novetat on completion. |
| `/api/winds/chat` | POST | Chat with a wind → `{reply,wind,history}`. Body: `{wind,message}`. Context includes persona, wind knowledge, the authenticated user's memory, active project, up to 5 open goals, last work-session digest, and the previous conversation (last 40 messages, digest capped at 16,000 characters plus an omission marker). The durable history is not truncated. Keeps the existing direct individual conversation; named independent threads use singleton rotllanes. |
| `/api/winds/chat` | DELETE | Clear the chat with a wind → `{ok:true}`. Body: `{wind}`. Removes the `company-<wind>` session from disk and memory; the wind itself stays. Unknown wind is 400. |
| `/api/winds/profile` | GET | `?wind=<id>` → `{wind:{id,name,color,role,persona,style},knowledge:string,personal_memory:string}`. Full persona and exact knowledge markdown; personal memory is a read-only prompt preview (up to 2,000 characters plus truncation marker). Unknown wind is 400; other methods are 405. |
| `/api/winds/memory` | POST | `{wind,text}` → the same full profile shape. Replaces all that wind's knowledge markdown atomically; `text:""` clears it. Text is required, must be a string, and is bounded to 24,576 UTF-8 bytes. Unknown wind, missing/null/non-string/oversized text, extra fields or trailing JSON are 400; other methods are 405. Never edits personal memory. |
| `/api/winds/rotllanes` | GET | → `{rotllanes:[{id,name,members:[windID]}]}`. Group chats: each member sees the whole conversation and answers in turn. Members that no longer exist are dropped from the list. |
| `/api/winds/rotllanes` | POST | Create/update a rotllana → `{rotllana}`. Body: `{id?,name,members:[windID]}`. Needs a name and ≥1 existing wind: one member is a named independent chat. Without id, always creates a new thread with a generated id (name slug plus random suffix), even after deleting a same-name thread. Explicit id edits an existing thread without changing id/history; unknown explicit id is 400. Existing groups keep their original ids/history without migration. |
| `/api/winds/rotllanes` | DELETE | Remove a rotllana. Body: `{id}`. Also removes the `rotllana-<id>` conversation from disk and memory (no orphan history). |
| `/api/winds/rotllana` | GET | `?rotlla=<id>` → `{rotllana,history:[{from,text,speaker_id,speaker_name}]}`; the group conversation, persisted as `rotllana-<id>`. New assistant replies persist speaker identity even if a wind is later renamed. Old name-prefixed entries resolve identity only when a unique matching wind exists; unknown/ambiguous speakers and user messages use empty speaker fields. Legacy `from` (`user`/`pet`) and `text` (including the assistant's `Name: ` prefix) remain unchanged. |
| `/api/winds/rotllana` | POST | Talk to a group → `{reply:{wind,text},history:[{from,text,speaker_id,speaker_name}]}`. Body: `{rotllana,message,wind?}`. With `wind`, that member answers. Without `wind`, a router (the same wind model, one cheap call with member cards: role + knowledge descriptions + memory notes) picks 1-3 speakers by relevance: the knowledgeable expert first, else the closest role/affinity (they search the web and discuss findings); nobody relevant = silence (`{reply:{wind:"",text:""},silenci:true}`, the user message is still saved). If the router fails, the least-recently-spoken member answers. Each speaker runs its own read-only turn (`web_search`/`web_fetch`, up to 6 steps) seeing the previous speakers' replies; multi-voice `Name: ` lines still split into separate messages. |
| `/api/winds/suggest` | POST | AI-written persona + face for a new wind → `{persona,style}`. Body: `{name,role}`. Style is 3 letters (eyes/mouth/shape); invalid letters fall to defaults. |
| `/api/user/skills` | GET | User's own skills → `{skills:[{nom,titol,descripcio}]}` (index only); `?nom=x` adds `cos`. Requires a logged-in user (400 otherwise). |
| `/api/user/skills` | POST | Create/update a user skill → `{ok}`. Body: `{nom,titol,descripcio,cos}`. Name is slugified; description is required (falls back to titol). Stored at `users/<name>/skills/`. |
| `/api/user/skills` | DELETE | Delete a user skill. Body: `{nom}`. |
| `jobs` (convention) | — | Jobs with id `wind-<windID>` are wind heartbeats: the server builds the prompt (search the wind's topic; speak only if something is worth telling; reply `(res)` to stay silent) and appends the message to the wind's chat (`company-<wind>`). The run output carries `Wind: message` when it speaks. Jobs with id `remind-<windID>-<stamp>` and kind `remind` are one-shot wind reminders (tool `remind`): at fire time the prompt text enters the wind's chat, the run output carries it (notification), and the job disables itself. |

## Language

| Route | Method | Request and response |
|---|---|---|
| `/api/lang` | POST | `{lang}` (`ca` or `en`): sets the UI and agent-response language. Stored in config; application-wide, not per session. |
| `/api/brands` | GET/POST | GET → `{active, brands:[{id, name, version, author?, mode, theme, colors, fonts, logo?, swatches, css}]}`. `theme` is the `data-theme` value (`brand-<id>`) and `css` its `:root` block. POST takes a branding file as the body (`gregal-brand/1` JSON) and installs it or replaces the one with the same id → same as GET plus `installed`. Application-wide; with several users only the administrator can install. |
| `/api/brands/preview` | POST | Validates a branding file without installing it → `{brand, status, installed_version?}`, where `status` is `new`, `update`, `same` or `older` compared with the installed one. Invalid file → 400 `{error}`. |
| `/api/brands/active` | POST | `{id}` (`""` = none): the branding used by the app on every surface and by generated documents (docx, pptx, xlsx) that do not ask for another theme. Stored in config as `brand`. Administrator only with several users. |
| `/api/brands/{id}` | DELETE | Uninstalls a branding; if it was active, no branding stays active. |
| `/api/brands/{id}/file` | GET | Downloads the branding file (`<id>.gregal-brand.json`) to share it. |

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
| `/api/sessions/live` | GET | `{sessions:[{id,title,msgs,busy,mode,role,cwd,project,created_at,activity?}], current}` |
| `/api/sessions/open` | POST | `{id?,title?,cwd?}` → `{id,title,cwd,project}`; generates an ID when omitted. |
| `/api/sessions/close` | POST | `{id}` → `{ok,closed}`; stops that session's processes. The `default` session cannot be closed (400). |
| `/api/agent/cancel` | POST | Stops the active turn in the selected session (agent turns and team pipeline runs alike) → `{ok,cancelled}`. |
| `/api/md` | POST | `{text,tema}` → `{html}`: renders response Markdown on the server (goldmark + Chroma with the `internal/tema` palette) and sanitizes it with bluemonday. The client requests it after a turn; while text is arriving, `app/md.js` renders it without a request per fragment. |

## Users and Authentication

The optional session `activity` snapshot describes the latest real agent turn:
`{state,task,model?,step?,limit?,tool?,message?,started_at,updated_at,agents}`.
`busy` in `/api/sessions/live` covers agent turns and team pipeline runs.
A team run publishes into its session as `kind: "team"` with its `flow`
spec, so the single 2D room renders pipeline runs next to agent sessions;
role agents arrive as child entries with live `working` states.
States are `thinking`, `tools`, `waiting`, `compacting`, `reviewing`,
`completed`, `failed`, and `cancelled`. Step/limit counts describe the step
budget, not a percentage of task completion. Child entries carry
`{id,task,status,model?,step?,max_steps?,tool?,message?,output?,started_at,updated_at}`;
their status is `working`, `completed`, `failed`, or `cancelled`.
The latest 32 children are retained, keeping active children in preference to
finished entries. A new turn resets the snapshot. Snapshots are session- and
user-scoped and are kept in memory until the session closes or server restarts.
Delegate updates also use SSE event `delegate` and the durable event payload.
Output previews are limited to 2000 Unicode characters plus an ellipsis.
Turn and child entries also report real model consumption: `up_tokens`,
`down_tokens` and `calls`, plus `tokens_source` (`api` when every call
declared usage, `mixed` when only some did, absent when the provider said
nothing) on the turn, `usage_known` per child, and `cost_usd` on the turn only
when the model has a configured price — never estimated. The turn keeps an
inspection-only step timeline (`history`, newest last, bounded to 40 entries)
with `{step,state,tool?,message?,at}` per rung.
Child entries may carry `agent` (user-defined subagent name; `scout` for the
turn-start researcher). Agent turns also report role lanes for the room:
`role` (who holds the turn: `coordinator`, `researcher`, `builder`, `tester`
or `reviewer`), `lanes` (newest last, at most 60, `lanes_trimmed` counts the
dropped ones) with
`{seq,step,role,kind,target?,n?,tools?,files?,command?,result?,done?,agent?,started_at,duration_ms?}`
(`kind`: `scout`, `plan`, `delegate`, `read`, `search`, `web`, `edit`,
`commit`, `run`, `test`, `review`, `verify`, `tool`; `result`: `ok`|`fail`
once done), `handoffs` (at most 30) with
`{seq,step,from,to,kind,n?,target?,files?,at}` (`kind`: `read`, `web`,
`edit`, `run`, `commit`, `test`, `review`, `brief`, `report`) and
`role_usage` (`{<role>: {up_tokens?,down_tokens?,calls?}}`, the token delta
observed while each role held the turn). Lanes and handoffs share one `seq`
counter per turn. They are visual attribution only; the coordinator remains a
single agent.

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
| `/api/state` | GET | `{role:[], current_role, mode, model, provider, verify, agent_busy, cwd, project, branch, git, worktree?, worktree_repo?, permissive, auto_review, ui_mode, access, sandbox, used_tokens, context_window}`. `ui_mode` is what the mode picker shows (`ask` for chat/inspect, `plan` for plan/goal, `act` for code/autonomous/debug) and `access` is the Act permission (`ask`, `edits` or `full`); see [modes and permissions](modes.md). `git` says whether `cwd` is inside a git working tree (`branch` is empty on a detached HEAD); `worktree`/`worktree_repo` appear only when `cwd` is a gregal worktree (`<repo>/.gregal/worktrees/<name>`) and give its name and the main repository root. `used_tokens` estimates conversation context usage; `context_window` is 0 when the role does not declare it. `auto_review` is true in auto-review permission mode. `sandbox` is the shell sandbox status: `{mode: off\|workspace\|strict, requested_backend, backend: none\|bwrap\|landlock\|sandbox-exec\|docker, active, refuses, network: on\|off, network_isolated, allow_write?, warning?, guarantee}`; `active` means commands really run confined, `refuses` means strict mode without a usable backend (shell commands are denied). |
| `/api/browser/extension` | WebSocket | Only from a `chrome-extension://` origin. The Gregal extension connects when the user turns on "Let Gregal use this browser"; its first message is `{type:"hello",key,version}` (the key is checked like a Bearer token when the server has one). The backend then sends `{id,op}` requests (`open`, `cdp`, `tabs`, `select`, `close`, `detach`) and the extension answers `{id,result}` or `{id,error}`; it also sends `{event:"detached"|"stop"|"ping"}`. |
| `/api/browser/extension/status` | GET | `{connected, stopped, version?, since?}`: whether the user's Chrome is connected for the agent. |
| `/api/corner` | GET/POST | The winds' Corner. GET → `{paused, running, current, versions:[{n,at,builder,builder_name,reviewer,reviewer_name,summary,review,reverted_from,bytes}], journal:[{at,kind,wind,name,text}], winds, budget:{sessions,max_sessions,calls,max_calls,talks,max_talks}}`. POST `{action}`: `pause`, `resume`, `meet` (a session now; 409 if one is running, 429 over budget), `stop` (cancels the running session and pauses), `comment` (`text`, read at the next meeting), `revert` (`version` → a new version equal to it). A new Corner starts paused. |
| `/api/corner/page` | GET | `?v=<n>` (default current) → `{version, doc?, data:{winds,version,lang}}`: `doc` is the page wrapped with its CSP (no connections) and the `gregal` bridge, for an `iframe sandbox="allow-scripts"` srcdoc or a WebView. The page posts `{gregalCorner:'hello'}` and `{gregalCorner:'ask',id,wind,text}` to its parent (or `GregalAndroid.send`); the host answers with `{gregalCorner:'init',data}` and `{gregalCorner:'reply',id,reply|error}`. |
| `/api/corner/ask` | POST | `{wind,text}` → `{reply}`: a visitor of the Corner talks to a wind (in character, 1-4 sentences). 60 a day (429 after). |
| `/api/football` | GET/POST | The winds' American football league. GET → `{teams:[{id,name,abbr,color,color2,coach,record,players:[{id,name,pos,number,color,style,wind,attrs}]}], games:[{id,status,started,score,teams}], today, max_per_day}` (the league is created on first use from the user's winds, completed with team players). POST `{action:"play"}` → `{id}` starts a game (409 if one is running or 3 were played today); `{action:"reset"}` builds a new league. |
| `/api/football/match` | GET/POST | GET `?id=&after=N` → the game with the plays numbered above `N` (`{status: pregame|live|final|stopped|failed, teams, plans:[{q,team,coach,note}], plays:[{n,q,clock,off,down,dist,spot,kind,detail,dir,depth,yards,end,complete,int,fumble,td,safety,good,first,turnover,penalty,actors,score,hl,text,comment,drive}], score, quarter, summary, mvp, commentator, stats}`; with `after>0` the rosters are omitted). POST `?id=` `{action:"stop"}` stops it. The viewer page `/app/football-viewer.html#id=<id>&token=<key>&lang=ca` draws it on its own (the phone uses it in a WebView). |
| `/api/arena` | GET/POST | The Arena (prompt competition). GET → `{me, display, admin, games:[{id,title,starter,rules}], next:{game:{owner,display,level}}, seeding:[game], progress:{game:[{id,started,kind,opponent,result,score,coach}]}, mine:{game:{md,display,updated,versions}}, bots:{game:[{owner,display,level,md}]}, leaderboard:{game:[{owner,display,elo,played,won,lost,drawn,bot}]}, competitions, matches, practice_left}`. POST `{action}`: `save` (`game, md, display`; max 12000 chars, keeps 20 versions), `practice` (`game, md?, display?, opponent?` → `{id}`: saves `md` first (the template if there is none yet); without `opponent`, the easiest bot not yet beaten; 6 a day per player), `review` (`game, md?` → coach feedback), `seed_bots` (admin; `games?`: the model writes the bots' `.md`). Users are the server's accounts (`local` without accounts). |
| `/api/arena/match` | GET/POST | GET `?id=&after=N` → `{id, game, kind, comp, stage, owners, names, titles, status, meta (when after=0), events:[{n,kind,text,hl,score,decision,data}], decisions:[{n,side,short,decision,why,rule,fallback,rule_found}], followed:[share per side of decisions whose rule is really in the .md], model, score, winner, feedback:[{scores,summary,strengths,mistakes,suggestions}], md}`. Another player's `.md`, rules quoted and report stay hidden until it is an official finished match. `?situation=N&side=S` → `{n,side,situation,schema}`: what the model saw for that decision. POST `?id=` stops it (owners or admin). One match is played at a time; others wait in line. |
| `/api/arena/competition` | GET/POST | GET `?id=` → `{title, games, status: ready|running|done|stopped, model, schedule:[{game,stage,phase,a,b,match,matches,series,replays,winner,score,done,pending,from}] (playoffs are best of 3: score = series wins), current, champion, names, titles, tables, entries (revealed when done)}`; without `id`, the list. POST (admin) `{action:"create", title, games?, bots?}` snapshots every player's `.md`, seeds groups by Elo and builds groups + playoffs; `start`; `stop`. |
| `/api/arena/stream` | GET | `{competition, script}`: the running (or latest) competition, as above, plus the broadcast script `{lang, voice, opening:{text,lines,audio}, explain:{game:{title,lines,audio}}, champions?:{text,audio}}`. Matches add `preview`, `preview_audio`, `lines:[{n,text,audio}]` (caster) and `analysis:{headline,text,side,rule,audio}`. For `/app/arena-stream.html#token=<key>` or `#spectator=<link key>`. |
| `/api/arena/voice` | GET/POST | GET `?k=` → the cached narrator audio (wav/mp3; 404 while it is being made). GET → `{config:{engine: piper|http|off, url, model, voices, extra}, install:{state,step,done,total,error}, ready:{ca,en}, platform, sizes}` (never the key). POST (admin) `{action}`: `install` (`lang`; downloads Piper and the voices), `config` (`config:{engine, url, key, model, voices, extra}`), `test` (`who: caster|analyst, lang, text?` → `{key}`). |
| spectator link | — | `X-Arena-Spectator: <key>` (or `#spectator=` on the stream page) allows only `GET /api/arena/stream`, `/api/arena/match` and `/api/arena/voice?k=`. POST `/api/arena/competition {action:"spectator", reset?}` (admin) returns or renews it. |
| `/api/arena/starter` | GET | `?game=` → the template `.md` as a download. |
| `/api/health` | GET | `{status, protocol, instance_id, capabilities}` for discovering and validating the service instance. `capabilities.interactive_events` is `true` only when durable storage and payload serialization are available. |
| `/api/active` | GET | `{active:{id,task,started_at,role,mode,project,events[],progress?}}` for the active turn. `progress` is the latest `progress` event payload (see below); progress events never enter `events[]`. |
| `/api/v2/events` | GET | `?after=<cursor>&limit=1..500&session=<id>` → `{events:[{id,at,session_id,run_id,kind,text,payload?}],cursor,next}`; durable reads can resume after a disconnect. |
| `/api/v2/events/stream` | GET **SSE** | `?after=<cursor>&limit=1..500&session=<id>` → streams subsequent events and stays open; also accepts `Last-Event-ID`. Each message has `id: <cursor>`, `event: <kind>` and the same event JSON (including `payload` when present); sends `: heartbeat` every 15 seconds. |
| `/api/v2/runs` | POST | `{task, images?, mode?, idempotency_key?, workspace?}` → 202 `{run:{id,state,session,workspace,...},cursor}` when queued. `workspace` is the absolute directory the turn runs in (tools, map, verify); empty = the session cwd. It must exist and pass the user's guard (400 if relative/missing, 403 outside their roots). `cursor` is the last event before this run, so a client can follow its stream without replaying older history. Returns 200 with an existing run if the idempotency key was already used; 423 if strict review has blocked; 429 if the session queue is full. |
| `/api/steer` | POST | `{text, interrupt?}` → 200 `{ok:true}`: a message for the turn that is running in this session. It is added at the next safe point (after the current tool batch) and the agent takes it into account in its next step; if the agent had already written its final answer, the turn reopens. With `interrupt:true` the model call in progress is cut and redone with the message (running tools are never cut). The turn emits a `steer` event `{text}` when the message enters, and `steer_restart` when a model call was cut. 409 if no turn is running (send it as a normal message instead); 400 if empty. |
| `/api/v2/runs` | GET | `{runs:[...]}`: runs in the session, newest first. |
| `/api/v2/runs/{id}` | GET | `{run}` with state and timestamps; 404 if it does not exist or belongs to another session. |
| `/api/v2/runs/{id}/cancel` | POST | Cancels the identified turn: a queued run will not execute; a running turn has its context cancelled → `{ok,cancelled}`. Returns 404 outside the session. |
| `/api/models` | GET | `{models:{provider:[]}, errors:{}, current, role, vents_selected}`. `vents_selected` is the winds' own default model (`""` = they use the work one). |
| `/api/sessions` | GET/DELETE | GET: saved conversations on disk, newest first: `[{name,title,msgs,when,at,current,pinned,workspace}]` (`title` comes from the first message; `at` is RFC3339 for date grouping; `current` marks the conversation currently being written). DELETE `?name=` removes one. Saved automatically after each turn. |
| `/api/sessions/pin` | POST | `{name,pinned}` updates the navigation pin without cloning the conversation and returns `{name,pinned}`. |
| `/api/goal` | GET | `{goals:[{id,title,body,status,project}]}`. |
| `/api/checkpoints` | GET | `[{seq,op,path,at,git?,kind?}]` sorted by time (journal entries filtered to the workspace when multiple sessions are live). In git repositories it also lists the session's git snapshots: `op: "snapshot"`, `git: N` (rewind with `{"git": N}`), `kind: turn|step` (`turn` = start of a turn, `step` = before a shell command that may write), `path` = label. |
| `/api/worktrees` | GET | `{available, main, current, worktrees:[{name,path,branch,head,base,dirty,ahead,main}]}`: gregal worktrees of the session's repository (`current` = the worktree the session runs in, if any). `available:false` outside git. |
| `/api/providers` | GET | `{providers:[{name,url,type?,key,used_by}], roles}`; `type` is omitted for Chat Completions providers. |
| `/api/github` | GET | `{available, cwd}`. |
| `/api/jobs` | GET | Scheduled jobs; `next_due_at` is the next slot in RFC 3339. |
| `/api/jobs/runs` | GET | `?job=` (or legacy `?id=`) returns run history. Wind routine runs retain their owner even after deletion. |
| `/api/mcp` | GET | `{summary,servers:[{name,status,transport,tools,prompts,resources?,auth?,host?,error?}],tools:[name],prompts:[{command,server,name,description,arguments}]}`: safe diagnostics for MCP connectors; does not expose commands, headers, tokens or environment. `status` is `connected`, `error` or `needs_auth`; `auth` (remote servers only) is `none`, `authorized` or `required`. |
| `/api/mcp/connect` | POST | Admin. `{name}` → `{status:"connected",servers}` or `{status:"needs_auth",auth_url}`: (re)connects a server; when it asks for OAuth 2.1 the UI opens `auth_url` in the browser (PKCE, dynamic client registration when offered). 502 `{error,servers}` if it cannot connect. |
| `/api/mcp/disconnect` | POST | Admin. `{name}` → `{servers}`: forgets the stored OAuth token of a remote server and disconnects it. |
| `/api/mcp/servers` | POST/DELETE | Admin. POST `{name,command?,args?,env?,url?,transport?,headers?,client_id?,scopes?,readonly?,preset?}` (`readonly: true` lets the server's tools run without permission prompts; `preset: "demo"` replaces the other fields with the demo database: `name` defaults to `demo`, command = the running Gregal executable with `mcp demo`, `readonly: true`) validates, writes `mcp.<name>` to config.yaml and connects it → `{saved,servers,error?}` (`${VAR}` in headers is stored as written and expanded when connecting). DELETE `?name=` removes it from the config and from the live manager. |
| `/api/mcp/prompt` | POST | `{text:"/mcp__server__prompt args"}` → `{text}`: expands an MCP prompt for the composer (positional or `key=value` arguments). 404 if it is not an MCP prompt, 400 on a missing required argument. |
| `/api/mcp/oauth/callback` | GET | OAuth redirect target (`?code&state` or `?error`). It carries no Gregal token: the single-use `state` authenticates it. Exchanges the code, stores the token in `<data dir>/mcp-oauth.json` (0600), reconnects the server and returns a short HTML page. |


## Turn Execution

| Route | Method | Notes |
|---|---|---|
| `/api/skills` | GET/PUT | Admin global preference. GET → `{skills:[{name,title,description,enabled}]}`; PUT `{enabled:[name...]}` saves selected specialist skills to `skills.enabled`. Only short routing descriptions enter the system prompt; the full skill body is loaded on demand through the `skill` tool. |
| `/api/chat` | POST **SSE** | `{message}` → `token`, `done`, `verify`, `error`. |
| `/api/vscode/completion` | POST | `{language,prefix,suffix,workspace}` → `{completion}`. Tool-free, does not read or change conversation history, requires an allowed workspace, limits cursor context to 9,000 characters, output to 128 tokens and each call to 10 seconds; returns 429 when its four-call shared limit is full. |
| `/api/agent` | POST **SSE** | `{task, images?, mode?}` → see the event table below. `mode: chat|inspect` only restricts the turn (the Office add-in uses `chat` for read-only requests). If the session is already working, the request waits in the shared turn queue with SSE heartbeats; it does not return 409. |
| `/api/parallel` | POST | `{message}` → `{results:[{role,provider,model,reply?,error?}]}`, up to 4 roles. |
| `/api/plan` | POST | `{task}` → `{plan, critic?}`: read-only exploration that proposes a plan. With `plan.critic: on` (default) the reviewer role critiques it: `critic: {issues:[{kind: missing\|risky\|assumption, text}], model?, skipped?}` (`skipped` explains why no critique ran, e.g. no reviewer role). |
| `/api/goal` | POST | Creates, runs or deletes goals. |
| `/api/approve` | POST | `{key, approve, remember?}` → `{ok, note?}`. |
| `/api/question` | POST | Answers a selectable question: `{key, answer}` (label or `text:<free text>`) → `{ok}`. |
| `/api/verify` | POST | `{mode}`: `off`, `manual`, `auto`, `both` or `strict`. |
| `/api/verify-approve` | POST | Clears a `strict` review block. |
| `/api/access` | GET/POST | `{access}`: the Act permission, `ask` (ask before changes, including edits inside the project), `edits` (auto-approve edits in the project; the default) or `full` (full autonomy: autonomous mode). Picking one switches to Act (`debug` stays in debug for `ask`/`edits`), turns off the legacy `permissive` flag and saves `permissions.access` and `mode` to the config. The old menu names `manual`, `accept`, `review` (→ `edits`) and `auto` (→ `full`) are accepted. Returns `{access, mode}`. |
| `/api/permissive` | GET/POST | Legacy; no client shows it any more. `{on, review}`. `on` lets the agent proceed without asking; `deny` rules still block. `review: true` selects auto-review mode instead (a reviewer model decides calls that need approval; exclusive with `on`). Session only; the initial value of `review` comes from `permissions.mode: auto-review`. |
| `/api/mode`, `/api/role`, `/api/model` | POST | Changes the active mode, role or model (per session). `/api/mode` accepts the internal ids (`chat`, `plan`, `code`, `autonomous`, `inspect`, `goal`, `debug`) and the picker names `ask`, `plan`, `act`. `/api/model` accepts `{model, scope?}`: without scope it changes the work model (persisted per role); with `scope: "vents"` it sets the winds' own default model (xat, rotllana i cor) without touching the work one. The provider must be explicit (`"provider/model"`) for the winds scope. |
| `/api/save`, `/api/new`, `/api/resume` | POST | Operates on conversations saved to disk. |
| `/api/rewind`, `/api/rewind-to` | POST | Reverts file changes → `{summary}`. `/api/rewind` undoes the journal and, in git repositories, restores the workspace to the session's first git snapshot (shell changes included). `/api/rewind-to` takes `{seq: N}` (journal checkpoint, as before) or `{git: N}` (git snapshot: journal back to that point, then every file that differs from the snapshot is restored and files created afterwards that are not ignored are deleted; a `turn` snapshot also trims the conversation to that turn). HEAD, the index, branches and the stash are never touched. |
| `/api/worktrees` | POST/DELETE | POST `{name?, open_session?=true, title?}` creates `<repo>/.gregal/worktrees/<name>` on a new branch `gregal/<name>` from HEAD (files listed in `.worktreeinclude` are copied) and, by default, opens a new session in it → `{worktree, session?:{id,title,cwd,project,worktree}}`. DELETE `?name=&delete_branch=1` runs `git worktree remove` (never forced; 409 if it has uncommitted changes or a live session uses it); the branch is deleted only with `git branch -d`. |
| `/api/worktrees/apply` | POST | `{name, how}`: `diff` → `{worktree, diff, truncated}` (all the worktree's work against its base, uncommitted and new files included); `patch` applies that diff to the main working tree with `git apply` (no commit, index untouched; nothing changes if it does not apply); `merge` runs `git merge --no-ff gregal/<name>` only if the main tree is clean and the worktree has no uncommitted changes (aborted on conflicts) → `{summary}`. |
| `/api/git/commit` | POST | `{message, paths?, all?}` → `{summary}`. From the UI, no paths = every change in the workspace. Only those paths are committed (other staged files are left out); git hooks run. |
| `/api/git/pr` | POST | `{title, body?, base?, draft?, branch?}` → `{summary}` with the PR URL. Uses `gh`; on the default branch it first creates a new branch (`branch` or `gregal/<title>`), pushes without force and runs `gh pr create`. |
| `/api/git/suggest` | POST | `{kind: commit|pr}` → `{message}` or `{title, body}`: a suggestion generated by the chat role from the workspace diff (and the branch commits for a PR). Nothing is committed. |
| `/api/providers`, `/api/github` | POST | Adds/tests providers; `gh` remains read-only. |
| `/api/office/upload` | POST | Uploads a docx/xlsx/pptx as base64 → `{id, kind, name, size, path}`; `path` is the disk path for giving the document to the agent with `@path`. |
| `/api/office/read` | POST | Reads text or data from an uploaded document. |
| `/api/office/edit` | POST | `{id, op, …}` with the same operations as the `office_edit` tool: docx `replace`, `insert`, `append`, `delete`, `comment`, `fill_vars`, `accept_changes` (`track` for tracked changes); pptx `replace`, `add_slide`, `duplicate_slide`, `delete_slide`, `move_slide`, `fill`, `notes`; xlsx `set_cell`, `set_cells`, `set_range`, `format`, `recalc`. An edit that would leave the package structurally broken is refused (400). Records the change in the journal. |
| `/api/office/download` | GET | `?id=` → returns the edited file. |
| `/api/office/render` | POST | `{id, pages?}` → `{report, images, engine}`: renders the document with LibreOffice (or Microsoft Office on Windows) to PNG data URLs (a numbered thumbnail grid, or the requested pages at full size) plus automatic QA. Without a render engine, `images` is empty and `report` says so. |
| `/api/office/open` | POST | `{id}` → opens the document with the server's associated application (local/desktop only). |
| `/api/jobs/save` | POST | Creates or updates a scheduled job. Optional `wind` binds a reading/research routine to the wind's current persona, memory and knowledge. The server assigns its owner; other users cannot read, change or run it. Existing shared jobs keep their behavior. Gregal must remain running for the local scheduler. |
| `/api/jobs/delete` | POST | Deletes a scheduled job. |
| `/api/jobs/run` | POST | Runs a job immediately. `scheduled: true` is a slot fired by an external scheduler (`GREGAL_JOBS_SCHEDULER=external`, the Android app): it queues behind an interactive turn and refuses a paused job. |

### `/api/agent` SSE Events

`route` (the router changed roles) · `thinking` (reasoning accompanying a
tool call) · `tool_call` · `approve_request` (`{key, call_id, name, args,
timeout_s, auto_approve_allowed}`, plus `reason` and `forced` for `browser`:
why it asks — a new site or an irreversible action — and whether it can never
be remembered or auto-approved; `timeout_s` is a number of seconds from
`agent.approval_timeout_s`, default 1800; `0` means no limit) ·
`approve_timeout` (no response within `timeout_s` seconds means denied) ·
`question_request` (selectable question:
`{key, query, options:[{label, description}], timeout_s}`; answer with
`/api/question`) · `question_timeout` (no response within `timeout_s` seconds
means the agent continues without it) · `tool_result` (`{id, name, output}`;
`browser` screenshots add `image`, a `data:image/…;base64` URL of at most
1.5 MB) · `blocked` ·
`review` (auto-review decision `{id, name, decision: allow|deny|ask, reason}`;
sent before the call's `tool_result`, `blocked` or `approve_request`; also
stored in the durable log as kind `review`) ·
`bet` (confidence bet resolved for a verification command: `{id, p, passed,
command, source, text}`, all strings; `p` is 0-100, `passed` is `true`/`false`,
`source` is `model` or `side`; sent after the call's `tool_result`; durable
kind `bet` with payload `{id, p, passed, command, source}`) ·
`assistant` · `fallback` (the
secondary model responded) · `escalate` · `budget` · `gated` (strict review
required) · `steps_exhausted` (the `agent.max_steps` limit was reached; the
following response is a summary, not completed work) · `progress` (turn
liveness, see below) · `lane` (a step attributed to a room role; same
fields as an activity `lanes` entry plus `text`; sent when the step starts and
again when it ends with `result`/`done`/`duration_ms`) · `handoff` (the turn
moves between roles: activity `handoffs` fields plus `text`) · `delegate`
(child agent progress, including the turn-start researcher `scout`) ·
`verify_start` · `verify` · `done` · `error`.
`/api/chat` emits a subset (`token`, `done`, `verify`, `error`).

`progress` tells a client what the turn is doing right now, so a long silent
model call (queued at the provider, or reasoning while the answer is still
buffered) is distinguishable from a hung agent. `data` is a JSON object of
strings:

| Field | Present | Meaning |
|---|---|---|
| `phase` | always | `model` (a model call for the next step), `tools` (tool calls executing), `review` (autonomous checkpoint, step-extension decision or final summary), `compact` (history compaction). |
| `state` | always | For `model`: `waiting` (request sent, no bytes yet) or `receiving` (the response is streaming). Otherwise `running`. |
| `since` | always | RFC 3339 UTC timestamp (nanosecond precision) when the current phase started; it does not change on `waiting` → `receiving`. |
| `reasoning`, `text`, `tool` | `phase=model` | Cumulative characters received in the current model call (decimal strings): reasoning, answer text and tool-call arguments. They count every delta as it arrives, including output that is buffered and only shown when the call finishes, and output of attempts that are later retried. |
| `tools` | `phase=tools` | Comma-separated tool names of the step, in call order. |

Cadence: one event when a phase starts (`model`/`waiting` when the request is
sent, `tools` before the calls run, `review`, `compact`); one immediately when
the first chunk of a model response arrives (`receiving`), and again on
`waiting` if the provider call is retried; while receiving, at most one count
update every 3 seconds and only when a count changed; and one final count
update when the model call returns if the counts changed since the last one.
A silent provider therefore produces no events: clients should derive elapsed
time from `since`. Approval and question waits are signalled by their own
events, not by `progress`. In the durable log (`/api/v2/events` and
`/api/v2/events/stream`) the event kind is `progress`, `text` is a short label
such as `model · receiving` or `tools · read,web_fetch`, and `payload` holds
exactly the fields above.

Durable interactive events `approve_request` and `question_request` include an
optional JSON `payload` so a reconnecting client can restore the interaction.
The approval payload contains only `{key,call_id,name,args}`; the question
payload contains `{key,call_id,query,options}`. Older events have only `text`
and no `payload`.

Autonomous runs also emit `autonomous_checkpoint` with a structured durable
`payload`: `{number, checks:[{command,code,output?}], review}`. The same data
is available through event polling and SSE replay, even when the event's
display `text` is shortened. `checks` is always an array: `[]` when no
`agent.autonomous.verify` commands are configured (older servers sent
`null`). `review` is empty when no review ran at this checkpoint, otherwise
the reviewer verdict (`APROVAT`, `CAL REVISAR: <summary>` or
`error: <message>`). A zero `code` means the check passed; a missing or
invalid code does not establish a result. A `status` line in the session
language accompanies the checkpoint only when there is something to report
(check results or a review). Older events may contain only JSON in `text`;
clients should ignore checkpoint details they cannot parse.

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
  unchanged. Git snapshots are per session, but restoring one rewrites the
  whole workspace directory: two sessions on the same directory share it.
  Snapshot refs live under `refs/gregal/checkpoints/` (the newest
  `git.checkpoint_keep`, default 50, are kept per repository).
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

## Plot library and project dashboard

`/api/plots` uses the authenticated user and the workspace of `X-Gregal-Session`.
Libraries are stored on the server, separately for each user and project, and survive restarts.

- GET returns `{revision, plots}`. Each plot has `id`, `spec`, `session` (source), `created`, `dashboard`, `width` (1 or 2), and `note`.
- POST accepts `{revision, spec, dashboard}` and creates a saved snapshot. A spec contains `title`, `type` (exactly `line`, `bar`, `scatter`, `area`, `horizontal-bar`, `pie`, `doughnut`, or `radar`), string `labels`, and `series` of `{label, values}`. This data-only contract is shared by the server, web/desktop, and Android clients; executable options are never accepted. For scatter, labels contain finite numeric x coordinates written as JSON-number strings. Values must be finite and match the labels. Area supports signed values with a zero baseline; horizontal-bar supports signed, grouped series. Pie/doughnut require exactly one series, nonnegative values, and at least one positive value. Radar requires 3–32 category labels and nonnegative values; all-zero series are valid. Generic limits remain 2000 labels, 20 series, 200 plots per library; request bodies at most 1 MiB.
- Web rendering maps `area` to a filled line and `horizontal-bar` to a bar chart with a horizontal value axis. Pie/doughnut use per-slice palette colors and no Cartesian scales; radar uses a radial scale. Circular render values are divided by their largest slice when needed to avoid overflow/underflow; trusted local tooltip code displays original values, and saved/exported JSON and data tables retain the original values. No model-supplied callbacks or options are forwarded.
- PATCH accepts `{revision, id, dashboard, width, note, spec?, direction?, dashboard_only?}`. It updates the card, optionally replaces its data/title, and moves it one position with `direction: -1` or `1`. Set `dashboard_only: true` to skip cards outside the dashboard when reordering. Notes are limited to 4000 bytes.
- DELETE accepts `{revision, id}` and removes the saved plot.

GET also returns `boards: [{id, name}]`; saved plots have a `board` ID. The
initial dashboard is `default`; existing libraries migrate to it automatically.
POST/PATCH plot requests accept `board` (default: `default`). Each saved plot
can belong to one dashboard at a time.
POST with `{revision, operation, board?, name?}` manages named dashboards:
`create_board` needs a name, `rename_board` needs a board ID and name, and
`delete_board` needs a board ID. The default dashboard cannot be deleted.
Deleting another dashboard removes its membership but preserves all plots,
data and notes in the library. Names are limited to 200 bytes and a project
can have up to 30 dashboards. All operations use the same revision control.

Mutations return the updated library. A stale revision returns 409; reload before retrying. Invalid requests return 400, unknown plot IDs 404, and storage failures 500. Chart options and executable code are not accepted. Snapshots do not refresh automatically.

## Knowledge collections

Knowledge collections are searchable sets of documents and notes. All routes use the project (workspace) and session of `X-Gregal-Session`. A collection has a `scope`:

- `global`: every project; stored under `<data>/knowledge/` (`GREGAL_DATA_DIR` or `~/.local/share/gregal`).
- `project`: stored in `<project>/.gregal/knowledge/` and travels with the repository; sources inside the project are saved as relative paths. Index files (`*/index.gob`) are git-ignored automatically.
- `session`: tied to one session; stored under `<data>/knowledge/sessions/<session>/`.

IDs carry the scope prefix (`g-`, `p-`, `s-`). A collection object is `{id, name, description, scope, enabled, sources: [{id, path, include, exclude, added}], notes: [{id, title, text, author, created}], created, updated}` plus `stats: {docs, chunks, errors, unsupported, skipped, truncated, limit, indexed, embed_model, embed_error}`, `progress: {running, phase, done, total, current, started, finished, error}`, `source_paths` (source ID to absolute path) and `missing_sources`.

- `/api/knowledge`: GET returns `{collections, project, session, embeddings, embed_model, limits: {max_files, max_mb, max_file_mb}, can_global}`. POST `{name, description, scope, path?, include?, exclude?}` creates a collection (default scope `project`); with `path` it also adds that folder or file as a source and starts indexing in the background.
- `/api/knowledge/{id}`: GET returns the collection plus `docs: [{key, path, source, kind, size, mtime, status, error, chunks, indexed}]` (`status` is `indexed`, `empty`, `skipped`, `unsupported` or `error`). PATCH `{name?, description?, enabled?}`. DELETE removes the collection and its index (never the source files); 409 while indexing.
- `/api/knowledge/{id}/sources`: POST `{path, include?, exclude?}` links a folder or file; DELETE `{id}` unlinks a source. Globs match the path relative to the source (`*.md`, `docs/**`, `**/*.pdf`); without `include` every readable file is indexed. Both start a background reindex.
- `/api/knowledge/{id}/reindex`: POST `{force?, wait?}`. Incremental by default: files with the same size and modification time keep their chunks and vectors; `force` re-reads everything. Without `wait` it returns immediately; poll GET `/api/knowledge/{id}` for `progress`. 409 if an indexing run is already active.
- `/api/knowledge/{id}/notes`: POST `{title?, text}` adds a note (at most 8000 characters) and indexes it immediately; DELETE `{id}` removes it.
- `/api/knowledge/search`: GET `?q=&collection=&k=` or POST `{query, collection?, k?}` returns `{query, hits: [{collection_id, collection, scope, path, file, loc, kind, snippet, score}]}`. Without `collection` it searches every enabled collection. Ranking is BM25 over words (accent- and case-insensitive); with `knowledge.embeddings` configured and indexed vectors it fuses BM25 and vector similarity (reciprocal rank fusion).
- `/api/knowledge/memory`: GET returns `{memories: [{kind, path, text, hash, exists, editable}], project}` for the user memory (`user`, `~/.config/gregal/memory.md` or `GREGAL_MEMORY`), the project notes (`project`, `.gregal/memory.md`) and the read-only project rules (`rules`, `AGENTS.md` or `CLAUDE.md`). PUT `{kind: "user"|"project", text, hash}` replaces the file only if `hash` matches the current content; otherwise 409 with `current`.

Indexing reads Markdown, plain text and code, HTML (main content), PDF (text layer; encrypted or scanned PDFs are reported as `unsupported` or `error`), and `.docx`/`.xlsx`/`.pptx`. Binary files, images and archives are skipped silently; legacy office formats are listed as `unsupported`. Folders such as `.git`, `node_modules`, `vendor`, `dist` and hidden folders are skipped. Limits come from `knowledge.max_files` (2000), `knowledge.max_mb` (200) and `knowledge.max_file_mb` (25). Chunk locations name the heading path and line range, PDF page, sheet or slide.

On servers with configured users, only administrators can change global collections or the user memory, and source folders must be inside the user's roots (403 otherwise). Errors return `{error}` with 400, 403, 404 or 409.

The agent uses the same collections through the `knowledge_search` tool (allowed in every mode) and `knowledge_note` (asks for approval; denied in read-only modes; allowed without asking in autonomous mode for project and session collections). Enabled collections are listed in the system prompt.

## Retro channel (#retro)

A Slack-style debrief for a finished turn. The agents that worked on it (`coordinator`, delegated children such as `builder`/`researcher`, the `reviewer` verification gate, or team roles) post short, informal messages grounded in an evidence pack built from the run's durable events and the current git diff. All routes use the session of `X-Gregal-Session`. Channels are stored next to the session data (`<sessions>/retro/<conversation>.json`) and survive restarts.

- `/api/retro` GET with optional `?key=`, `?run_id=` or `?source=team` → `{channel|null, key, latest:{run_id, key, title, running, team?}, cursor, emojis}`. Without parameters, the channel of the latest run (or latest finished team run).
- `/api/retro/start` POST `{run_id?, source?: "team", regenerate?, persona?}` → `{channel, cursor}`. Default: last run of the session. 409 while the run is still in progress. Idempotent unless `regenerate`; on an existing channel a valid `persona` only updates the channel tone (no model call).
- `/api/retro/message` POST `{text, thread_id?, key?, persona?}` → `{message, replies, mentions, channel}`. `@agent` mentions choose who answers (default: coordinator); replies go into the thread of the user message.
- `/api/retro/react` POST `{message_id, emoji, action: "add"|"remove", key?, persona?}` → `{message_id, action, reactions, replies, channel}`. Emojis: 👍 🎉 🔥 🤔 👀 ✅ 👎 😂 🙏. 🤔 on an agent message makes it justify (or concede) in its thread; 👎 stores a `Prefer …` line in `.gregal/memory.md` and the agent acknowledges.
- `/api/retro/rewrite` POST `{key?, persona}` → `{channel, cursor}`. Applies the new tone to the channel and rewrites the already-published agent messages with the model, one by one, in place (same IDs and order; user/system messages untouched, receipts kept). On model failure the rewritten-so-far result is kept.
- `/api/retro/fix` POST `{message_id, key?}` → same body as `POST /api/v2/runs` (`{run, cursor}`) plus `channel`; enqueues a normal turn with the message's `fix_suggestion`.

Channel: `{key, run_id, source, title, lang, model, persona, created_at, degraded, agents:[{id,name,avatar,role,model}], evidence:[{id,agent,kind,title,detail,ok}], messages:[{id,thread_id,author,text,receipts,reactions:[{emoji,by}],concede,fix_suggestion,fix_run,note,at}]}`. Durable events: `retro_message` `{key, message}`, `retro_reaction` `{key, message_id, emoji, by, action, reactions}`, `retro_typing` `{key, agents, typing}`. Optional `retro.webhook_url` in the config file posts the stand-up summary as `{text}` (Slack/Teams incoming-webhook compatible).

Personas: `persona` is `neutral` | `sarcastic` | `playful` | `sharp` | `flirty` | `serious` (`friendly` | `mentor` still accepted for old channels). It changes only the tone of agent messages: the prompt adds a tone line *after* the strict grounding rules, which stay identical (receipts, no invented facts, honest failures). New channels take the request `persona`, else the previous channel's, else `retro.persona` from the config file, else `neutral`. `message`/`react` with a valid `persona` update the channel before replying; `rewrite` additionally regenerates the published agent messages in the new tone.

## Room replay

Replays a finished run in the 2D agents room from the durable event log. The server only lists runs and returns their events; the room states (thinking/tools/reviewing/waiting, delegated children entering and finishing, tool names, role lanes and handoffs from the durable `lane`/`handoff` events) are reconstructed in the browser (`app/replay-core.js`). Uses the session of `X-Gregal-Session` and the authenticated user.

- `/api/replay` GET → `{runs:[{run_id, state: "completed"|"failed"|"cancelled"|"running", started_at, ended_at?, events, tools, agents}]}`, newest first, at most 50.
- `/api/replay` GET `?run_id=N` → `{run_id, events:[durable event…], truncated}` in log order. `token` and `retro_*` events are omitted; at most 4000 events (`truncated: true` beyond that). 400 for an invalid id, 404 for a run that is not in this conversation, 503 if the event log is unavailable.

## Usage stats (Gregal Wrapped)

- `/api/stats` GET `?range=week|month` (default `week`; anything else → 400). Computed from the authenticated user's durable events (all conversations) plus saved conversations. Days are local server time; `week` = the last 7 days including today, `month` = the last 30.

Response: `{range, from, to, sessions, saved_sessions, turns, tool_calls, tools_by_kind:{read,search,edit,shell,web,delegate,mcp,other}, top_tools:[{name,count}], files_changed, top_files:[{name,count}], check_runs, checks_failed, tests_turned_green, stubborn_bug:{run_id, session, command, failures, at}|null, tokens:{known, up, down, estimated, cost_usd|null}, favorite_model, favorite_model_turns, active_days, longest_streak_days, current_streak_days, days:[{date, turns, tools}]}`.

- `turns`: distinct `run_started` events; `sessions`: conversations with events in range; `saved_sessions`: saved conversations whose `saved_at` is in range.
- `files_changed`: distinct paths of `write`/`edit`/`patch` calls whose result was not an `ERROR`.
- Checks are test-like `bash` commands (same detector as #retro: `go test`, `npm test`, `pytest`, `cargo test`, …) and autonomous checkpoint checks. `tests_turned_green` counts passes that follow one or more failures of the same command in the same conversation; `stubborn_bug` is the run with most failures of a command before it turned green.
- Tokens, cost and favourite model come from durable `usage` events, written at the end of each web turn with payload `{model, provider, up_tokens, down_tokens, estimated: true, cost_usd?}` (local estimates; `cost_usd` only when the config has prices). Older logs without `usage` events report `tokens.known: false`, and the favourite model falls back to delegated children's models.
- Streaks are computed over all days with events (all time); the current streak still counts if today has no activity yet.

## Transcript export and share links

Exports mirror what the conversation view shows: user messages, assistant work groups (intermediate notes plus tool calls with name, pretty-printed arguments and results truncated to 2000 characters) rendered collapsed, and final answers. System messages, the project map appended to tasks (`[project map …]`, legacy `[mapa del projecte …]`), `[hook context]` blocks, compaction summaries, autonomous checkpoints and the loop's internal guidance messages are never exported. `todowrite`/`todoread` calls are omitted, as in the UI.

| Route | Method | Description |
|---|---|---|
| `/api/export` | GET | Downloads the live conversation of the `X-Gregal-Session` tab. Query: `format=md\|html` (default `md`), `redact=1` (sanitize), `emails=1` (also hide e-mail addresses; only with `redact`). Responds with `Content-Disposition: attachment; filename="gregal-<title>.<ext>"`; 400 on an unknown format. |
| `/api/sessions/{name}/export` | GET | Same as `/api/export` for a saved conversation of the authenticated user (404 if it does not exist). |
| `/api/shares` | GET/POST | GET: `{shares:[{id,owner,session,title,created,bytes,emails_hidden,url}]}`, the caller's share links, newest first. POST `{name?, emails?}` creates a read-only snapshot of a saved conversation (`name`) or of the live one (empty body) → `{id,url,title,created}`; `emails` defaults to `true`. 400 for an empty conversation, 404 for an unknown `name`. |
| `/api/shares/{id}` | DELETE | Revokes one of the caller's share links (deletes the snapshot) → `{deleted}`; 404 if unknown or owned by someone else. |

`GET /share/{id}` (outside `/api`) serves the stored snapshot. Share snapshots are always sanitized and stored as rendered HTML under `<data>/shares/<id>.html` with metadata in `<id>.json` (`<data>` is the parent of the sessions directory, honouring `GREGAL_DATA_DIR`). The id is 160 random bits (`crypto/rand`, 40 hex characters). Like `/` and `/app/`, the page needs no API token — the unguessable id is the capability — but it is only reachable where the server itself is reachable (without a token or users the server listens on loopback only). Responses carry `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'` (no scripts), `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` and `Cache-Control: no-store`. Unknown, malformed or revoked ids return 404.

Sanitization replaces with `[REDACTED]`: API keys and tokens (`sk-…`, `sk-ant-…`, `ghp_`/`gho_`/`ghu_`/`ghs_`/`ghr_`, `github_pat_…`, `xox[baprs]-…`, `AKIA…`/`ASIA…`, `AIza…`, JWTs, `Bearer …`), generic assignments such as `api_key=…`, `"token": "…"` or `PASSWORD: …` whose value looks like a secret (contains a digit, is not a call or expression), PEM private-key blocks, every secret value known to the server (expanded provider `api_key`s, the Telegram token, MCP `env`/`headers` values, environment variables whose name contains KEY/TOKEN/SECRET/PASSWORD/CREDENTIAL, and the API token). Home directories (`C:\Users\<name>`, `/home/<name>`, `/Users/<name>` and the server's own home, any slash style, case-insensitive) become `~`. With `emails`, addresses become `[email]`. HTML exports are self-contained (inline CSS, no scripts or external assets) and carry the same CSP as a `<meta>` tag.

## Agent capabilities: debug mode, best-of-N, plan critic, confidence bets

**Debug mode.** `mode: debug` (`/api/mode`, `/api/agent` `mode`, `--mode debug`, TUI `/mode debug`) runs the code-mode engine with a debugging contract: ranked hypotheses, temporary instrumentation tagged `GREGAL-DEBUG`, reproduction (the agent runs the repro command when one is given or obvious, otherwise asks the user), evidence, root-cause fix, clean-up and a final answer that lists each hypothesis as confirmed or refuted with its evidence. Writes follow the normal code policy. Before the turn closes, the engine searches the workspace for `GREGAL-DEBUG` (`git grep`, or a bounded walk outside git) and compares it with the count at the start of the turn; if new tags remain it sends the agent back to clean up (at most twice) and otherwise closes with a warning `status`.

**Confidence bets.** Before a verification command (tests, build, vet, lint) the agent can pass `bet` (0-100) in the `bash` call. The engine resolves it against the outcome, emits `bet` and appends `{ts, session, project, command, p, passed, source}` to `<data>/bets.jsonl` (`GREGAL_DATA_DIR` or `~/.local/share/gregal`). `agent.bets`: `on` (default), `side` (also asks the chat role for a probability when the model gave none) or `off`.

| Route | Method | Notes |
|---|---|---|
| `/api/calibration` | GET | `?scope=session\|project\|all` (default `project`) → `{scope, enabled, mode, calibration:{count, brier, mean_p, pass_rate, overconfidence, buckets:[{from,to,count,mean_p,pass_rate}], recent:[bet]}}`. `brier` is the mean of (p − outcome)²; `null` fields when there are no bets. |
| `/api/bestof` | GET/POST/DELETE | POST `{task, n: 2-4, models?: [role or provider/model per attempt], verify?: [commands], max_steps?}` → 202 with the run snapshot. It creates `n` worktrees `bestof-<id>-<k>` from HEAD, runs the task in each (non-interactive, session mode or `code`; ask-actions are auto-approved only while the session is permissive) through the shared run queue (one queue session per attempt, keyed by the worktree, so attempts run in parallel and serialise with any other turn on the same directory), then runs `verify` (default `agent.autonomous.verify`) in each worktree and asks the reviewer role for a rubric score. GET `?id=` → `{id, task, main, running, applied, message, recommended, started_at, attempts:[{index, worktree, path, branch, role, model, state: queued\|running\|checking\|done\|failed, answer, error, files, added, deleted, diff_bytes, checks:[{command,code,output}], checks_passed, score, score_note, cost_usd, steps, seconds}]}`; GET without `id` → `{runs:[…]}` (this session's). DELETE `?id=` cancels and force-removes every attempt worktree and branch → `{id, removed}`. Runs live in memory (lost on restart; the worktrees stay on disk). |
| `/api/bestof/apply` | POST | `{id, attempt, keep?}` → `{id, applied, message, removed}`: applies the attempt's diff to the main working tree with `git apply` (no commit; nothing changes if it does not apply → 409), then force-removes the other worktrees (and the chosen one unless `keep`). 409 while attempts are still running or the session agent is busy. |

**Plan critic.** Besides `/api/plan`, in autonomous mode the first milestone checklist (`todowrite` with two or more items) is critiqued once by the reviewer role; if there are concerns the agent is asked to revise the checklist once before executing, and a `status` line `plan critic: N concerns` lists them. `plan.critic: off` disables both.
