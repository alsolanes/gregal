# Logo animat (mentre treballa)

`build.py` munta el logo de Gregal (la ratxa de vent de
`internal/web/app/ventet.svg`) en 3D amb Blender i en renderitza un bucle de
3 s pensat per a quan l'agent està treballant: el logo sura i gira suaument,
una ratxa de llum recorre la superfície i els ulls parpellegen un cop per
cicle. Tot quadra amb el primer fotograma, així que el bucle no té costura.

Blender 5.3 (`winget install BlenderFoundation.Blender`). No cal el MCP ni la
GUI: va tot en `--background`.

    blender -b -P tools/logo-anim/build.py -- --out output/logo-anim/frames

Opcions: `--start`/`--end` (per defecte 1..72), `--res` (512), `--bevel`
(radi del traç, 30), `--blend` (desa també el .blend).

## Com es fa la geometria

1. El path de l'SVG es reconstrueix com a corba Bézier amb els punts de
   control originals (és C1 contínua) i s'aplana en un polígon dens.
2. La silueta és el path **dilatat 30 px**, que és el que fa el
   `stroke-width="60"` de l'SVG: es desplaça la vora cap enfora i es
   descarten els trossos que cauen dins de la dilatació (el notch queda
   tancat, com al logo). A cada punta convexa aguda de radi de curvatura
   petit hi va un arc de radi 30 (el `stroke-linejoin: round`), i al fons
   del notch mana la intersecció de les dues bandes.
3. El polígon s'extrudeix (44 unitats de gruix) i se li arrodoneix la vora
   amb un bevel de 5. Els ulls són dues el·lipssis del path de l'SVG,
   aferrades a la cara de davant.
4. El color és el degradat de la marca en espai lineal
   (`escumaClara` → `escuma` → `#6F7FF0`), aplicat amb coordenades generades
   i un `Map Range` perquè recorri tot el cos, i emissió a 1.0 perquè el
   color no depengui de la llum. Les llums només hi posen el brillantor.

## Sortida

Els fotogrames van a `output/` (ignorat per git). A partir d'aquí:

    ffmpeg -framerate 24 -i output/logo-anim/frames/f_%04d.png \
        -c:v libvpx-vp9 -pix_fmt yuva420p -b:v 0 -crf 30 \
        output/logo-anim/gregal-working.webm

El WebM porta alfa (per sobre de qualsevol fons). Per a un fons fosc fix,
`-i` amb `-vf "format=rgba,color=#131518"` i H.264, o un GIF amb paleta.

## Regla de la marca

`docs/identitat.md`: «es mou només mentre hi ha feina». Aquest clip és per
a l'estona de treball; en repòs, el logo va quiet.
