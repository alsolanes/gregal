---
title: Vents: paquet Pelut, vents peluts en 3D fets amb Blender (web i servidor)
status: todo
depends_on: []
---

## Why

The person asked for a more 3D, furry avatar style in the avatar editor. Today the packs are Cares, Esperit, Cap, Rosa, Mapa and Minairons; only Cares' large faces and the minairons are 3D clay (Blender, tools/characters/build.py, tasks 112, 119, 142). A new pack «Pelut» («Furry» in English) gives every wind a soft furry 3D body, its own colour in the fur, rendered in Blender with real hair, and the avatar pickers show the packs as the 3D figures they are.

## Acceptance criteria

- [ ] build.py renders a `pelut` silhouette for every mood (the props of 142 included): a round body covered in short fur (Blender hair curves or particle hair, Cycles), big eyes, small feet; its sheets and mask are in internal/web/app/characters/pelut/ and in manifest.json, and `go test ./internal/web -run TestCharactersManifestMatchesSheets` passes. The new sheets add at most about 400 KB.
- [ ] `pelut` is a pack like `minairo`: the account theme (POST /api/winds/avatar-theme) and a wind's own `avatar` accept it, the server keeps it, and docs/api-contract.md lists it. A Go test.
- [ ] On the web every wind in the Pelut pack is drawn with the pelut clay sheets in its colour, at every size (the small faces use the sheet's still frame or a matching furry SVG fallback while it loads); the avatar pickers (Preferences › Avatars and the wind editor in Free mode) list «Pelut» / «Furry» with ca and en strings. Node tests.
- [ ] The avatar pickers show each pack's sample larger (about 64 px) and, for the clay packs (Cares, Minairons, Pelut), as the 3D clay figure, so the person sees the 3D style before choosing. A Node test and a Playwright screenshot in light and dark.

## Out of scope

- Android (task 156).
- Re-rendering the other families.

## Notes

- Follow task 119 (the minairó clay family) for the whole shape of the change: build.py SILHOUETTES and CAPS, the manifest, characters.js `play`, winds.js `packs`/`themeChoices`/`packFace`, winds_web.go and winds_names.go for the pack names.
- Render only the new family: `blender -b -P tools/characters/build.py -- --only pelut`, and look at the contact sheets (`--preview`) before committing; fur must read as fur at 128 px and as a fuzzy outline at 40 px.
- Blender 5.3 is installed; no new dependency.
