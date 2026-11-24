"""Render the clay winds: one sprite sheet and one tint mask per silhouette and mood.

    blender -b -P tools/characters/build.py -- [--only pebble,cloud] [--moods idle,done] [--preview DIR]

Every frame comes from pose(), a pure function of the mood and the frame
number, so the same script renders the same sheets. See README.md.
"""

import argparse
import json
import math
import sys
import tempfile
from pathlib import Path

import bmesh
import bpy
import numpy as np
from mathutils import Euler, Matrix, Vector
from mathutils.bvhtree import BVHTree

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / "internal/web/app/characters"
SIZE, FPS, QUALITY = 128, 12, 82
NEUTRAL = (0.8, 0.8, 0.8, 1)
INK = (0.012, 0.011, 0.02, 1)
# Each wind family (the third letter of a wind's style, as in winds.js) keeps one
# silhouette: the width it is scaled to, the height of its eyes and a few metaballs (centre, radius, x/y/z size).
PEBBLE = [((0, 0, 0.95), 1.3, (1.05, 0.9, 1.2)), ((-0.4, 0, 1.25), 0.95, (1, 0.9, 1.12)),
          ((0.4, 0, 1.25), 0.95, (1, 0.9, 1.12))]
SILHOUETTES = {
    "pebble": ("o", 2.05, 1.05, PEBBLE),
    "cloud": ("n", 2.3, 0.86, [((0, 0, 0.78), 1.2, (1.25, 0.8, 0.8)), ((0, 0, 1.25), 1.0, (1, 1, 1)),
                         ((-0.68, 0, 0.92), 0.7, (1, 1, 1)), ((0.7, 0, 0.86), 0.64, (1, 1, 1))]),
    # The pebble with a gust on top: a crest that rises and breaks over to one side.
    "curl": ("s", 1.9, 0.98, PEBBLE + [
        ((0.14 + (0.3 - 0.14 * s) * math.cos(math.pi * (1 - 1.15 * s)), 0,
          2.2 + (0.3 - 0.14 * s) * math.sin(math.pi * (1 - 1.15 * s))), 0.36 - 0.2 * s, (1, 1, 1))
        for s in [k / 9 for k in range(10)]]),
    # Not a style letter: the face of every wind in the minairó pack (winds.js, WindFace.kt).
    "minairo": ("minairo", 1.7, 0.96, [((0, 0, 0.8), 1.25, (1, 0.9, 0.9)), ((0, 0, 1.15), 0.95, (0.95, 0.9, 0.88))]),
}
# Moods of winds.js; frames at FPS, loop or play once and hold the last frame.
MOODS = {"idle": (24, True), "thinking": (16, True), "searching": (12, True), "reading": (16, True),
         "writing": (12, True), "working": (8, True), "discussing": (16, True), "waiting": (24, True),
         "failed": (20, False), "done": (16, False), "news": (16, True)}


def ease(x):
    x = min(max(x, 0.0), 1.0)
    return x * x * (3 - 2 * x)


def bump(t, at, width):
    d = min(abs(t - at), 1 - abs(t - at)) / width
    return math.cos(d * math.pi / 2) ** 2 if d < 1 else 0.0


def hold(t, start, end, edge):
    return ease((t - start + edge) / edge) * ease((end + edge - t) / edge)


