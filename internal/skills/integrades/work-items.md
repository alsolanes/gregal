---
name: work-items
description: How to write, split and move a work item (feature, bug, task, spike, epic); read it before creating, picking up or closing one
---

A work item is one outcome that someone can check, small enough to finish
and review as one change. If it cannot be checked, it is a wish; if it
cannot fit in one change, it is an epic.

## Types

Use these five and nothing else. Labels can come later; types do not
multiply.

- **feature** — new behaviour a person can observe. Says who gets it and
  what they can do afterwards that they could not do before.
- **bug** — the software does something other than what it should. Needs
  steps to reproduce, what was expected, what happened, and where
  (version, system, data). Without steps it stays `proposed`.
- **task** — work with no visible change in behaviour: refactor,
  dependency update, CI, docs, cleanup. Says why it is worth doing now.
- **spike** — a question with a time limit. The result is an answer or a
  decision written in the item, not code to merge. If code comes out of
  it, that code becomes a new item.
- **epic** — a goal too big for one change. It only groups items and is
  never worked on directly; it is done when its items are.

When in doubt between feature and task, ask: would a user notice? Between
bug and feature: did it ever work, or was it promised to work?

## Fields

```markdown
---
id: 42
type: feature
title: Export the monthly report as CSV
status: ready
priority: next
size: S
parent: 17        # the epic, if any
depends_on: [38]
---

## Why
Accounting rebuilds the report by hand in a spreadsheet every month.

## Acceptance criteria
- [ ] The report page has an "Export CSV" button.
- [ ] The file has one row per invoice with the same columns as the table.
- [ ] Amounts use a dot as decimal separator, whatever the UI language.
- [ ] An empty month gives a file with only the header row.

## Out of scope
PDF and Excel export.

## Log
- 2026-11-18 created from a conversation; criteria confirmed.
```

- **title** — imperative and specific, about the outcome: "Export the
  monthly report as CSV", not "CSV stuff" or "Improve reports".
- **why** — one or two sentences: who benefits and what it costs today.
- **acceptance criteria** — each one checkable by a test, a command or
  something visible, and true or false with no debate. "Fast" is not a
  criterion; "the list loads in under 1 s with 10,000 rows" is.
- **out of scope** — what someone might expect and is not included. It
  stops the item from growing.
- **priority** — `now`, `next` or `later`. Not numbers: a 1-to-10 scale
  ends with everything at 8.
- **size** — `S` (hours), `M` (a day or two), `L` (more). `L` means it
  must be split before it is `ready`.
- **depends_on** — items that must be done first. An item whose
  dependencies are open cannot be `in_progress`.

## States

`proposed` → `ready` → `in_progress` → `in_review` → `done`, plus
`blocked` and `dropped` from any state.

- **proposed** — written down, not yet clear. Anyone may create one.
- **ready** — why and criteria confirmed by the person who decides, size
  `S` or `M`, dependencies done. Only `ready` items are picked up.
- **in_progress** — one owner (person or agent), one branch or worktree.
- **in_review** — the change exists and the check command passes; someone
  who did not write it checks it against the criteria.
- **done** — every criterion ticked with its evidence: the test, the
  command output, the screenshot. A criterion without evidence is not
  ticked.
- **blocked** — always with the reason and who or what unblocks it.
- **dropped** — always with the reason. Never delete an item: the reason
  it was dropped is useful later.

## Splitting

Split by what a user can do, not by layer. "Export CSV for one month" and
"Export CSV for a date range" are two items; "CSV backend" and "CSV
frontend" are not, because neither is useful or checkable alone. Each
piece must be able to reach `done` on its own.

## Keeping order

- One item, one outcome. Something new found while working becomes a new
  item linked in the log, not extra scope. Only a fix of a few lines
  that the change already needs goes in silently.
- Before creating an item, look for an existing one. A duplicate is closed
  as `dropped` pointing at the original.
- The log only grows: dated lines with decisions, what was tried and
  failed, links to commits and sessions. Never rewrite earlier lines.
- Criteria do not change while `in_progress` without saying so in the log
  and having it confirmed. Otherwise "done" means nothing.
- Every so often go over `later`: drop what no longer matters, with the
  reason.

## For an agent

- Turning a request into an item: ask only what is missing to write the
  why and the criteria, propose the item, and leave it `proposed` until
  the person confirms it.
- Picking one up: only `ready` items whose dependencies are done. Set
  `in_progress`, write in the log which branch and session.
- Working: only on the criteria. If one is ambiguous or wrong, stop and
  move to `blocked` with the question, instead of guessing.
- Finishing: tick each criterion with its evidence, run the project's
  check command, move to `in_review`, and log what changed and anything
  left over as new items. `done` is decided by the reviewer, not by the
  agent that wrote the code.

## In Gregal

Every project has a backlog, with or without git and with or without a
forge. Use the `backlog` tool: `list`, `get`, `create`, `edit`, `move`,
`check` (tick a criterion with its evidence) and `log`. It applies the
rules above: an item cannot be `ready` without why and criteria, an `L`
cannot be `ready`, `done` needs evidence on every criterion, `blocked`
and `dropped` need a reason. You never move an item to `done`.

Items live in Gregal's data, one backlog per project. If the project has
a `.gregal/backlog/` folder they live there instead, one Markdown file
per item in the format shown above, versioned with the code; the
Backlog screen can move them there. A session in a worktree sees the
backlog of the main checkout.

Moving an item to `in_review` starts an independent review: Gregal runs
the project's check command, and a reviewer model that did not write the
code judges every criterion against the diff since the work started.
Your ticks and evidence are claims it checks, not proof. If it is NOT
MET, the item comes back to `in_progress` with what is missing in the
log and the unproven criteria unticked: read the log, fix that, and send
it to review again.

The Backlog screen (web and Android) shows the same list. "Work on it"
opens a session (in its own worktree when the project is a git repo),
moves the item to `in_progress`, logs where, and gives you the item as
the task. Follow the "For an agent" rules from there.
