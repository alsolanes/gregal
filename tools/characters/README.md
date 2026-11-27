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
frames per mood (`contact-*`), every frame of every mood per silhouette
(`motion-*`), and six minairons each in its own barretina colour
(`minairons-*`), at 128 and 40 px, on light and dark. With `--only` the
contact sheets show only those silhouettes, in the moods all of them have.

Run `go test ./internal/web -run TestCharactersManifestMatchesSheets` after
rendering: it checks that `manifest.json` and the sheets agree.

## Output

`internal/web/app/characters/`:

- `<silhouette>/<mood>.webp`: the frames of one mood side by side, each
  `size` px square, in a neutral grey clay with dark eyes and mouth. Its
  alpha is rounded to 32 levels: WebP keeps alpha lossless, and that halves
  what it costs without a visible step.
- `<silhouette>/<mood>.mask.webp`: the same frames at half size
  (`maskSize`), white with alpha where the wind's colour goes (body, feet,
  lids); the eyes, their catchlights and the mouth are cut out, and it fades
  under the pink cheek blush so the blush stays pink on every wind.
- `<silhouette>/<mood>.cap.webp`, only for the silhouettes listed in `caps`
  (`minairo`): the same kind of mask for the barretina (its tube and crown,
  not the black band), which the sheet shows in the neutral clay too.
- `manifest.json`: `size`, `maskSize`, `fps`, the silhouettes (`pebble`,
  `cloud`, `curl`, and `gota`, `sol` and `onada` from task 170, with the letter of the wind style each one stands for, as
  in `winds.js`; and `minairo` and `pelut`, which stand for the minairó and
  pelut avatar packs whatever the wind's style), and per mood the number of `frames` and whether it `loop`s
  or plays once and holds the last frame (`done`, `failed`). `look` (task 169) is not a
  mood of winds.js: 9 still frames looking at a spot, `3 * row + column` with row 0 up
  and column 0 left (4 looks straight ahead), which a large idle face shows toward the
  pointer or the finger; it is saved at `LOOK_QUALITY` to stay in budget. The work moods also
  name the `prop` the wind handles in them and the `still` frame a player shows
  with reduced motion (one with the prop in view; the first frame otherwise).

To tint, `multiply` the sheet by the wind's colour where the mask is, scaling
the mask up to `size` (canvas: draw the mask scaled, fill with the colour
using `source-in`, then draw the sheet with `multiply`; keep the sheet's
alpha). Tint the barretina the same way through the cap mask, with the
minairó's barretina colour (`barretinaOf` in `winds.js`, `minairoLook` in
`WindFace.kt`). All silhouettes share one framing: the ground line and the scale are
the same in every sheet, with room above for hops and the curl.

## Change a design

- Silhouettes: `SILHOUETTES` lists, per wind family, the width it is scaled
  to, the height of its eyes and the metaballs that make the body. The body
  is normalised to that width and settled flat on the ground, so a new shape
  only needs its balls. The minairo adds a white felt beard with a two-lobed
  moustache, a rosy nose and a barretina with a black ribbon (`minairo()`,
  `barretina()`), none of them in the mask (the barretina has its own, `CAPS`); `follow()` makes the beard and the
  cap's tip lag behind the body with two damped springs. The pelut (task 155)
  is a round ball covered in short particle hair (`fur()`), in the mask so the
  fur takes the wind's colour, shorter round the face so the eyes stay clear;
  its feet step out of the fur. Its sheets keep 8 levels of alpha and
  `FUR_QUALITY`: the fur's fringe is noisy anyway, and so it stays in budget.
- Moods: `MOODS` gives frames and loop; `pose()` says what every frame looks
  like (squash, hop, tilt, turn, lean, gaze, blink, lids, happy eyes, mouth).
  It is a pure function of the frame number, so renders are reproducible:
  the clay grain is seeded per silhouette, Cycles runs on the CPU with a
  fixed seed and no denoiser, and dithering is off (both the denoiser and
  dithering change pixels on every run). A few pixels of some frames can
  still differ by one level in 255 between two runs (inside Blender, even on
  one thread), which changes those WebP files' bytes but not what you see.
- Props (task 142): in the work moods the wind grows two clay hands (in the
  mask, so they take its colour) and handles an object that keeps its own
  colours: `searching` sweeps a magnifier and peeks through it with one big
  eye, `reading` turns a page of a book, `writing` scribbles on a pad with a
  pencil, `working` types on a laptop and hits enter, `thinking` pushes two
  cogs round with a hand on its chin, `done` pops a party popper whose
  confetti lands on its head. `PROPS` names each one and its still frame,
  `props()` builds them and `handle()` moves them and the hands frame by
  frame, from the same curves `pose()` uses for the body and the eyes.
- Look: `material()` (clay with subsurface, sheen and a seeded grain bump),
  the pink cheek blush and contact shadow (`glow_material()`), `scene()`
  (camera, a warm key, a dim fill and a cool rim light), `SIZE` and `QUALITY`.

Keep all the sheets together under 2 MB (`TestCharactersMinairoWithinBudget`).

## Licence

The script and the renders are Gregal's, under the project's MIT licence. Blender
is GPL, but its licence does not cover what it renders.
