# GitHub Publication Checklist

This checklist prepares a new source repository. It does not authorize pushing
the existing history or publishing binaries.

## Before Creating the Repository

- Resolve the blockers in [the audit](open-source-audit.md), including vendored
  license texts, asset provenance and remaining untranslated primary flows.
- Choose the public owner, repository name and copyright attribution. Preserve
  required upstream and third-party notices.
- Review icons, screenshots and bundled files manually for private information.
- Run `node scripts/prepare-public.cjs` from the private source checkout. Existing
  exports are never overwritten; archive a reviewed candidate before regenerating.
- Run tests in the exported tree as well as the private source. Set
  `PYTHONDONTWRITEBYTECODE=1` when testing Python in the export to avoid caches.
- Run `node scripts/audit-public.cjs public-release` after verification.

## New Repository

Initialize Git only inside the reviewed `public-release/` directory. Do not copy
the private `.git` directory, branches, tags, remote configuration or release
artifacts. Do not push the private checkout. Review the staged file list before
the first commit; configure the intended public Git author identity separately.

Use an empty GitHub repository and a fresh initial commit. Its URL is deliberately
not preconfigured here. Before making the repository public, enable private
vulnerability reporting and ensure SECURITY.md accurately describes the channel.
Enable CI and branch protection appropriate to the maintainers. Verify the first
Linux/Windows CI runs; local Windows tests do not prove Linux runtime behavior.

## Source Versus Binaries

A source publication is separate from desktop, Android and extension releases.
Do not attach debug-signed APKs or unvalidated Electron packages. Keep signing
credentials outside the public source. The Windows desktop update destination
is explicitly configured in `desktop/package.json`; forks must change its
public GitHub owner/repository before distributing their own installers.

## Windows Releases and Updates

Build locally with `npm ci` and `npm run dist-win` inside `desktop`.
The NSIS installer supports automatic updates from published GitHub Releases.
The portable EXE does not auto-update: replace it manually. The standalone
backend also does not use the desktop updater; its `update` command requires
a source checkout and Go, rather than downloading a release binary.

The `Windows Release` workflow runs when a matching `v<desktop-version>` tag
is pushed. It tests the source, builds the installer and portable, and creates
a **draft** release with `latest.yml`, blockmap, standalone backend and
`SHA256SUMS.txt`. The job uses GitHub's ephemeral token, never a bundled token.
Version values in `main.go`, `desktop/package.json` and its lockfile must agree.

Review and test the draft installer, then publish the release from GitHub.
Drafts are invisible to the updater. Keep the installer and update metadata
from the same build, and never replace the assets of a published version.
Run `node desktop/verify-release.js output/release` before uploading: it
validates that the update metadata points to the matching NSIS installer and
that its size and SHA512 checksum match. Preserve `latest.yml` and the
installer blockmap when cleaning build outputs.

Installed desktop applications check at launch and show the result in
**Preferences → App updates**. Users can check again, explicitly download a
newer published version, then restart to install. Downloaded updates are not
installed automatically on ordinary app exit. The portable opens the official
download page so users can replace its executable. Updating the desktop also updates its bundled
backend; a separately hosted backend is managed independently.

These initial Windows packages are unsigned. Windows may show SmartScreen
warnings. GitHub HTTPS and update checksums are not publisher authentication;
configure code signing in CI before requiring signed enterprise distribution.
macOS, Linux, Android and VS Code release/update publishing remain separate.

## Final Verification

```sh
go test ./...
go vet ./...
node --experimental-vm-modules --test internal/web/*.test.js internal/web/app/*.test.cjs desktop/*.test.js
node --test scripts/audit-public.test.cjs
node scripts/verify-vendor.cjs
python -m pip install -e ./python
python -m unittest discover -s python/tests
```

Dependency vulnerability scans must be repeated at publication time. They do
not replace vendored-library review, security testing or tool-execution isolation.
Never include real prompts, corporate documents, accounts or tokens as fixtures.