def pose(mood, i, n):
    """The pose at frame i of n: squash (+ flatter), hop, tilt/turn/lean in degrees, gaze, eye
    opening and size, upper lid (cover) and its slant (brow: + determined, - sad), happy arcs
    over half way (happy), smile, open mouth, wavy mouth, and which foot is lifted (step)."""
    t = i / n if MOODS[mood][1] else i / (n - 1)
    w = lambda k, shift=0.0: math.sin(2 * math.pi * (k * t + shift))  # k cycles per loop
    p = dict(sq=0.0, hop=0.0, tilt=0.0, turn=0.0, lean=0.0, gx=0.0, gz=0.0, open=1.0, wide=1.0,
             cover=0.0, brow=0.0, happy=0.0, smile=0.6, mouth=0.0, wave=0.0, step=0.0)
    if mood == "idle":  # breathes, glances aside, blinks
        look = hold(t, 0.25, 0.5, 0.08)
        p.update(sq=0.025 * w(1), gx=0.07 * look, gz=0.02 * look, tilt=4 * look, open=1 - bump(t, 0.8, 0.06))
    elif mood == "thinking":  # slow tilt, eyes drifting up
        p.update(gx=0.05 + 0.02 * w(1), gz=0.08, tilt=6 + 3 * w(1), brow=-0.3, cover=0.12, mouth=0.25)
    elif mood == "searching":  # peers forward and scans fast
        p.update(gx=0.09 * w(1), turn=-10 * w(1, -0.08), lean=6, wide=1.08, mouth=0.35, sq=0.02 * w(2))
    elif mood == "reading":  # two lines read left to right, a quick return
        u = (2 * t) % 1
        p.update(gx=0.07 * (2 * ease(u / 0.8) - 1 if u < 0.8 else 1 - 2 * ease((u - 0.8) / 0.2)),
                 gz=-0.06, lean=4, tilt=2 * w(2), smile=0.3, cover=0.25)
    elif mood == "writing":  # head down, bobbing to its own rhythm
        p.update(gz=-0.07, gx=0.03 * w(1), sq=0.035 * math.cos(4 * math.pi * t), lean=6, tilt=4 * w(1),
                 cover=0.3, brow=0.25, smile=0.35)
    elif mood == "working":  # hard squash-bob, stepping feet, determined squint
        u = (2 * t) % 1
        p.update(hop=0.08 * math.sin(math.pi * u) ** 2, sq=0.13 * max(0.0, math.cos(2 * math.pi * u)) ** 3
                 - 0.05 * math.sin(math.pi * u) ** 2, brow=0.55, cover=0.35, gz=-0.02, tilt=3 * w(1),
                 smile=0.2, step=math.sin(2 * math.pi * t))
    elif mood == "discussing":  # turns to the others and talks
        p.update(turn=14 * w(1), gx=0.06 * w(1), mouth=0.5 * max(0.0, w(4)), tilt=3 * w(2))
    elif mood == "waiting":  # leans toward the person, wide eyes looking out and down
        p.update(lean=7 + 3 * w(1), tilt=-4 * w(1), gz=-0.03, wide=1.1, open=1 - bump(t, 0.5, 0.07), smile=0.6)
    elif mood == "failed":  # flinch, droop, then a sigh; holds the drooped pose
        d, sigh, flinch = ease((i - 2) / 6), math.sin(math.pi * ease((i - 10) / 8)), 1 - ease(i / 3)
        p.update(hop=0.04 * math.sin(math.pi * min(i / 3, 1)), wide=1 + 0.15 * flinch, sq=0.09 * d - 0.05 * sigh,
                 lean=10 * d, tilt=-6 * d, gz=-0.08 * d, gx=-0.03 * d, brow=-0.6 * d, cover=0.3 * d + 0.2 * sigh,
                 smile=0.6 - 0.7 * d, wave=0.025 * d)
    elif mood == "done":  # anticipation squash, stretched hop, landing squash, wobble to rest
        if i <= 3:
            p.update(sq=0.17 * ease(i / 3), cover=0.35 * ease(i / 3))
        elif i <= 8:
            u = (i - 3) / 5
            p.update(hop=0.2 * math.sin(math.pi * u), sq=-0.12 * (1 - u), happy=ease(u * 2))
        else:
            k = i - 9
            p.update(sq=0.15 * math.exp(-0.5 * k) * math.cos(1.2 * k), tilt=5 * math.exp(-0.45 * k) * math.sin(1.2 * k),
                     happy=1.0)
        p.update(smile=1.0)
    elif mood == "news":  # hello: two little hops, rocking side to side
        u = (2 * t) % 1
        p.update(hop=0.12 * math.sin(math.pi * u), sq=0.1 * max(0.0, math.cos(2 * math.pi * u)) ** 3,
                 wide=1.1, smile=1.0, tilt=7 * w(1))
    return p


