---
title: "Android: un torn de Codi sobreviu a l'app en segon pla"
status: done
depends_on: []
---

## Why

In Code mode, leaving the app mid-turn for a couple of minutes ends with a raw «Unable to resolve host …» error bubble, and the turn looks cut off even though it keeps running on the server. Android cuts the app's network in the background; `followRun` spends its 30 retries (~2 min) on `UnknownHostException` and gives up, and coming back to the app never reattaches to the running turn.

## Acceptance criteria

- [x] While the app is in the background, network-unreachable failures (`isUnreachable` from `VentsOffline.kt`) while following a run do not use up the retry budget and do not end the turn; it keeps waiting. A test drives `followRun` with a fake API that fails with `UnknownHostException` more than 30 times while backgrounded, then succeeds, and the turn finishes normally with no error bubble.
- [x] If a turn still stops on an unreachable network, no error bubble with the raw host is shown; when the app comes back to the foreground (`onForegrounded`) and is connected and not already sending, it calls `attachToRunning()` and the running turn's events show up. A test covers the foreground reattach.
- [x] `requestMessage` maps unreachable errors (including «Unable to resolve host») to the existing connection message, never the raw hostname. A unit test.
- [x] `ANDROID_HOME=... ./gradlew.bat testDebugUnitTest -Pgregal.skipBackend -Pgregal.plotsPreview` passes for the touched tests.

## Out of scope

- A foreground service or push changes. Vents and the server. The poll interval.

## Notes

- `GregalViewModel.kt`: `runTurn` (~1969, catch ~2004–2011), `runV2Turn`/`followRun` (~2552–2592, unguarded tail `getEvents` ~2623), `requestMessage` (~2697), `onBackgrounded` (~796), `onForegrounded` (~824), `attachToRunning` (called from `connectOnce` ~1757). `AppVisibility.foreground`. `VentsOffline.isUnreachable` (VentsOffline.kt:38).
- Done: `followRun` doesn't count unreachable failures while `!AppVisibility.foreground` (waits 5 s per try instead); in the foreground, after 30 unreachable failures it returns false (turn ends quietly, no bubble) instead of throwing. Other errors still throw as before. The tail `getEvents` after a terminal `getRun` retries on unreachable errors. `onForegrounded` calls `attachToRunning()` when connected and not sending (also for the local server). `requestMessage` (now `internal` for the test) maps `isUnreachable(e)` to `app.status.requestConnection`; no new strings.
- Tests: `android/app/src/test/java/xyz/solanes/gregal/CodeTurnBackgroundTest.kt`. The fake server simulates the lost network by closing the socket without answering (OkHttp «unexpected end of stream», which `isUnreachable` treats like `UnknownHostException`): `GregalApi` has no seam to inject a real DNS failure. The `UnknownHostException` itself is covered by the `requestMessage` test.
- A submit that fails unreachable in the foreground still shows a bubble, now with the connection message instead of the raw host (the request really was not sent).
