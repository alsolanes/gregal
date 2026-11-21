---
title: "Android: el xat de Codi respira"
status: done
depends_on: []
---

## Why

In a screenshot of a Codi turn on Android (2026-11-21, `CodeChatLookTest`), the user's message is hard to tell from the background in dark mode and, when it is long, it spans the whole width like one more line of the answer; inline code in the answer is as big as the text and outweighs the sentence; and the finished «Ha treballat 13s · 4 eines» summary is a bordered box that competes with the answer below it.

## Acceptance criteria

- [x] The user's message is a bubble tinted with the accent, on the right, at most 85 % of the width, with a tighter bottom-right corner (`CodeChatLookTest` checks it sits on the right with a margin on the left; it failed with the old bubble).
- [x] Inline code in answers is 0.88 of the text size.
- [x] A finished, folded work group has no box: no background and no border, just its summary line. Running, failed or opened, it keeps its card.

## Out of scope

- The Vents thread bubbles (`WindBubble`), which have their own look.
- Paragraph spacing inside answers (`groupRich` joins paragraphs into one block): a separate change.
- The go-to-end button, which branches 015/016 reworked.

## Notes

- Files: `Screens.kt` (`MsgRow`, user bubble), `Md.kt` (`renderSegs`, inline code), `ChatWork.kt` (`WorkGroupCard`).
- Screenshots: `android/app/build/code-chat-look/` (after) and `code-chat-before/` (before), dark and light.
- Inline code size and the work group look are checked in the screenshots only.
- Checked: `CodeChatLookTest`, the whole Android unit suite (run without the Gradle cache). Not checked on a phone.
