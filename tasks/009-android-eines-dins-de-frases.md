---
title: "Android Codi: les eines dins de frases"
status: done
depends_on: []
---

## Why

Same as the web (task 008, branch `task/008-codi-eines-dins-de-frases`): the work group of a turn is still a list of tool rows. The user wants the tools hidden by default inside sentences that say what the agent is doing, on Android too. The web implementation is the reference: `internal/web/app/tool-summary.js` (`sentence`, `chip`, `outcome`) and its tests in `tool-summary.test.cjs` on that branch (`git show task/008-codi-eines-dins-de-frases:internal/web/app/tool-summary.js`).

## Acceptance criteria

- [x] In the work group (`ChatWork.kt`), each tool call is a small chip (icon by family plus a few words) placed after the narration sentence that precedes it; consecutive calls share the sentence and the chips wrap (FlowRow). The call's detail is hidden until its chip is tapped, then it opens under the sentence; tapping again closes it.
- [x] When the model wrote nothing before a run of calls, the app shows a grey sentence of its own, picked by the most telling call of the run, in this priority: edit > test > run > git > web > browser > calendar > mail > remind > delegate > other tool > search > read. Same texts as the web, in both languages:
  - edit: "Edited files" / «Ha editat fitxers»
  - test: "Ran the tests" / «Ha passat els tests»
  - run: "Ran a command" / «Ha executat una ordre»
  - git: "Worked with git" / «Ha treballat amb git»
  - web: "Looked it up on the web" / «Ha consultat la web»
  - browser: "Used the browser" / «Ha fet servir el navegador»
  - calendar: "Checked the calendar" / «Ha consultat el calendari»
  - mail: "Checked the mail" / «Ha mirat el correu»
  - remind: "Checked the reminders" / «Ha mirat els recordatoris»
  - delegate: "Handed work to helpers" / «Ha delegat feina»
  - search: "Searched the code" / «Ha cercat al codi»
  - read: "Looked through the files" / «Ha mirat els fitxers»
  - other: "Used {x}" / «Ha fet servir {x}», where x is the MCP server for `mcp_<server>_<tool>`, else the tool's label.
- [x] Chip texts follow the web's `chip()`: edit/patch `login.go +3 −3`, write `n.txt +2`, read the file name, web_fetch the domain, bash the command (without a leading `cd x &&`), git `git …`, an exploration fold its counts (`1 fitxer · 2 cerques`), tools with no verb of their own `calendar · today`. At most 48 characters.
- [x] A failed call has a red chip (bash: `codi N` / `exit N`) and its detail is open. A read-only bash command (one the exploration classifier in `Work.kt` recognises) that exits with 1 or 2 is grey with «sense resultats» / "nothing found", not red and not opened.
- [x] Unit tests in `WorkTest.kt` cover the sentence choice, the chip texts and the failed/quiet outcome with the same cases as the web tests.

## Out of scope

- The web (task 008) and the curl block message (task 007).
- A diff view for edits and a richer terminal block.

## Notes

- `android/app/src/main/java/xyz/solanes/gregal/ChatWork.kt` (`WorkGroupCard`, `StepRow`), `Work.kt` (labels, classifier, `detail = output.take(4000)`), `WorkTest.kt`, `Strings.kt`.
- Bash failures arrive as `ERROR: exit: exit status N\n<output>` (`internal/tools/tools.go` wraps the error as `exit: %v`, `loop.go` prefixes `ERROR: `), so match `^ERROR: (?:exit: )?exit status (\d+)` and strip that first line when showing the output; other failures start with `ERROR:`, `EINA BLOQUEJADA` or `TOOL BLOCKED`.
- Android tests: from `android/`, `ANDROID_HOME='C:\Users\alsol\AppData\Local\Android\Sdk' ./gradlew.bat testDebugUnitTest -Pgregal.skipBackend -Pgregal.plotsPreview`. Edit `Strings.kt` by hand; never run `regen-strings.py` (it adds ~1500 keys).
- Done: `Work.kt` has `toolSentence`, `toolChip`, `toolOutcome` (ports of the web's `sentence`, `chip`, `outcome`), `stepChip` (chip and state of one call or an «Explorat» fold), `workLines` (which narration owns which calls) and `toolOutputShown` (drops the `ERROR: exit: exit status N` line from the bash output shown). The texts live in `TOOL_EN`/`TOOL_CA` next to the other tool-summary labels, not in `Strings.kt`. `WorkStep` now keeps the call's `args`.
- `WorkGroup.finishTool` takes the step state from `toolOutcome`: a quiet call (read-only bash exiting 1 or 2) is `Ok`, so it is not red anywhere and does not count in the header's error total; a `TOOL BLOCKED` / `EINA BLOQUEJADA` output is `Failed` even though `toolOutputFailed` does not recognise it.
- A fold's chip takes its state from the calls inside it: failed if any failed, otherwise running if any is running. Android has no separate "interrupted" state: `WorkGroup.close` turns calls still running into failed or done.
- Only tool calls become chips. Thinking, review, blocked, subagent, user and info steps keep their own rows, visible as before.
- Follow-up: on Android the narration written before a tool call is settled as a chat message outside the work group (`settleLiveAsMessage` in `GregalViewModel.kt`); only narration followed by reasoning becomes a `Note` step inside it. So most runs of calls get Gregal's grey sentence rather than the model's own words. Moving that narration into the group, as the web does, is a ViewModel change of its own.
- Follow-up: the chips are about 26dp tall, below the 48dp touch target guideline.
- Not verified on a device or emulator: the layout (FlowRow inside the timeline's `IntrinsicSize.Min` row) is only checked by compiling.
- `ExploreFoldUiTest.consecutiveExplorationFoldsAndExpands` (task 005) asserted that the «Explorat» row is visible as soon as the card opens. This task changes that requirement (calls hide behind chips), so it became `consecutiveExplorationFoldsIntoOneChipThatOpensAndCloses`, which asserts the fold chip, opening it and closing it. UI tests were added next to it: narration owning its chips, a failed call opening red while a quiet one stays closed, a failed chip you closed staying closed when another call fails, and long narration not being cut.
- A debug APK builds (`./gradlew.bat assembleDebug -Pgregal.skipBackend`).
- Narration that owns chips is shown whole (no `maxLines`): when the reasoning is blank or repeats it, the `Note` step is the only copy of what the model said.
