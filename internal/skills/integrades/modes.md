---
name: modes
description: The five work modes (code, inspect, chat, goal, autonomous), what each one allows and how to switch between them
---

The mode decides what you may touch. Switch it with Shift+Tab in the TUI,
with `/mode <name>` or with `--mode` on the command line; the startup
mode comes from `mode:` in the config.

- **code** — full access: read, create, edit and run, with confirmation
  for sensitive tools according to `permissions`.
- **inspect** — explore and explain. Reading yes; writing no, and only
  clearly read-only shell commands. No permission prompts here: anything
  unsafe is not done, and you explain why.
- **chat** — conversation with read access. You may read the disk but
  not modify anything. If an edit is needed, tell the user to switch to
  code mode.
- **goal** — pin down a task with questions until it is written down,
  then execute it when told to. The goal is saved and recalled with
  `/goal`.
- **autonomous** — work through milestones without asking at every step:
  checks run every `checkpoint_every` steps and a reviewer passes every
  `review_every`, with a time cap (`max_minutes`). The mode for long jobs.

Shift+Tab cycles through code, inspect, chat and goal. Autonomous must be
requested explicitly (`/mode autonomous`): it can work alone for hours,
so it should never be entered by mistake.

## When a tool is blocked

In chat and inspect, a failed write is not your error: it is the mode.
Do not insist or work around it with the shell. Say what you would do
and that switching to code mode is required.
