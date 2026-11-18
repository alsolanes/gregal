# Contributing

Gregal uses the Go version declared in `go.mod` (currently Go 1.27.1). CI also uses Node.js 24 and Python 3.10.

From the repository root, install the Python client and run the checks used in CI:

```sh
python -m pip install -e ./python
python -m unittest discover -s python/tests
node --test scripts/audit-public.test.cjs
go test ./...
go vet ./...
node --experimental-vm-modules --test internal/web/*.test.js internal/web/app/*.test.cjs desktop/*.test.js
```

Android and VS Code changes also need their platform builds and relevant tests. Keep changes focused, include regression coverage for behavior changes, and describe the checks you ran in the pull request.

English is the default interface language; Catalan is also supported. Keep translations consistent where both are available. Use synthetic examples and environment variables for credentials. Do not submit session history, screenshots of private workspaces, personal hostnames, user identifiers, or local configuration. Contributions are distributed under the repository's MIT license; preserve third-party notices.
