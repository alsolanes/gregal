"""Felt cast prototype: the folklore winds as small plush toys (minairó, dona d'aigua, tió).

    blender -b -P tools/characters/felt.py -- --out output/ui-review/felt [--only minairo] [--samples 96]

What makes a small avatar look like a real object rather than a drawing: one
body in one piece, short fur on it (particle hair, Cycles), soft studio light,
no outlines, and a minimal face of two felt eyes and a blush. Each character
keeps a single strong mark of who it is: the red barretina, the long hair and
golden comb, the pale cut end of the log. The eyes are their own pieces so a
player can animate them on their own later (a look, a blink) without
re-rendering the body.
"""
import math
import os
import sys

import bpy
from mathutils import Vector

ARGS = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []


def arg(name, default):
    return ARGS[ARGS.index(name) + 1] if name in ARGS else default


OUT = arg("--out", "output/ui-review/felt")
ONLY = arg("--only", "")
SAMPLES = int(arg("--samples", "96"))
SIZE = int(arg("--size", "512"))


def lin(hex_):
    h = hex_.lstrip("#")
    c = [int(h[i:i + 2], 16) / 255 for i in (0, 2, 4)]
    return tuple(x / 12.92 if x <= 0.04045 else ((x + 0.055) / 1.055) ** 2.4 for x in c) + (1.0,)


def reset(frame_z, width):
    bpy.ops.wm.read_factory_settings(use_empty=True)
    sc = bpy.context.scene
    sc.render.engine = "CYCLES"
    sc.cycles.samples = SAMPLES
    sc.cycles.use_denoising = True
    sc.render.resolution_x = sc.render.resolution_y = SIZE
    sc.render.film_transparent = True
    sc.render.image_settings.file_format = "PNG"
    sc.render.image_settings.color_mode = "RGBA"
    sc.view_settings.view_transform = "AgX"
    sc.view_settings.look = "AgX - Medium High Contrast"
    world = bpy.data.worlds.new("w")
    world.use_nodes = True
    bg = world.node_tree.nodes["Background"]
    bg.inputs[0].default_value = (0.9, 0.92, 0.96, 1)
    bg.inputs[1].default_value = 0.35
    sc.world = world
    cam = bpy.data.cameras.new("cam")
    cam.lens = 85
    co = bpy.data.objects.new("cam", cam)
    sc.collection.objects.link(co)
    dist = width * 85 / 36 * 1.0
    co.location = (dist * 0.12, -dist, frame_z + dist * 0.06)
    look = Vector((0, 0, frame_z)) - co.location
    co.rotation_euler = look.to_track_quat("-Z", "Y").to_euler()
    sc.camera = co
    # Studio: a big soft key from the upper left, a dim fill, a rim from behind.
    for name, energy, size, loc in (("key", 900, 4.0, (-3.5, -4.0, 5.0)), ("fill", 220, 5.0, (4.5, -3.0, 1.5)),
                                    ("rim", 500, 2.5, (1.5, 4.0, 4.0))):
        lt = bpy.data.lights.new(name, "AREA")
        lt.energy = energy
        lt.size = size
        lo = bpy.data.objects.new(name, lt)
        sc.collection.objects.link(lo)
        lo.location = loc
        lo.rotation_euler = (Vector((0, 0, 1.0)) - Vector(loc)).to_track_quat("-Z", "Y").to_euler()
    return sc


def felt(name, hex_, sheen=1.0, rough=0.85, bump=0.35):
    """Felt: a rough diffuse surface with sheen and a fine fibrous bump."""
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    nt = m.node_tree
    p = nt.nodes["Principled BSDF"]
    p.inputs["Base Color"].default_value = lin(hex_)
    p.inputs["Roughness"].default_value = rough
    p.inputs["Sheen Weight"].default_value = sheen
    p.inputs["Sheen Roughness"].default_value = 0.4
    p.inputs["Sheen Tint"].default_value = (1, 1, 1, 1)
    noise = nt.nodes.new("ShaderNodeTexNoise")
    noise.inputs["Scale"].default_value = 180.0
    noise.inputs["Detail"].default_value = 12.0
    noise.inputs["Roughness"].default_value = 0.7
    b = nt.nodes.new("ShaderNodeBump")
    b.inputs["Strength"].default_value = bump
    b.inputs["Distance"].default_value = 0.004
    nt.links.new(noise.outputs["Fac"], b.inputs["Height"])
    nt.links.new(b.outputs["Normal"], p.inputs["Normal"])
    return m


