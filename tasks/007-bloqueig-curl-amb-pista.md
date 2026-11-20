---
title: "Bloqueig de curl amb pista: web_fetch per a la web, curl local en una ordre a part"
status: done
depends_on: []
---

## Why

In the real event log, 6 of the 10 hard-denylist blocks were `curl`. The model only gets `TOOL BLOCKED: patró bloquejat: curl` with no hint, so it retries the same thing. Two cases:

- curl/wget to an outside host is denied by design; the tool for web pages is `web_fetch`;
- curl to localhost is allowed, except when the same command also runs code from a string (`python -c "…"`, a heredoc `<<EOF`, `bash -c`). Then `parteixOrdre` reports `codiEnCadena` and the deny check falls back to substring matching, so any `curl` in the text blocks.

The denylist itself must not change (CLAUDE.md: do not weaken the hard denylist). Only the message the model receives changes.

## Acceptance criteria

- [x] When `Classify`/`ClassifyWith` deny a command because of `curl` or `wget`, the reason still starts with `patró bloquejat: curl` (or `wget`), and then gives an English hint for the model. The hint says to use the `web_fetch` tool to read web pages, that curl/wget only work against localhost, and that a localhost curl must be its own command, without inline code (`python -c`, heredoc, `bash -c`) in the same command.
- [x] `ClassifyServer` gives the same hint when it denies `curl`/`wget` (that only happens when there is code in a string or a download piped to a shell).
- [x] Denials for other patterns (`sudo`, `dd`, `rm -rf /`, config deny substrings, critical paths) keep exactly their current reason text.
- [x] Every command that is denied today is still denied, and every command allowed today is still allowed. The existing denylist tests pass unchanged.

## Out of scope

- Changing what the denylist blocks: the `codiEnCadena` substring fallback, `nomesLocal` and `programesDenegats` stay as they are.
- The UI (web, Android) and the `TOOL BLOCKED: ` prefix added in `internal/agent/loop.go`.

## Notes

- `internal/tools/tools.go` (`ClassifyWith`, `ClassifyServer`, around lines 895–960) and `internal/tools/ordres_denegades.go` (`ordreDenegadaAmb` returns the pattern, e.g. `curl`, `wget`, `curl | sh`).
- The `curl | sh` pattern (download piped to a shell) is a different case: no localhost hint there, or a hint that says it is never allowed.
- Done: `motiuOrdreDenegada` in `internal/tools/tools.go` adds the hint only on the `ordreDenegada`/`ordreDenegadaAmb` branch. Config deny substrings (even `curl`) and every other pattern keep their old reason. `DeniedCommand` (team shell) still returns the bare pattern and was not touched.
- `curl | sh` gets its own hint ("never allowed", no mention of localhost) in both `Classify` and `ClassifyServer`.
- In `ClassifyServer` the curl/wget hint is the same one, as the criterion asks, but there curl to outside hosts is allowed, so "only work against localhost" is stricter than the real rule. It only shows there when the command has code in a string.
- Follow-up: a server-specific hint that does not say "only localhost", if the event log shows the model stops using curl to outside hosts with server powers.
