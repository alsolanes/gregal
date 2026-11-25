---
title: Vents: polit final (reaccions, idioma de l'editor, Racó, targetes d'Android i proves inestables)
status: todo
depends_on: ["137"]
---

## Why

Loose ends left by tasks 128-152 that the person would notice, gathered so they get done together.

## Acceptance criteria

- [ ] Android: on an error the corner wind only shakes (no extra old shrug), as on the web (129). A test.
- [ ] The Docs editor toolbar follows the app language on web and Android (doc-page.js and the Android bridge), not always Catalan (124, 129). A test each.
- [ ] Android: secret, handoff and connection cards take the asking wind's colour like approval cards (131); Android Feina running items are cards with the owner's face that expand, and the «Què pot fer» guide opens with 3 cards and «Més», as on the web (136). Tests.
- [ ] Web: on the Racó page the sidebar's El pati row is marked current (132); the mobile Avui glance strip stops fading at its last card (127); the md.js pager survives a carousel without arrow buttons (134). Node tests.
- [ ] `WindChatLookTest.ventClar` no longer fails in the full Android suite with «performMeasureAndLayout called during measure layout» (it passes alone): draw the frame outside a layout pass. Full suite green twice in a row.

## Out of scope

- New features.

## Notes

- Sources: Notes of tasks 124, 127, 129, 131, 132, 134, 136.
