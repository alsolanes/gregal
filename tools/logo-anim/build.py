"""Logo de Gregal animat (bucle de 3 s) — escena headless per a Blender.

Geometria: la ratxa de vent de internal/web/app/ventet.svg (viewBox 1024),
extrudida i amb els ulls. Animació: el logo sura i gira suaument, la llum
recorre la superficie (la "ratxa" que passa) i els ulls parpellegen un cop
per cicle. Tot quadra als extrems perque el bucle sigui perfecte.

Us:
  blender -b -P build_logo_anim.py -- --out output/logo-anim/frames \
      --start 1 --end 72 --res 1024 [--blend logo.blend]
"""
import math
import os
import re
import sys

import bmesh
import bpy
from mathutils import Matrix, Vector, geometry

FPS = 24
FRAMES = 72  # 3 s de bucle
VIEW = 1024.0
CENTER = VIEW / 2.0

# --- paleta de la marca (docs/identitat.md) -------------------------------
ESCUMA_A = "#A3B0FF"   # escumaClara
ESCUMA_B = "#8193FF"   # mig del degradat del logo
ESCUMA_C = "#6F7FF0"
ULL = "#13151B"

# --- el path de ventet.svg ------------------------------------------------
PATH_D = ("M800 118C955 370 925 655 655 812C490 910 265 925 95 865"
          "C270 798 410 748 500 682C450 612 370 556 232 520"
          "C520 430 765 345 800 118Z")
EYES = [  # cx, cy, rx, ry, angle (graus, SVG)
    (668.0, 548.0, 38.0, 64.0, 18.0),
    (800.0, 530.0, 36.0, 61.0, 18.0),
]


def hex_rgb(h):
    h = h.lstrip("#")
    return tuple(int(h[i:i + 2], 16) / 255.0 for i in (0, 2, 4))


def svg_to_world(x, y):
    """SVG (y avall) -> pla XZ de Blender (z amunt), centrat."""
    return Vector((x - CENTER, CENTER - y))


def parse_beziers(d):
    """Path absolut amb M i C -> llista de segments (p0, c1, c2, p3)."""
    tokens = re.findall(r"[MmCcZz]|-?\d*\.?\d+", d)
    segs, cur, i = [], Vector((0.0, 0.0)), 0
    while i < len(tokens):
        cmd = tokens[i]
        i += 1
        if cmd == "M":
            cur = Vector((float(tokens[i]), float(tokens[i + 1])))
            i += 2
        elif cmd == "C":
            c1 = Vector((float(tokens[i]), float(tokens[i + 1])))
            c2 = Vector((float(tokens[i + 2]), float(tokens[i + 3])))
            end = Vector((float(tokens[i + 4]), float(tokens[i + 5])))
            i += 6
            segs.append((cur, c1, c2, end))
            cur = end
        elif cmd in ("Z", "z"):
            continue
        else:
            raise ValueError("comanda no suportada: %r" % cmd)
    return segs


def clean_scene():
    bpy.ops.wm.read_factory_settings(use_empty=True)


def hex_lin(h):
    """Hex sRGB -> RGBA lineal, que és el que esperen els nodes."""
    def lin(c):
        return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4
    r, g, b = hex_rgb(h)
    return (lin(r), lin(g), lin(b), 1.0)


