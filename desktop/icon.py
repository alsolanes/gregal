"""Genera la marca Gregal (Ventet) i les icones de l'app des d'una sola geometria.

Dues peces:
- la marca sola (ventet.svg i /app/icon.png, la que es veu dins la UI): la ratxa
  de vent amb ulls, gruixuda i amb les puntes arrodonides;
- la icona de l'app (escriptori, favicon, manifest, extensió i Office): la
  mateixa marca sobre la rajola fosca del llançador d'Android
  (android/.../ic_gregal_background.xml), que contrasta amb barres clares i fosques.

Les mides petites (16-32 px) no són una reducció de la gran: cada una té la
rajola, el gruix i els ulls ajustats i encaixats al píxel. El .ico s'escriu a mà
(BMP de 32 bits fins a 128 px i PNG a 256 px) perquè Windows tria el marc exacte.

Colors de docs/identitat.md: escumaClara #A3B0FF → escuma #8193FF → #6F7FF0 al cos,
fons #13151B als ulls, abisme/superficieAlta a la rajola.
"""
from pathlib import Path
import io
import math
import struct

from PIL import Image, ImageDraw, ImageFilter

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parent

# Vista 1024. Sense l'escletxa prima de la cua (desapareixia per sota de 48 px):
# un sol cos, una cua i la boca en falca.
START = (800, 118)
CURVES = [
    ((955, 370), (925, 655), (655, 812)),    # panxa, a la dreta
    ((490, 910), (265, 925), (95, 865)),     # vora de sota fins a la punta de la cua
    ((270, 798), (410, 748), (500, 682)),    # vora de dalt de la cua, cap a la boca
    ((450, 612), (370, 556), (232, 520)),    # la boca, fins a la barbeta de dalt
    ((520, 430), (765, 345), (800, 118)),    # esquena, fins a la punta
]
ROUND = 30          # radi del traç que engreixa i arrodoneix les puntes (vista 1024)
EYES = [(668, 548, 38, 64), (800, 530, 36, 61)]  # cx, cy, rx, ry; girats 18°
EYE_TILT = 18
BODY = ('#A3B0FF', '#8193FF', '#6F7FF0')
EYE = '#13151B'
TILE = ('#262B3A', '#161922', '#0E1015')  # radial des de dalt a la dreta
TILE_EDGE = (163, 176, 255, 46)             # filet d'escumaClara, perquè la rajola es vegi a la barra fosca


def outline(steps=48):
    points, p0 = [START], START
    for p1, p2, p3 in CURVES:
        for i in range(1, steps + 1):
            t, u = i / steps, 1 - i / steps
            points.append(tuple(u**3*p0[k] + 3*u*u*t*p1[k] + 3*u*t*t*p2[k] + t**3*p3[k] for k in (0, 1)))
        p0 = p3
    return points


def bbox():
    pts = outline()
    xs, ys = [p[0] for p in pts], [p[1] for p in pts]
    return min(xs) - ROUND, min(ys) - ROUND, max(xs) + ROUND, max(ys) + ROUND


def ellipse(cx, cy, rx, ry, tilt, n=64):
    a = math.radians(tilt)
    out = []
    for i in range(n):
        t = i * math.tau / n
        dx, dy = rx * math.cos(t), ry * math.sin(t)
        out.append((cx + dx*math.cos(a) - dy*math.sin(a), cy + dx*math.sin(a) + dy*math.cos(a)))
    return out


def lerp(c1, c2, t):
    return tuple(round(a + (b - a) * t) for a, b in zip(c1, c2))


def hexrgb(h):
    return tuple(int(h[i:i+2], 16) for i in (1, 3, 5))


def gradient(size, stops, start, end):
    """Degradat lineal de start a end (coordenades en píxels de size)."""
    w, h = size
    cols = [hexrgb(s) for s in stops]
    img = Image.new('RGB', size)
    px = img.load()
    vx, vy = end[0] - start[0], end[1] - start[1]
    n2 = vx*vx + vy*vy
    for y in range(h):
        for x in range(w):
            t = max(0.0, min(1.0, ((x - start[0])*vx + (y - start[1])*vy) / n2))
            if t < 0.55:
                c = lerp(cols[0], cols[1], t / 0.55)
            else:
                c = lerp(cols[1], cols[2], (t - 0.55) / 0.45)
            px[x, y] = c
    return img


