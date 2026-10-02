# Stable Source Baseline

Verified locally on Windows on 2026-10-02. This baseline is intended for source
reuse and development, not production approval.
It keeps the existing component versions: Go/desktop 1.6.1, Android 0.9.7 and
VS Code 0.3.1. A source ZIP contains current source, not signed applications.

When importing a ZIP on Unix, restore the executable bit on entry-point scripts:

```sh
chmod +x build.sh android/gradlew evals/run.sh
```

If initializing Git on Windows, record those bits with
`git update-index --chmod=+x build.sh android/gradlew evals/run.sh` after adding
the files. ZIP extraction does not reliably preserve Unix executable permissions.

## Verification

| Check | Result |
| --- | --- |
| `go test ./...` and `go vet ./...` | Passed in the working tree and clean export |
| Go vulnerability scan | No vulnerabilities found |
| Web/desktop JavaScript suite | 52 tests passed |
| VS Code compilation and tests | Passed, 16 tests |
| Python SDK tests, including authenticated Go backend smoke test | 10 tests passed |
| Android unit tests and debug assembly | Passed, 71 tests |
| Windows Electron unpacked build and authenticated startup | Passed; managed backend stopped on exit |
| Desktop and VS Code clean npm installs/audits | Zero reported vulnerabilities |
| Privacy scanner regression tests | 4 tests passed |
| Vendor hashes and spreadsheet round-trip | Passed |

The live contract and desktop tests used isolated profiles and a synthetic
provider without invoking a model. They do not validate a real model's tool
calling, RAG retrieval, analysis execution or plotting.

## Included Boundaries

- Go is the implemented agent engine; Python is an HTTP client SDK, not a port.
- The harness supports the shared HTTP backend and verified-identity integration.
- Telegram is an optional integration outside the harness HTTP contract.
- English is the default and Catalan is selectable. Some secondary UI text,
  legacy command syntax and backend diagnostics remain untranslated.
- The runtime is not a multi-tenant security boundary. Some configuration and
  integrations use process-wide state; run one configuration/runtime per process.
- Tool approvals and filesystem-root checks do not replace an OS-level sandbox.

## Before Production Deployment

Implement and validate SSO, document permissions, secret management, retention,
audit requirements and execution isolation for the actual deployment. Choose
and test the model, RAG connectors and analysis/plotting workflow. Use synthetic
fixtures in source control and keep company data in private storage.

Linux/macOS runtime QA, physical Android device QA, native-dialog coverage and
release signing remain binary-distribution checks. GitHub CI must pass on the
chosen repository; its remote execution was not performed by these local tests.
Enable private vulnerability reporting before public publication. See the
[publication audit](open-source-audit.md) and [harness guide](enterprise-harness.md).
