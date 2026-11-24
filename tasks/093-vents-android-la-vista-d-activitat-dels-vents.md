---
title: Vents Android: la vista d'activitat dels vents
status: todo
depends_on: ["091"]
---

## Why

The person should see what each wind did while they were away, in one place, as on agent products that show an activity view.

## Acceptance criteria

- [ ] Vents on Android has an «Activitat» screen (from the drawer and from a wind's profile) that reads `/api/winds/activity` and shows the timeline grouped by day with the wind's face, time, title, status chip and a tap that opens the chat, document or run. A unit test of the grouping; a Compose test of one item.
- [ ] Scrolling to the end loads older items with `before`. A unit test with a fake server.
- [ ] Empty and error states as on the web.
- [ ] Strings in Catalan and English in `Strings.kt`.

## Out of scope

- Web (092).

## Notes

- Similar screens: `FeinesScreen.kt`, `ProgramatsScreen.kt`, `VentsAvui.kt`; drawer in `Drawer.kt`. A fake server like `DocServer` in `DocFollowTest.kt`.
- Android: run gradle tests with `ANDROID_HOME` set; edit `Strings.kt` by hand (do not run `regen-strings.py`); check on the emulator (`Medium_Phone_API_36`, x86_64 build with `-Pgregal.abi=x86_64`) with a screenshot in the notes. Regex: no `(?U)` (Android's ICU refuses it; `UserAvatarTest` guards this).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