def mark(px, box, round_=ROUND, eye_scale=1.0, eyes=True):
    """La marca dins el quadre box=(x, y, mida) d'un llenç px×px, ja antialiasada."""
    ss = 4 if px >= 64 else 16
    big = px * ss
    bx0, by0, bx1, by1 = bbox()
    span = max(bx1 - bx0, by1 - by0)
    k = box[2] * ss / span
    ox = box[0] * ss + (box[2] * ss - (bx1 - bx0) * k) / 2 - bx0 * k
    oy = box[1] * ss + (box[2] * ss - (by1 - by0) * k) / 2 - by0 * k
    tr = lambda p: (ox + p[0] * k, oy + p[1] * k)
    shape = Image.new('L', (big, big))
    d = ImageDraw.Draw(shape)
    pts = [tr(p) for p in outline()]
    d.polygon(pts, fill=255)
    w = max(1, round(2 * round_ * k))
    d.line(pts + [pts[0]], fill=255, width=w, joint='curve')
    r = w / 2
    for x, y in pts:
        d.ellipse((x - r, y - r, x + r, y + r), fill=255)
    if eyes:
        for cx, cy, rx, ry in EYES:
            d.polygon([tr(p) for p in ellipse(cx, cy, rx * eye_scale, ry * eye_scale, EYE_TILT)], fill=0)
    alpha = shape.resize((px, px), Image.Resampling.LANCZOS)
    top, bottom = tr(START), tr(CURVES[1][2])
    body = gradient((px, px), BODY, (top[0] / ss, top[1] / ss), (bottom[0] / ss, bottom[1] / ss)).convert('RGBA')
    body.putalpha(alpha)
    return body, (ox / ss, oy / ss, k / ss)


def pixel_eyes(icon, xf, size):
    """Ulls encaixats al píxel per a les mides petites: rectangles plens de fons
    (w×h px) al centre arrodonit de cada ull, en lloc d'una el·lipse borrosa."""
    ox, oy, k = xf
    d = ImageDraw.Draw(icon)
    w, h = size
    for cx, cy, _, _ in EYES:
        x = round(ox + cx * k - w / 2)
        y = round(oy + cy * k - h / 2)
        d.rectangle((x, y, x + w - 1, y + h - 1), fill=hexrgb(EYE) + (255,))


# Ajust per mida: marge i cantonades de la rajola, mida de la marca dins la
# rajola, gruix (radi del traç a la vista 1024) i ulls. Fins a 32 px els ulls són
# rectangles de píxels sencers (pix); a partir de 40, el·lipses (eye = escala).
TUNE = {
    16:  dict(inset=0,   radius=3,   mark=12.0, round=54, pix=(1, 2), edge=False),
    20:  dict(inset=0,   radius=4,   mark=15.0, round=50, pix=(1, 3), edge=False),
    24:  dict(inset=0.5, radius=5,   mark=18.6, round=44, pix=(2, 3), edge=True),
    32:  dict(inset=1,   radius=6.5, mark=24.0, round=42, pix=(2, 3), edge=True),
    40:  dict(inset=1.5, radius=8,   mark=29.5, round=36, eye=1.12, edge=True),
    48:  dict(inset=2,   radius=9.5, mark=35.0, round=34, eye=1.08, edge=True),
}


def tune(px):
    if px in TUNE:
        return TUNE[px]
    s = px / 256  # a partir de 64 px, proporcions de la de 256
    return dict(inset=12*s, radius=50*s, mark=184*s, round=ROUND, eye=1.0, edge=True)