def follow(mood, i, n):
    """What hangs off the body lags behind it: two damped springs pushed by the body's
    motion up to frame i (loops run twice first, so they start settled). sway: degrees the
    beard and the cap swing sideways; flop: how far the cap's tip flies up (+) or droops."""
    loop = MOODS[mood][1]
    sway = vsway = flop = vflop = 0.0
    top = lambda q: q["hop"] - 1.8 * q["sq"]  # height of the top of the head
    last = [pose(mood, 0, n)] * 2
    for k in range(2 * n + i + 1 if loop else i + 1):
        p = pose(mood, k % n, n)
        vsway += -0.3 * sway - 0.3 * vsway - 0.8 * (p["tilt"] - last[0]["tilt"]) - 0.3 * (p["turn"] - last[0]["turn"])
        vflop += -0.3 * flop - 0.3 * vflop - 4 * (top(p) - 2 * top(last[0]) + top(last[1]))
        sway, flop, last = sway + vsway, flop + vflop, [p, last[0]]
    return dict(sway=max(-14.0, min(14.0, sway)), flop=max(-1.0, min(1.0, flop)))


def material(name, color, roughness, sss=0.0, sheen=0.0, coat=0.0, emit=0.0, seed=0, spec=0.5, grain=4.0):
    mat = bpy.data.materials.new(name)
    nodes, links = mat.node_tree.nodes, mat.node_tree.links
    bsdf = nodes["Principled BSDF"]
    bsdf.inputs["Base Color"].default_value = color
    bsdf.inputs["Roughness"].default_value = roughness
    bsdf.inputs["Subsurface Weight"].default_value = sss
    bsdf.inputs["Subsurface Radius"].default_value = (0.6, 0.45, 0.4)
    bsdf.inputs["Subsurface Scale"].default_value = 0.12
    bsdf.inputs["Sheen Weight"].default_value = sheen
    bsdf.inputs["Coat Weight"].default_value = coat
    bsdf.inputs["Specular IOR Level"].default_value = spec
    if emit:
        bsdf.inputs["Emission Color"].default_value = color
        bsdf.inputs["Emission Strength"].default_value = emit
    if sss:  # hand-made clay: slow lumps down to a fine grain, seeded per silhouette
        bump, noise = nodes.new("ShaderNodeBump"), nodes.new("ShaderNodeTexNoise")
        bump.inputs["Strength"].default_value, bump.inputs["Distance"].default_value = 1.0, 0.025
        noise.noise_dimensions, noise.inputs["W"].default_value = "4D", seed
        noise.inputs["Scale"].default_value, noise.inputs["Detail"].default_value = grain, 6.0
        links.new(noise.outputs["Fac"], bump.inputs["Height"])
        links.new(bump.outputs["Normal"], bsdf.inputs["Normal"])
    return mat


def mask_material():
    """White where the wind's colour goes, held out (no alpha) everywhere else."""
    mat = bpy.data.materials.new("mask")
    nodes, links = mat.node_tree.nodes, mat.node_tree.links
    nodes.remove(nodes["Principled BSDF"])
    attr, mix = nodes.new("ShaderNodeAttribute"), nodes.new("ShaderNodeMixShader")
    attr.attribute_type, attr.attribute_name = "OBJECT", "mask"
    emit = nodes.new("ShaderNodeEmission")
    links.new(attr.outputs["Fac"], mix.inputs[0])
    links.new(nodes.new("ShaderNodeHoldout").outputs[0], mix.inputs[1])
    links.new(emit.outputs[0], mix.inputs[2])
    links.new(mix.outputs[0], nodes["Material Output"].inputs[0])
    return mat


def glow_material(name, color, amount=0.75):
    """A soft round decal: the contact shadow and the cheek blush. Without a colour it
    cuts the mask instead, so the blush keeps its pink on every wind."""
    mat = bpy.data.materials.new(name)
    nodes, links = mat.node_tree.nodes, mat.node_tree.links
    nodes.remove(nodes["Principled BSDF"])
    coords, length = nodes.new("ShaderNodeTexCoord"), nodes.new("ShaderNodeVectorMath")
    length.operation = "LENGTH"
    fall = nodes.new("ShaderNodeMapRange")
    fall.inputs["To Min"].default_value, fall.inputs["To Max"].default_value = 1.0, 0.0
    fall.interpolation_type = "SMOOTHSTEP"
    strength, mix = nodes.new("ShaderNodeMath"), nodes.new("ShaderNodeMixShader")
    strength.operation = "MULTIPLY"
    strength.name, strength.inputs[1].default_value = "strength", amount
    emit = nodes.new("ShaderNodeEmission" if color else "ShaderNodeHoldout")
    if color:
        emit.inputs["Color"].default_value = color
    links.new(coords.outputs["Object"], length.inputs[0])
    links.new(length.outputs["Value"], fall.inputs["Value"])
    links.new(fall.outputs[0], strength.inputs[0])
    links.new(strength.outputs[0], mix.inputs[0])
    links.new(nodes.new("ShaderNodeBsdfTransparent").outputs[0], mix.inputs[1])
    links.new(emit.outputs[0], mix.inputs[2])
    links.new(mix.outputs[0], nodes["Material Output"].inputs[0])
    return mat


