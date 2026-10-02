# Getting Started

[Catala](getting-started.ca.md)

Gregal runs on your machine and sends model requests to the endpoint configured for each role. It supports OpenAI-compatible APIs; the endpoint and model must support the capabilities you plan to use, such as tool calling.

## Requirements

- Go 1.27.1 or newer.
- An OpenAI-compatible model endpoint and model IDs available from that endpoint.

## Build and Configure

From a checkout of the repository, create the initial configuration:

```sh
go run . init
```

By default, the config is written to `~/.config/gregal/config.yaml`; on Unix-like systems it is created with mode `0600`. `init` also creates an `AGENTS.md` in the current project directory if one does not already exist. Use `go run . init --config=/path/to/config.yaml` to choose another config path.

New configurations include public provider endpoints and start with OpenAI's `gpt-4.1-mini` for all roles. Set `OPENAI_API_KEY` to use that default, or select another [provider preset](provider-presets.md) in the web/desktop provider settings. You can enter its key directly in the preset form and choose **Use for all roles**. Existing configurations are not replaced.

The template reads keys from provider-specific environment variables:

```yaml
providers:
  openai:
    base_url: https://api.openai.com/v1
    api_key: ${OPENAI_API_KEY}
```

Set that variable in the environment used to launch Gregal, or use your secret manager. A local endpoint may not require a key. Avoid committing credentials or private configuration.

Check the local config, then launch the TUI:

```sh
go run . --check-config
go run .
```

`--check-config` checks the configuration structure and references; it does not test network access, credentials or model availability. In the TUI, `/model` opens a live picker for models advertised by the configured endpoints.

## Local Web Interface

Start the web interface on loopback:

```sh
go run . --serve --addr 127.0.0.1:8097
```

Open `http://127.0.0.1:8097/` in a browser. The default local listener is restricted to this machine. Gregal rejects non-loopback listeners unless a token or user authentication is configured. The server does not provide TLS; configure HTTPS before making it reachable outside a trusted local machine.

## Data and Runtime Boundaries

Prompts, conversation context and any code or files included in model requests are sent to the endpoint selected for that role. A URL named `local` is only local if the service at that URL runs on your machine; a local proxy can still forward requests elsewhere. Web retrieval and configured MCP connectors can also make network requests.

File and shell tools run on the host in the selected project. Gregal's tool permissions and confirmations are application controls, not an operating-system sandbox. Review proposed changes and commands, and use a disposable checkout or your own OS-level isolation for untrusted code.

Providers and models differ in tool-calling, context, image and other capabilities. The example configuration is not a hosted model subscription, and successful config validation does not imply endpoint compatibility. Check your provider's API behavior and terms for the data you send.

## More

- [HTTP API contract](api-contract.md)
- [Compatibility notes](compatibility.md)
- [Security policy](../SECURITY.md)
- [Contributing](../CONTRIBUTING.md)
