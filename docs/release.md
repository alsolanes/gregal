# Release Checklist

1. Update the version in `main.go`, `desktop/package.json`, Android and VS
   Code metadata, and `CHANGELOG.md`.
2. Run `go test ./...`, `go vet ./...`, `go test -race ./...` on Linux amd64
   with CGO enabled, `cd desktop && npm test`, and the relevant VS Code tests.
3. From a clean checkout, build with `./build.sh vX.Y.Z`. In `dist/`, verify
   `sha256sum --check SHA256SUMS` and publish the binaries with the checksum.
   The build script uses `-trimpath` and disables VCS metadata for repeatable
   builds from the same source and version.
4. Build the desktop package with `cd desktop && npm run dist`. Test the
   AppImage with a local backend, and check `gregal --doctor` and
   `--check-config`.
5. Sign `dist/*` with the release key; keep the key out of the repository and
   publish the signature alongside the artifacts.
6. Run the backend contract tests and review
   [the HTTP API contract](api-contract.md).
7. Update the [compatibility matrix](compatibility.md), create an annotated
   tag, and verify that the installer does not overwrite a binary in use.

Artifact layout (all gitignored; names decide the home, not the date):
- `dist/` — Go CLI binaries only, versioned by `build.sh`
  (`gregal-vX.Y.Z-<os>-<arch>[.zip]`) plus `SHA256SUMS`. Never put
  installers or loose unversioned binaries here.
- `desktop/dist/` — local Electron builds (`npm run dist-win` writes here
  by default; CI overrides to `output/release/`). Installer sets
  (`Gregal-Setup-*`, `Gregal-Portable-*`, `latest.yml`, `.blockmap`).
- `output/release/` — CI release staging (verified installers, update
  metadata, checksums for upload). Do not build into it by hand.
- `desktop/backend/gregal.exe` — dev backend, rebuilt automatically by
  `npm start` / `build-backend.js`. Never copy it anywhere by hand.
- Installed app (`%LOCALAPPDATA%/Programs/Gregal/resources/backend/`) —
  only refreshed by running a matching installer, or by replacing that
  single file with a same-version build in an emergency (keep a `.bak`).

The root-level `gregal.exe` is a historical development binary retained for
existing local workflows. It is not a release package: `build.sh` compiles
from source and writes versioned artifacts to `dist/`. Do not replace it
manually or copy it into `dist/`.

The desktop package binds to loopback by default and chooses an available port;
it does not listen on the LAN by default.