def obj(name, data, parent, mat=None, mask=0.0, cut=None):
    ob = bpy.data.objects.new(name, data)
    bpy.context.collection.objects.link(ob)
    ob.parent = parent
    ob["mask"] = mask
    if mat:  # the object keeps its mask-pass material in its own slot, used by render()
        ob.data.materials.append(mat)
        slot = ob.material_slots[0]
        slot.link, slot.material, slot.link = "OBJECT", cut or bpy.data.materials["mask"], "DATA"
    return ob


def sphere(name, parent, mat, mask=0.0, half=False):
    mesh = bpy.data.meshes.new(name)
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=24, v_segments=16, radius=1.0)
    if half:
        bmesh.ops.delete(bm, geom=[v for v in bm.verts if v.co.z < -1e-4], context="VERTS")
    bm.to_mesh(mesh)
    mesh.shade_smooth()
    return obj(name, mesh, parent, mat, mask)


def arc(name, parent, mat):
    curve = bpy.data.curves.new(name, "CURVE")
    curve.dimensions, curve.bevel_resolution, curve.use_fill_caps = "3D", 3, True
    curve.splines.new("POLY").points.add(16)
    return obj(name, curve, parent, mat)


def bend(ob, half, rise, thick, wave=0.0):
    ob.data.bevel_depth = thick
    for k, pt in enumerate(ob.data.splines[0].points):
        x = half * (k / 8 - 1)
        pt.co = (x, 0, rise * (1 - (x / half) ** 2) + wave * math.sin(math.pi * x / half), 1)


def scene():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    sc = bpy.context.scene
    # Cycles on the CPU with a fixed seed, no denoiser and no dithering repeats its frames.
    sc.render.engine, sc.cycles.device, sc.cycles.seed = "CYCLES", "CPU", 1
    sc.render.resolution_x = sc.render.resolution_y = SIZE
    sc.render.dither_intensity = 0  # dithering adds noise that differs on every run
    sc.render.film_transparent = True
    sc.render.image_settings.file_format, sc.render.image_settings.color_mode = "PNG", "RGBA"
    sc.view_settings.view_transform, sc.view_settings.look = "AgX", "AgX - Medium High Contrast"
    world = bpy.data.worlds.new("studio")
    world.node_tree.nodes["Background"].inputs["Color"].default_value = (0.25, 0.25, 0.27, 1)
    sc.world = world
    cam = bpy.data.objects.new("camera", bpy.data.cameras.new("camera"))
    cam.data.type, cam.data.ortho_scale = "ORTHO", 2.8
    cam.location, cam.rotation_euler = (0, -10, 2.655), Euler((math.radians(82), 0, 0))
    sc.collection.objects.link(cam)
    sc.camera = cam
    # Soft studio: a big key from the upper left, a dim fill, a rim from behind.
    for name, at, power, size, tint in (("key", (-3.5, -4.5, 5), 1100, 4, (1, 0.93, 0.84)),
                                        ("fill", (4.5, -3, 1.5), 170, 5, (0.9, 0.95, 1)),
                                        ("rim", (2.5, 4.5, 4), 1600, 2.5, (0.92, 0.96, 1))):
        light = bpy.data.objects.new(name, bpy.data.lights.new(name, "AREA"))
        light.data.energy, light.data.size, light.data.color = power, size, tint
        light.location = at
        light.rotation_euler = (Vector((0, 0, 1)) - Vector(at)).to_track_quat("-Z", "Y").to_euler()
        sc.collection.objects.link(light)
    return sc


