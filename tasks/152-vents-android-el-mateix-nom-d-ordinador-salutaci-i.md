---
title: Vents Android: el mateix nom d'ordinador, salutació i Desat que al web, i cada vent amb el seu paquet a les rutines
status: todo
depends_on: ["127", "141"]
---

## Why

Android parity left by 127 and 141: Android still uses the old computer name, greeting hours and Desat tab name, and the routine and reminder rows (`WindAutomationRow`, in Feina and the wind's Activity tab) draw winds without their own pack in Lliure.

## Acceptance criteria

- [ ] Android names the computer «L'ordinador de Gregal» / «Gregal's computer» wherever the web does, greets with the same words and hours as the web helper (`salutacio()` in i18n.js: night before 6, morning before 13, afternoon before 20, then evening), and Desat's first tab is «Respostes» / «Replies». Tests.
- [ ] `WindAutomationRow` draws each wind with its own pack and style (as the list does since 141). A test with an esperit wind in Lliure.

## Out of scope

- Web.

## Notes

- Files: AppStrings.kt (by hand, ca+en), the Vents home greeting (VentsStart.kt), Desat screen, WindAutomationRow; see task 127 notes for the web wording.
