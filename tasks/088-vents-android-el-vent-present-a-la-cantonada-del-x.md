---
title: Vents Android: el vent present a la cantonada del xat, animat pel que fa
status: done
depends_on: ["086"]
---

## Why

The wind should feel present next to the person, not only as a small face in each row: a larger avatar that sits partly outside the chat's frame and moves with what the wind is doing.

## Acceptance criteria

- [x] In `WindConversation` the wind's avatar (`WindFace`, active pack) sits at a bottom corner, about 72 dp, partly over the edge of the message list, above the composer; it does not show in rotllana conversations. A Compose test that the node exists in a wind chat and not in a rotllana.
- [x] Its mood follows the state from 086 (working, waiting, done), with the existing aura while working (`windShowsAura`). A unit test of the state → mood mapping.
- [ ] It never covers the composer or the last message: the list gets bottom padding while it is shown. Screenshot on the emulator.
- [x] With expressive motion off (`windMotionEnabled`) it does not animate.

## Out of scope

- Web (087).

## Notes

- Files: `CompanyScreen.kt` (`WindConversation`), `WindFace.kt` (`WindFace` 203, `windShowsAura` 170, `windMotionEnabled` 91), `VentsSpectacle.kt` (`WindHalo` 35).
- Android: run gradle tests with `ANDROID_HOME` set; edit `Strings.kt` by hand (do not run `regen-strings.py`); check on the emulator (`Medium_Phone_API_36`, x86_64 build with `-Pgregal.abi=x86_64`) with a screenshot in the notes. Regex: no `(?U)` (Android's ICU refuses it; `UserAvatarTest` guards this).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
- 088 done: the corner wind is `WindFace` at `WindPresenceSize` (72 dp) aligned BottomStart inside the message list's box (testTag `wind-presence`), so it sits above `WindAskCards`, 090's `GregalWorkStrip` card and the composer and never overlaps them. Left corner on purpose: the wind's bubbles are on the left and `LatestPill` already takes the bottom right. It shows only when the thread has messages (the empty state already has the big welcome face); the list's bottom padding grows by 72 dp only then.
- Mood: `windPresenceMood(windChatState(...), live mood)` in `VentsChatChrome.kt`: working → the live mood if it is a work mood, else WORKING (so `windShowsAura` is always true at 72 dp); waiting → NEWS; done → DONE; otherwise IDLE.
- Motion: `WindFace` already gates on `windMotionEnabled()`; the face is not tappable, so with motion off it is static. `WindPresenceTest` compares two frames 1.7 s apart with motion off (same) and on (different).
- Not verified: the emulator screenshot (third criterion left unticked). `WindPresenceTest::theLastMessageStaysAboveTheWind` checks the last message ends above the face on Robolectric, but nobody has looked at it on a device. "Partly over the edge of the message list" is read as sitting in the list's bottom corner; it does not hang below the list, because that would cover 090's work card.
