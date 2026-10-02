# Security

Do not post credentials or exploitable vulnerability details in public issues. Use a private security report through the hosting platform if enabled. A private reporting channel must be configured before the public launch.

Gregal runs tools on the host machine. Review tool permissions, MCP servers, shell commands, and approvals before enabling autonomous execution. Use a dedicated workspace and operating-system account for untrusted projects.

Keep the server on loopback for local use. Remote access requires authentication and a TLS proxy. Model providers receive prompts and the project content attached to them; web, GitHub, Telegram, and MCP integrations contact their configured services. Conversations, memory, credentials, and checkpoints may be stored locally. Do not publish runtime data with source code.

Release maintainers must configure distribution and update endpoints. Automatic updates require an explicitly configured release channel.
