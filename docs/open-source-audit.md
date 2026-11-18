# Source Quality and Security Checks

See the [verified source baseline](stable-baseline.md) for locally executed
checks and the [release checklist](github-release.md) for publication steps.

## Data Boundaries

Never include credentials, local configuration, conversations, memory or
deployment data in source releases. Source exports exclude Git metadata and
local runtime artifacts. Preserve required copyright and third-party notices.
Review bundled images and other binary assets manually.

The privacy scanner checks supported credential formats and runtime files.
Its results are heuristic, not proof that every sensitive value is absent.
Additional literal markers can be provided through `GREGAL_PRIVATE_MARKERS`,
a JSON array in the environment; do not commit deployment-specific markers.

## Runtime Security

- Non-loopback listeners require authentication. Remote access also requires TLS.
- Desktop navigation uses parsed origins and restricts native IPC to trusted
  application frames. Desktop tokens are transferred through the guarded bridge,
  not URL parameters or persisted browser storage.
- Embedded browser guests have no Node integration or application preload access.
- Run one harness runtime/configuration per process. Application authorization
  checks are not a multi-tenant or operating-system security boundary.
- Review connectors and tool permissions. Use a separate sandbox for untrusted
  code execution; a prompt is not an access-control mechanism.
- Shutdown cancels work but does not drain arbitrary executors or automatically
  close an embedding application's HTTP server.

## Dependencies and Verification

Full npm audits include Electron even though it is a development dependency.
Go vulnerability checks cover Go dependencies; they do not audit Android or
vendored browser parsers. Vendor verification checks artifact hashes and an
XLSX round-trip; complete notices accompany the bundled components.

The [baseline report](stable-baseline.md) records Go, web/desktop, Python,
VS Code and Android checks. Python is an HTTP SDK for the Go backend.
The authenticated SDK smoke test does not validate model inference or connectors.

## Known Limits

English is the default and Catalan is selectable. Some secondary UI text,
legacy command syntax and backend diagnostics remain untranslated. Windows
startup and local packaging were tested; Linux/macOS runtime QA, physical-device
QA, native-dialog coverage and binary signing require separate validation.
Remote CI results must be checked before release. Configure a private security
reporting channel and distribution/update endpoints appropriate to each release.

## Repeat Checks

```sh
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
node --experimental-vm-modules --test internal/web/*.test.js internal/web/app/*.test.cjs desktop/*.test.js
node --test scripts/audit-public.test.cjs
node scripts/verify-vendor.cjs
node scripts/audit-public.cjs .
```

See [the harness guide](enterprise-harness.md) for the shared HTTP contract,
identity integration and deployment boundaries.