def character(silhouette, seed):
    clay = material("clay", NEUTRAL, 0.55, sss=0.25, sheen=0.35, seed=seed)
    ink = material("ink", INK, 0.4, coat=0.2, spec=0.2)
    spark = material("spark", (1, 1, 1, 1), 0.5, emit=6.0)
    rig = obj("rig", None, None)
    meta = bpy.data.metaballs.new("body")
    meta.resolution = meta.render_resolution = 0.04
    _, width, eyes, balls = SILHOUETTES[silhouette]
    for co, radius, (sx, sy, sz) in balls:
        el = meta.elements.new(type="ELLIPSOID")
        el.co, el.radius, el.size_x, el.size_y, el.size_z = co, radius, sx, sy, sz
    blob = obj("blob", meta, None)
    mesh = bpy.data.meshes.new_from_object(blob.evaluated_get(bpy.context.evaluated_depsgraph_get()))
    bpy.data.objects.remove(blob)
    xs = [v.co.x for v in mesh.vertices]
    mesh.transform(Matrix.Scale(width / (max(xs) - min(xs)), 4))
    # Settle it: the bottom flattens on the ground like soft clay.
    low = min(v.co.z for v in mesh.vertices)
    for v in mesh.vertices:
        z = v.co.z - low
        v.co.z = 0.04 + (z if z > 0.3 else 0.3 - (0.3 - z) * 0.35) - 0.3 * 0.65
    mesh.shade_smooth()
    body = obj("body", mesh, rig, clay, 1.0)
    tree = BVHTree.FromPolygons([v.co for v in mesh.vertices], [p.vertices for p in mesh.polygons])
    feet = [sphere("foot", rig, clay, 1.0) for _ in (-1, 1)]
    shadow = obj("shadow", bpy.data.meshes.new("shadow"), None, glow_material("shadow", (0, 0, 0, 1)))
    shadow.data.from_pydata([(-1, -1, 0), (1, -1, 0), (1, 1, 0), (-1, 1, 0)], [], [(0, 1, 2, 3)])
    parts = {"rig": rig, "body": body, "tree": tree, "shadow": shadow, "feet": feet, "eyes": eyes}
    for side in (-1, 1):
        anchor = obj("eye", None, rig)
        eye = sphere("eye", anchor, ink)
        sparkle = sphere("spark", eye, spark)
        sparkle.location, sparkle.scale = (0.3, -0.85, 0.4), (0.4, 0.2, 0.36)
        # Lids are clay shells just over the eye, hinged at its centre like real ones.
        lid = sphere("lid", eye, clay, 1.0, True)
        lid.scale = (1.12, 1.12, 1.12)
        parts[side] = dict(anchor=anchor, eye=eye, lid=lid, spark=sparkle, happy=arc("happy", anchor, ink))
    blush, cut = glow_material("blush", (1, 0.42, 0.55, 1), 0.8), glow_material("cut", None, 0.8)
    for side in (-1, 1):
        cheek = obj("cheek", None, rig)
        stick(cheek, tree, side * 0.68, eyes - 0.27)
        decal = obj("blush", bpy.data.meshes.new("blush"), cheek, blush, cut=cut)
        decal.data.from_pydata([(-1, 0, -1), (1, 0, -1), (1, 0, 1), (-1, 0, 1)], [], [(0, 1, 2, 3)])
        decal.location, decal.scale = (0, -0.02, 0), (0.24, 1, 0.15)
    mouth = obj("mouth", None, rig)
    parts["mouth"] = dict(anchor=mouth, line=arc("smile", mouth, ink), open=sphere("open", mouth, ink))
    if silhouette == "minairo":
        minairo(parts, seed)
    return parts


