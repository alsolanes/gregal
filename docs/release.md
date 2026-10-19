# Releasing Gregal

There is one way to release. Everything that knows about versions lives in
`scripts/release.cjs` (rules in `scripts/release-lib.cjs`), and everything
that builds assets lives in `.github/workflows/release.yml`.

## Steps

1. **Write the changes** under `## Unreleased` in `CHANGELOG.md` as you go.
2. **Bump** on a branch:

   ```
   node scripts/release.cjs bump 1.9.1 --title "Short release title" [--android 0.9.9]
   ```

   This sets the version in `main.go`, `desktop/package.json` and its lock
   file, turns `## Unreleased` into `## v1.9.1 - <date> - <title>` (a fresh
   empty `## Unreleased` stays on top) and, with `--android`, bumps the
   Android `versionName` and `versionCode`. Open a PR; the Quality workflow
   runs `node scripts/release.cjs check`, which fails if any of these drift.
3. **Merge, then tag** `main`:

   ```
   git tag -a v1.9.1 -m "Gregal v1.9.1" && git push origin v1.9.1
   ```

   The Release workflow checks that the tag, the versions and the changelog
   agree, builds the Windows installer, portable app and update metadata, the
   CLI/backend for Linux, macOS and Windows (amd64 and arm64), and the VS Code
   extension package (`gregal-X.Y.Z.vsix`) with
   `build.sh`, writes one `SHA256SUMS.txt`, and creates a **draft** release
   titled and described from the changelog section.
4. **Attach the Android APKs** (built locally because they are signed with
   this machine's key, so phones can update over the previous version):

   ```
   node scripts/release.cjs apk 1.9.1
   ```

   The APKs include the Go backend for the "This device" mode, so this
   machine needs Go and, preferably, an Android NDK in the SDK (with an
   NDK the backend uses the system DNS resolver). See `docs/mobile.md`.

   Gradle builds one APK per ABI plus a universal one, and the command
   attaches all three with `gregal-android-version.json`, the in-app
   updater's manifest. Its top-level `downloadUrl`, `sha256` and
   `sizeBytes` describe the universal APK (older apps read only those), and
   `apks` has one entry per ABI; the app downloads the first of its
   `Build.SUPPORTED_ABIS` listed there. All APKs share the `versionCode`
   and the signing key, so any of them installs over any other.

5. **Review the draft on GitHub and publish it.** The desktop app's
   automatic updates only see published releases.

## Assets

| File | What it is |
| --- | --- |
| `Gregal-Setup-X.Y.Z-x64.exe` | Windows installer with automatic updates (`latest.yml` + `.blockmap`) |
| `Gregal-Portable-X.Y.Z-x64.exe` | Portable Windows app |
| `Gregal-VSCode-Setup-x64.exe` | Standalone Windows installer for the VS Code extension and backend |
| `gregal-<os>-<arch>[.exe]` | Standalone backend/CLI (linux, darwin, windows × amd64, arm64) |
| `gregal-X.Y.Z.vsix` | VS Code extension; install from Extensions → **…** → **Install from VSIX…** |
| `Gregal-Android-A.B.C-arm64-v8a.apk` | Android app for phones (about 27 MB), attached by step 4 |
| `Gregal-Android-A.B.C-x86_64.apk` | Android app for x86_64 emulators |
| `Gregal-Android-A.B.C.apk` | Universal Android app, both backends (about 42 MB) |
| `gregal-android-version.json` | In-app updater manifest for the APKs above |
| `SHA256SUMS.txt` | Checksums for every CI-built file |

Windows packages are unsigned.

## Version lines

- **Gregal** (CLI, server, desktop): `main.go` and `desktop/package.json` —
  always equal, checked in CI.
- **Android**: its own `versionName` in `android/app/build.gradle.kts`,
  bumped with `--android` when the app changes.
- **VS Code extension**: its own version in `vscode-gregal/package.json`.

## Local files

All gitignored:

- `dist/` — output of `./build.sh` (same names as the release assets). It
  is wiped and rebuilt on every run.
- `desktop/dist/` — local Electron builds (`npm run dist-win`).
- `output/` — scratch space, e.g. `output/release-X.Y.Z/` for the APKs
  and manifest attached in step 4. Nothing here is part of a release by itself.
