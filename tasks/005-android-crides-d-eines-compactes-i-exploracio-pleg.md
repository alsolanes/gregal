---
title: "Android: crides d'eines compactes i exploracio plegada"
status: done
depends_on: [1]
---

## Why

Task 001 made the web Codi transcript read as what the agent does (List, Read, Search, Test, Git) and folded consecutive exploration into one "Explored" row. The Android work card still shows "Run <raw command>" for every bash call, so the parity gap named in task 001 stays open.

## Acceptance criteria

- [x] On Android a read-only bash call reads as List/Read/Search (ca Llista/Llegeix/Cerca) with the paths or pattern as target (and the lines for `sed -n a,bp`), the same cases as `internal/web/app/tool-summary.test.cjs`.
- [x] Tests (`go test`, `npm test`, `pytest`, `cargo test`, `node --test`) read as Test/Prova, a single `git …` command as Git; chains and any other command keep Run with the whole command.
- [x] A command that can write or execute (redirect to a file, `$(…)`, `sed -i`/`w`/`-f`, `sort -o`, `uniq - out`, `rg --pre`, `tree -o`, `find -exec`/`-delete`/`-fprint`, `rm`, …) is never treated as exploration.
- [x] Two or more consecutive exploration steps (read, grep, glob and read-only bash) in a work card fold into one "Explored · N files · N searches · N listings" row that expands to the individual steps; any other step in between starts a new row. A failed step inside the fold opens it.

## Out of scope

- A `description` argument for the bash tool (follow-up of task 001).
- VS Code and Telegram tool rendering.
- The approval dialog (`ToolText.kt`): it shows what will run, so it keeps the raw command.

## Notes

- `android/app/src/main/java/xyz/solanes/gregal/Work.kt` (pure, JVM tests in `WorkTest.kt`), `ChatWork.kt` (`WorkGroupCard`/`StepRow`), `GregalViewModel.kt` (`tool_call`).
- Numbered 005 because the Android glass series already uses branches `task/001`–`task/004` with its own task files.
- The Android classifier is stricter than the web one. The review found commands that write or execute but that `tool-summary.js` shows as List/Read: `$'…'` quoting, a `#` comment hiding a newline, `${…@P}`/`$[…]` with `printf -v`, `\r` after `>&1`, and `tree -R`. Android rejects them; they are listed as null cases in `WorkTest.kt`. Follow-up: apply the same rules to `internal/web/app/tool-summary.js` and its test.
- Follow-up (shared with the web): Go decodes the bash arguments case-insensitively, so `{"command":"ls","Command":"rm x"}` runs the second command while the summary shows the first.
- The fold has a Robolectric test (`ExploreFoldUiTest.kt`); not checked on a device.
