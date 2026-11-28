"""The welcome scene of the app's server setup: the felt winds hop in, gather
round a little felt server, its light turns green, they cheer, and the Gregal
mark rises behind them.

    blender -b -P tools/characters/intro.py -- --out output/intro [--size 420] [--samples 32] [--frames 1-120]

Renders transparent PNG frames with a shadow catcher, so the characters stand
on the app's own background; tools/characters/intro_pack.py turns them into the
animated WebP the Android app plays (res/raw/intro_folk.webp).
"""
import math
import os
import sys

import bpy
from mathutils import Vector

os.environ["FELT_LIBRARY"] = "1"
HERE = os.path.dirname(os.path.abspath(__file__))
felt = {}
exec(compile(open(os.path.join(HERE, "felt.py"), encoding="utf-8").read(), os.path.join(HERE, "felt.py"), "exec"), felt)
ARGS = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []


def arg(name, default):
    return ARGS[ARGS.index(name) + 1] if name in ARGS else default


OUT = arg("--out", "output/intro")
SIZE = int(arg("--size", "420"))
SAMPLES = int(arg("--samples", "32"))
FPS, N = 24, 120
FIRST, LAST = (int(x) for x in arg("--frames", f"1-{N}").split("-"))

# The Gregal mark (Brand.kt MARK_D, a 1024 box) and where its two eyes sit.
MARK_D = ("M810,100C950,360 900,640 640,795C470,900 240,915 70,880C300,870 490,795 565,705"
          "C460,765 340,800 225,810C490,665 535,575 225,510C510,425 770,340 810,100Z")
MARK_EYES = ((675, 530, 29, 53), (792, 518, 27, 51))


def grouped(name, build):
    """Build a character and hang every new root object from one empty."""
    before = set(bpy.context.scene.objects)
    build()
    pivot = felt["link"](bpy.data.objects.new(name, None))
    for ob in set(bpy.context.scene.objects) - before - {pivot}:
        if ob.parent is None:
            ob.parent = pivot
    return pivot


def emission(name, rgba, strength=4.0):
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.clear()
    out = nt.nodes.new("ShaderNodeOutputMaterial")
    em = nt.nodes.new("ShaderNodeEmission")
    em.inputs[0].default_value = rgba
    em.inputs[1].default_value = strength
    nt.links.new(em.outputs[0], out.inputs["Surface"])
    return m, em


def server():
    """A small felt server: a rounded box, two slots and a light that turns green."""
    box = felt["rounded_box"]("server", (0, 0, 0.36), (0.42, 0.32, 0.36), felt["felt"]("server", "#3a4150"), bevel=0.12)
    felt["fuzz"](box, "#3a4150", count=9000, length=0.025)
    for z in (0.5, 0.32):
        felt["rounded_box"]("slot", (0.02, -0.33, z), (0.26, 0.01, 0.035), felt["felt"]("slot", "#1c2029"), bevel=0.02)
    mat, em = emission("led", (1.0, 0.12, 0.08, 1))
    led = felt["ball"]("led", (0.31, -0.33, 0.62), (0.05, 0.02, 0.05), mat, seg=24)
    return box, em


def mark():
    """The Gregal mark as a soft felt cut-out, with its two dark eyes."""
    import tempfile
    svg = os.path.join(tempfile.gettempdir(), "gregal-mark.svg")
    with open(svg, "w", encoding="utf-8") as f:
        f.write('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024"><path d="' + MARK_D + '"/></svg>')
    before = set(bpy.context.scene.objects)
    bpy.ops.import_curve.svg(filepath=svg)
    curves = [o for o in bpy.context.scene.objects if o not in before]
    cu = curves[0]
    # Scale the 1024 box to 2.2 m wide, stand it up, give it depth and round edges.
    xs = [p.co.x for s in cu.data.splines for p in s.bezier_points]
    ys = [p.co.y for s in cu.data.splines for p in s.bezier_points]
    w = max(xs) - min(xs)
    k = 2.2 / w
    depth = 0.09 / k
    cu.data.extrude = depth
    cu.data.bevel_depth = 0.03 / k
    cu.data.bevel_resolution = 6
    for o in bpy.context.selected_objects:
        o.select_set(False)
    bpy.context.view_layer.objects.active = cu
    cu.select_set(True)
    bpy.ops.object.convert(target="MESH")
    logo = bpy.context.active_object
    logo.data.materials.clear()
    logo.data.materials.append(felt["felt"]("mark", "#6f83ee"))
    felt["smooth"](felt["outward"](logo), 0)
    felt["fuzz"](logo, "#6f83ee", count=16000, length=0.03)
    # The path's box (x 70..950, y 100..915) maps onto the object's box.
    minx, maxx, miny, maxy = min(xs), max(xs), min(ys), max(ys)

    def at(sx, sy):
        return (minx + (sx - 70) / 880 * (maxx - minx), maxy - (sy - 100) / 815 * (maxy - miny))

    ink = felt["felt"]("mark-eye", "#141a33", sheen=0.5)
    eyes = []
    for sx, sy, rx, ry in MARK_EYES:
        x, y = at(sx, sy)
        unit = w / 880  # one path unit in the imported curve's units
        e = felt["ball"]("mark-eye", (x, y, depth + 0.045 / k), (rx * unit * 1.5, ry * unit * 1.5, 0.05 / k), ink, seg=24)
        e.rotation_euler = (0, 0, math.radians(-18))
        eyes.append(e)
    pivot = felt["link"](bpy.data.objects.new("mark", None))
    cx, cy = (minx + maxx) / 2, (miny + maxy) / 2
    for o in [logo] + eyes:
        o.location.x -= cx
        o.location.y -= cy
        o.parent = pivot
    pivot.scale = (k, k, k)
    pivot.rotation_euler = (math.radians(90), 0, 0)  # stand the cut-out up, facing the camera
    return pivot, eyes


