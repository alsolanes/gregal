---
title: Vents Android: el mateix nom d'ordinador, salutació i Desat que al web, i cada vent amb el seu paquet a les rutines
status: done
depends_on: ["127", "141"]
---

## Why

Android parity left by 127 and 141: Android still uses the old computer name, greeting hours and Desat tab name, and the routine and reminder rows (`WindAutomationRow`, in Feina and the wind's Activity tab) draw winds without their own pack in Lliure.

## Acceptance criteria

- [x] Android names the computer «L'ordinador de Gregal» / «Gregal's computer» wherever the web does, greets with the same words and hours as the web helper (`salutacio()` in i18n.js: night before 6, morning before 13, afternoon before 20, then evening), and Desat's first tab is «Respostes» / «Replies». Tests.
- [x] `WindAutomationRow` draws each wind with its own pack and style (as the list does since 141). A test with an esperit wind in Lliure.

## Out of scope

- Web.

## Notes

- Files: AppStrings.kt (by hand, ca+en), the Vents home greeting (VentsStart.kt), Desat screen, WindAutomationRow; see task 127 notes for the web wording.
- Done: one string, `app.computer.name` («L’ordinador de Gregal» / «Gregal’s computer», with the web's typographic apostrophe), for the drawer row (was «Ordinador»), Gregal's header button (`threadCopy("computer")`, used by VentsChatChrome, not touched) and the Ordinador sheet title in local mode and on error (was «El navegador de Gregal, en aquest ordinador» / «Ordinador»).
- Done: the drawer's Desat group keeps «Desat» as its title and its first row is now «Respostes» / «Replies» (`saved.tab.replies`); the Desat screen title stays «Desat», like the web H1.
- Greeting: `askGreeting` already had the hours and words of `salutacio()` (night <6, morning <13, afternoon <20, evening); `VentsWebParityTest` now pins every boundary in both languages, so it passed from the start. Android still adds the first name and a full stop («Bon dia, Aleix.»), the web says just «Bon dia».
- Follow-up: drop the name and the dot from `askGreeting` if the greeting must be word for word the web's; the name is passed down from CompanyScreen.kt (VentsOpening/VentsStartPage), which task 145 is reworking.
- Done: `WindAutomationRow` takes `winds` (from `vm.winds`) and draws the wind's `faceStyle` (pack, kit, bearing) when it finds it, the style code otherwise; Feina and the Activity list pass it. Test: an Esperit wind in Lliure (compared with a tolerance of 2 levels per channel: the row's surface blends one level apart).
