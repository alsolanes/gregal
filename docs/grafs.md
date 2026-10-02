# Graphs

A **graph** is a drawn procedure: its steps, their order, and the edge followed
based on each result. Create it in the **Graphs** view (`Ctrl+G`). Graphs are
saved in `.gregal/flows/*.json` in the **project** because repository-specific
procedures, such as a pull-request review, should travel with that repository.

## When to Use a Graph

“Agent graphs” can refer to different things:

1. **Execution graph** (this feature): step order and branches.
2. **Dependency graph**: what depends on what in a project.
3. **Knowledge graph**: connected facts used to retrieve context.

This feature is an execution graph; it is not a dependency or knowledge graph
and does not add memory or context by itself. It is useful when a procedure is
repeated, a step needs the previous step's output, and you want to inspect how
a result was produced. For a one-off question, drawing a graph may cost more
than simply running the agent.

Keep the condition syntax small. Procedures that need several comparisons can
put that logic in an agent step rather than extending the condition language.

## Steps

| Type | Behavior |
|---|---|
| **Agent** | Runs the agent with a task; the normal step type. |
| **Tool** | Runs one tool without a model call (`bash`, `read`, `grep`, etc.). |
| **Note** | Passes fixed text to the next step as instructions or data. |

There must be exactly one **start** step, with no incoming edges. The editor
marks it with a start label.

## State

Each step saves its output in state under its ID, or under the key set in
**Save result to**. Reference values in instructions and arguments with
`{{key}}`:

```
The tests failed. Their output:

{{tests}}

Fix the code so they pass.
```

The state also always contains:

- `last`: output from the most recent step.
- `<id>.error` and `last.error`: the error message if that step failed.

## Edge Conditions

An edge can have a condition; an empty condition always matches.

| Form | True when |
|---|---|
| `key` | The key has a value. |
| `!key` | The key is empty or absent. |
| `key == "x"` | The value is exactly `x`. |
| `key != "x"` | The value is not `x`. |
| `key conté "x"` or `key conte "x"` | The value contains `x`. |

The condition operator is spelled `conté` or `conte` in saved graphs. If
multiple outgoing edges match, the first one is followed and a warning is
recorded in state as `avis`. Matching edges do not run in parallel; doing so
without an explicit request could change the order of disk writes.

## Failed Steps

After a step fails, **only edges with a written condition are eligible**. If
none match, the graph stops and reports which step failed. To recover from an
error, add a condition such as:

```
check ──[ last.error ]──▶ fix
```

## Cycles and Step Limit

Cycles are allowed, for example to retry until tests pass. A run is limited to
**100 steps**; it stops at the limit instead of running indefinitely.

## Run a Graph

Select **Run** to start a graph; the view highlights steps as they execute.
When it finishes, a summary is added to the session conversation for follow-up
questions.

- **Run without asking permission** lets a graph proceed unattended, so tools
  marked as requiring permission are allowed. Explicitly denied tools remain
  blocked.
- A graph does not start while an agent turn, scheduled job or another graph
  is running; they share a worker.
- Closing the view or selecting **Stop** stops the graph.

## On-Disk Format

```json
{
  "name": "green tests",
  "desc": "Run the tests and fix failures.",
  "nodes": [
    { "id": "tests", "kind": "tool", "title": "Run tests",
      "tool": "bash", "args": "{\"command\":\"go test ./...\"}", "x": 40, "y": 40 },
    { "id": "fix", "kind": "agent", "title": "Fix failures",
      "task": "The tests failed:\n\n{{tests}}\n\nFix them.", "mode": "code", "x": 40, "y": 200 }
  ],
  "edges": [
    { "from": "tests", "to": "fix", "when": "tests conté \"FAIL\"" }
  ]
}
```

`x` and `y` specify node positions on the canvas. The engine ignores them, but
they are stored so reopening a graph restores its layout.
