---
title: "Android: el Vidre es nota"
status: done
depends_on: [023]
---

## Why

The Glass style of v1.89.0 is too discreet: with light spots at 20 % and surfaces at 62 % opacity it reads as a slightly grey version of the normal style. The user picked option B («Aurora») from three mock-ups on 2026-11-21.

## Acceptance criteria

- [x] With Glass on, the light of the background is clearly visible: in a screenshot of a real chat (`GlassLookTest`), the strongest background colour in the empty lower part differs from the base colour by at least 60 levels in dark and 50 in light (the old Glass reached 43 at most; the new one, 90 in dark and 60 in light).
- [x] There are four light spots: accent, aqua, lilac and a warm one (`sand`) at the bottom right.
- [x] In dark Glass, cards have a lit top edge (`GlassLookTest` compares the top border with the side one). In light the card is already near-white, so it keeps the usual line.
- [x] Text stays readable at the centre of every spot, in every theme and mode (`GlassTest`): 7:1 for text on a surface and 4:1 for grey text on a surface or a raised surface; 4.5:1 (WCAG AA) for text directly on the background, down from 7:1; and, new, 3:1 for grey text directly on the background (receipts and small labels outside bubbles).

## Out of scope

- The frosted header and composer (Android 12+) were not reworked, but they change: their tint is `surface`, so in light mode it goes from 66 % to 44 % and the blurred chat behind them shows about twice as much. Not checked: `FrostTest` and `ChatFrostUiTest` only cover dark mode.
- Web and desktop Glass.

## Notes

- **A test threshold changed by the user's decision.** `GlassTest` required 7:1 (WCAG AAA) for answer text directly on the background. With 7:1 the strongest spot possible in dark was 24 % on the Calma theme, barely more than the old 20 %. The user chose 4.5:1 (AA) for that case only; text inside cards and bubbles keeps 7:1.
- Values (`Theme.kt`): spots at 38 % in dark and 30 % in light, slightly larger; surfaces at 60 % / 68 % in dark and 44 % / 52 % in light. In dark, 60 % leaves some margin over the Calma theme's minimum (about 54 %) for grey text at 4:1 over the brightest spot, and 38 % is the most the Calma theme allows for 4.5:1 on the background; other themes would allow more.
- Grey text (`muted`) is moved 30 % towards the body text colour in Glass: without it, grey text straight on the background fell to 2.5:1 on Calma dark. `GlassTest` used to require `muted` to stay unchanged in Glass; it now requires it to stay distinct from the body text (requirement changed).
- `GlassTest` checks each spot alone at its centre. The spots overlap, so the real worst case on screen is a little lower than the test sees (estimated by the review at about 4.3:1 for body text on Calma dark where spots and the top veil add up). `faint` text is not checked and is weaker on the background than before.
- The lit edge is in `GlassCard` (`Marea.kt`): white at 40 % at the top in dark, fading to the usual line. At 16 % the test showed no difference with the side.
- Screenshots: `android/app/build/glass-look/` (new) and `glass-look-old/` (old), taken with `GlassLookTest`. When re-taking them, run Gradle with `--rerun --no-build-cache`: a cached test run leaves the previous PNGs in place.
- Not checked on a phone.
