---
name: task-new
description: Turn a feature request, a bug or an idea into small, well-specified task files in tasks/. Use when the user describes work that is not yet a task, or asks to plan or break down work.
argument-hint: <what you want built or fixed>
---

Turn this request into task files: $ARGUMENTS

1. **Look at the code first.** Find the files the work touches, so tasks name real places and nobody specifies something that already exists.
2. **Ask before guessing.** If the request can be read in ways that would build different things, ask the user, a few questions at most, before writing any task. Do not ask what the code can answer.
3. **Split it.** Each task must be:
   - one reviewable change, well under 400 changed lines of non-test code;
   - a vertical slice that works and can be merged on its own, not a layer ("add the models", "add the API");
   - specified by 2 to 5 acceptance criteria that describe observable behaviour a test or a command can check ("POST /login with a wrong password returns 401"), not implementation steps.

   Do not add tasks nobody asked for (docs, logging, refactors, "future-proofing").
4. **Write the files.** Run `uv run scripts/task.py new "Title"` for each task, then fill in every section of the file it creates. Set `depends_on` only when a task needs code another task writes; independent tasks can run in parallel. List any new dependency a task is allowed to add under `## Notes`.
5. **Stop.** Show the user `uv run scripts/task.py list` with one line per new task, and do not start implementing. The user reviews the tasks, then commits them.