def brand_material():
    """Degradat de la marca: clar a dalt a la dreta, fons a baix a l'esquerra."""
    mat = bpy.data.materials.new("marca")
    mat.use_nodes = True
    nt = mat.node_tree
    nt.nodes.clear()
    out = nt.nodes.new("ShaderNodeOutputMaterial")
    out.location = (700, 0)
    bsdf = nt.nodes.new("ShaderNodeBsdfPrincipled")
    bsdf.location = (400, 0)

    coord = nt.nodes.new("ShaderNodeTexCoord")
    coord.location = (-600, 0)
    sep = nt.nodes.new("ShaderNodeSeparateXYZ")
    sep.location = (-420, 0)
    add = nt.nodes.new("ShaderNodeMath")
    add.operation = "ADD"
    add.location = (-240, -60)
    half = nt.nodes.new("ShaderNodeMath")
    half.operation = "MULTIPLY"
    half.inputs[1].default_value = 0.5
    half.location = (-80, -60)
    stretch = nt.nodes.new("ShaderNodeMapRange")
    stretch.inputs["From Min"].default_value = 0.16
    stretch.inputs["From Max"].default_value = 0.84
    stretch.location = (0, -60)
    ramp = nt.nodes.new("ShaderNodeValToRGB")
    ramp.location = (100, 60)
    els = ramp.color_ramp.elements
    els[0].position = 0.0
    els[0].color = hex_lin(ESCUMA_C)
    els[1].position = 1.0
    els[1].color = hex_lin(ESCUMA_A)
    mid = els.new(0.55)
    mid.color = hex_lin(ESCUMA_B)

    nt.links.new(coord.outputs["Generated"], sep.inputs[0])
    nt.links.new(sep.outputs["X"], add.inputs[0])
    nt.links.new(sep.outputs["Y"], add.inputs[1])
    nt.links.new(add.outputs[0], half.inputs[0])
    nt.links.new(half.outputs[0], stretch.inputs["Value"])
    nt.links.new(stretch.outputs["Result"], ramp.inputs["Fac"])
    nt.links.new(ramp.outputs["Color"], bsdf.inputs["Base Color"])
    if "Emission Color" in bsdf.inputs:
        nt.links.new(ramp.outputs["Color"], bsdf.inputs["Emission Color"])
        bsdf.inputs["Emission Strength"].default_value = 1.0
    bsdf.inputs["Roughness"].default_value = 0.45
    if "Specular IOR Level" in bsdf.inputs:
        bsdf.inputs["Specular IOR Level"].default_value = 0.5
    if "Coat Weight" in bsdf.inputs:
        bsdf.inputs["Coat Weight"].default_value = 0.0
        bsdf.inputs["Coat Roughness"].default_value = 0.35
    bsdf.inputs["Metallic"].default_value = 0.0
    nt.links.new(bsdf.outputs["BSDF"], out.inputs["Surface"])
    return mat


def eye_material():
    mat = bpy.data.materials.new("ulls")
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs["Base Color"].default_value = hex_lin(ULL)
    bsdf.inputs["Roughness"].default_value = 0.45
    if "Emission Color" in bsdf.inputs:
        bsdf.inputs["Emission Color"].default_value = hex_lin(ULL)
        bsdf.inputs["Emission Strength"].default_value = 0.05
    return mat


def smooth_path(pts, steps=26):
    """Aplana els segments de Bézier en un polígon dens."""
    out = []
    for p0, c1, c2, p3 in pts:
        for i in range(steps):
            t = i / steps
            u = 1.0 - t
            out.append(p0 * (u ** 3) + c1 * (3 * u * u * t)
                       + c2 * (3 * u * t * t) + p3 * (t ** 3))
    return out


def resample(pts, step=4.0):
    """Reparteix el polígon en punts cada `step` unitats."""
    out = []
    n = len(pts)
    for i in range(n):
        a, b = pts[i], pts[(i + 1) % n]
        seg = b - a
        k = max(1, int(seg.length / step))
        for j in range(k):
            out.append(a + seg * (j / k))
    return out


def chaikin(pts, rounds=2):
    """Arrodoneix les cantonades del contorn tallant-les (Chaikin)."""
    for _ in range(rounds):
        out = []
        n = len(pts)
        for i in range(n):
            a, b = pts[i], pts[(i + 1) % n]
            out.append(a * 0.75 + b * 0.25)
            out.append(a * 0.25 + b * 0.75)
        pts = out
    return pts


def point_in_poly(p, poly):
    inside = False
    n = len(poly)
    for i in range(n):
        a, b = poly[i], poly[(i + 1) % n]
        if (a.y > p.y) != (b.y > p.y):
            x = a.x + (p.y - a.y) * (b.x - a.x) / (b.y - a.y)
            if x > p.x:
                inside = not inside
    return inside


