# Gregal

Open-source coding agent (Go + Bubble Tea) with web (`internal/web`), desktop (Electron, `desktop/`), Android (`android/`, Kotlin), Chrome and Telegram clients over one Go backend. The HTTP contract shared by the clients is `docs/api-contract.md`.

## Commands

- `go build -o gregal .` builds; `./gregal --check-config` validates the config.
- `go test ./internal/<pkg>` tests one package (`go test ./...` takes about 8 minutes).
- `uv run scripts/check.py` runs gofmt, vet, lint and the tests of what changed.

## Do not commit

`config.yaml` (it holds keys), binaries, session history or personal identifiers.
