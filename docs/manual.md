# Gregal User Manual

[Català](manual.ca.md)

Gregal is a coding agent with terminal, desktop and web interfaces. It sends
model requests to the endpoint configured for the selected role. This guide
covers the common setup and safety boundaries; use the in-app help and the
linked references for details that depend on your build.

## Requirements

- Go 1.27.1 or newer to run from source.
- An OpenAI-compatible model endpoint and model IDs available from that endpoint.

The endpoint and model must support the capabilities you intend to use, such
as tool calling. Configuration validation does not test network access,
credentials or model availability.

## Initialize and Configure

From the repository root, create the initial configuration:

```sh
go run . init
```

On Unix-like systems, the default path is `~/.config/gregal/config.yaml` and
the file is created with mode `0600`. The command also creates an `AGENTS.md`
in the current project directory if one does not already exist. Choose a
different config path with `go run . init --config=/path/to/config.yaml`.

Set the provider URL and model IDs for the `chat`, `think`, `code` and
`reviewer` roles. The generated `local` and `local-direct` URLs refer to
loopback services that Gregal does not start. The generated `cloud` URL is an
example; replace it and the model IDs with values supported by your endpoint.
You can assign multiple roles to one provider if it supports their required
capabilities.

The example cloud provider reads its key from an environment variable:

```yaml
providers:
  cloud:
    base_url: https://api.example.org/v1 # replace with your endpoint
    api_key: ${GREGAL_CLOUD_API_KEY}
```

Set the variable in the environment used to launch Gregal, or use a secret
manager. Do not commit credentials or private configuration.

## Run the Terminal Interface

Check the configuration and start Gregal:

```sh
go run . --check-config
go run .
```

`--check-config` checks configuration structure and references only. In the
terminal interface, `/help` lists available commands and `/model` opens a
picker for models advertised by configured endpoints.

## Run the Local Web Interface

Start the server on loopback:

```sh
go run . --serve --addr 127.0.0.1:8097
```

Open `http://127.0.0.1:8097/` in a browser. Gregal rejects non-loopback
listeners unless a token or user authentication is configured. The server
does not provide TLS; configure HTTPS before making it reachable outside a
trusted local machine.

## Data and Tool Execution

Prompts, conversation context, and any code or files included in a model
request are sent to the endpoint configured for that role. A provider named
`local` is local only if the service actually runs on your machine; a local
proxy may forward requests elsewhere. Web retrieval and configured MCP
connectors can also make network requests.

File and shell tools run on the host in the selected project. Gregal's tool
permissions and confirmation prompts are application controls, not an
operating-system sandbox. Review proposed changes and commands. For untrusted
code, use a disposable checkout or operating-system-level isolation.

## References

- [Getting Started](getting-started.md)
- [HTTP API contract](api-contract.md)
- [Compatibility notes](compatibility.md)
- [Security policy](../SECURITY.md)
- [Contributing](../CONTRIBUTING.md)
