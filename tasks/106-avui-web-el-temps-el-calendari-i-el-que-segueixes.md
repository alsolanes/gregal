---
title: Avui web: el temps, el calendari i el que segueixes a dalt de tot
status: done
depends_on: ["105"]
---

## Why

Today should open with what matters at a glance: today's weather where the person is, the next match of the teams they follow (or the next thing of anything else they follow) and today's calendar, without asking anyone.

## Acceptance criteria

- [x] The Today view (web/desktop) opens with a glance row above the tray: a weather card (icon for the weather code, now/min/max, the rain hours highlighted, sunrise/sunset), the next calendar events, and one card per follow (title, when in the person's words: «dissabte a les 21:00», detail, link). A node test of each card from a sample `glance`.
- [x] A small «Segueix…» control adds a follow (team or other) and sets the place, and each follow can be removed; empty states invite to add them. A node test.
- [x] It feels alive: cards slide in, the weather icon animates gently (sun turns, rain drops fall), countdowns to a match tick («d'aquí a 2 h»); all off under reduced motion.
- [x] Strings in Catalan and English.

## Out of scope

- Android (107).

## Notes

- Files: `winds-workspace.js` (`todayView`/`paintAvui` ~465-523) or `vents-activity.js` style; CSS in `winds-workspace.css`.
- Asked by the user on 2026-11-24: «a Avui hem de tenir la previsió del temps, next matches dels equips que seguim i si seguim alguna altra cosa també allà, o coses del calendari». It should feel alive and fun on the phone (user's direction for Vents), not a dashboard of grey boxes.
- Web: `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; screenshots from a sandboxed server (never type into credential fields) in `output/ui-review/` (ignored), dark and light.
- Done in a new module `internal/web/app/vents-avui-glance.js` (loaded from `index.html`, exposed as `window.gregalAvuiGlance.renderGlance`); `paintAvui` in `winds-workspace.js` puts the row right after the header. Tests: `internal/web/app/vents-avui-glance.test.cjs`. CSS block «Avui d’una ullada» in `winds-workspace.css`; the sky colours are local `--gl-*` tokens with a light-theme override.
- Assumed API shape (105 was not built yet): follows are saved by POSTing the whole list `{follows:[{id?,kind,name,wind?}]}` (a new one has no `id`) and the place with `{place}`, both to `/api/winds/avui/settings`. The current list comes from `settings.follows` (falls back to `glance.follows`); `glance.follows[]` is matched by `id` and carries `{title, when, detail, url}`. If 105 picks another POST shape, only the `save(...)` calls in `renderGlance` change.
- With `glance.weather` present, the older per-wind weather card (`weatherCard` of a weather wind) is not shown, so there are not two weather cards; without `glance` Today looks as before.
- Countdowns tick every 30 s and stop when the row leaves the page; under reduced motion nothing animates and they do not tick.
- Follow-up: `weatherKind` in the module duplicates `wxKind` in `winds-workspace.js` (a classic script, it cannot import the module); unify them when winds-workspace becomes a module.
