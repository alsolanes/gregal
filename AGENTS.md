# AGENTS.md — gregal

Coding agent (Go + Bubble Tea). Read this before making changes.

## Commands

- 'go build -o gregal .' — build
- 'go test ./...' — tests (must pass before committing)
- './gregal --check-config' — validate the config

## Conventions

- Reply in the user's chosen language (English by default; Catalan supported).
- Tools that write ask for confirmation; the hard denylist blocks on its own.
- '/verify' calls the reviewer model with the diff included; without an
  explicit verdict there is no approval.
- Do not commit 'config.yaml' (it holds keys) or binaries.
