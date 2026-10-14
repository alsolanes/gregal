# GitHub Action: gregal review

`.github/actions/gregal-review` is a reusable composite action that reviews a
pull request with gregal and posts the result as a PR review comment. It:

1. builds gregal from the source the action lives in (Go from `go.mod`);
2. downloads the PR diff with `gh pr diff` (truncated to `max-diff-bytes`);
3. runs `gregal -p "<review prompt + diff>" --mode inspect --output json`.
   Inspect mode is read-only: no writes, no background processes, no MCP,
   and only shell commands classified as read-only;
4. posts the answer with `gh pr review <n> --comment`.

This repository does not enable any workflow that uses it. To use it, add a
workflow to the repository you want reviewed.

## Required secrets and permissions

| What | Why |
|---|---|
| `GREGAL_CONFIG` (secret, recommended) | The contents of a gregal `config.yaml` with the providers and roles for the review. Keep API keys as `${VAR}` references in it. Alternatively commit a config without secrets and use `config-path`. |
| Provider key, e.g. `ZEN_API_KEY`, `OPENAI_API_KEY` (secret) | Passed to the step through `env:`; the config expands `${VAR}`. |
| `GITHUB_TOKEN` permissions `pull-requests: write`, `contents: read` | `gh pr diff` and `gh pr review --comment`. The default `github.token` is used unless `github-token` is set. |

Secrets are not available to workflows triggered by pull requests from forks.
Do **not** switch to `pull_request_target` together with checking out the PR
head: that would run gregal on untrusted code with your secrets. The PR content
is untrusted input for the model as well (prompt injection); inspect mode keeps
the run read-only, but review the comment as you would any automated output.

## Example workflow

```yaml
name: gregal review
on:
  pull_request:
    types: [opened, synchronize, ready_for_review]
permissions:
  contents: read
  pull-requests: write
jobs:
  review:
    if: github.event.pull_request.draft == false
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.pull_request.head.sha }}
      - uses: alsolanes/gregal/.github/actions/gregal-review@main   # pin a tag or SHA
        env:
          ZEN_API_KEY: ${{ secrets.ZEN_API_KEY }}
        with:
          config: ${{ secrets.GREGAL_CONFIG }}
          max-steps: "30"
```

Replace the `uses:` owner/repository with wherever your copy of gregal lives,
and pin a tag or commit SHA.

## Inputs

| Input | Default | Meaning |
|---|---|---|
| `config` | | Config YAML contents (from a secret). |
| `config-path` | | Path to a config file in the checked-out repository (used if `config` is empty). |
| `pr-number` | event PR | Pull request to review. |
| `github-token` | `github.token` | Token for `gh`. |
| `max-steps` | `30` | Hard step limit (`--max-steps`). |
| `max-diff-bytes` | `90000` | Diff size sent to the model (larger diffs are truncated and the prompt says so). |
| `instructions` | | Extra instructions appended to the review prompt. |
| `post-comment` | `true` | `false` only writes the review file (output `review-file`). |
| `fail-on-error` | `false` | Fail the job when gregal fails or posting fails. |

Outputs: `review-file` (Markdown review) and `exit-code` (gregal's exit code).

gregal's state (`GREGAL_DATA_DIR`, `GREGAL_AUTH`, `GREGAL_MEMORY`) is kept in
the runner's temporary directory. The action expects a Linux runner with `bash`,
`jq` and `gh` (all present on `ubuntu-latest`).
