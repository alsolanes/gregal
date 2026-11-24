# Clay winds

`build.py` models, lights, animates and renders the winds as small clay
characters with Blender, and writes the sprite sheets the web and Android
players use (tasks 113 and 114).

## Re-render

Blender 5.3 (free, `winget install BlenderFoundation.Blender`):

    blender -b -P tools/characters/build.py
    blender -b -P tools/characters/build.py -- --only curl --moods idle,done
    blender -b -P tools/characters/build.py -- --moods "" --preview output/ui-review/112

The first renders everything (about 6 minutes with Cycles on the CPU of a laptop),
the second only some sheets, the third only writes contact sheets of what is
already rendered: every silhouette tinted in two wind colours with a few
frames per mood (`contact-*`), and every frame of every mood per silhouette
(`motion-*`), at 128 and 40 px, on light and dark.

Run `go test ./internal/web -run TestCharactersManifestMatchesSheets` after
rendering: it checks that `manifest.json` and the sheets agree.

## Output

`internal/web/app/characters/`:

- `<silhouette>/<mood>.webp`: the frames of one mood side by side, each
  `size` px square, in a neutral grey clay with dark eyes and mouth.
- `<silhouette>/<mood>.mask.webp`: the same frames at half size
  (`maskSize`), white with alpha where the wind's colour goes (body, feet,
  lids); the eyes, their catchlights and the mouth are cut out, and it fades
  under the pink cheek blush so the blush stays pink on every wind.
- `manifest.json`: `size`, `maskSize`, `fps`, the silhouettes (`pebble`,
  `cloud`, `curl`, with the letter of the wind style each one stands for, as
  in `winds.js`), and per mood the number of `frames` and whether it `loop`s
  or plays once and holds the last frame (`done`, `failed`).

To tint, `multiply` the sheet by the wind's colour where the mask is, scaling
the mask up to `size` (canvas: draw the mask scaled, fill with the colour
using `source-in`, then draw the sheet with `multiply`; keep the sheet's
alpha). All silhouettes share one framing: the ground line and the scale are
the same in every sheet, with room above for hops and the curl.

## Change a design

- Silhouettes: `SILHOUETTES` lists, per wind family, the width it is scaled
  to, the height of its eyes and the metaballs that make the body. The body
  is normalised to that width and settled flat on the ground, so a new shape
  only needs its balls.
- Moods: `MOODS` gives frames and loop; `pose()` says what every frame looks
  like (squash, hop, tilt, turn, lean, gaze, blink, lids, happy eyes, mouth).
  It is a pure function of the frame number, so renders are reproducible:
  the clay grain is seeded per silhouette, Cycles runs on the CPU with a
  fixed seed and no denoiser, and dithering is off (both the denoiser and
  dithering change pixels on every run). A few pixels of some frames can
  still differ by one level in 255 between two runs (inside Blender, even on
  one thread), which changes those WebP files' bytes but not what you see.
- Look: `material()` (clay with subsurface, sheen and a seeded grain bump),
  the pink cheek blush and contact shadow (`glow_material()`), `scene()`
  (camera, a warm key, a dim fill and a cool rim light), `SIZE` and `QUALITY`.

Keep all the sheets together under 1.5 MB.

## Licence

The script and the renders are Gregal's, under the project's MIT licence. Blender
is GPL, but its licence does not cover what it renders.
