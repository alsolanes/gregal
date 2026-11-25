---
title: Vents Android: permisos amb el color del vent i la cara a cada missatge
status: done
depends_on: ["129", "130"]
---

## Why

Same as 130 on Android.

## Acceptance criteria

- [x] Approval cards show the wind's face, an edge in its colour, a one-line reason and the Allow button in the wind's colour with checked contrast. A test.
- [x] Wind messages show a small face beside the first bubble of a run; one bubble rule per speaker. A Compose test. Robolectric frames or emulator screenshots (light, dark) in `output/ui-review/131/`.

## Out of scope

- Web (130).

## Notes

- Files: WindAsks.kt, CompanyScreen.kt and the chat bubbles.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
- Done (WindAsks.kt only). Approval cards take the colour of the wind that asks (`winds` by `a.wind`, else the chat's wind, else the screen accent), so in Feina and in a group the card is the asker's, not the screen's: tint and border as before plus a 3 dp left edge in that colour. The face perched on the card replaces the shield; the shield (tag `ask-shield`) stays for an unknown wind and in the wind's own 1:1 chat, where the face is the chat's corner face (109, `WindFlightTest.aWindChatShowsTheWindOnce` still holds one face). `a.reason` is one line under the question (`ask-why`, ellipsis) and is no longer folded into `approvePlan`'s sub for `kind: rule` (the `reason` parameter is gone), so it is said once. Allow uses `allowPaint(color, ink, deepTop)`, the same rule as the web: the theme ink that keeps most of the wind's colour, mixing the fill (sRGB) toward the other ink only until 4.5:1. No new strings.
- Criterion 2 was already met by 145 (byline with a 24 dp face at the start of each run in groups, replies always on the reading surface whatever their length; 1:1 chats have the single corner face, as on the web). Added a test that proves it in a real `RotllanaConversation` with a run of two replies, one of them long. The face is 24 dp (web 20 px); left as is.
- Tests: `WindAsksTest` (allowPaint in every theme; face/why/shield), `WindIdentityLookTest` (edge and Allow pixels, group runs; frames `ask-xaloc|ask-gregal` and `group` × light/dark in `output/ui-review/131/`, not committed).
- Follow-up: the secret, handoff and connection cards still take the screen's accent, not the asking wind's colour (the web paints every non-question card from a known wind).
- Follow-up: `chatSurfaceFor`/`READING_CHARS` in ChatThread.kt are no longer used by the app (replies are always `reading = true` since 145); remove them and their tests in their own task.