def minairo(parts, seed):
    """A white felt beard with a two-lobed moustache over the lower face, a rosy nose and a
    red barretina with its black ribbon, folded to one side, as the drawn minairó (winds.js).
    None of them is in the mask: they keep their colours on every wind."""
    tree, eyes, rig = parts["tree"], parts["eyes"], parts["rig"]
    felt = material("felt", (0.9, 0.88, 0.84, 1), 0.85, sss=0.15, sheen=0.6, seed=seed, grain=9.0)
    red = material("barretina", (0.42, 0.016, 0.012, 1), 0.7, sss=0.15, sheen=0.1, seed=seed, grain=6.0)  # #C9342C
    band = material("band", (0.012, 0.011, 0.013, 1), 0.9, sheen=0.1, spec=0.2)  # the black ribbon of a barretina, matte felt
    rosy = material("nose", (0.8, 0.25, 0.3, 1), 0.5, sss=0.3, sheen=0.2, seed=seed)
    meta = bpy.data.metaballs.new("beard")
    meta.resolution = meta.render_resolution = 0.03
    # Cheeks, the beard narrowing down to a scalloped point and, standing out of it, the
    # two lobes of the moustache; seated on the face (x, depth below the eyes, radius, out, flat).
    for x, dz, r, out, flat in ((0.5, 0.3, 0.3, 0.05, 1), (-0.5, 0.3, 0.3, 0.05, 1), (0.24, 0.3, 0.34, 0.05, 1),
                                (-0.24, 0.3, 0.34, 0.05, 1), (0.36, 0.55, 0.37, 0.05, 1), (-0.36, 0.55, 0.37, 0.05, 1),
                                (0, 0.55, 0.4, 0.05, 1), (0.2, 0.78, 0.3, 0.05, 1), (-0.2, 0.78, 0.3, 0.05, 1),
                                (0, 0.84, 0.3, 0.05, 1), (0.15, 0.3, 0.22, 0.2, 0.7), (-0.15, 0.3, 0.22, 0.2, 0.7)):
        hit, normal, _, _ = tree.ray_cast(Vector((x, -3, eyes - dz)), Vector((0, 1, 0)))
        el = meta.elements.new(type="ELLIPSOID")
        el.co, el.radius, el.size_x, el.size_y, el.size_z = hit + normal * out, r, 1.1 + 0.4 * (flat < 1), 0.5, flat
    blob = obj("blob", meta, None)
    beard = bpy.data.meshes.new_from_object(blob.evaluated_get(bpy.context.evaluated_depsgraph_get()))
    bpy.data.objects.remove(blob)
    beard.shade_smooth()
    parts["beard_tree"] = BVHTree.FromPolygons([v.co for v in beard.vertices], [p.vertices for p in beard.polygons])
    hinge = Vector((0, -0.5, eyes - 0.2))
    beard.transform(Matrix.Translation(-hinge))
    parts["beard"] = obj("beard", beard, rig, felt)
    parts["beard"].location = hinge
    nose = sphere("nose", rig, rosy)
    stick(nose, tree, 0, eyes - 0.24)
    nose.location.y -= 0.1  # on the moustache, which stands out of the face
    nose.scale = (0.14, 0.12, 0.13)
    # The band sits just above the eyes and the cap is as wide as the head there.
    head = obj("head", None, rig)
    head.location = (0, 0, eyes + 0.36)
    wide = tree.ray_cast(Vector((3, 0, eyes + 0.36)), Vector((-1, 0, 0)))[0].x
    cap = obj("cap", bpy.data.curves.new("cap", "CURVE"), head, red)
    cap.data.dimensions, cap.data.bevel_depth, cap.data.bevel_resolution = "3D", 1.0, 4
    cap.data.use_fill_caps = True
    cap.data.splines.new("POLY").points.add(15)
    cap["wide"] = wide
    ring = obj("band", bpy.data.curves.new("band", "CURVE"), head, band)
    ring.data.dimensions, ring.data.extrude, ring.data.bevel_depth, ring.data.bevel_resolution = "3D", 0.1, 0.04, 3
    spline = ring.data.splines.new("POLY")
    spline.points.add(31)
    spline.use_cyclic_u = True
    for k, pt in enumerate(spline.points):
        a = 2 * math.pi * k / 32
        pt.co = ((wide + 0.04) * math.cos(a), 0.95 * (wide + 0.04) * math.sin(a), 0.04, 1)
    parts["cap"] = cap


def barretina(cap, sway, flop):
    """The cap rises from the head and folds over to the right, its tip hanging down to the
    eyes; flop lifts or drops the fold and sway bends it sideways."""
    x, z, length = 0.0, 0.0, 1.8
    pts = cap.data.splines[0].points
    for k, pt in enumerate(pts):
        s = k / (len(pts) - 1)
        pt.co, pt.radius = (x, 0, z, 1), cap["wide"] * (1 - 0.7 * s ** 0.7) * (1 if k < len(pts) - 1 else 0.6)
        a = (3.05 - 0.8 * flop) * ease((s + 0.05) / 0.75) + math.radians(sway) * s
        x, z = x + math.sin(a) * length / (len(pts) - 1), z + math.cos(a) * length / (len(pts) - 1)


def stick(anchor, tree, x, z):
    """Seat an anchor on the body surface at (x, z), facing out of it."""
    hit, normal, _, _ = tree.ray_cast(Vector((x, -3, z)), Vector((0, 1, 0)))
    anchor.location = hit
    anchor.rotation_euler = normal.to_track_quat("-Y", "Z").to_euler()


