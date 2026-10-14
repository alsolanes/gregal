---
name: skills
description: How skills work and how to write a new one for a project or for yourself
---

A skill is a document that enters your context only when you ask for it.
Your prompt holds the index (name and one line); the body arrives when
you call the `skill` tool with the name.

Use skills for knowledge too long to carry all the time and too specific
to deduce: how this project is deployed, the team's conventions, how to
do an operation that has no dedicated command.

## Where they come from

1. Built-in, inside the binary: they explain Gregal to itself.
2. `~/.config/gregal/skills/*.md` — this person's skills, for every
   project.
3. `.gregal/skills/*.md` at the project root — this project's skills,
   shipped with the repository.

If two share a name, the one lower in the list wins: a project can
correct what Gregal thinks it knows.

## The format

```markdown
---
name: deployment
description: how this service is deployed to production and who must approve it
---

The body, in Markdown. As long as needed: it is only read when someone
asks for it.
```

Both header keys are required (`nom:` and `descripcio:` are also
accepted). A file without a header is ignored, so that random READMEs do
not end up in the prompt index.

The description decides whether the skill gets opened: it must say when
the skill is useful, not what it contains. "how to deploy to production
and who must approve it" is useful; "deployment notes" is not.

## Writing one

When asked to remember how something is done in this project, propose a
skill: `.gregal/skills/<name>.md`, with the header and what was learned.
Create it with `write` like any other file. The next session will
already have it in the index.

For a short fact (a preference, a decision) a skill is not needed:
`/nota` saves it to `.gregal/memory.md`, which always goes into the
prompt.
