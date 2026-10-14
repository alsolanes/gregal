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
   CLI/backend for Linux, macOS and Windows (amd64 and arm64) with
   `build.sh`, writes one `SHA256SUMS.txt`, and creates a **draft** release
   titled and described from the changelog section.
4. **Attach the Android APK** (built locally because it is signed with this
   machine's key, so phones can update over the previous version):

   ```
   node scripts/release.cjs apk 1.9.1
   ```

5. **Review the draft on GitHub and publish it.** The desktop app's
   automatic updates only see published releases.

## Assets

| File | What it is |
| --- | --- |
| `Gregal-Setup-X.Y.Z-x64.exe` | Windows installer with automatic updates (`latest.yml` + `.blockmap`) |
| `Gregal-Portable-X.Y.Z-x64.exe` | Portable Windows app |
| `gregal-<os>-<arch>[.exe]` | Standalone backend/CLI (linux, darwin, windows × amd64, arm64) |
| `Gregal-Android-A.B.C.apk` | Android app, attached by step 4 |
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
- `output/` — scratch space, e.g. `output/release-X.Y.Z/` for the APK
  attached in step 4. Nothing here is part of a release by itself.
