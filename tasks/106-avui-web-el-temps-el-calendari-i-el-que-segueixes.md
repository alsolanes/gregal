---
title: Avui web: el temps, el calendari i el que segueixes a dalt de tot
status: todo
depends_on: ["105"]
---

## Why

Today should open with what matters at a glance: today's weather where the person is, the next match of the teams they follow (or the next thing of anything else they follow) and today's calendar, without asking anyone.

## Acceptance criteria

- [ ] The Today view (web/desktop) opens with a glance row above the tray: a weather card (icon for the weather code, now/min/max, the rain hours highlighted, sunrise/sunset), the next calendar events, and one card per follow (title, when in the person's words: «dissabte a les 21:00», detail, link). A node test of each card from a sample `glance`.
- [ ] A small «Segueix…» control adds a follow (team or other) and sets the place, and each follow can be removed; empty states invite to add them. A node test.
- [ ] It feels alive: cards slide in, the weather icon animates gently (sun turns, rain drops fall), countdowns to a match tick («d'aquí a 2 h»); all off under reduced motion.
- [ ] Strings in Catalan and English.

## Out of scope

- Android (107).

## Notes

- Files: `winds-workspace.js` (`todayView`/`paintAvui` ~465-523) or `vents-activity.js` style; CSS in `winds-workspace.css`.
- Asked by the user on 2026-11-24: «a Avui hem de tenir la previsió del temps, next matches dels equips que seguim i si seguim alguna altra cosa també allà, o coses del calendari». It should feel alive and fun on the phone (user's direction for Vents), not a dashboard of grey boxes.
- Web: `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; screenshots from a sandboxed server (never type into credential fields) in `output/ui-review/` (ignored), dark and light.
