# Tasks

One markdown file per task, named `NNN-slug.md`, created with
`uv run scripts/task.py new "Title"` from `_template.md`.

| State | Meaning |
| --- | --- |
| todo | `status: todo` and no branch yet |
| in-progress | a branch `task/NNN-*` exists |
| done | `status: done`, set on the task branch, so it arrives with the merge |
| blocked | `status: blocked`; the file says why |

A task is ready when it is todo and everything in `depends_on` is done.
`uv run scripts/task.py list` shows every task, `next` the ready ones.
