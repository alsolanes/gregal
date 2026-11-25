---
title: Vents web: una fila de cares viva a l'inici i el pati a primera fila
status: done
depends_on: ["125", "126", "127"]
---

## Why

The home shows only some winds with no cue, and the fun (El pati, El Racó) is buried under settings.

## Acceptance criteria

- [x] The home faces row scroll-snaps with a fading edge on mobile so every wind is reachable; faces breathe out of sync and show a dot when busy; tapping a face zooms it into the chat header (View Transitions where supported, plain navigation otherwise). Motion respects reduced motion (web: prefers-reduced-motion; Android: LocalWindMotion). A node test for the busy dot and that every wind is in the row.
- [x] El pati is in the bottom bar and the sidebar (with El Racó folded into it), not under «Configuració i més»; the home has a highlighted tab in the bottom bar; Mestral is no longer cut at the pati's left edge. A node test. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/132/`.

## Out of scope

- Android (133).

## Notes

- Files: winds-workspace.js (~132-138 nav), winds-workspace.css.
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».

### Done (2026-11-25)

- Start row (`home()` in winds-workspace.js): every wind, Gregal first. On ≤760 px it scroll-snaps to the start of each face and `data-fade` (`start`, `end`, `start end`, set on scroll and resize) puts a mask fade only on the side that still hides faces; ≥761 px keeps 127's wrap, so no fade. Each face breathes on a 3.6 s cycle with `--vw-breath` offset by position (individual `scale`, so the news bounce still wins while it plays). A wind whose turn runs in this client (`running(w)`, the same rule the face tap uses) gets `.vw-busy-dot` in its colour with «Treballant…» / «Working…» for screen readers; `busyDots()` runs on paint and when the turn ends. Breath, dot pulse and the flight's timing sit inside `prefers-reduced-motion:no-preference`.
- Tap: like 133 on Android, a start face resumes the wind's newest thread (`state.solo`, newest first, via `session/open`), or its usual chat (`/api/winds/chat`) if it has none, or the running one; no more `POST /api/winds/sessions` from a face (`startWith` is gone). Requirement change: the old test «tapping a face opens a new conversation» now checks the resume, and the `paneChat` fixture's «back to Xaloc» chat is the thread the face opens (`company-xaloc-s2`).
- Flight: `showWindThread` wraps its render in `document.startViewTransition` when the tap came from a start face (`flyFrom`): the face is named `vw-face` before, the chat's `.vw-wind-corner` after, and the name is cleared when it finishes. No View Transitions or reduced motion: it just renders. With history the wind lands in the header slot and then flies to its corner as in 138.
- El pati: a top-level sidebar row after Desat and a bottom-bar tab; it and El Racó left «Configuració i més» (and `settingsViews`). The Racó has no row: «El Racó dels vents» is a pill at the right of the pati's tool row (hidden when the `raco` plugin is off, `gregalPlugins.viewAllowed`). Bottom bar: Inici, Avui, Feina, Desat, El pati, Vents (6 columns); the Racó/Automatitzacions page bars mark El pati on the Racó.
- Mestral: centring the square cut the corner next to it mid-way. `startLeft` (winds-pati.js, exported and tested in `winds-pati.test.cjs`) moves the left edge to the nearest corner start that still shows the target zone whole, else centres it (phones). The far edge now has the same fade as the start row (`.pati-scene[data-fade]`), so a wind at that edge reads as «more this way»; that fade has no node test.
- Screenshots (git-ignored): `output/ui-review/132/` before/after `home`, `home-scrolled` (390), `pati`, plus after `busy`, `flight`, `landed`, light/dark, 1280/390. Sandbox on port 18940 with mocked winds/threads. The flight's middle frame was not caught headless (the 220 ms frame is already landed); its wiring is checked by the node test only.
- Follow-up: on the Racó page (a shell view) the sidebar's El pati row is not marked current.
- Follow-up: the mobile glance fade (127) and this row use the same fade idea in two places; the glance still fades at its last card.
