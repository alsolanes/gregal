---
title: "Accés complet esborra el «demana abans de canviar» desat"
status: done
depends_on: []
---

## Why

Choosing «full» access (`POST /api/access`) leaves an old `permissions.access: ask` in the config, while the menu shows «full» (`AccessOf` reports full for the autonomous mode). Since task 018 winds follow `AskEdits`, so they ask before every write, test, local install and app start in their own folder, and never act on their own.

## Acceptance criteria

- [x] `handleAccess` (`internal/web/serve.go` ~3678) clears `cfg.Permissions.Access` when the requested access is full, as it does for edits, so `AskEdits()` is false afterwards and the saved config no longer says `ask`. A Go test: config starts with `access: ask`, POST full, then `AskEdits()` is false, the policy's `AskEdits` is false and the saved file has no `ask`.
- [x] A Go test through the wind path: after that POST, a wind turn's policy (`windLoopScope`) lets a `write` in the workspace and a project `npm test` through without asking, while `git push` and `npm i -g` still ask.
- [x] `uv run scripts/check.py` passes.

## Out of scope

- Changing task 018's rule that winds follow «ask before changing». The hard denylist, `windRisky`, first public publish and writes outside the workspace stay as they are.

## Notes

- `internal/web/serve.go` `handleAccess`; `internal/web/winds_permissions.go` `windLoopScope` (copies `AskEdits`), `windTurnMode`; `internal/agent/loop.go` ~778/~797 (`actRule`, `WindAutoAllow` only when `!AskEdits`); tests near `internal/agent/windauto_test.go` and the access × action table from commit 72e53bb8.
- Clients: web, desktop and Android only post `/api/access`; the contract does not change, but check `docs/api-contract.md` says what full does.
- Done: `handleAccess` clears `Permissions.Access` for both edits and full (only ask sets it), so the policy's `AskEdits` follows. Tests in `internal/web/access_test.go`: `TestAccesCompletEsborraElDemanaDesat` (config, policy and saved file) and `TestAccesCompletDeixaTreballarElsVents` (a wind turn's policy after full: workspace write and `npm test` allowed, `git push` and `npm i -g` ask). Both fail without the fix. The contract does not describe the saved field, so it is unchanged.