def segments_cross(a1, a2, b1, b2):
    d1 = a2 - a1
    d2 = b2 - b1
    den = d1.x * d2.y - d1.y * d2.x
    if abs(den) < 1e-12:
        return False
    t = ((b1.x - a1.x) * d2.y - (b1.y - a1.y) * d2.x) / den
    u = ((b1.x - a1.x) * d1.y - (b1.y - a1.y) * d1.x) / den
    return 0.0 <= t <= 1.0 and 0.0 <= u <= 1.0


def trim_loops(ring):
    """Retalla els bucles que fa la corba desplaçada on es creua amb ella
    mateixa: és el que fa que el notch quedi tancat com al logo."""
    changed = True
    while changed:
        changed = False
        n = len(ring)
        for i in range(n):
            a1, a2 = ring[i], ring[(i + 1) % n]
            for j in range(i + 2, n):
                if (j + 1) % n == i:
                    continue
                if segments_cross(a1, a2, ring[j], ring[(j + 1) % n]):
                    ring = ring[:i + 1] + ring[j + 1:]
                    changed = True
                    break
            if changed:
                break
    return ring


def cap_gaps(ring, pts, r):
    """Tanca les puntes amb l'arc de radi r: la unió de discos acaba sempre
    en un arc, i el retall hi deixa un salt recte."""
    out, n = [], len(ring)
    for i in range(n):
        a, b = ring[i], ring[(i + 1) % n]
        out.append(a)
        if (b - a).length <= r * 0.6:
            continue
        mid = (a + b) * 0.5
        c = min(pts, key=lambda q: (q - mid).length_squared)
        rad = ((a - c).length + (b - c).length) * 0.5
        a0 = math.atan2(a.y - c.y, a.x - c.x)
        a1 = math.atan2(b.y - c.y, b.x - c.x)
        da = (a1 - a0 + math.pi) % (2.0 * math.pi) - math.pi
        k = max(3, int(abs(da) / 0.22))
        for j in range(1, k):
            ang = a0 + da * j / k
            out.append(c + Vector((math.cos(ang), math.sin(ang))) * rad)
    return out