def apply(parts, p):
    rig, tree = parts["rig"], parts["tree"]
    rig.location.z = p["hop"]
    rig.scale = (1 + 0.6 * p["sq"], 1 + 0.6 * p["sq"], 1 - p["sq"])
    rig.rotation_euler = Euler((math.radians(p["lean"]), math.radians(p["tilt"]), math.radians(p["turn"])))
    shadow = parts["shadow"]
    shadow.scale = (1.35 * (1 - 0.9 * p["hop"]) * (1 + 0.6 * p["sq"]), 1.1, 1)
    shadow.data.materials[0].node_tree.nodes["strength"].inputs[1].default_value = 0.75 * (1 - 1.6 * p["hop"])
    w, h = (0.23 * p["wide"], 0.28 * p["wide"]) if "beard" not in parts else (0.19 * p["wide"], 0.235 * p["wide"])
    for side in (-1, 1):
        f = parts[side]
        stick(f["anchor"], tree, side * 0.36 + p["gx"], parts["eyes"] + p["gz"])
        f["eye"].scale = (w, 0.05, max(h * p["open"], 0.015))
        # The lid comes down to 1 - 2 * cover of the eye, slanted to look determined or sad;
        # open, it tucks a little further back so its edge sinks into the body.
        f["lid"].rotation_euler = (math.asin(2 * p["cover"] - 1) - 0.4 * (1 - p["cover"]) ** 4,
                                   -side * 0.5 * p["brow"], 0)
        for ob in (f["eye"], f["spark"], f["lid"]):
            ob.hide_render = p["happy"] > 0.5
        f["happy"].hide_render = p["happy"] <= 0.5
        bend(f["happy"], w * 0.85, h * 0.6, 0.05)
        f["happy"].location = (0, -0.06, -0.06)
    for side, foot in zip((-1, 1), parts["feet"]):
        foot.location = (side * 0.5, -0.3, 0.12 + 0.09 * max(0.0, side * p["step"]))
        foot.scale = (0.32, 0.36, 0.16)
    m = parts["mouth"]
    under = 0.5 if "beard" in parts else 0.38  # the minairó's mouth is under its moustache
    stick(m["anchor"], parts.get("beard_tree", tree), p["gx"] * 0.5, parts["eyes"] - under + p["gz"] * 0.5)
    if "beard" in parts:
        parts["beard"].rotation_euler = (0, math.radians(p["sway"]), 0)
        barretina(parts["cap"], p["sway"], p["flop"])
    talking = p["mouth"] > 0.12
    m["line"].hide_render, m["open"].hide_render = talking, not talking
    bend(m["line"], 0.13 + 0.04 * p["smile"], -0.12 * p["smile"], 0.042, p["wave"])
    m["line"].location = (0, -0.03, 0)
    m["open"].scale = (0.08, 0.05, 0.035 + 0.08 * p["mouth"])


def render(sc, path, mask):
    sc.cycles.samples, sc.cycles.use_denoising = (16, False) if mask else (160, False)
    for ob in bpy.data.objects:
        for slot in ob.material_slots:
            slot.link = "OBJECT" if mask else "DATA"
    bpy.data.objects["shadow"].hide_render = mask
    sc.render.filepath = str(path)
    bpy.ops.render.render(write_still=True)


def pixels(path):
    img = bpy.data.images.load(str(path))
    px = np.array(img.pixels[:], dtype=np.float32).reshape(img.size[1], img.size[0], 4)
    bpy.data.images.remove(img)
    return px


def save(px, path, quality):
    img = bpy.data.images.new(path.stem, px.shape[1], px.shape[0], alpha=True)
    img.pixels[:] = px.ravel()
    img.file_format = "WEBP"
    path.parent.mkdir(parents=True, exist_ok=True)
    img.save(filepath=str(path), quality=quality)
    bpy.data.images.remove(img)


