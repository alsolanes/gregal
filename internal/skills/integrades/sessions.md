---
name: sessions
description: Where conversations are saved and how to list, resume, delete or search inside them
---

Each conversation is saved as a JSON file. There is no database: they
are files, and you can read and delete them with the normal tools.

## Where they are

`~/.local/share/gregal/sessions/` (Windows:
`C:/Users/<you>/.local/share/gregal/sessions/`), or
`$GREGAL_DATA_DIR/sessions` if that variable is set.

Each file is named `sessio-AAAAMMDD-HHMMSS-mmm.json` (`AAAAMMDD` is
YYYYMMDD). The same directory holds `events.jsonl`, which is not a
session.

## What is inside

```json
{
  "name": "sessio-20260921-145033-171",
  "title": "first message of the conversation, truncated",
  "saved_at": "2026-09-21T14:50:33Z",
  "role": "chat",
  "workspace": "/path/to/project",
  "pinned": false,
  "compacted": "summary of the compacted part, if any",
  "convo": [{"role": "user", "content": "..."}],
  "activity": [{"name": "bash", "args": "...", "output": "...", "failed": false}]
}
```

`convo` holds the user's messages and your replies. Tool calls and their
full outputs are NOT there: only the `activity` summary remains. That is
why the message count of a long session is lower than the screen
suggests.

The conversation is saved at the end of every turn, and also on exit
with `/quit` or Ctrl+C. If the process dies abruptly, everything up to
the last finished turn is kept.

## TUI commands

- `/sessions` — arrow-key picker, with the latest conversation
  preselected.
- `/resume <name or N>` — resume a conversation.
- `/save [name]` — save the current one under a name.
- `/pin` — pin or unpin the current conversation.
- `/clear` — clear the conversation from the screen.

## What has NO dedicated command

Deleting, renaming or searching inside conversations. Do it with the
tools; that is why they are files:

```bash
# the ten most recent, with date and size
ls -lt ~/.local/share/gregal/sessions/sessio-*.json | head

# what one contains
cat ~/.local/share/gregal/sessions/sessio-20260921-145033-171.json

# search inside them (title and content)
grep -l "platforms" ~/.local/share/gregal/sessions/*.json

# delete a specific one
rm ~/.local/share/gregal/sessions/sessio-20260918-191945-786.json

# delete those older than thirty days
find ~/.local/share/gregal/sessions -name 'sessio-*.json' -mtime +30 -delete
```

Before deleting anything: show what you will delete and wait for
confirmation. A deleted conversation cannot be recovered, and those with
`"pinned": true` are ones someone deliberately chose to keep.
