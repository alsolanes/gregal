---
title: Vents web: coherència (llistes, ordinador/navegador, salutació, Desat, accent, Avui buit)
status: done
depends_on: ["125", "126"]
---

## Why

Small things that make Vents feel unfinished or contradictory.

## Acceptance criteria

- [x] Numbered lists in wind messages are readable: the number does not cover the text (`winds-workspace.css` ~1551 overrides the padding set ~1319). A node/CSS test.
- [x] Ordinador and navegador are named consistently: the button and the panel it opens say the same thing, and the sidebar «Ordinador» opens it without cutting the home faces row. Codi and Vents greet with the same words at the same hour (one helper). A node test for each.
- [x] Desat has one title (no tab + H1 + H2 repeating), Feina's tabs sit under its title with one text scale; the primary button keeps one brand accent in light and dark; «Ajusta el de demà» looks the same in both themes; «Tots els vents» leads with the roster and the tidy-up becomes a small chip.
- [x] Avui's empty cards look intentional (no grey smudge: `.vw-glance-card::before` uses `--accent`), no orphan «Segueix…», and on mobile the cards show a scroll cue. Before/after screenshots (light, dark; 1280 and 390 px) in `output/ui-review/127/`.

## Out of scope

- Android (its own review in 124).
- Bigger redesigns (128-137).

## Notes

- Files: winds-workspace.js/.css, vents-avui-glance.js, i18n.js (ca+en), index.html (no hard-coded hex: identitat test).
- Design review of 2026-11-24 (screenshots in output/ui-review/design-review/, not committed; principles: Dots - one named character, paced permissions, see its computer, recent activity; Claude Duet - polish, task cards that open in place, swipe cards, daily brief; Grok companions - the character reacts with its body; shared - the character is the protagonist, one colour per character, short motion tied to events). Person asked: «revisa que tot estigui amb sentit, i afegeix millores estètiques basades en els referents».
- Also from 126's notes: the computer pane still opens on its own after the person closed the pane in that chat, and switching chats leaves the pane open so the other chat's toggle shows pressed; fix both with a node test each.
- Done: the reply rules at the end of `winds-workspace.css` only pad `ul` now; `ol:not([class])` keeps its 36 px for the number circle.
- Done: one name, «L'ordinador de Gregal» / «Gregal's computer» (`computerName()` in winds-workspace.js), for the sidebar row (was «Ordinador»), Gregal's header button and the pane title (was «El navegador de Gregal, en aquest ordinador»). On wide screens (≥761 px) the start faces wrap, so a side pane no longer cuts them; on mobile the row still scrolls (132 adds snap and fade there).
- Done: `salutacio(hora)` in i18n.js (night <6, morning <13, afternoon <20, evening), exposed by i18n-boot.js as `window.gregalGreeting`; home.js and winds-workspace.js both use it. ca `hero.evening` is now «Bon vespre» and `hero.night` «Bona nit» is new; `hero.title` was unused after this and is gone. Vents' greeting lost its trailing dot so both say exactly the same.
- Done: Feina (En marxa, Fes, Activitat) and Desat put the H1 first («Feina» / «Desat») and the tabs under it; Desat's first tab is now «Respostes» / «Replies» and `gregalNotes.saved()` has no «Els teus desats» H2. Tabs, Feina section heads, their text and heading copy use `--t3`.
- Done: `.vw-primary` wears the send button's pair (`--send-bg` / `--send-fg`): unchanged in dark; in light the sea button now carries the logo blue as its text, like send. Read as «one brand accent in both themes»; `docs/identitat.md` still says the light primary accent is `escumaClar`, which it still is (the background).
- Done: «Ajusta el de demà» (`.vw-avui-tune`) is underlined body text in both themes (was `--accent-hi`: a blue link in dark, near-ink in light).
- Done: «Tots els vents» lists your own winds (no squad) and «Crea un vent propi» first, then the squads; the tidy-up banner is a chip under the title («3 sense ús · Posa ordre»).
- Done: Avui: the card glow takes only the card's colour (no `--accent` fallback), invitations have none, the calendar glows `--gl-other`; «Segueix…» shows only once something is followed (the invite card does it otherwise; the old test asserting «Follow…» in the empty state was changed for this requirement); on ≤600 px the row fades at the right edge.
- Done (126 follow-ups): a browser turn in a chat where the person closed the pane leaves it closed and puts the dot on the toggle; switching to another chat closes the pane via `gregal:workpane` `none` (docs-panel and the computer pane both close on it) without marking the chat as closed by the person.
- Screenshots: `output/ui-review/127/` (not committed), before/after, light/dark, 1280 and 390 px, sandbox server with winds, thread and tidy report mocked.
- Follow-up: Android still says «El navegador de Gregal, en aquest ordinador» (OrdinadorSheet.kt), «Desat» as the first saved tab and its own greeting hours (out of scope here).
- Follow-up: the Notes tab of Desat keeps tabs on top and its own «Notes» H1, unlike the Respostes tab.
- Follow-up: the mobile glance fade stays on at the last card; the calendar card's event borders and times still use the accent.
- Follow-up: a browser turn in another wind's chat still opens the computer pane over the chat you are looking at.
