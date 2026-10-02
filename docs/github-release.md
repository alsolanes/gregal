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
Do not attach debug-signed APKs or unvalidated Electron packages. Configure
independent publisher IDs, signing credentials and update endpoints outside the
public source; test those release workflows on their supported platforms.

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
