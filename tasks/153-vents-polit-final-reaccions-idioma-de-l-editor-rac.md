---
title: Vents: polit final (reaccions, idioma de l'editor, Racó, targetes d'Android i proves inestables)
status: done
depends_on: []
---

## Why

Loose ends left by tasks 128-152 that the person would notice, gathered so they get done together.

## Acceptance criteria

- [x] Android: on an error the corner wind only shakes (no extra old shrug), as on the web (129). A test.
- [x] The Docs editor toolbar follows the app language on web and Android (doc-page.js and the Android bridge), not always Catalan (124, 129). A test each.
- [x] Android: secret, handoff and connection cards take the asking wind's colour like approval cards (131); Android Feina running items are cards with the owner's face that expand, and the «Què pot fer» guide opens with 3 cards and «Més», as on the web (136). Tests.
- [x] Web: on the Racó page the sidebar's El pati row is marked current (132); the mobile Avui glance strip stops fading at its last card (127); the md.js pager survives a carousel without arrow buttons (134). Node tests.
- [x] `WindChatLookTest.ventClar` no longer fails in the full Android suite with «performMeasureAndLayout called during measure layout» (it passes alone): draw the frame outside a layout pass. Full suite green twice in a row.

- [x] Web: the workspace wind editor (the one people use; the legacy editor is hidden and goes away in 137) has the minairó barretina colour picker from task 120: shown with the Minairons pack or theme, saved as the fourth letter of `style`, preview follows. A node test.

## Out of scope

- New features.

## Notes

- Sources: Notes of tasks 124, 127, 129, 131, 132, 134, 136.

### Done (2026-11-25)

- Barretina: `winds.js` exposes `barretinaField(parent, w)` (the «Barretina» select: Automàtica + the six, starting on the 4th style letter) and `avatarPack(w)`. `winds-workspace.js` uses them in the profile (saves on change as `style.slice(0,3) + letter`, redraws the profile face, hidden unless the wind is drawn as a minairó; it re-checks when the pack changes) and in «Crea el vent» (in the appearance row, preview and saved `style` follow). The legacy `windForm` was not touched (137 removes it); until then the option list exists twice. Tests: `winds-avatar-picker.test.cjs` (real field), `winds-start.test.cjs` «a wind’s profile picks its barretina…», «with the Minairons theme…», «in the create dialog the barretina picker shows only with the Minairons pack».
- Racó: `mount()` watches `#racoPage`'s `hidden` and re-renders the nav; `renderNav` treats the app view `raco` as `pati` for `aria-current`. Test: `winds-start.test.cjs` «on the Racó page the sidebar marks El pati…» (the test page now has a `#racoPage`).
- Avui glance (mobile): the row sets `data-fade` (`start`, `end`, `start end`) on scroll and resize like the start faces; the CSS masks only that side. Requirement change: the old CSS assertion «the row always fades at the right» now checks the three `data-fade` rules and no unconditional mask.
- md.js pager: `gxSync` checks `.gx-prev`/`.gx-next` exist. Test in `md-cards.test.cjs`.
- Doc editor language: `doc-page.js` calls `defineixPerDefecte(GregalAndroid.lang() || ?lang=)` before loading the panel; the panel's «Open in a window» adds `&lang=` (requirement change in two `docs-panel.test.cjs` URL assertions); Android `DocPageBridge(key, appLang)` has `lang()` fed from `vm.idioma`. Tests: `doc-page.test.cjs` «the document window speaks the app language…», `LiveDocTest.theBridgeGivesThePageTheAppLanguage`.
- Android corner shake: `WindFace(shrugs = false)` for `WindPresence`, so `windTurnCue` does not add the FAILED shrug there; other faces keep it. Test: `WindVidaTest.aFaceThatDoesNotShrugStaysStillOnAFailedTurn` (fails with the shrug forced on).
- Android cards: `WindAskCards` passes `colourOf(a)` to secret, handoff and connection cards (questions keep the accent, as on the web). `OrdButton`'s primary with an accent now uses `allowPaint` (wind colour with readable ink, like Permet), which also applies in the Ordinador sheet. Test: `WindIdentityLookTest.lesAltresTargetesDEsperaSonDelVentQueLesDemana`.
- Android Feina: running tasks are `FeinaCard` (owner's face from the first agent's name, title, «En marxa», chevron), tinted with the owner's colour; tapping opens files (up to 4) and «Obre el fil» in place. Test: `FeinesDialogTest.aRunningTaskIsACardWithItsOwnersFaceThatOpensInPlace`.
- Android guide: three topics and «Més» / «More» (`app.vents.more`) that adds the rest. Test: `GregalGuideUiTest`.
- `WindChatLookTest`: the frame is drawn in `ui.runOnIdle` instead of `runOnUiThread`. The failure did not reproduce in a full run before the change either, so the fix is not proven by a red-then-green run; the full suite (1246 tests, `--rerun-tasks`) passed twice in a row after it.
- Follow-up: Android `FeinaCard` has no step list (Android `WindTask` carries no events) and no expanded/collapsed state for screen readers; the web shows the last 5 steps and `aria-expanded`.
- Follow-up: Android «Més» in the guide does not move focus to the first new card as the web does.
- Follow-up: the desktop's `openDocWindow` (Electron) builds its own URL without `lang`; with no saved language the window still uses the default.
- Follow-up: when 137 removes `windForm`, its barretina select can go and `barretinaField` is the only one.