def tile_icon(px):
    t = tune(px)
    ss = 8
    big = px * ss
    inset, radius = t['inset'] * ss, t['radius'] * ss
    mask = Image.new('L', (big, big))
    ImageDraw.Draw(mask).rounded_rectangle((inset, inset, big - inset - 1, big - inset - 1), radius, fill=255)
    # Fons radial, la llum a dalt a la dreta com a l'Android.
    cols = [hexrgb(c) for c in TILE]
    bg = Image.new('RGB', (px, px))
    p = bg.load()
    cx, cy, rad = px * 0.74, px * 0.22, px * 0.89
    for y in range(px):
        for x in range(px):
            t_ = min(1.0, math.hypot(x + .5 - cx, y + .5 - cy) / rad)
            p[x, y] = lerp(cols[0], cols[1], t_ / .6) if t_ < .6 else lerp(cols[1], cols[2], (t_ - .6) / .4)
    icon = Image.new('RGBA', (px, px))
    icon.paste(bg, (0, 0), mask.resize((px, px), Image.Resampling.LANCZOS))
    if t['edge']:
        ring = Image.new('L', (big, big))
        dr = ImageDraw.Draw(ring)
        lw = max(ss, round(px / 128 * ss))
        dr.rounded_rectangle((inset, inset, big - inset - 1, big - inset - 1), radius, outline=255, width=lw)
        edge = Image.new('RGBA', (px, px), TILE_EDGE[:3] + (0,))
        edge.putalpha(ring.resize((px, px), Image.Resampling.LANCZOS).point(lambda v: v * TILE_EDGE[3] // 255))
        icon = Image.alpha_composite(icon, edge)
    m = t['mark']
    box = ((px - m) / 2, (px - m) / 2 + px * 0.015, m)
    pix = t.get('pix')
    body, xf = mark(px, box, t['round'], t.get('eye', 1.0), eyes=not pix)
    if pix:
        body = body.filter(ImageFilter.UnsharpMask(radius=0.6, percent=50, threshold=0))
        pixel_eyes(body, xf, pix)
    icon = Image.alpha_composite(icon, body)
    return icon


def write_ico(path, frames):
    """ICO amb marcs BMP de 32 bits (fins a 128) i PNG (256)."""
    blobs = []
    for im in frames:
        w, h = im.size
        if w >= 256:
            buf = io.BytesIO()
            im.save(buf, 'PNG', optimize=True)
            blobs.append(buf.getvalue())
            continue
        rgba = im.convert('RGBA')
        header = struct.pack('<IiiHHIIiiII', 40, w, h * 2, 1, 32, 0, 0, 0, 0, 0, 0)
        rows = []
        for y in range(h - 1, -1, -1):
            row = rgba.crop((0, y, w, y + 1)).tobytes()
            rows.append(bytes(b for i in range(0, len(row), 4) for b in (row[i+2], row[i+1], row[i], row[i+3])))
        mask_row = ((w + 31) // 32) * 4
        blobs.append(header + b''.join(rows) + b'\0' * (mask_row * h))
    out = struct.pack('<HHH', 0, 1, len(frames))
    offset = 6 + 16 * len(frames)
    for im, blob in zip(frames, blobs):
        w, h = im.size
        out += struct.pack('<BBBBHHII', w % 256, h % 256, 0, 0, 1, 32, len(blob), offset)
        offset += len(blob)
    path.write_bytes(out + b''.join(blobs))


def svg_mark():
    path = f'M{START[0]} {START[1]}' + ''.join('C' + ' '.join(f'{x} {y}' for x, y in c) for c in CURVES) + 'Z'
    svg = ('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" role="img" aria-label="Ventet, mascota de Gregal">\n'
           f'<defs><linearGradient id="g" gradientUnits="userSpaceOnUse" x1="{START[0]}" y1="{START[1]}" '
           f'x2="{CURVES[1][2][0]}" y2="{CURVES[1][2][1]}">'
           f'<stop offset="0" stop-color="{BODY[0]}"/><stop offset=".55" stop-color="{BODY[1]}"/>'
           f'<stop offset="1" stop-color="{BODY[2]}"/></linearGradient></defs>\n'
           f'<path fill="url(#g)" stroke="url(#g)" stroke-width="{2*ROUND}" stroke-linejoin="round" d="{path}"/>\n')
    for x, y, rx, ry in EYES:
        svg += f'<ellipse cx="{x}" cy="{y}" rx="{rx}" ry="{ry}" transform="rotate({EYE_TILT} {x} {y})" fill="{EYE}"/>\n'
    return svg + '</svg>\n'


def generate():
    (REPO / 'internal/web/app/ventet.svg').write_text(svg_mark(), encoding='utf-8')
    # La marca sola, dins la UI (rail, hero): sense rajola, sobre el fons de l'app.
    mark(256, (8, 8, 240))[0].save(REPO / 'internal/web/app/icon.png', optimize=True)

    tiles = {px: tile_icon(px) for px in (16, 20, 24, 32, 40, 48, 64, 96, 128, 256, 512)}
    for px in (512, 256, 128):
        tiles[px].save(ROOT / f'build/icon{px}.png', optimize=True)
    write_ico(ROOT / 'build/icon.ico', [tiles[px] for px in (16, 20, 24, 32, 40, 48, 64, 96, 128, 256)])
    # Web: favicon i manifest amb la rajola (la pestanya del navegador pot ser clara).
    tiles[512].save(REPO / 'internal/web/app/app-icon.png', optimize=True)
    tiles[32].save(REPO / 'internal/web/app/favicon-32.png', optimize=True)
    tiles[128].save(REPO / 'internal/web/favicon.png', optimize=True)
    tiles[64].save(REPO / 'internal/web/app/addin/icon.png', optimize=True)
    tiles[32].save(REPO / 'internal/web/app/addin/icon-32.png', optimize=True)
    for px in (16, 32, 48, 128):
        tiles[px].save(REPO / f'extension/chrome/icons/icon{px}.png', optimize=True)


if __name__ == '__main__':
    generate()
