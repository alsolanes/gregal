---
title: "Vents (web): la barra lateral no es mou en canviar de vent"
status: done
depends_on: ["026"]
---

## Why

On the web and the desktop app, changing wind from the sidebar makes the sidebar scroll, which is uncomfortable (reported 2026-11-21). `renderNav` (`internal/web/app/winds-workspace.js` ~79) runs on every change and:

1. empties the sidebar first (`nav.replaceChildren(top)`), so the scrolling ancestor (`#side .side-scroll`) loses its position;
2. sets `more.open` from localStorage when «Settings & more» was left open; the browser fires `toggle` for that as if the person had opened it, and the listener scrolls smoothly to the end of the menu.

## Acceptance criteria

- [x] Changing wind (or any re-render of the sidebar) keeps the sidebar's scroll position. A node test with the fake DOM checks `scrollTop` of `.side-scroll` is the same before and after a `talkTo`.
- [x] Re-rendering with «Settings & more» open does not scroll; opening it by hand still brings its options into view. A node test for both.

## Notes

- Builds on the fake-DOM harness that task 026 adds (`internal/web/app/winds-start.test.cjs`).
- Tests: `internal/web/app/winds-start.test.cjs` («changing wind from the sidebar keeps the sidebar where it was», «a sidebar drawn with «Settings & more» left open does not scroll; opening it by hand does»). The fake `Element` now models a scroll box (20 px per node, 100 px window, `scrollTop` clamped on every mutation, like a forced layout) and `<details>` firing one asynchronous `toggle` when `open` changes; `boot` takes a `storage` for localStorage and puts `#windContextNav` inside `#side .side-scroll` as in `index.html`.
- Fix: `renderNav` keeps the `.side-scroll` ancestor's `scrollTop` and puts it back at the end; the first `toggle` after drawing the menu open is ignored for scrolling (it still saves the state).
- Not checked in a real browser; only the node tests.