def hair_mat(name, hex_):
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.clear()
    out = nt.nodes.new("ShaderNodeOutputMaterial")
    h = nt.nodes.new("ShaderNodeBsdfHairPrincipled")
    h.parametrization = "COLOR"
    h.inputs["Color"].default_value = lin(hex_)
    h.inputs["Roughness"].default_value = 0.55
    h.inputs["Radial Roughness"].default_value = 0.8
    nt.links.new(h.outputs[0], out.inputs["Surface"])
    return m


def fuzz(ob, hex_, count=18000, length=0.035):
    """Short fur all over: the plush look. The hair takes the surface colour."""
    ob.data.materials.append(hair_mat(ob.name + "-fur", hex_))
    mod = ob.modifiers.new("fur", "PARTICLE_SYSTEM")
    ps = mod.particle_system.settings
    ps.type = "HAIR"
    ps.count = count
    ps.hair_length = length
    ps.use_advanced_hair = True
    ps.material = len(ob.data.materials)
    ps.root_radius = 0.0035
    ps.tip_radius = 0.0005
    ps.radius_scale = 1.0
    ps.child_type = "INTERPOLATED"
    ps.rendered_child_count = 6
    ps.child_length = 0.9
    # Hair-mode randomness is in scene units: keep it at the scale of the fur.
    ps.roughness_1 = length * 0.15
    ps.roughness_2 = length * 0.3
    ps.roughness_endpoint = length * 0.2
    ps.factor_random = length * 0.08
    ps.child_radius = 0.04
    # In hair mode normal_factor is the length: set it last, or it overrides hair_length.
    ps.hair_length = length
    return ob


def link(ob):
    bpy.context.scene.collection.objects.link(ob)
    return ob


def outward(ob):
    """Point every normal outwards: fur grows along them (mirrored curves and edited meshes flip some)."""
    bpy.context.view_layer.objects.active = ob
    for o in bpy.context.selected_objects:
        o.select_set(False)
    ob.select_set(True)
    bpy.ops.object.mode_set(mode="EDIT")
    bpy.ops.mesh.select_all(action="SELECT")
    bpy.ops.mesh.normals_make_consistent(inside=False)
    bpy.ops.object.mode_set(mode="OBJECT")
    return ob


def smooth(ob, levels=2):
    for p in ob.data.polygons:
        p.use_smooth = True
    if levels:
        sub = ob.modifiers.new("sub", "SUBSURF")
        sub.levels = sub.render_levels = levels
    return ob


