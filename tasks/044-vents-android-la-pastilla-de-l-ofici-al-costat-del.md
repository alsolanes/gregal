---
title: "Vents (Android): la pastilla de l'ofici al costat del nom de cada vent"
status: todo
depends_on: []
---

## Why

The same as task 043, on Android, which is the main client: every wind's name gets a small pill with its trade, in the wind's colour, so you know who to ask without opening profiles.

## Acceptance criteria

- [ ] On the Vents start page (the faces row), in the chat header and in the winds directory, each wind's name has a pill with its `role` in the wind's colour. A Compose test (Robolectric) finds the pill text next to the name in each of the three screens.
- [ ] A wind with a `kit` shows that kit's icon in the pill. A wind without a kit shows the pill with no icon, and a wind with an empty `role` shows no pill. A Compose test covers the three cases.
- [ ] The pill is readable in light and dark themes and with large text: it truncates with an ellipsis instead of pushing the name off the row. A Compose test at font scale 1.5 checks that the name is still shown.

## Out of scope

- Web: task 043.
- A new field on the wind (see 043).

## Notes

- Places: `VentsHomeParts.kt` (start page faces), the chat header in `VentetChat.kt`/`WindMessageUi.kt`, `VentsCrew` in `CompanyScreen.kt`.
- Kit icons: map the `windKits` keys in `WindFace.kt` to Material icons that are already in the project, with no new dependency.
- Strings: edit `Strings.kt` by hand if a string is added (do not run `regen-strings.py`). Set `ANDROID_HOME` for the gradle tests.
