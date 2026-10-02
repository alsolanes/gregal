"""Genera Ventet des de la mateixa geometria vectorial per al web i l'app."""
from pathlib import Path
from PIL import Image, ImageDraw
import math

ROOT = Path(__file__).resolve().parent
START = (810, 100)
CURVES = [
    ((950, 360), (900, 640), (640, 795)),
    ((470, 900), (240, 915), (70, 880)),
    ((300, 870), (490, 795), (565, 705)),
    ((460, 765), (340, 800), (225, 810)),
    ((490, 665), (535, 575), (225, 510)),
    ((510, 425), (770, 340), (810, 100)),
]

def generate():
    path = f'M{START[0]} {START[1]}' + ''.join('C' + ' '.join(f'{x} {y}' for x, y in curve) for curve in CURVES) + 'Z'
    eyes = [(675, 530, 29, 53), (792, 518, 27, 51)]
    svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" role="img" aria-label="Ventet, mascota de Gregal">\n'
    svg += f'<path fill="#7FD4C1" d="{path}"/>\n'
    for x, y, rx, ry in eyes:
        svg += f'<ellipse cx="{x}" cy="{y}" rx="{rx}" ry="{ry}" transform="rotate(18 {x} {y})" fill="#0B1220"/>\n'
    (ROOT.parent / 'internal/web/app/ventet.svg').write_text(svg + '</svg>\n', encoding='utf-8')
    image = Image.new('RGBA', (1024, 1024))
    draw = ImageDraw.Draw(image)
    points = [START]
    p0 = START
    for p1, p2, p3 in CURVES:
        for i in range(1, 81):
            t = i / 80
            u = 1 - t
            points.append(tuple(u**3*p0[k] + 3*u*u*t*p1[k] + 3*u*t*t*p2[k] + t**3*p3[k] for k in (0, 1)))
        p0 = p3
    draw.polygon(points, fill='#7FD4C1')
    angle = math.radians(18)
    for x, y, rx, ry in eyes:
        polygon = []
        for i in range(80):
            t = i * math.tau / 80
            dx, dy = rx * math.cos(t), ry * math.sin(t)
            polygon.append((x + dx*math.cos(angle)-dy*math.sin(angle), y + dx*math.sin(angle)+dy*math.cos(angle)))
        draw.polygon(polygon, fill='#0B1220')
    for pixels in (512, 256, 128):
        image.resize((pixels, pixels), Image.Resampling.LANCZOS).save(ROOT / f'build/icon{pixels}.png')
    for target in ('internal/web/app/icon.png', 'internal/web/favicon.png'):
        image.resize((128, 128), Image.Resampling.LANCZOS).save(ROOT.parent / target)
    image.save(ROOT / 'build/icon.ico', sizes=[(16,16), (32,32), (48,48), (64,64), (128,128), (256,256)])

if __name__ == '__main__':
    generate()