def ease(x):
    x = max(0.0, min(1.0, x))
    return x * x * (3 - 2 * x)


def back(x):
    x = max(0.0, min(1.0, x))
    c = 1.5
    return 1 + (c + 1) * (x - 1) ** 3 + c * (x - 1) ** 2


def key(ob, f, loc=None, rot=None, scale=None):
    if loc is not None:
        ob.location = loc
        ob.keyframe_insert("location", frame=f)
    if rot is not None:
        ob.rotation_euler = rot
        ob.keyframe_insert("rotation_euler", frame=f)
    if scale is not None:
        ob.scale = scale
        ob.keyframe_insert("scale", frame=f)


def build_scene():
    sc = felt["reset"](1.35, 5.4)
    sc.cycles.samples = SAMPLES
    sc.render.resolution_x = sc.render.resolution_y = SIZE
    sc.render.fps = FPS
    sc.frame_start, sc.frame_end = 1, N
    sc.render.use_persistent_data = True
    # A floor that only catches shadows: the characters stand on the app's background.
    bpy.ops.mesh.primitive_plane_add(size=20, location=(0, 0, 0))
    floor = bpy.context.active_object
    floor.is_shadow_catcher = True

    mina = grouped("minairo", felt["minairo"])
    dona = grouped("dona", felt["dona"])
    tio = grouped("tio", felt["tio"])
    box, led = server()
    logo, mark_eyes = mark()
    for p, s in ((mina, .62), (dona, .62), (tio, .56)):
        p.scale = (s, s, s)
    box_pivot = grouped("srv", lambda: None)  # placeholder so the server can be moved as one
    for o in bpy.context.scene.objects:
        if o.name.split(".")[0] in ("server", "slot", "led") and o.parent is None:
            o.parent = box_pivot
    box_pivot.location = (0, -0.55, 0)
    box_pivot.scale = (0.55, 0.55, 0.55)

    # Where each one stands around the server, and where it comes from.
    spots = {mina: ((-3.4, -0.2), (-1.05, -0.15), 0.35), dona: ((-0.4, 3.2), (0.0, 0.75), 0.0), tio: ((3.6, 0.1), (1.15, -0.05), -0.45)}
    for f in range(1, N + 1):
        t = f / FPS
        for i, (p, (start, end, turn)) in enumerate(spots.items()):
            # In: 1.4 s of hops, each one a little after the other.
            u = ease((t - 0.1 * i) / 1.4)
            x = start[0] + (end[0] - start[0]) * u
            y = start[1] + (end[1] - start[1]) * u
            hop = 0.22 * abs(math.sin(math.pi * 3 * u)) if 0 < u < 1 else 0.0
            lean = 0.0
            squash = 1.0
            # Working round the server: a nod towards it.
            if 1.6 < t < 2.7:
                lean = 0.12 * math.sin((t - 1.6 - 0.15 * i) * 2 * math.pi * 1.6)
            # Cheer when the light turns green: two hops together, squashing on landing.
            if 2.75 < t < 3.55:
                c = (t - 2.75) / 0.8
                hop = 0.34 * abs(math.sin(math.pi * 2 * c))
                squash = 1 - 0.08 * (1 - abs(math.sin(math.pi * 2 * c)))
            # Then they turn to look up at the mark rising behind them.
            look = ease((t - 3.4) / 0.5) * 0.35
            base = {mina: .62, dona: .62, tio: .56}[p]
            # Turned towards the server while walking in; then facing the camera a little more.
            face = turn * (1 - 0.6 * ease((t - 1.5) / 0.4))
            key(p, f, loc=(x, y, hop), rot=(lean - look * 0.6, 0, face),
                scale=(base * (2 - squash) ** 0.5, base * (2 - squash) ** 0.5, base * squash))
        # The mark rises behind them after the cheer.
        r = back((t - 3.2) / 1.1)
        key(logo, f, loc=(0, 3.0, -1.6 + 3.55 * r))
    # The light: red, then a blink, then green.
    for f, rgba in ((1, (1, .12, .08, 1)), (58, (1, .12, .08, 1)), (61, (1, .9, .3, 1)), (64, (.2, 1, .45, 1)), (N, (.2, 1, .45, 1))):
        led.inputs[0].default_value = rgba
        led.inputs[0].keyframe_insert("default_value", frame=f)
    # Blinks: every character's eyes and the mark's, at their own moments.
    for o in bpy.context.scene.objects:
        if o.name.split(".")[0] in ("eye", "mark-eye"):
            base = tuple(o.scale)
            when = (46, 100) if o.name.startswith("eye") else (108,)
            for b in when:
                key(o, b - 2, scale=base)
                key(o, b, scale=(base[0], base[1], base[2] * 0.12) if o.name.startswith("eye") else (base[0], base[1] * 0.12, base[2]))
                key(o, b + 2, scale=base)
    return sc


sc = build_scene()
os.makedirs(OUT, exist_ok=True)
sc.render.image_settings.file_format = "PNG"
sc.render.image_settings.color_mode = "RGBA"
for f in range(FIRST, LAST + 1):
    sc.frame_set(f)
    sc.render.filepath = os.path.abspath(os.path.join(OUT, f"{f:04d}.png"))
    bpy.ops.render.render(write_still=True)
    print("frame", f, flush=True)
