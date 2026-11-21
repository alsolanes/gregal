---
title: "Els tests d'internal/web triguen menys"
status: done
depends_on: []
---

## Why

`go test ./internal/web` takes about four minutes alone (229 s on 2026-11-21, 911 tests, none parallel) and much longer when `scripts/check.py` runs it with every other touched package (39 minutes for the v1.89.0 integration). Six tests take 89 s of it, and the slowest are slow because of waits in the product, not in the tests.

## Acceptance criteria

- [x] Listing a folder (`GET /api/dirs`) is not held by a drive that does not answer: on Windows the drive letters are probed at the same time and the list is returned after at most 300 ms, without the ones that have not answered. `TestDirsLlistaCarpetesIRepos` goes from 21 s to under a second on this machine and now checks that the drive of the temp folder is among the roots; `TestArrelsNoEsperaUnaUnitatQueNoContesta` fakes a drive that never answers.
- [x] `TestWindInteractiveBashAutoAllow/mkdir dins` goes from 28.6 s to under two seconds without changing what it checks.
- [x] Applying the same language-server configuration again does not search the `PATH` again (`TestConfigureIgualNoTornaABuscar` in `internal/lsp`).
- [x] `go test ./internal/web -count=1` takes at most 150 s on this machine: 229 s before, 117 s after (912 tests, all passing).

## Out of scope

- `t.Parallel()` across the package: the tests share package-level state (`intervalProgres`, `llm.RetryBase`, the event store of `goalTestServer`) and were not written for it.
- Changing what any test asserts.
- Other packages.

## Notes

- Measured with `go test ./internal/web -json -count=1` (throwaway `HOME`/`USERPROFILE`, no `GREGAL_DATA_DIR`), nothing else running. Before: 229 s; tests of 1 s or more: 37, adding up to 142 s. After: 117 s; tests of 1 s or more: 19, adding up to 51 s. A run in between, with only the first two fixes and another test run going on at the same time, gave 199 s.
- **The drives** (`internal/web/dirs.go`). `arrels()` did `os.Stat` on `C:\` to `Z:\` one after another. On this machine `Y:\` is a network drive that is not connected and its stat takes 2 to 10 seconds, so the folder picker waited that long every time it opened. The first wording of this task asked for the system's list of logical drives (`windows.GetLogicalDrives`); that list still has the dead drive in it (checked: `Y:` and `Z:` are both there as remote drives and both fail), so a readiness check with a time limit is needed anyway, and probing every letter with a limit is the smaller change, with no Windows-only file. A healthy but slow drive (a disk spinning up) can miss one listing and shows in the next: nothing is cached. With a dead drive mapped, each listing costs the 300 ms and leaves one goroutine waiting in `os.Stat` until Windows gives up.
- **The empty-reply wait** (`internal/web/retry_main_test.go`). In the mkdir case the fake provider (`windPermissionProvider`) answers with the tool's output as text, and `mkdir -p` prints nothing, so the reply is empty and `llm.reintentaBuida` retries it after 2, 4 and 8 seconds. The product is right to retry; the wait is meant for real providers. `TestMain` now shortens `llm.EmptyRetryWait` as it already shortened `llm.RetryBase`, and as `internal/llm`'s own `TestMain` does. Giving the fake a non-empty answer instead would change what the test exercises.
- **The language servers** (`internal/lsp/manager.go`). This was the common cost: `lsp.Configure` threw away the list of available servers every time, each new web server applies the configuration (`web.New` → `agent.SetupPrices` → `agent.SetupLSP`), and finding the servers again walks the whole `PATH` for eight commands, about 250 ms here. Every test that creates a server and runs a turn paid it. `Configure` now returns at once when nothing changed. In the product this changes nothing: each front end applies the configuration once at start-up, and a language server installed while Gregal runs is picked up after a restart, as before. The `lsp` tests that swap `lookPath` without changing the configuration now reset the list themselves (`oblidaDisponibles`).
- Still slow, not touched: `TestTeamReviewerLateErrorReportsFailed` (12 s: its provider sleeps 2 s before each of six failing attempts, which is the late failure it reproduces), `TestBestOfAPI` (6 s, real git worktrees), `TestSaveTurnShotPrunesOldOnes` (4 s, many small files), `TestMateixaReglaDePermisosACodiIVents` (4 s for 26 cases, down from 15 s).
- To see where a slow test waits: `go test ./internal/web -run Name -count=1 -timeout 8s` and read the goroutine dump. For a cost spread over many tests, build the binary once (`go test -c`) and run it from `internal/web` with several `-test.timeout` values to sample the stacks.
- The race detector could not be run on this machine (it needs cgo).
- Follow-ups: `agent.SetupLSP` builds the custom servers by ranging over a map, so with two or more of them the order changes between calls and the early return is missed about half the time (sort by language). The `internal/web` tests never turn `lsp` off, unlike `internal/agent`'s `TestMain`, so their tool set depends on what is on the machine's `PATH`.