def ball(name, loc, scale, mat, seg=48):
    bpy.ops.mesh.primitive_uv_sphere_add(segments=seg, ring_count=seg // 2, location=loc)
    ob = bpy.context.active_object
    ob.name = name
    ob.scale = scale
    bpy.ops.object.transform_apply(scale=True)
    ob.data.materials.append(mat)
    return smooth(ob, 0)


def rounded_box(name, loc, size, mat, bevel=0.35):
    """A squircle-ish block: a cube with a wide rounded bevel and subdivision."""
    bpy.ops.mesh.primitive_cube_add(location=loc)
    ob = bpy.context.active_object
    ob.name = name
    ob.scale = size
    bpy.ops.object.transform_apply(scale=True)
    bv = ob.modifiers.new("bevel", "BEVEL")
    bv.width = bevel
    bv.segments = 8
    ob.data.materials.append(mat)
    return smooth(ob, 2)


def tube(name, pts, radii, mat):
    cu = bpy.data.curves.new(name, "CURVE")
    cu.dimensions = "3D"
    cu.bevel_depth = 1.0
    cu.bevel_resolution = 10
    cu.resolution_u = 20
    cu.use_fill_caps = True
    sp = cu.splines.new("NURBS")
    sp.points.add(len(pts) - 1)
    for p, co, r in zip(sp.points, pts, radii):
        p.co = (co[0], co[1], co[2], 1.0)
        p.radius = r
    sp.use_endpoint_u = True
    sp.order_u = 4
    ob = link(bpy.data.objects.new(name, cu))
    # convert() works on every selected object and would bake (and drop) the fur of the others.
    for o in bpy.context.selected_objects:
        o.select_set(False)
    bpy.context.view_layer.objects.active = ob
    ob.select_set(True)
    bpy.ops.object.convert(target="MESH")
    ob = bpy.context.active_object
    ob.data.materials.append(mat)
    return smooth(ob, 0)


def face(center, normal_y=-1, spread=0.2, eye=(0.085, 0.12), blush=True, lift=0.0):
    """Two black felt eyes (flat ovals sitting on the surface) and a soft pink blush."""
    ink = felt("eye", "#151317", sheen=0.6, rough=0.7, bump=0.2)
    pink = felt("blush", "#f39aa8", sheen=0.8)
    x, y, z = center
    for side in (-1, 1):
        ball("eye", (x + side * spread, y, z + lift), (eye[0], 0.035, eye[1]), ink, seg=32)
        if blush:
            ball("blush", (x + side * spread * 1.55, y + 0.012, z - eye[1] * 1.25), (0.07, 0.02, 0.045), pink, seg=24)


def barretina(base, r, lean=1, color="#c8212b"):
    red = felt("barretina", color)
    x, y, z = base
    pts, radii = [], []
    for k in range(7):
        t = k / 6
        side = 0.62 * t ** 1.5 * lean
        # Rises, then flops over to one side and hangs a little.
        pts.append((x + side, y + 0.05 * t, z + 0.62 * math.sin(t * math.pi * 0.8)))
        radii.append(r * (1.0 - 0.72 * t))
    cap = tube("barretina", pts, radii, red)
    fuzz(cap, color, count=5000, length=0.025)
    bpy.ops.mesh.primitive_torus_add(major_radius=r * 0.98, minor_radius=r * 0.16, location=(x, y, z + 0.02))
    b = bpy.context.active_object
    b.data.materials.append(felt("band", "#2b2430"))
    smooth(b, 1)
    fuzz(b, "#2b2430", count=3000, length=0.02)


def minairo():
    body = rounded_box("body", (0, 0, 0.64), (0.62, 0.54, 0.6), felt("body", "#f2e6d8"), bevel=0.5)
    fuzz(body, "#f2e6d8")
    face((0, -0.55, 0.7), spread=0.19)
    barretina((0, 0.0, 1.2), 0.47, lean=1)
    return 1.0, 2.55


def dona():
    """A drop of water with long wool hair down both sides and her golden comb."""
    bpy.ops.mesh.primitive_uv_sphere_add(segments=64, ring_count=32, location=(0, 0, 0.72))
    body = bpy.context.active_object
    body.name = "body"
    body.scale = (0.68, 0.62, 0.68)
    bpy.ops.object.transform_apply(scale=True)
    for v in body.data.vertices:  # draw the top up into a soft point
        if v.co.z > 0:
            t = v.co.z / 0.68
            v.co.x *= 1 - 0.25 * t ** 1.6
            v.co.y *= 1 - 0.25 * t ** 1.6
            v.co.z *= 1.0
    body.data.update()
    body.data.materials.append(felt("water", "#8fd3e3"))
    smooth(outward(body), 1)
    fuzz(body, "#8fd3e3")
    hair = felt("hair", "#2a5c9e")
    hood = ball("hood", (0, 0.12, 1.12), (0.62, 0.55, 0.5), hair, seg=64)
    smooth(hood, 1)
    fuzz(hood, "#2a5c9e", count=12000, length=0.035)
    for side in (-1, 1):
        pts = [(side * 0.3, -0.1, 1.3), (side * 0.6, -0.05, 1.0), (side * 0.66, 0.0, 0.62), (side * 0.62, 0.02, 0.28),
               (side * 0.72, 0.0, 0.08)]
        lock = tube("lock", pts, (0.14, 0.17, 0.18, 0.16, 0.11), hair)
        fuzz(lock, "#2a5c9e", count=6000, length=0.03)
    gold = bpy.data.materials.new("gold")
    gold.use_nodes = True
    gp = gold.node_tree.nodes["Principled BSDF"]
    gp.inputs["Base Color"].default_value = lin("#f0c050")
    gp.inputs["Metallic"].default_value = 1.0
    gp.inputs["Roughness"].default_value = 0.3
    comb = rounded_box("comb", (0.3, -0.32, 1.3), (0.15, 0.03, 0.055), gold, bevel=0.025)
    comb.rotation_euler = (0, math.radians(-28), 0)
    face((0, -0.62, 0.72), spread=0.19)
    return 0.85, 2.4


def tio():
    """A felt log seen from its pale cut end, with a barretina and a cream blanket over its back."""
    pivot = link(bpy.data.objects.new("tio", None))
    before = set(bpy.context.scene.objects)
    R, L, cz = 0.6, 1.5, 0.62
    bpy.ops.mesh.primitive_cylinder_add(vertices=64, radius=R, depth=L, location=(0, 0, cz),
                                        rotation=(math.radians(90), 0, 0))
    log = bpy.context.active_object
    bv = log.modifiers.new("bevel", "BEVEL")
    bv.width = 0.12
    bv.segments = 8
    log.data.materials.append(felt("bark", "#8a5a36"))
    smooth(log, 1)
    fuzz(log, "#8a5a36", count=14000)
    bpy.ops.mesh.primitive_cylinder_add(vertices=64, radius=R * 0.84, depth=0.04, location=(0, -L / 2 - 0.005, cz),
                                        rotation=(math.radians(90), 0, 0))
    cut = bpy.context.active_object
    cut.data.materials.append(felt("wood", "#efd2a2", bump=0.25))
    smooth(cut, 0)
    blanket = rounded_box("blanket", (0, 0.25, cz + R * 0.78), (0.66, 0.5, 0.12), felt("blanket", "#ede5d6"), bevel=0.1)
    fuzz(blanket, "#ede5d6", count=6000)
    face((0, -L / 2 - 0.03, cz + 0.04), spread=0.2)
    barretina((0, -0.25, cz + R * 0.95), 0.32, lean=-1)
    for ob in set(bpy.context.scene.objects) - before:
        if ob.parent is None:
            ob.parent = pivot
    pivot.rotation_euler = (0, 0, math.radians(18))
    return 0.85, 2.2


CAST = {"minairo": minairo, "dona": dona, "tio": tio}
# The part of each character that takes the wind's colour: the minairó's body, the
# dona d'aigua's hair, the tió's blanket. The rest keeps its own colours (the red
# barretina, the water, the log), which is what says who the character is.
TINTS = {"minairo": ("body",), "dona": ("hood", "lock"), "tio": ("blanket",)}
NEUTRAL = "#ededed"
# --assets DIR: the layers the players draw (web winds.js, Android WindFace.kt):
#   <name>.webp       the character without its eyes, the tinted part in neutral felt;
#   <name>.mask.webp  white where the wind's colour goes (multiply it over the body);
#   <name>.eyes.webp  the eyes alone, which blink and glance on their own.
ASSETS = arg("--assets", "")


def parts(prefixes):
    return [ob for ob in bpy.context.scene.objects if ob.type == "MESH" and ob.name.split(".")[0] in prefixes]


def neutral(obs):
    for ob in obs:
        for m in ob.data.materials:
            for n in m.node_tree.nodes:
                if n.type == "BSDF_PRINCIPLED":
                    n.inputs["Base Color"].default_value = lin(NEUTRAL)
                elif n.type == "BSDF_HAIR_PRINCIPLED":
                    n.inputs["Color"].default_value = lin(NEUTRAL)


def white(obs):
    m = bpy.data.materials.new("mask")
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.clear()
    out = nt.nodes.new("ShaderNodeOutputMaterial")
    em = nt.nodes.new("ShaderNodeEmission")
    em.inputs[0].default_value = (1, 1, 1, 1)
    nt.links.new(em.outputs[0], out.inputs["Surface"])
    for ob in obs:
        for i in range(len(ob.data.materials)):
            ob.data.materials[i] = m


def shoot(sc, path):
    sc.render.filepath = os.path.abspath(path)
    bpy.ops.render.render(write_still=True)
    print("wrote", sc.render.filepath)


for name, build in CAST.items():
    if ONLY and name not in ONLY.split(","):
        continue
    bpy.ops.wm.read_factory_settings(use_empty=True)
    frame = build()
    # Rebuild the scene around the character: reset() clears it, so build again after.
    sc = reset(*frame)
    build()
    if not ASSETS:
        os.makedirs(OUT, exist_ok=True)
        shoot(sc, os.path.join(OUT, name + ".png"))
        continue
    os.makedirs(ASSETS, exist_ok=True)
    sc.render.image_settings.file_format = "WEBP"
    sc.render.image_settings.quality = 88
    eyes, tint = parts(("eye",)), parts(TINTS[name])
    rest = [ob for ob in sc.objects if ob.type == "MESH" and ob not in eyes]
    neutral(tint)
    for ob in eyes:
        ob.hide_render = True
    shoot(sc, os.path.join(ASSETS, name + ".webp"))
    # The eyes alone: everything else still hides what is behind it, but draws nothing.
    for ob in eyes:
        ob.hide_render = False
    for ob in rest:
        ob.is_holdout = True
    shoot(sc, os.path.join(ASSETS, name + ".eyes.webp"))
    # The mask: the tinted part in flat white, its fur too; the eyes cut it out.
    for ob in tint:
        ob.is_holdout = False
    white(tint)
    for ob in eyes:
        ob.is_holdout = True
    sc.world.node_tree.nodes["Background"].inputs[1].default_value = 0.0
    sc.view_settings.view_transform = "Standard"
    sc.view_settings.look = "None"
    shoot(sc, os.path.join(ASSETS, name + ".mask.webp"))
