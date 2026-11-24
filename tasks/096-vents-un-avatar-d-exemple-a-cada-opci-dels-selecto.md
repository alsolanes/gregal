---
title: Vents: un avatar d'exemple a cada opció dels selectors d'avatar
status: done
depends_on: []
---

## Why

In the avatar pickers the options are only names («Cares», «Esperit», «Cap de vent»…): the person cannot see what each style looks like until they choose it.

## Acceptance criteria

- [x] Web: in Preferences › Avatars (`avatarPrefs` in `winds.js`) each option shows a sample face drawn with that pack (`face` with that pack, a sample wind colour) above or beside its name; the chosen one stays marked. A node test that each choice holds a face of its own pack.
- [x] Web: the per-wind avatar select of the «Lliure» theme (`avatarField`) shows the same samples, drawn in that wind's colour, instead of a plain `<select>`, keyboard accessible (arrow keys, Enter). A node test.
- [x] Android: the pack choice in `PrefsScreen.kt` (the `ChoicePills` for avatars) shows a sample `WindFace` per pack above its name. A Compose test that each option has its face.
- [x] Strings unchanged except any new label, in Catalan and English on both clients.

## Out of scope

- New packs or art (094, 095).

## Notes

- Files: `winds.js` (`avatarPrefs` 1003, `avatarField` 995, `themeChoices`), `PrefsScreen.kt` (~219), `WindFace.kt` (`LocalAvatarTheme` 132, `windPack` 137).
- Asked by the user on 2026-11-24 («als selectors de vents de la config estaria bé tenir l'avatar d'exemple»).
- Web: node tests with `node --experimental-vm-modules --test internal/web/app/*.test.cjs`; check in a sandboxed server in the browser pane, dark and light theme, with before/after screenshots in `output/ui-review/` (ignored).
- Android: run gradle tests with `ANDROID_HOME` set; edit `Strings.kt` by hand (do not run `regen-strings.py`); check on the emulator (`Medium_Phone_API_36`, x86_64 build with `-Pgregal.abi=x86_64`) with a screenshot in the notes. Regex: no `(?U)` (Android's ICU refuses it; `UserAvatarTest` guards this).
- Direction agreed with the user on 2026-11-24: the general principles of agent products like OpenAI Dots (each agent a character with its own colour, present next to you, its work visible beside the chat), with our own identity: our wind avatars and packs, never copies of their characters or assets.
- Web: `avatarField` is now a `role="radiogroup"` of buttons (`#windAvatar`) with `value` and a `change` event like the `<select>` it replaces, so `windForm` and `winds-workspace.js` (which uses `onchange`) work unchanged. Arrow keys move and pick (wrapping), Enter/Space are the buttons' own click. Samples are drawn by `packFace`, which sets the theme to Free just while calling `face()`; samples are drawn without the wind id so mood/news updates never touch them.
- Free's sample is two winds with different packs (Xaloc with Esperit, Mestral with Rosa), on web and Android; pack samples in Preferences use Tramuntana's colour.
- `internal/web/winds.test.js` was updated because the requirement changed: the editor's picker has radios instead of `<option>`s (its harness also gained a canvas, `Event` and `IntersectionObserver`, since the Mapa sample is now drawn there). No assertion was dropped.
- No new strings: the options reuse the existing pack names.
- Web checked in a sandboxed server (headless Chrome), dark and light, before/after in `output/ui-review/096/`. The Free field screenshot renders `avatarField` on a bare host with the app CSS, not inside the real editor dialog. Android not checked on the emulator (Compose test only).
- Follow-up: in the wind editor the Free samples keep the colour the wind had when the dialog opened; they do not repaint when the colour input changes. The same holds for the «new wind» dialog in `winds-workspace.js`, which calls `avatarField` with no wind, so its samples use the default colour.