def build(silhouettes, moods):
    tmp = Path(tempfile.mkdtemp(prefix="gregal-characters-"))
    for seed, silhouette in enumerate(SILHOUETTES):
        if silhouette not in silhouettes:
            continue
        sc = scene()
        mask_material()
        parts = character(silhouette, seed + 1)
        for mood in moods:
            n = MOODS[mood][0]
            sheets = {False: [], True: []}
            for mask in (False, True):
                for i in range(n):
                    apply(parts, pose(mood, i, n) | follow(mood, i, n))
                    render(sc, tmp / "frame.png", mask)
                    sheets[mask].append(pixels(tmp / "frame.png"))
            colour, alpha = np.concatenate(sheets[False], axis=1), np.concatenate(sheets[True], axis=1)
            # WebP keeps alpha lossless: 32 levels look the same and halve what it costs.
            colour[..., 3] = np.round(colour[..., 3] * 31) / 31
            save(colour, OUT / silhouette / f"{mood}.webp", QUALITY)
            # The mask keeps only coverage, at half size: constant white compresses to
            # almost nothing. The largest of each 2x2 so the tint reaches the very edge.
            half = alpha[..., 3].reshape(SIZE // 2, 2, -1, 2).max((1, 3))
            white = np.ones(half.shape + (4,), dtype=np.float32)
            white[..., 3] = half
            save(white, OUT / silhouette / f"{mood}.mask.webp", 100)


def manifest():
    data = {"size": SIZE, "fps": FPS, "tint": "multiply",
            "files": "<silhouette>/<mood>.webp", "mask": "<silhouette>/<mood>.mask.webp", "maskSize": SIZE // 2,
            "silhouettes": {name: family for name, (family, *_) in SILHOUETTES.items()},
            "moods": {mood: {"frames": n, "loop": loop} for mood, (n, loop) in MOODS.items()}}
    (OUT / "manifest.json").write_text(json.dumps(data) + "\n", encoding="utf-8")


def preview(folder, silhouettes=tuple(SILHOUETTES), motion=False, tints=((0.36, 0.55, 0.94), (0.95, 0.65, 0.25))):
    """Contact sheets at 128 and 40 px: each silhouette in two wind colours with a few
    frames per mood, or with motion every frame of one silhouette, one mood per row."""
    rows = []
    for silhouette in silhouettes:
        for tint in tints:
            cells = []
            for mood, (n, _) in MOODS.items():
                if not (OUT / silhouette / f"{mood}.webp").exists():
                    continue
                colour, mask = pixels(OUT / silhouette / f"{mood}.webp"), pixels(OUT / silhouette / f"{mood}.mask.webp")
                mask = mask.repeat(2, 0).repeat(2, 1)
                for i in range(n) if motion else (0, n // 3, 2 * n // 3):
                    c, m = colour[:, i * SIZE:(i + 1) * SIZE], mask[:, i * SIZE:(i + 1) * SIZE, 3:]
                    rgb = c[..., :3] * (1 - m + m * np.array(tint, dtype=np.float32))
                    cells.append(np.concatenate([rgb, c[..., 3:]], axis=2))
                if motion:
                    rows.append(np.concatenate(cells + [np.zeros_like(cells[0])] * (24 - n), axis=1))
                    cells = []
            if cells:
                rows.append(np.concatenate(cells, axis=1))
    if not rows:
        return
    sheet = np.concatenate(rows[::-1], axis=0)  # Blender images start at the bottom row
    rgb, a = sheet[..., :3], sheet[..., 3:]
    for name, bg in (("light", 0.96), ("dark", 0.09)):
        img = bpy.data.images.new("preview", sheet.shape[1], sheet.shape[0])
        img.pixels[:] = np.concatenate([rgb * a + bg * (1 - a), np.ones_like(a)], axis=2).ravel()
        img.file_format = "PNG"
        for size in (SIZE, 40):
            img.scale(sheet.shape[1] * size // SIZE, sheet.shape[0] * size // SIZE)
            img.save(filepath=str(Path(folder) / f"{'motion-' + silhouettes[0] if motion else 'contact'}-{size}-{name}.png"))
        bpy.data.images.remove(img)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--only", default=",".join(SILHOUETTES))
    parser.add_argument("--moods", default=",".join(MOODS))
    parser.add_argument("--preview", help="also write contact sheets to this folder")
    args = parser.parse_args(sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else [])
    build(args.only.split(","), [m for m in args.moods.split(",") if m])
    manifest()
    if args.preview:
        Path(args.preview).mkdir(parents=True, exist_ok=True)
        preview(args.preview)
        for silhouette in SILHOUETTES:
            preview(args.preview, (silhouette,), True, ((0.36, 0.55, 0.94),))


main()