def dilated_outline(pts, r=30.0):
    """Silueta del path dilatat r: la corba desplaçada cap enfora.

    On el desplaçament es creua amb ell mateix (l'escletxa del notch, les
    puntes molt fines) se'n retalla el bucle: queda la unió de debò, que és
    el que pinta el traç de 60 px de l'SVG.
    """
    n = len(pts)
    area = 0.0
    for i in range(n):
        area += pts[i].x * pts[(i + 1) % n].y - pts[(i + 1) % n].x * pts[i].y
    ccw = area > 0.0

    normals = []
    for i in range(n):
        t = pts[(i + 1) % n] - pts[(i - 1) % n]
        t = t.normalized() if t.length > 1e-9 else Vector((1.0, 0.0))
        normals.append(Vector((-t.y, t.x)))
    probe = 1.5
    out = sum(1 for i in range(0, n, max(1, n // 12))
              if not point_in_poly(pts[i] + normals[i] * probe, pts))
    if out < 6:  # les normals apunten cap a dins: les girem
        normals = [-v for v in normals]
    _ = ccw

    ring = [pts[i] + normals[i] * r for i in range(n)]
    return chaikin(cap_gaps(trim_loops(ring), pts, r), 2)


def solid_mesh(ring, half=22.0):
    """Extrudeix un polígon pla (z=±half) amb vores tancades."""
    n = len(ring)
    tris = geometry.tessellate_polygon([ring])
    verts = [(p.x, p.y, half) for p in ring] +             [(p.x, p.y, -half) for p in ring]
    faces = []
    for a, b, c in tris:
        faces.append((a, b, c))
        faces.append((a + n, c + n, b + n))
    for i in range(n):
        j = (i + 1) % n
        faces.append((i, j, j + n, i + n))
    mesh = bpy.data.meshes.new("ventet")
    mesh.from_pydata(verts, [], faces)
    mesh.validate()
    bm = bmesh.new()
    bm.from_mesh(mesh)
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
    bm.to_mesh(mesh)
    bm.free()
    return mesh


def logo_mesh(bevel=30.0):
    """Logo: la silueta del path dilatat, extruïda i amb la vora arrodonida."""
    flat = [svg_to_world(p.x, p.y)
            for p in smooth_path(parse_beziers(PATH_D))]
    ring = dilated_outline(resample(flat, 4.0), bevel)
    mesh = solid_mesh(ring)
    obj = bpy.data.objects.new("Gregal", mesh)
    bpy.context.collection.objects.link(obj)
    cantell = obj.modifiers.new("cantell", "BEVEL")
    cantell.width = 5.0
    cantell.segments = 3
    cantell.limit_method = "ANGLE"
    cantell.angle_limit = math.radians(30)
    depsgraph = bpy.context.evaluated_depsgraph_get()
    final = bpy.data.meshes.new_from_object(obj.evaluated_get(depsgraph))
    bpy.data.objects.remove(obj, do_unlink=True)
    final.name = "ventet"
    final.materials.clear()
    final.materials.append(brand_material())
    obj = bpy.data.objects.new("Gregal", final)
    bpy.context.collection.objects.link(obj)
    obj.rotation_euler = (math.pi / 2.0, 0.0, 0.0)  # dempeus: XY -> XZ
    return obj


def eye_mesh(index, cx, cy, rx, ry, angle):
    bm = bmesh.new()
    bmesh.ops.create_circle(bm, cap_ends=True, cap_tris=False, segments=48,
                            radius=1.0)
    for v in bm.verts:
        v.co.x *= rx
        v.co.y *= ry
    me = bpy.data.meshes.new("ull%d" % index)
    bm.to_mesh(me)
    bm.free()
    obj = bpy.data.objects.new("Ull%d" % index, me)
    bpy.context.collection.objects.link(obj)
    obj.data.materials.append(eye_material())
    obj.rotation_euler = (math.pi / 2.0, math.radians(angle), 0.0)
    pos = svg_to_world(cx, cy)
    obj.location = (pos.x, -20.0, pos.y)
    solid = obj.modifiers.new("gruix", "SOLIDIFY")
    solid.thickness = 24.0
    solid.offset = 1.0
    bevel = obj.modifiers.new("cantell", "BEVEL")
    bevel.width = 4.0
    bevel.segments = 3
    return obj


def aim(obj, target=(0.0, 0.0, 0.0)):
    """Orienta una llum cap a un punt (la -Z mira el punt)."""
    direction = Vector(obj.location) - Vector(target)
    obj.rotation_euler = direction.to_track_quat("Z", "Y").to_euler()


def setup_world_and_lights():
    world = bpy.data.worlds.new("mon")
    world.use_nodes = True
    bg = world.node_tree.nodes["Background"]
    bg.inputs[0].default_value = (0.035, 0.045, 0.075, 1.0)
    bg.inputs[1].default_value = 0.35
    bpy.context.scene.world = world

    def area(name, loc, size, energy, color=(1, 1, 1)):
        data = bpy.data.lights.new(name, "AREA")
        data.energy = energy
        data.size = size
        data.color = color
        obj = bpy.data.objects.new(name, data)
        bpy.context.collection.objects.link(obj)
        obj.location = loc
        aim(obj)
        return obj

    area("clau", (-820.0, -1250.0, 980.0), 760.0, 0.55e6)
    area("contra", (1000.0, 620.0, 420.0), 520.0, 0.15e6, (0.72, 0.80, 1.0))
    area("omplert", (-950.0, -900.0, -760.0), 900.0, 0.12e6,
         (1.0, 0.95, 0.92))
    # la ratxa de llum que recorre la superficie un cop per cicle
    sweep = area("ratxa", (-1500.0, -650.0, 900.0), 220.0, 0.0,
                 (1.0, 1.0, 1.0))
    return sweep


def animate(root, eyes, sweep):
    scn = bpy.context.scene
    scn.frame_start = 1
    scn.frame_end = FRAMES
    scn.render.fps = FPS

    for f in range(1, FRAMES + 2):  # el frame 73 == 1, bucle net
        t = (f - 1) / FRAMES
        w = 2.0 * math.pi * t

        root.location = (0.0, 0.0, 26.0 * math.sin(w + math.pi / 2))
        root.rotation_euler = (
            0.05 * math.sin(w * 2.0),
            math.radians(4.0) * math.sin(w),
            0.035 * math.sin(w + 0.9),
        )
        root.scale = (1.0 + 0.018 * math.sin(w * 2.0),) * 3
        root.keyframe_insert("location", frame=f)
        root.keyframe_insert("rotation_euler", frame=f)
        root.keyframe_insert("scale", frame=f)

        # parpelleig: una finestra suau cap al 62% del cicle
        b = max(0.0, 1.0 - abs(t - 0.62) / 0.055)
        blink = 1.0 - 0.88 * (b * b * (3.0 - 2.0 * b))
        for eye in eyes:
            eye.scale = (1.0, 1.0, blink)
            eye.keyframe_insert("scale", frame=f)

        # la llum travessa el logo d'esquerra a dreta
        s = t
        sweep.location = (-1500.0 + 3000.0 * s,
                          -600.0,
                          900.0 - 900.0 * s)
        sweep.data.energy = 8.0e6 * math.sin(math.pi * min(1.0, s * 1.15))
        sweep.keyframe_insert("location", frame=f)
        sweep.data.keyframe_insert("energy", frame=f)

    # hi ha clau a cada frame: la interpolació no afecta el resultat


def setup_camera_and_render(res):
    cam_data = bpy.data.cameras.new("camera")
    cam_data.type = "PERSP"
    cam_data.lens = 85.0
    cam_data.ortho_scale = 1120.0
    cam_data.clip_start = 1.0
    cam_data.clip_end = 20000.0
    cam = bpy.data.objects.new("camera", cam_data)
    bpy.context.collection.objects.link(cam)
    cam.location = (0.0, -2680.0, 0.0)
    cam.rotation_euler = (math.pi / 2.0, 0.0, 0.0)
    bpy.context.scene.camera = cam

    scn = bpy.context.scene
    engines = [e.identifier for e in
               bpy.types.RenderSettings.bl_rna.properties["engine"].enum_items]
    scn.render.engine = next(
        (e for e in engines if "EEVEE" in e), "CYCLES")
    if scn.render.engine == "CYCLES":
        scn.cycles.samples = 32
        scn.cycles.use_denoising = True
    else:
        eevee = scn.eevee
        for name, value in (("taa_render_samples", 64),
                            ("use_raytracing", True),
                            ("use_bloom", True)):
            if hasattr(eevee, name):
                try:
                    setattr(eevee, name, value)
                except Exception:
                    pass
    scn.render.resolution_x = res
    scn.render.resolution_y = res
    scn.render.resolution_percentage = 100
    scn.render.film_transparent = True
    scn.render.image_settings.file_format = "PNG"
    scn.render.image_settings.color_mode = "RGBA"
    scn.render.image_settings.color_depth = "8"
    scn.view_settings.view_transform = "Standard"
    scn.view_settings.look = "None"


def main():
    argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
    opts = {"out": "../../output/logo-anim/frames", "start": "1", "end": str(FRAMES),
            "res": "1024", "blend": ""}
    for i, a in enumerate(argv):
        if a.startswith("--") and i + 1 < len(argv) and a[2:] in opts:
            opts[a[2:]] = argv[i + 1]
    out = os.path.abspath(opts["out"])
    os.makedirs(out, exist_ok=True)

    clean_scene()
    root = bpy.data.objects.new("arrel", None)
    bpy.context.collection.objects.link(root)
    logo = logo_mesh(float(opts.get("bevel", "30")))
    logo.parent = root
    eyes = [eye_mesh(i, *e) for i, e in enumerate(EYES)]
    for eye in eyes:
        eye.parent = root
    sweep = setup_world_and_lights()
    animate(root, eyes, sweep)
    setup_camera_and_render(int(opts["res"]))
    print("logo: %d vertexs, %d cares, %.0fx%.0f" % (
        len(logo.data.vertices), len(logo.data.polygons),
        logo.dimensions.x, logo.dimensions.z))

    scn = bpy.context.scene
    scn.render.filepath = os.path.join(out, "f_")
    if opts["blend"]:
        bpy.ops.wm.save_as_mainfile(filepath=os.path.abspath(opts["blend"]))
    scn.frame_start = int(opts["start"])
    scn.frame_end = int(opts["end"])
    bpy.ops.render.render(animation=True)
    print("FET", out)


main()
