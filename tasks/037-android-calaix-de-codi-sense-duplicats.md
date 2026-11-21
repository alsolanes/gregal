---
title: "Android: el calaix de Codi sense duplicats"
status: done
depends_on: []
---

## Why

The Codi drawer (`Drawer.kt`, the non-simple drawer) still lists a «Winds» section (Gregal, Ventet, Notes) although the Codi | Vents switch above it already takes you there, and has three rows for the same thing: «Activity» (Jobs: scheduled + results), «Graphs» (Flows) and «Automations», which already loads and groups the jobs and the flows. In the page tour the routines showed up identical in Automations and Activity. One place per thing.

## Acceptance criteria

- [x] The Codi drawer has no «Winds» section; Notes stays reachable from Codi (under Tools) and from Vents.
- [x] One automation row in the Codi drawer: «Automations». Whatever only Activity or Graphs offered (the run results, the flow editor) is reachable from inside Automations (a header action or a section), so no feature is lost. Admin-only parts stay admin-only.
- [x] A Compose test of the Codi drawer: no duplicate rows, no «Winds» section, Automations present; and one that the results are reachable from Automations.
- [x] Screenshots of the Codi drawer and Automations, dark and light.

## Out of scope

- The Vents drawer (already grouped).
- Merging Saved, Notes and Webs: waits for the saved-items tasks 028–033.

## Notes

- `Drawer.kt` lines ~101–157; `AutomatitzacionsScreen.kt`, `JobsScreen.kt`, `FlowsScreen.kt`. Deep links that go to `Screen.Jobs` (`GregalViewModel.kt` ~6402, 6409, 6445: a notification opening a run) must keep working.
- Done: the Codi drawer lost the «Winds» section (Gregal, Ventet, Notes); Notes moved under Tools. «Activity» and «Graphs» are gone from the drawer; the «Automations» top bar has two new actions, Activity (History icon, opens `Screen.Jobs` with the Scheduled and Results tabs, shown only when `conn.local || isAdmin`, the same rule the drawer row had) and Graphs (AccountTree icon, opens `Screen.Flows`). The drawer marks «Automations» as selected on Jobs and Flows too, so a notification that opens a run still lights the right row. `Screen.Jobs` and its deep links are unchanged. The unused `app.nav.sectionWinds` string was removed (ca and en).
- Tests: `CodiDrawerTest` (behaviour) and `CodiDrawerLookTest` (captures in `android/app/build/codi-drawer-look/`: drawer-dark, drawer-light, automations-dark, automations-light). The tests point the view model at an unreachable server (127.0.0.1:9) because an empty `Conn` reaches whatever Gregal server runs on localhost, which put real data on screen.
- Not tested: the Activity action hidden for a non-admin remote account; `isAdmin` can only be set by connecting, and there is no test hook.
- Follow-up: `WindAutomationList` shows «No s'ha pogut carregar l'activitat: %1$s» with the placeholder unfilled when the load fails (visible in the automations captures).
- Ventet no longer has a row in the Codi drawer; it stays reachable from Vents.
