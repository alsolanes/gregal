---
title: "Escriptori: sense drecera separada de Vents a Windows"
status: done
depends_on: []
---

## Why

Codi and Vents are chosen from the selector in the same window, so the separate "Gregal Vents" icon on the desktop and in the Start menu is redundant. The user does not want a second app icon or link.

## Acceptance criteria

- [x] The Windows installer creates no "Gregal Vents" shortcut, neither in the Start menu nor on the desktop.
- [x] Installing over an older version removes the "Gregal Vents" shortcuts it left behind.
- [x] The `--vents` command-line switch, only used by those shortcuts, is gone from the desktop app; the app opens Codi and Vents is reached from the selector (or the app menu).

## Out of scope

- The "Gregal Codi" / "Gregal Vents" entries in the app menu: they switch the same window, like the selector.
- Android (one app with both products already) and Linux (no Vents entry).

## Notes

- `desktop/build/installer.nsh`, `desktop/main.js`, `desktop/release.test.js`.
