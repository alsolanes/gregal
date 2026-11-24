---
title: Vents: un avatar d'exemple a cada opció dels selectors d'avatar
status: todo
depends_on: []
---

## Why

In the avatar pickers the options are only names («Cares», «Esperit», «Cap de vent»…): the person cannot see what each style looks like until they choose it.

## Acceptance criteria

- [ ] Web: in Preferences › Avatars (`avatarPrefs` in `winds.js`) each option shows a sample face drawn with that pack (`face` with that pack, a sample wind colour) above or beside its name; the chosen one stays marked. A node test that each choice holds a face of its own pack.
- [ ] Web: the per-wind avatar select of the «Lliure» theme (`avatarField`) shows the same samples, drawn in that wind's colour, instead of a plain `<select>`, keyboard accessible (arrow keys, Enter). A node test.
- [ ] Android: the pack choice in `PrefsScreen.kt` (the `ChoicePills` for avatars) shows a sample `WindFace` per pack above its name. A Compose test that each option has its face.
- [ ] Strings unchanged except any new label, in Catalan and English on both clients.

## Out of scope

- New packs or art (094, 095).

## Notes

- Files: `winds.js` (`avatarPrefs` 1003, `avatarField` 995, `themeChoices`), `PrefsScreen.kt` (~219), `WindFace.kt` (`LocalAvatarTheme` 132, `windPack` 137).
- Asked by the user on 2026-11-24 («als selectors de vents de la config estaria bé tenir l'avatar d'exemple»).
- Web: node tests with `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server in the browser pane, dark and light theme, with before/after screenshots in `output/ui-review/` (ignored).
- Android: run gradle tests with `ANDROID_HOME` set; edit `Strings.kt` by hand (do not run `regen-strings.py`); check on the emulator (`Medium_Phone_API_36`, x86_64 build with `-Pgregal.abi=x86_64`) with a screenshot in the notes. Regex: no `(?U)` (Android's ICU refuses it; `UserAvatarTest` guards this).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
