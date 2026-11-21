---
title: "Android: Notes s'obre a la llista"
status: done
depends_on: []
---

## Why

Opening Notes from the drawer drops you into an empty note with the keyboard up, and the notes you already have are a tap away. In the page tour this read as an empty, broken screen: when you open Notes you mostly want to find a note, not write a new one. Quick capture (the share sheet and the «Quick note» shortcut) should still open straight into a note.

## Acceptance criteria

- [x] Opening the Notes screen shows the list (search, filters, notes) with a clear «New note» action; a Compose test checks the list shows first and no editor has focus.
- [x] «New note» opens the agile note (blank, cursor in the body, autosave) as today; back returns to the list.
- [x] Shared text and the «Quick note» shortcut still open the agile note with the text; the existing capture test keeps passing.
- [x] Screenshots of Notes, dark and light.

## Notes

- `Notes.kt` `NotesScreen` (line ~411): `showList` starts false and the `else` branch is `AgileNote`. Tests in `NotesTest.kt` and `AgileNotesTest.kt` may assert the old opening: if one does, change it and say in the final message which and why (the requirement changed).
- Done: `NotesScreen` now starts on `NotesList` (`writing` = false); «New note» (FAB), Share and the «Quick note» shortcut set `writing` and show `AgileNote`; back, the list icon, or opening a note/editor returns to the list. `NotesList` lost its `onBack` (it is always the root now).
- Tests: `NotesOpeningUiTest` in `AgileNotesTest.kt`. No existing test asserted the old opening (`AgileNoteUiTest` renders `AgileNote` directly), so none changed.
- Screenshots: `android/app/build/notes-ui/list-dark.png`, `list-light.png`, `empty-dark.png`.
- Follow-up: in the agile note the «Your other notes» block and list icon are now partly redundant with the list being home; worth a look in a later pass.
