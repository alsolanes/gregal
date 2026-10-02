# Enterprise Chat Harness

[Catala](enterprise-harness.ca.md)

Gregal implements its agent engine in Go. The harness embeds and extends the
shared HTTP backend. Telegram is a separate integration outside the harness HTTP
contract. The Python SDK is an HTTP client, not a Python agent engine.

## Supported Paths Today

`harness.New` embeds the same Go runtime and HTTP handler used by `gregal serve`.
The web app, desktop, Android, VS Code and connected TUI use that HTTP contract.
The local TUI, headless runner and upstream Telegram client still call the Go
engine directly; those paths are not exposed by this HTTP harness.

The Go `client` package and the async SDK in `python/` are HTTP clients for the
existing Go service. The Python SDK does not run the agent engine. Supported
extension points are an OpenAI-compatible model endpoint, server-side MCP stdio
connectors, a custom client built against the HTTP contract, and an embedding
application's `Options.Authenticate` callback for verified identity-provider
principals. Connector commands and credentials stay on the server.

Run one runtime and configuration per process. Tool integrations and provider
caches currently include process-wide state, and some API surfaces such as
providers and scheduled jobs are application-wide. Separate deployments with
private configuration, data directories, workspaces and operating-system
permissions. The current runtime is not a certified multi-tenant security
boundary.

## Start a Deployment

Copy `examples/company-chat/config.example.yaml` to a private file and replace
the example endpoint and model names. It targets an OpenAI-compatible service
on loopback; a local endpoint can leave the model API key empty. The provider
key accepts `${GREGAL_MODEL_API_KEY}` when the endpoint requires a secret.

For a local smoke deployment on a Unix-like host:

```sh
install -m 600 examples/company-chat/config.example.yaml /private/gregal.yaml
mkdir -p /private/gregal-data && chmod 700 /private/gregal-data
export GREGAL_DATA_DIR=/private/gregal-data
export GREGAL_API_TOKEN="$(openssl rand -hex 32)"
# Set GREGAL_MODEL_API_KEY from a secret manager if the model endpoint requires it.
go run ./examples/company-chat -config /private/gregal.yaml -addr 127.0.0.1:8097
```

The runner binds to loopback and serves the existing UI by default. Use
`-ui=false` to expose only `/api/`. A shared `GREGAL_API_TOKEN` is a single
service/admin identity; use it only with a trusted local UI or a trusted
server-side client. Do not put it in an untrusted browser or mobile app. For
For employee sign-in in the built-in UI, add native `users:` to the private
config. Set each password with `GREGAL_DATA_DIR` pointing to the service data
directory and `gregal --config /private/gregal.yaml --passwd <user>`. For an
IdP-backed client, embed the handler and supply `Options.Authenticate`. The
example runner demonstrates the shared-token path; it does not configure an
IdP callback. The callback must verify the IdP session/JWT itself, derive a
stable namespace with `harness.AccountID(verifiedSubject)`, and assign roots,
home and admin rights from trusted server policy. For each `AccountID`, the
runtime pins the normalized roots, home and admin policy on first use; policy
changes are rejected until the runtime restarts. Never derive identity or
filesystem access from browser-supplied headers.

For remote access, terminate TLS at the ingress. If it supplies user
assertions, have `Authenticate` verify signed assertions from a trusted issuer;
do not trust forwarded identity headers by themselves. Keep provider keys, tokens,
configuration and the persistent data directory readable only by the service
account; back up that data under the same access controls. Run with a
least-privilege operating-system account. Filesystem roots and tool approvals
are application checks, not an OS sandbox. Put untrusted code execution in a
separate sandbox with explicit filesystem and network limits. Review every
exposed API route and configured connector before production use. During
shutdown, `CloseContext` cancels active and queued runs and waits for the
scheduler, but does not wait for tool executors to exit or close the embedding
application's HTTP server. Shut down both components with a bounded policy
that matches the deployment's workload.

## Python Client

Install the HTTP SDK:

```sh
python -m pip install -e ./python
```

Use a token for one authenticated principal. Native login returns a user token
from `/api/login`; an embedded application can issue its own after verifying
the IdP identity. Keep it server-side:

```python
import asyncio
import os
import uuid
from gregal_client import Client

async def main():
    async with Client(os.environ["GREGAL_URL"], os.environ["GREGAL_USER_TOKEN"]) as backend:
        await backend.health()
        session = await backend.open_session(title="Document question")
        request_key = str(uuid.uuid4())
        submitted = await backend.submit(
            session["id"],
            "Summarize the approved document",
            idempotency_key=request_key,
        )
        async for event in backend.stream(session["id"], after=submitted["cursor"]):
            print(event)
            if event.get("kind") in {"run_completed", "run_failed", "run_cancelled"}:
                break

asyncio.run(main())
```

Persist each consumed event ID and reconnect from the last cursor. Persist and
reuse the same idempotency key when retrying a submission whose response was lost. Render
structured approval and question payloads and send a decision with `approve`
or `answer`; never approve tool use automatically. Stop long-lived streams
when their consumer is done and handle disconnects. Use HTTPS outside a trusted
local network.

## Modes and Connectors

The supported modes are `chat`, `inspect`, `code`, `goal` and `autonomous`.
Keep the selected mode and tool policy explicit. MCP calls are disabled in
`chat` and `inspect`; approved-source retrieval through MCP and analysis that
writes plots require `code` plus explicit tool permissions. `set_mode` changes
the session capability set; a per-turn mode can only restrict it. Retrieval
authorization and citations belong in the server or connector. A prompt does
not enforce access control. Review connector executables, arguments, secrets
and network access as server-side code.

## Backend Compatibility

A Python client can use this Go service through the HTTP SDK. Implementations must preserve the documented
HTTP contract in `docs/api-contract.md` and capability negotiation in
`docs/compatibility.md`, including health/protocol identity, sessions, v2 runs,
cancellation, idempotency, durable event cursors, interactive approval and
question payloads, authentication, workspace boundaries, and mode/permission
semantics. Run conformance tests before switching backend implementations;
the current Python SDK tests cover its transport, not an alternative engine.
Keep credentials and deployment configuration outside source control.
