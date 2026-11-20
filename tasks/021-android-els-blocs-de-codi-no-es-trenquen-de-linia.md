---
title: "Android: els blocs de codi no es trenquen de línia"
status: done
depends_on: []
---

## Why

In the Android chat a code block wraps long lines to the width of the phone and has no line height of its own, so indentation is lost and a ten-line snippet reads as a paragraph. Web and the other coding agents keep each line whole and let the block scroll sideways.

## Acceptance criteria

- [x] A code block shows each source line on one visual line: a line longer than the screen does not wrap, and the block scrolls horizontally (Compose test under Robolectric: with a 400-character line, the code text is laid out on as many lines as the source has, and the block has a horizontal scroll action).
- [x] The header of the block (language and Copy) stays in place while the code scrolls sideways (same test: after scrolling the code to the end, the Copy button is still displayed).
- [x] Code text uses a line height of 19 sp with its 13 sp monospace size (asserted on the text layout in the same test).
- [x] Charts, previews and plot blocks (`CodePreviewBlock`, `PlotCard`) are unchanged: the existing markdown tests pass.

## Out of scope

- Syntax highlighting, line numbers and a "wrap lines" toggle.
- The tool detail block inside the work group (`StepRow` in `ChatWork.kt`): the unmerged branches 005 and 009 rewrite that file.
- Vents and the wiki: they do not use `CodeBlock`. Their code is drawn inline (`CompanyScreen.kt:2099`, `WikiScreen.kt:191`) and already scrolled sideways without wrapping, at 12 sp with no header or Copy.

## Notes

- `android/app/src/main/java/xyz/solanes/gregal/Screens.kt`: `CodeBlock` (around line 1182), called from `AssistantResponse` (line 1146).
- Tests: `android/app/src/test/java/xyz/solanes/gregal/` (Robolectric 4.14 with compose ui-test; `MdLookTest` is the closest existing one). Run with `ANDROID_HOME` set: `./gradlew.bat testDebugUnitTest -Pgregal.skipBackend -Pgregal.plotsPreview` from `android/`.
- Checked with `CodeBlockTest` (three Compose tests under Robolectric, which failed before the change), the whole Android unit suite (`testDebugUnitTest`, passed 2026-11-20) and a before/after screenshot. Not checked on a device.
- Side effect to try on a phone: the chat sits inside the navigation drawer (`MainActivity.kt:287`), and a horizontal drag that starts on a code block now scrolls the code, so it will probably not open the drawer from there, as already happens on wide tables. The menu button still opens it.
- Follow-ups: there are now three looks for code (13/19 sp in the chat, 12 sp in Vents and the wiki, 12/16 sp in the tool detail of `ChatWork.kt`); Vents and the wiki could reuse `CodeBlock`. Tabs are drawn one space wide. Nothing hints that code continues to the right (`MdTable` draws an edge gradient for the same case).
- Where Android stands otherwise (2026-11-20 survey), so the next tasks start from facts:
  - Already built on branches that are not merged here: glass style (001, 002), screen transitions (004), compact tool rows and the «Explorat» fold (005), tool chips inside sentences with the work group open while running (009), chat that does not jump while streaming and opens at the end (015, 016), Codi/Vents selector (017). The largest visual jump for the APK is merging those, which is the user's call.
  - Not built anywhere: a speed or elapsed line under the composer (web task 013), live output of a running command and the file being written (web tasks 011 and 012 add the server side), `progress.since` is ignored in favour of the phone's clock and the progress texts are hardcoded in `Work.kt` instead of `AppStrings.kt`, the user bubble does not match the shape its own comment describes (`Screens.kt:1385`), and the go-to-end button is a bare arrow (`Screens.kt:802`; branches 015 and 016 rewrite the follow logic around it).
  - Live text: the app already flushes tokens every 60 ms and only rebuilds the paragraphs that changed (`Screens.kt:1156`); the whole text is still re-parsed on each flush (`richBlocksLive`, `Md.kt:573`). No visible problem was measured.
