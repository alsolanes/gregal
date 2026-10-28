# Gregal

[English](README.md) | [Catala](README.ca.md)

Gregal is an open-source coding agent with terminal, desktop and web interfaces, plus Android, Telegram and VS Code clients. It connects to OpenAI-compatible model endpoints and includes project tools, approvals, checkpoints, MCP and persistent sessions.

English is the default interface and agent-response language. Catalan is available with `lang: ca` in clients that support it.

## Install

**Windows desktop app (recommended).** Download `Gregal-Setup-<version>-x64.exe`
from the [latest release](https://github.com/alsolanes/gregal/releases/latest)
and run it. It includes the backend and updates itself from GitHub releases;
`Gregal-Portable-<version>-x64.exe` runs without installing (no automatic
updates). The installer is not code-signed yet: if Windows SmartScreen stops it,
choose **More info → Run anyway**. From the app you can also prepare the Chrome
extension and install the Word add-in (Preferences).

**Android (Gregal Vents).** Install `Gregal-Vents-<version>-arm64-v8a.apk` from
the release that carries it (currently
[v1.23.0](https://github.com/alsolanes/gregal/releases/tag/v1.23.0)); from 1.35.0
on, the app updates itself.

**From source (any OS).** See below.

## Getting Started

Requires Go 1.27.1 or newer. From the repository root:

```sh
go run . init
```

Configure a working endpoint and model IDs before the first request, then launch the terminal interface with `go run .`. The generated endpoints and model IDs are examples. See the [Getting Started guide](docs/getting-started.md) for provider setup, local web access and data-handling details. Catalan: [Primers passos](docs/getting-started.ca.md).

To run the web interface locally, use `go run . --serve` and open the URL printed in the terminal. For other clients and the HTTP API, see the [API contract](docs/api-contract.md) and [compatibility notes](docs/compatibility.md).

### VS Code

The one-click `Gregal-VSCode-Setup-x64.exe` and the `install-vscode.ps1` command
need a release with the `.vsix` package and the standalone backend, and current
releases do not attach them (the last one was v1.13.1). Until they return, build
the extension from source as described in the
[VS Code guide](vscode-gregal/README.md) and point it at the desktop app's
backend (`http://127.0.0.1:8097`) or at `go run . --serve`.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and checks. Report security issues according to [SECURITY.md](SECURITY.md). The [user manual](docs/manual.md) is in English and [Catalan](docs/manual.ca.md); the [Getting Started guide](docs/getting-started.ca.md) is also available in Catalan.

## License

Gregal is distributed under the MIT license; see [LICENSE](LICENSE). Bundled third-party components retain their own licenses and notices.

## Release Preparation

The [stable source baseline](docs/stable-baseline.md) records local verification
and known limitations for source reuse.

Maintainers should follow the [GitHub publication checklist](docs/github-release.md)
and [publication audit](docs/open-source-audit.md). The sanitized source export
does not include the private Git history; source preparation is not approval to
distribute unvalidated desktop or mobile binaries.
