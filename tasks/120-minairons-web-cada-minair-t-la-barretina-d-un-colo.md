---
title: Minairons web: cada minairó té la barretina d'un color i trets propis
status: done
depends_on: []
---

## Why

With every wind drawn as a minairó, they look too alike; each should be recognisable at a glance while staying clearly a Catalan minairó.

## Acceptance criteria

- [x] Each minairó's barretina takes a colour from the curated palette (notes), chosen deterministically per wind and never too close to its body colour; the black band stays. A node test over many ids: deterministic, spread across the palette, contrast with the body.
- [x] Small per-wind traits on top of 117's (beard length, fold side, accessory): eyebrow shape (straight, arched, bushy), a moustache style, and rarely round glasses — readable at 48 px, dropped below 28 px. Screenshots of 8 different minairons at 24/48/120 px, dark and light, in `output/ui-review/120/`.
- [x] The wind editor (Lliure mode and the profile) lets the person override the barretina colour from the palette; saved per wind in the wind's style. A node test.

## Out of scope

- Android (121).
- Clay (122).

## Notes

- Files: `winds.js` (minairó drawing ~666-740, `face`, wind editor), `winds.css`.
- Barretina palette (curated, each keeps the wide black band so it still reads as a barretina): vermell clàssic #C9342C, morada #6E2F6B (the traditional purple barretina), plus a few that stay folk-like and readable on every body colour: verd fosc, mostassa/ocre, blau marí, granat. Pick per wind deterministically from its id, avoiding a barretina hue too close to the wind's own colour (fall back to the next).
- User (2026-11-24): «busca forma de diferenciar una mica els minairons, amb colors diferents de barretines per exemple». Earlier: «la cinta negra de baix de les barretines, així se sabrà que és més català» (done in 137ae81b: black band #17141A).

### For Android (121): exact values

Palette, in this order (index, style letter, hex, label ca/en):

| i | letter | hex | ca | en |
|---|---|---|---|---|
| 0 | v | #C9342C | Vermella | Red |
| 1 | m | #6E2F6B | Morada | Purple |
| 2 | f | #2E6B3F | Verd fosc | Dark green |
| 3 | o | #C8962E | Mostassa | Mustard |
| 4 | b | #26406E | Blau marí | Navy blue |
| 5 | g | #7D1F33 | Granat | Garnet |

Pick (`barretinaOf` in `winds.js`):

1. If the 4th letter of `style` is one of `v m f o b g`, that colour (no hue check: it is the person's choice). Anything else (absent, unknown) is automatic.
2. Automatic: `h` = the same id hash as `direction()` (now `idHash`): over `windId(w)` (the id, or the slug of the name for an unsaved wind), `h = (h * 31 + UTF-16 unit) >>> 0`, i.e. unsigned 32-bit Java `String.hashCode`. `start = h % 6`.
3. Walk the palette from `start`, wrapping, and take the first colour whose hue is at least 30° from the body colour's hue (HSV/HSL hue in degrees, circular distance `min(d, 360 - d)`; a grey with max == min has hue 0). At most 3 palette hues fit in any 60° window, so a colour is always found.

Expected picks (h, start, pick with body #A8CDE2, pick with body #EFA68F):

| id | h | h % 6 | body #A8CDE2 | body #EFA68F |
|---|---|---|---|---|
| llevant | 254985750 | 0 | v #C9342C | m #6E2F6B |
| xaloc | 113819575 | 1 | m #6E2F6B | m #6E2F6B |
| migjorn | 1058105100 | 0 | v #C9342C | m #6E2F6B |
| garbi | 98125087 | 1 | m #6E2F6B | m #6E2F6B |
| ponent | 3312353340 | 0 | v #C9342C | m #6E2F6B |
| mestral | 954971012 | 2 | f #2E6B3F | f #2E6B3F |
| tramuntana | 1745173155 | 3 | o #C8962E | b #26406E |
| gregal | 3057194936 | 2 | f #2E6B3F | f #2E6B3F |
| minairo-1 | 586109339 | 5 | g #7D1F33 | m #6E2F6B |
| minairo-2 | 586109340 | 0 | v #C9342C | m #6E2F6B |

Traits use the face `seed` (key = `id || name`; `seed = (seed * 31 + code point) % 997`), not `h`. Only at 28 px and up; below, no brows, no glasses and the plain `lobes` moustache:

- brows: `['straight', 'arched', 'bushy'][seed % 7 % 3]`, white (#FFFDF7) with a crease outline, over the band's lower edge above each eye.
- moustache: `['lobes', 'walrus', 'handlebar'][seed % 11 % 3]` (`lobes` is 117's).
- round glasses when `seed % 13 === 5` (about 1 in 13): stroke #2B2530 1.4, r 7.4 around each eye, white fill at 0.14.
- Moduli 7, 11 and 13 are prime to 117's `seed % 2` (fold) and `seed % 3` (accessory), so ids one letter apart (minairo-1, minairo-2) still look different.

| id | seed | brows | moustache | glasses |
|---|---|---|---|---|
| llevant | 293 | straight | walrus | no |
| xaloc | 61 | bushy | lobes | no |
| migjorn | 251 | straight | lobes | no |
| garbi | 347 | arched | lobes | no |
| ponent | 300 | straight | lobes | no |
| mestral | 828 | bushy | lobes | no |
| tramuntana | 83 | straight | lobes | yes |
| gregal | 118 | straight | handlebar | no |
| minairo-1 | 302 | arched | handlebar | no |
| minairo-2 | 303 | bushy | lobes | no |

- The mouth ink stays on the classic red (#C9342C mixed 0.65 to #1B1530) whatever the barretina colour.
- Editor: a «Barretina» select (Automàtica + the six) in the wind editor (`windForm`: new wind, and «Edita el caràcter» from the profile), shown while the wind is drawn as a minairó (Lliure with the Minairons pack, or the Minairons theme). Saving sends `style` as eyes+mouth+shape plus the letter, or only the three letters for Automàtica. The server stores `style` as is; `/api/winds/cap/profile` still only accepts 3-letter styles, which is fine because the cap mascot is not edited here.
- Screenshots (not committed): `output/ui-review/120/sizes.png`, `sizes-2x.png` (8 rose winds at 24/48/120 px, dark and light), `big.png` (200 px), `glasses.png` (8 winds that wear glasses). Harness: `%TEMP%/g120/shot.cjs` + `page.html`.
- Follow-up: the Vents workspace profile (`winds-workspace.js` `profile()`) has its own pack picker but no barretina picker; it could reuse the same select.
- Follow-up: `docs/api-contract.md` says the `style` of `/api/winds/suggest` is 3 letters; a wind's `style` can now carry a fourth (barretina) letter, which the contract does not mention yet.
