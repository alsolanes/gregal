# Gregal

[English](README.md) | [Catala](README.ca.md)

Gregal is an open-source coding agent with terminal, desktop and web interfaces, plus Android, Telegram and VS Code clients. It connects to OpenAI-compatible model endpoints and includes project tools, approvals, checkpoints, MCP and persistent sessions.

English is the default interface and agent-response language. Catalan is available with `lang: ca` in clients that support it.

## Getting Started

Requires Go 1.27.1 or newer. From the repository root:

```sh
go run . init
```

Configure a working endpoint and model IDs before the first request, then launch the terminal interface with `go run .`. The generated endpoints and model IDs are examples. See the [Getting Started guide](docs/getting-started.md) for provider setup, local web access and data-handling details. Catalan: [Primers passos](docs/getting-started.ca.md).

To run the web interface locally, use `go run . --serve` and open the URL printed in the terminal. For other clients and the HTTP API, see the [API contract](docs/api-contract.md) and [compatibility notes](docs/compatibility.md).

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
