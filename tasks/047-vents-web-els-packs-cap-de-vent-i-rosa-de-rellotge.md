---
title: "Vents (web): els packs Cap de vent i Rosa de rellotge"
status: todo
depends_on: ["045"]
---

## Why

Two more avatar packs from `mockups/vents-avatars.html`: Cap de vent (the blowing heads of old maps: full cheeks, closed eyes, breath toward where the wind goes) and Rosa de rellotge (the wind rose as a dial, with the wind's point long and in its colour).

## Acceptance criteria

- [ ] `cap` and `rosa` are valid values for the wind's `avatar` and for the account theme; anything else is still 400. A Go test.
- [ ] Both packs appear in the preferences and in the wind editor's picker (Free mode), and every face in Vents uses them when chosen. A node test with the fake DOM.
- [ ] Cap de vent blows toward the opposite of the wind's direction (045's function). Rosa de rellotge points its long tip in the wind's direction. Below 28 px both drop the details (breath to two short lines; only the wind's point), as in the mockup. A node test checks the angle and the small variant.
- [ ] While working, the breath flows (Cap de vent) and the point swings (Rosa); idle, they are still unless expressive motion is on. A node test.

## Out of scope

- Android: 048.
- A combined rose for rotllanes.

## Notes

- Drawing reference: `bufador()` and `rosa()` in `mockups/vents-avatars.html`.
- Update `docs/api-contract.md` with the new values.
