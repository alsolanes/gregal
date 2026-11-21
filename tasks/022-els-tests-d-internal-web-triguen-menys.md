---
title: "Els tests d'internal/web triguen menys"
status: todo
depends_on: []
---

## Why

`go test ./internal/web` takes about four minutes alone (229 s on 2026-11-21, 911 tests, none parallel) and much longer when `scripts/check.py` runs it with every other touched package (39 minutes for the v1.89.0 integration). Six tests take 89 s of it, and the two slowest are slow because of waits in the product, not in the tests.

## Acceptance criteria

- [ ] Listing a folder (`GET /api/dirs`) no longer probes every drive letter from C to Z with `os.Stat`: on Windows the roots come from the system's list of logical drives, so a disconnected network drive does not hold the folder picker for about ten seconds. `TestDirsLlistaCarpetesIRepos` (21 s today on this machine) runs in under a second, and a Go test shows the roots still include the drive of the temp folder.
- [ ] `TestWindInteractiveBashAutoAllow/mkdir dins` (28.6 s today) runs in under two seconds without weakening what it checks: find why its fake provider's reply is treated as empty and retried with backoff (`reintentaBuida`, `internal/llm/continua.go:199`) and fix the cause, in the fake provider or in the product, whichever is wrong.
- [ ] `go test ./internal/web -count=1` takes at most 150 s on this machine, measured before and after and written in the notes.

## Out of scope

- `t.Parallel()` across the package: the tests share package-level state (`intervalProgres`, `llm.RetryBase`, the event store of `goalTestServer`) and were not written for it.
- Changing what any test asserts.
- Other packages.

## Notes

- Measured with `go test ./internal/web -json -count=1` (throwaway `HOME`/`USERPROFILE`, no `GREGAL_DATA_DIR`). Slowest: `TestWindInteractiveBashAutoAllow` 29.1 s, `TestDirsLlistaCarpetesIRepos` 21.1 s, `TestMateixaReglaDePermisosACodiIVents` 14.8 s (26 subtests of about 0.6 s each: a server per case), `TestTeamReviewerLateErrorReportsFailed` 12.1 s, `TestBestOfAPI` 7.0 s, `TestSaveTurnShotPrunesOldOnes` 5.2 s. Tests of 1 s or more: 37, adding up to 142 s.
- `internal/web/dirs.go:103`: `arrels()` stats `C:\` to `Z:\`. A stack dump at 8 s shows the handler blocked there in a syscall. `golang.org/x/sys` is already in `go.mod` (`windows.GetLogicalDrives`); the non-Windows branch returns `/`.
- A stack dump of the mkdir case at 12 s shows it inside `llm.reintentaBuida` → `ChatWithTools`, called from `runReadOnlyLoopForRoleStream` (`internal/web/plan.go:226`): a retry wait, not the command. `internal/llm` tests shorten `RetryBase` in their own `TestMain`; `internal/web` has `retry_main_test.go`, check what it sets.
- To see where a slow test waits: `go test ./internal/web -run Name -count=1 -timeout 8s` and read the goroutine dump.
- Not looked at yet: the other four slow tests.
