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
FUR_QUALITY = 75
# Silhouettes rendered larger than SIZE: the fur needs the pixels of a phone screen.
SIZES = {"pelut": 256}
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
    # Not a style letter either: every wind in the pelut pack, a round ball of short fur.
    "pelut": ("pelut", 1.7, 1.02, [((0, 0, 0.95), 1.3, (1, 0.95, 1.08))]),
}
# Silhouettes with a barretina in its own mask (<mood>.cap.webp), tinted per wind.
CAPS = ("minairo",)
# Moods of winds.js; frames at FPS, loop or play once and hold the last frame.
MOODS = {"idle": (24, True), "thinking": (16, True), "searching": (16, True), "reading": (20, True),
         "writing": (12, True), "working": (12, True), "discussing": (16, True), "waiting": (24, True),
         "failed": (20, False), "done": (20, False), "news": (16, True)}
# Moods where the wind handles a prop with its two clay hands (task 142), and the frame a
# still face shows (reduced motion): one where the prop reads best.
PROPS = {"searching": ("magnifier", 4), "reading": ("book", 13), "writing": ("pencil", 3), "working": ("laptop", 8),
         "thinking": ("gear", 6), "done": ("confetti", 19)}
PROP_SCALE = 1.4


def ease(x):
    x = min(max(x, 0.0), 1.0)
    return x * x * (3 - 2 * x)


def bump(t, at, width):
    d = min(abs(t - at), 1 - abs(t - at)) / width
    return math.cos(d * math.pi / 2) ** 2 if d < 1 else 0.0


def hold(t, start, end, edge):
    return ease((t - start + edge) / edge) * ease((end + edge - t) / edge)


def lens(t):
    """The magnifier sweeps across, lifts to the right eye to peek (peek) and dips to the ground (low)."""
    peek, low = bump(t, 0.25, 0.22), bump(t, 0.75, 0.22)
    return 0.42 * math.sin(2 * math.pi * t), peek, low


def leaf(t):
    """How far the page has turned (0 lying on the right, 1 on the left): read, then turn."""
    return ease((t - 0.55) / 0.3)


def scribble(t):
    """The pencil's tip on the pad: a word written left to right, then lifted back to the start."""
    if t < 0.67:
        u = t / 0.67
        return -0.16 + 0.32 * u, 0.028 * math.sin(4 * math.pi * u), 0.0
    v = (t - 0.67) / 0.33
    return 0.16 - 0.32 * ease(v), 0.0, 0.07 * math.sin(math.pi * v)


def ratchet(t):
    """Gear teeth turned so far: two pushes a loop, each with a small wind-back, a snap and an overshoot."""
    k, u = divmod(2 * t, 1)
    if u < 0.2:
        e = -0.12 * ease(u / 0.2)
    elif u < 0.5:
        e = -0.12 + 1.22 * ease((u - 0.2) / 0.3)
    else:
        e = 1.1 - 0.1 * ease((u - 0.5) / 0.25)
    return k + e, u


# How high each hand is over the keys, frame by frame: frame 8 winds up, frame 9 hits enter.
TYPING = [(0.0, 0.4), (0.4, 0.0), (0.0, 0.4), (0.4, 0.0), (0.0, 0.4), (0.4, 0.0), (0.0, 0.4),
          (0.25, 0.5), (0.3, 0.85), (0.4, 0.0), (0.25, 0.2), (0.4, 0.0)]


def pose(mood, i, n):
    """The pose at frame i of n: squash (+ flatter), hop, tilt/turn/lean in degrees, gaze, eye
    opening and size, upper lid (cover) and its slant (brow: + determined, - sad), happy arcs
    over half way (happy), smile, open mouth, wavy mouth, which foot is lifted (step) and how
    much the right eye is magnified behind the lens (zoom)."""
    t = i / n if MOODS[mood][1] else i / (n - 1)
    w = lambda k, shift=0.0: math.sin(2 * math.pi * (k * t + shift))  # k cycles per loop
    p = dict(sq=0.0, hop=0.0, tilt=0.0, turn=0.0, lean=0.0, gx=0.0, gz=0.0, open=1.0, wide=1.0,
             cover=0.0, brow=0.0, happy=0.0, smile=0.6, mouth=0.0, wave=0.0, step=0.0, zoom=0.0)
    if mood == "idle":  # breathes, glances aside, blinks
        look = hold(t, 0.25, 0.5, 0.08)
        p.update(sq=0.025 * w(1), gx=0.07 * look, gz=0.02 * look, tilt=4 * look, open=1 - bump(t, 0.8, 0.06))
    elif mood == "thinking":  # hand on the chin, eyes up on the gear it pushes round
        _, u = ratchet(t)
        push = bump(u, 0.35, 0.2)
        p.update(gx=-0.05 + 0.015 * w(2), gz=0.09, tilt=-6 - 3 * push, brow=-0.3, cover=0.12 - 0.1 * push,
                 mouth=0.2 * (1 - push), smile=0.4 + 0.4 * push, sq=0.05 * bump(u, 0.15, 0.12) - 0.04 * push)
    elif mood == "searching":  # sweeps the magnifier, peeks through it with one big eye, then low
        _, peek, low = lens(t)
        p.update(gx=0.08 * math.sin(2 * math.pi * t), gz=0.05 * peek - 0.07 * low, turn=9 * math.sin(2 * math.pi * (t - 0.04)),
                 lean=5 + 7 * low, tilt=-5 * peek + 3 * low, wide=1.06 + 0.1 * peek, zoom=peek, mouth=0.4 * peek,
                 smile=0.5, hop=0.04 * peek, sq=0.07 * bump(t, 0.08, 0.1) - 0.06 * peek + 0.04 * bump(t, 0.6, 0.1))
    elif mood == "reading":  # reads across the pages, then turns one: wind-up, lift, a happy settle
        turned = leaf(t)
        read = ease(t / 0.5)
        gx = 0.16 * read - 0.08 if t < 0.55 else 0.08 * math.cos(math.pi * turned)
        p.update(gx=gx, gz=-0.08, lean=5, tilt=3 * math.cos(math.pi * turned) * hold(t, 0.55, 0.85, 0.1),
                 cover=0.22 - 0.15 * bump(t, 0.7, 0.15), smile=0.35 + 0.5 * bump(t, 0.78, 0.15),
                 sq=0.06 * bump(t, 0.5, 0.06) - 0.05 * bump(t, 0.66, 0.1) + 0.04 * bump(t, 0.86, 0.06))
    elif mood == "writing":  # head down, eyes on the pencil, bobbing with each stroke
        x, y, lift = scribble(t)
        p.update(gz=-0.08, gx=0.06 * x / 0.16, sq=0.03 * math.cos(8 * math.pi * min(t / 0.67, 1)) + 0.05 * lift,
                 lean=6, tilt=4 * x / 0.16, cover=0.3, brow=0.3, smile=0.35 + 3 * lift)
    elif mood == "working":  # types on the laptop, winds up and hits enter
        left, right = TYPING[i]
        smash = 1.0 if i == 9 else 0.4 if i == 10 else 0.0
        p.update(sq=0.03 * (left == 0 or right == 0) + 0.12 * smash - 0.06 * (i == 8), hop=0.03 * (i == 8),
                 lean=8, gz=-0.07, gx=0.04 * (right - left) / 0.4, brow=0.5 - 0.3 * (i >= 10), cover=0.3 + 0.35 * smash,
                 smile=0.25 + 0.6 * (i >= 10), tilt=3 * (i == 8) - 2 * smash)
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
    """White where the wind's colour goes, held out (no alpha) everywhere else; render()
    points it at the "cap" property for the barretina's own mask."""
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


def scene(size):
    bpy.ops.wm.read_factory_settings(use_empty=True)
    sc = bpy.context.scene
    # Cycles on the CPU with a fixed seed, no denoiser and no dithering repeats its frames.
    sc.render.engine, sc.cycles.device, sc.cycles.seed = "CYCLES", "CPU", 1
    sc.render.resolution_x = sc.render.resolution_y = size
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
    if silhouette == "pelut":
        parts["fur"] = fur(parts)
    props(parts, seed)
    return parts


def fur(parts):
    """Short hair all over the body, in the mask so it takes the wind's colour, shorter round
    the face so the eyes and mouth stay clear: Cycles renders the strands as curves."""
    body, eyes = parts["body"], parts["eyes"]
    hair = bpy.data.materials.new("fur")
    nodes, links = hair.node_tree.nodes, hair.node_tree.links
    nodes.remove(nodes["Principled BSDF"])
    bsdf = nodes.new("ShaderNodeBsdfHairPrincipled")
    bsdf.parametrization = "COLOR"
    bsdf.inputs["Color"].default_value = NEUTRAL
    bsdf.inputs["Roughness"].default_value = 0.4
    bsdf.inputs["Radial Roughness"].default_value = 0.6
    links.new(bsdf.outputs[0], nodes["Material Output"].inputs[0])
    body.data.materials.append(hair)
    slot = body.material_slots[1]
    slot.link, slot.material, slot.link = "OBJECT", bpy.data.materials["mask"], "DATA"
    # The face: bare round the eyes and mouth, the fur growing back towards the edge.
    group = body.vertex_groups.new(name="length")
    for v in body.data.vertices:
        d = math.hypot(v.co.x / 0.62, (v.co.z - eyes + 0.18) / 0.5) if v.co.y < 0 else 9.0
        group.add([v.index], 0.15 + 0.85 * ease((d - 0.95) / 0.45), "REPLACE")
    psys = body.modifiers.new("fur", "PARTICLE_SYSTEM").particle_system
    psys.seed, psys.vertex_group_length = 155, "length"
    s = psys.settings
    s.type, s.count, s.hair_length, s.material = "HAIR", 2500, 0.2, 2
    s.child_type, s.child_percent, s.rendered_child_count = "INTERPOLATED", 12, 12
    # Tufts that clump a little and tips that stray: a soft, uneven outline.
    s.child_length, s.clump_factor, s.roughness_1, s.roughness_endpoint = 1.0, 0.25, 0.02, 0.07
    s.roughness_2, s.roughness_2_size = 0.04, 1.0
    s.root_radius, s.tip_radius, s.radius_scale = 1.0, 0.0, 0.006
    s.render_step = 3
    return psys


def minairo(parts, seed):
    """A white felt beard with a two-lobed moustache over the lower face, a rosy nose and a
    barretina with its black ribbon, folded to one side, as the drawn minairó (winds.js).
    None of them is in the mask: they keep their colours on every wind. The barretina is
    neutral felt in its own cap mask, which clients tint with each minairó's barretina colour."""
    tree, eyes, rig = parts["tree"], parts["eyes"], parts["rig"]
    felt = material("felt", (0.9, 0.88, 0.84, 1), 0.85, sss=0.15, sheen=0.6, seed=seed, grain=9.0)
    felt_cap = material("barretina", NEUTRAL, 0.7, sss=0.15, sheen=0.1, seed=seed, grain=6.0)
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
    cap = obj("cap", bpy.data.curves.new("cap", "CURVE"), head, felt_cap)
    cap["cap"] = 1.0
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


def rig_matrix(p):
    """Where the body is: hop, squash and stretch, lean, tilt and turn."""
    return Matrix.LocRotScale(Vector((0, 0, p["hop"])), Euler(tuple(math.radians(p[k]) for k in ("lean", "tilt", "turn"))),
                              Vector((1 + 0.6 * p["sq"], 1 + 0.6 * p["sq"], 1 - p["sq"])))


def apply(parts, p):
    rig, tree = parts["rig"], parts["tree"]
    rig.matrix_basis = rig_matrix(p)
    shadow = parts["shadow"]
    shadow.scale = (1.35 * (1 - 0.9 * p["hop"]) * (1 + 0.6 * p["sq"]), 1.1, 1)
    shadow.data.materials[0].node_tree.nodes["strength"].inputs[1].default_value = 0.75 * (1 - 1.6 * p["hop"])
    w, h = (0.23 * p["wide"], 0.28 * p["wide"]) if "beard" not in parts else (0.19 * p["wide"], 0.235 * p["wide"])
    for side in (-1, 1):
        f = parts[side]
        stick(f["anchor"], tree, side * 0.36 + p["gx"], parts["eyes"] + p["gz"])
        big = 1 + 0.6 * p["zoom"] * (side == 1)  # the right eye seen through the magnifier
        f["eye"].scale = (w * big, 0.05, max(h * p["open"] * big, 0.015))
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
        # The pelut's feet step out of its fur.
        foot.location = (side * 0.5, -0.3 - 0.25 * ("fur" in parts), 0.12 + 0.09 * max(0.0, side * p["step"]))
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


def lin(hex):
    return tuple(c ** 2.2 for c in hex_rgb(hex)) + (1,)


def shape(name, parent, mat, make, cut=None):
    """A mesh object made by make(bm), smooth like the rest of the clay."""
    mesh = bpy.data.meshes.new(name)
    bm = bmesh.new()
    make(bm)
    bm.to_mesh(mesh)
    bm.free()
    mesh.shade_smooth()
    return obj(name, mesh, parent, mat, cut=cut)


def block(name, parent, mat, size, at=(0, 0, 0), soft=0.35):
    """A box of clay with rounded edges, size along x, y and z, centred at `at`."""
    def make(bm):
        bmesh.ops.create_cube(bm, size=1.0)
        bmesh.ops.scale(bm, vec=size, verts=bm.verts)
        bmesh.ops.bevel(bm, geom=list(bm.edges), offset=soft * min(size) / 2, offset_type="OFFSET", segments=3,
                        profile=0.5, affect="EDGES", clamp_overlap=True)
        bmesh.ops.translate(bm, vec=at, verts=bm.verts)
    return shape(name, parent, mat, make)


def rod(name, parent, mat, r1, r2, length, start=0.0, sides=12):
    """A cylinder or cone along z from `start` to `start + length`."""
    def make(bm):
        bmesh.ops.create_cone(bm, cap_ends=True, segments=sides, radius1=r1, radius2=r2, depth=length)
        bmesh.ops.translate(bm, vec=(0, 0, start + length / 2), verts=bm.verts)
    return shape(name, parent, mat, make)


def ring(name, parent, mat, radius, tube):
    """A torus round the y axis: the magnifier's rim, facing the camera."""
    def make(bm):
        rows = []
        for a in range(28):
            u = 2 * math.pi * a / 28
            rows.append([bm.verts.new(((radius + tube * math.cos(v)) * math.cos(u), tube * math.sin(v),
                                       (radius + tube * math.cos(v)) * math.sin(u)))
                         for v in (2 * math.pi * b / 10 for b in range(10))])
        for a in range(28):
            for b in range(10):
                bm.faces.new((rows[a][b], rows[(a + 1) % 28][b], rows[(a + 1) % 28][(b + 1) % 10], rows[a][(b + 1) % 10]))
    return shape(name, parent, mat, make)


def gear(name, parent, mat, teeth, radius, depth):
    """A cog in the xz plane, facing the camera, with round-topped teeth."""
    def make(bm):
        pts = [bm.verts.new(((radius if k % 4 in (1, 2) else 0.76 * radius) * math.cos(2 * math.pi * k / (4 * teeth)),
                             -depth / 2, (radius if k % 4 in (1, 2) else 0.76 * radius) * math.sin(2 * math.pi * k / (4 * teeth))))
               for k in range(4 * teeth)]
        face = bm.faces.new(pts[::-1])
        top = bmesh.ops.extrude_face_region(bm, geom=[face])["geom"]
        bmesh.ops.translate(bm, vec=(0, depth, 0), verts=[v for v in top if isinstance(v, bmesh.types.BMVert)])
        bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
        bmesh.ops.bevel(bm, geom=list(bm.edges), offset=0.25 * depth, offset_type="OFFSET", segments=2,
                        profile=0.5, affect="EDGES", clamp_overlap=True)
    return shape(name, parent, mat, make)


def props(parts, seed):
    """Two clay hands (in the mask, so they take the wind's colour) and the props of PROPS,
    outside the mask so they keep their own colours; handle() shows and moves them."""
    rig, clay = parts["rig"], parts["body"].data.materials[0]
    tone = lambda name, hex, rough=0.5, **k: material(name, lin(hex), rough, sss=0.12, sheen=0.25, seed=seed, **k)
    clear = bpy.data.materials.new("clear")  # what a see-through prop leaves in the mask pass
    clear.node_tree.nodes.remove(clear.node_tree.nodes["Principled BSDF"])
    clear.node_tree.links.new(clear.node_tree.nodes.new("ShaderNodeBsdfTransparent").outputs[0],
                              clear.node_tree.nodes["Material Output"].inputs[0])
    parts["hands"] = [sphere("hand", rig, clay, 1.0) for _ in (-1, 1)]
    kit = {}
    # Magnifier: brass rim, a pale see-through lens and a wooden handle down to the right.
    root = obj("magnifier", None, rig)
    ring("rim", root, tone("brass", "#D39B32", 0.35), 0.27, 0.045)
    glass = material("glass", lin("#CDEFFF"), 0.05, spec=0.8)
    nodes, links = glass.node_tree.nodes, glass.node_tree.links
    see = nodes.new("ShaderNodeMixShader")  # mostly see-through: the big eye shows behind it
    see.inputs[0].default_value = 0.3
    links.new(nodes.new("ShaderNodeBsdfTransparent").outputs[0], see.inputs[1])
    links.new(nodes["Principled BSDF"].outputs[0], see.inputs[2])
    links.new(see.outputs[0], nodes["Material Output"].inputs[0])
    lens_ob = sphere("lens", root, glass)
    lens_ob.scale = (0.26, 0.03, 0.26)
    lens_ob.material_slots[0].link, lens_ob.material_slots[0].material, lens_ob.material_slots[0].link = "OBJECT", clear, "DATA"
    stick_ = rod("handle", root, tone("wood", "#7A4A2B", 0.6), 0.055, 0.045, 0.34)
    stick_.matrix_basis = Matrix.Translation((0.3 * math.sin(math.radians(35)), 0, -0.3 * math.cos(math.radians(35)))) @ \
        Matrix.Rotation(math.radians(180 - 35), 4, "Y")
    kit["searching"] = root
    # Book: red covers, cream pages with a few lines of text, and one page that turns.
    root = obj("book", None, rig)
    cover, paper, ink = tone("cover", "#C9473C"), tone("paper", "#F6EEDD", 0.7), tone("ink", "#6F6680", 0.6)
    for side in (-1, 1):
        block("cover", root, cover, (0.46, 0.56, 0.035), (side * 0.235, 0, -0.02))
        block("pages", root, paper, (0.42, 0.5, 0.06), (side * 0.215, 0, 0.02), soft=0.5)
        for row in range(4):
            block("text", root, ink, (0.3 - 0.06 * (row == 3), 0.022, 0.006), (side * 0.215 - 0.03 * (row == 3), 0.14 - 0.09 * row, 0.052), 1)
    page = bpy.data.meshes.new("page")  # a strip of quads whose vertices handle() bends every frame
    page.from_pydata([(k, y, 0) for k in range(11) for y in (-1, 1)], [], [(2 * k, 2 * k + 2, 2 * k + 3, 2 * k + 1) for k in range(10)])
    page.shade_smooth()
    parts["page"] = obj("page", page, root, paper)
    kit["reading"] = root
    # Writing pad and a chunky yellow pencil.
    root = obj("pad", None, rig)
    block("board", root, tone("kraft", "#B8875A", 0.7), (0.58, 0.48, 0.035), (0, 0, -0.02))
    block("sheet", root, tone("paper2", "#FAF5EA", 0.7), (0.52, 0.42, 0.03), (0, 0, 0.01), soft=0.5)
    line, written = tone("lines", "#9DB8D9", 0.6), tone("written", "#3B3550", 0.5)
    for row in range(4):
        block("line", root, line, (0.44, 0.014, 0.004), (0, 0.13 - 0.08 * row, 0.027), 1)
    for x, row, length in ((-0.13, 0, 0.14), (0.06, 0, 0.18), (-0.15, 1, 0.1), (0.0, 1, 0.12)):
        block("word", root, written, (length, 0.022, 0.006), (x + length / 2 - 0.05, 0.145 - 0.08 * row, 0.03), 1)
    pencil = obj("pencil", None, rig)
    rod("graphite", pencil, tone("lead", "#3A3A40", 0.4), 0.0, 0.022, 0.05, 0.0)
    rod("wood", pencil, tone("shaved", "#E8C28E", 0.6), 0.022, 0.07, 0.1, 0.05)
    rod("body", pencil, tone("yellow", "#F2B632", 0.45), 0.07, 0.07, 0.36, 0.15, sides=6)
    rod("ferrule", pencil, tone("metal", "#B9BCC4", 0.3), 0.074, 0.074, 0.05, 0.51)
    rod("eraser", pencil, tone("eraser", "#F08FA0", 0.6), 0.068, 0.06, 0.07, 0.56)
    kit["writing"] = (root, pencil)
    # Laptop on the ground: light grey, dark keys, its screen glowing on the wind's face.
    root = obj("laptop", None, None)
    shell = tone("shell", "#454B58", 0.4)
    block("base", root, shell, (0.8, 0.5, 0.05), (0, 0, 0.03))
    block("keys", root, tone("keyboard", "#3A3D46", 0.6), (0.64, 0.26, 0.012), (0, 0.04, 0.058), 1)
    lid = obj("lid", None, root)
    lid.location = (0, -0.24, 0.05)
    block("lid", lid, shell, (0.8, 0.035, 0.46), (0, 0, 0.23))
    parts["screen"] = material("screen", lin("#A8D8FF"), 0.3, emit=5.0)
    block("screen", lid, parts["screen"], (0.7, 0.006, 0.37), (0, 0.019, 0.24), 1)
    logo = sphere("logo", lid, material("logo", lin("#CFE9FF"), 0.3, emit=3.0))
    logo.location, logo.scale = (0, -0.02, 0.26), (0.06, 0.008, 0.06)
    shade = obj("laptop shadow", bpy.data.meshes.new("laptop shadow"), root, glow_material("laptop shadow", (0, 0, 0, 1), 0.45), cut=clear)
    shade.data.from_pydata([(-1, -1, 0), (1, -1, 0), (1, 1, 0), (-1, 1, 0)], [], [(0, 1, 2, 3)])
    shade.location, shade.scale = (0, 0, 0.002), (0.55, 0.4, 1)
    parts["lid"] = lid
    root.scale = (1.3,) * 3
    kit["working"] = root
    # Two meshed cogs over the head.
    root = obj("gears", None, rig)
    big = gear("cog", root, tone("ochre", "#EE9A12", 0.4), 8, 0.25, 0.09)
    small = gear("cog", root, tone("teal", "#1FA38F", 0.4), 6, 0.17, 0.09)
    for cog, r in ((big, 0.07), (small, 0.05)):
        hub = sphere("hub", cog, tone("hub", "#5A4636", 0.5))
        hub.location, hub.scale = (0, -0.05, 0), (r, 0.03, r)
    parts["cogs"] = (big, small)
    kit["thinking"] = root
    # Party popper in pink and cream stripes, and the confetti it pops.
    root = obj("popper", None, rig)
    stripes = (tone("pink", "#FF4F7B", 0.5), tone("lemon", "#FFD23F", 0.5))
    for k in range(4):
        rod("stripe", root, stripes[k % 2], 0.035 + 0.025 * k, 0.06 + 0.025 * k, 0.085, 0.085 * k, sides=14)
    rod("rim", root, tone("gold", "#E9B949", 0.35), 0.135, 0.14, 0.02, 0.34, sides=14)
    colours = [tone("c" + h, h, 0.45) for h in ("#FF5C8A", "#FFD23F", "#2EC4B6", "#7B5CFF", "#FF8A3D")]
    parts["confetti"] = [block("confetti", None, colours[k % 5], (0.15, 0.09, 0.02), soft=0.4) for k in range(16)]
    kit["done"] = root
    # Big enough to read on a 48 px face; the cogs fly over the head, where there is less room.
    for name in ("searching", "reading", "writing"):
        for ob in kit[name] if isinstance(kit[name], tuple) else (kit[name],):
            ob.scale = (PROP_SCALE,) * 3
    kit["thinking"].scale = (1.45,) * 3
    parts["kit"] = kit


def front(parts, x, z, out=0.07):
    """A point just in front of the body (and of the minairó's beard) at (x, z), in rig space."""
    hit = parts["tree"].ray_cast(Vector((x, -3, z)), Vector((0, 1, 0)))[0]
    y = (hit.y if hit else -0.4) - out - (0.16 if "beard" in parts and abs(x) < 0.55 and z < parts["eyes"] else 0)
    return Vector((x, y, z))


def handle(parts, mood, i, n):
    """Shows the prop of the mood, moves it and puts the hands on it; every other prop hides."""
    t = i / n if MOODS[mood][1] else i / (n - 1)
    kit, hands, eyes = parts["kit"], parts["hands"], parts["eyes"]
    for name, root in kit.items():
        for ob in (root if isinstance(root, tuple) else (root,)):
            for o in [ob] + list(ob.children_recursive):
                o.hide_render = name != mood
    for piece in parts["confetti"]:
        piece.hide_render = True
    for hand in hands:
        hand.hide_render = mood not in PROPS
        hand.scale, hand.rotation_euler = (0.19, 0.15, 0.16), (0, 0, 0)
    if mood not in PROPS:
        return
    to_rig = parts["rig"].matrix_basis.inverted()
    if mood == "searching":
        x, peek, low = lens(t)
        root = kit[mood]
        root.location = (x, -1.02 - 0.08 * peek, 0.6 + (eyes + 0.08 - 0.6) * peek - 0.14 * low)
        root.rotation_euler = (math.radians(8 * low), math.radians(-12 * math.sin(2 * math.pi * t)), 0)
        grip = Vector((math.sin(math.radians(35)), 0, -math.cos(math.radians(35)))) * 0.5
        hands[1].location = root.matrix_basis @ grip + Vector((0, -0.05, 0))
        hands[1].rotation_euler = (0, math.radians(-35), 0)
        hands[0].location = front(parts, -0.68, 0.42 + 0.03 * math.sin(2 * math.pi * t))
    elif mood == "reading":
        turned = leaf(t)
        root = kit[mood]
        root.location, root.rotation_euler = (0, -1.02, 0.36), (math.radians(48), 0, 0)
        # The turning page: from the spine, bent back at its edge while it is in the air.
        a, pts, page = math.pi * turned, [(0.0, 0.058)], parts["page"]
        for k in range(10):
            bend = a - 0.9 * math.sin(a) * (k / 10)
            pts.append((pts[-1][0] + 0.042 * math.cos(bend), pts[-1][1] + 0.042 * math.sin(bend)))
        page.hide_render = turned in (0.0, 1.0)
        for k, (px, pz) in enumerate(pts):
            page.data.vertices[2 * k].co, page.data.vertices[2 * k + 1].co = (px, -0.24, pz), (px, 0.24, pz)
        page.data.update()
        edge = Vector((pts[-1][0], -0.06, pts[-1][1] + 0.05))
        rest = Vector((0.45, -0.06, 0.02))
        grab = hold(t, 0.52, 0.86, 0.06)
        hands[1].location = root.matrix_basis @ rest.lerp(edge, grab)
        hands[0].location = root.matrix_basis @ Vector((-0.45, -0.04, 0.02))
    elif mood == "writing":
        pad, pencil = kit[mood]
        pad.location, pad.rotation_euler = (-0.08, -1.02, 0.34), (math.radians(48), 0, math.radians(6))
        x, y, lift = scribble(t)
        tip = Vector((x, y - 0.02, 0.03 + lift))
        axis = Vector((0.5 + 0.15 * math.sin(4 * math.pi * t), 0.35, 1.0)).normalized()
        place = pad.matrix_basis
        pencil.matrix_basis = Matrix.Translation(place @ tip) @ Matrix.Scale(1.1, 4) @ \
            (place.to_3x3() @ axis.to_track_quat("Z", "Y").to_matrix()).to_4x4()
        hands[1].location = place @ (tip + axis * 0.22) + Vector((0, -0.08, 0))
        hands[0].location = place @ Vector((-0.3, -0.05, 0.0))
    elif mood == "working":
        root = kit[mood]
        root.location, root.rotation_euler = (0.0, -1.1, 0), (0, 0, math.radians(-12))
        parts["lid"].rotation_euler = (math.radians(-10), 0, 0)
        parts["screen"].node_tree.nodes["Principled BSDF"].inputs["Emission Strength"].default_value = 5.0 + 6 * (i in (9, 10))
        for side, hand, up in zip((-1, 1), hands, TYPING[i]):
            key = Vector((side * 0.16 + 0.03 * ((i * (2 + side)) % 3 - 1), 0.05, 0.1 + up))
            hand.location = to_rig @ (root.matrix_basis @ key)
            down = up == 0.0
            hand.scale = (0.22 if down else 0.19, 0.15, 0.12 if down else 0.16)
    elif mood == "thinking":
        turned, u = ratchet(t)
        root = kit[mood]
        push = hold(u, 0.18, 0.45, 0.12)
        # Over the head on the left, or beside it for the minairó, whose barretina fills the top.
        side = "beard" in parts
        root.location = Vector((-0.94, -0.6, 1.42) if side else (-0.8, -0.5, max(eyes + 1.0, 1.75))) + Vector((0, 0, 0.03 * bump(u, 0.4, 0.15)))
        big, small = parts["cogs"]
        mesh = math.radians(80 if side else 20)
        small.location = Vector((math.cos(mesh), 0, math.sin(mesh))) * 0.37
        big.rotation_euler = (0, math.radians(45 * turned / 2), 0)
        small.rotation_euler = (0, math.radians(-60 * turned / 2 + 30), 0)
        rest = front(parts, -0.72, 0.62)
        hands[0].location = rest.lerp(root.location + Vector((0.05, -0.05, -0.3)), push)
        hands[1].location = front(parts, 0.14 + 0.03 * math.sin(4 * math.pi * t), eyes - 0.62)
        hands[1].rotation_euler = (0, math.radians(30), 0)
    elif mood == "done":
        root = kit[mood]
        root.matrix_basis = popper(i)
        hands[1].location = root.matrix_basis @ Vector((0, -0.07, 0.04))
        cheer = hold(i / 19, 0.25, 0.6, 0.12)
        hands[0].location = front(parts, -0.72, 0.5).lerp(Vector((-0.98, -0.55, 1.3)), cheer)
        confetti(parts, i, n)


def popper(i):
    """The popper in the right hand, in rig space: held low and wound back, thrust up on the
    hop, a recoil when it pops (frame 6), then a wobble as it comes down a little."""
    lift = ease((i - 4) / 3) - 0.2 * ease((i - 9) / 8)
    back = 25 * ease(i / 4) * (1 - ease((i - 4) / 2)) - 22 * ease((i - 4) / 2)
    wobble = 6 * math.sin(i) * ease((i - 8) / 2) * math.exp(-0.3 * max(i - 8, 0))
    return Matrix.LocRotScale(Vector((1.0, -1.0, 0.4 - 0.1 * ease(i / 4) + 0.5 * lift)),
                              Euler((0, math.radians(back + wobble), 0)), Vector((1, 1, 1 - 0.15 * (i == 7))) * 1.7)


def confetti(parts, i, n):
    """Pops out of the popper on frame 6 and flutters down: some land on the wind's head,
    the rest fall out of the picture before the last frame."""
    pop = 6
    if i < pop:
        return
    start = rig_matrix(pose("done", pop, n)) @ popper(pop) @ Vector((0, 0, 0.36))
    rng = np.random.default_rng(142)
    tree, body = parts["tree"], rig_matrix(pose("done", i, n))
    for k, piece in enumerate(parts["confetti"]):
        spin = rng.uniform(-9, 9, 3)
        height, sway = rng.uniform(0.45, 0.9), rng.uniform(-1, 1)
        if k < 4:  # stuck on the head, facing out of it
            x, slope = ((-0.5, 0.35), (-0.12, 0.75), (0.2, 0.5), (0.5, 0.9))[k]
            hit, normal, _, _ = tree.ray_cast(Vector((x, -3.5 * slope, 3.5)), Vector((0, slope, -1)).normalized())
            land = 12 + k
            end = body @ (hit + normal * 0.01)
            flat = ((body.to_3x3() @ normal).to_track_quat("Z", "Y") @ Euler((0, 0, spin[2])).to_quaternion()).to_euler()
        else:
            land = int(rng.integers(15, 19))
            end = Vector((rng.uniform(-1.3, 1.3), rng.uniform(-1.3, -0.9), -0.9))
            flat = Euler(spin * 2)
        tau = min((i - pop) / (land - pop), 1.0)
        out = 1 - (1 - tau) ** 2
        where = start.lerp(end, out)
        where.z = start.z + (end.z - start.z) * tau ** 1.4 + height * math.sin(math.pi * min(tau ** 0.6, 1.0))
        where.x += 0.08 * sway * math.sin(3 * math.pi * tau) * (1 - tau)
        piece.location = where
        piece.rotation_euler = flat if tau >= 1 else Euler(spin * tau)
        piece.hide_render = False


def render(sc, path, mask):
    """mask: None for the clay, "mask" for where the wind's colour goes, "cap" for the barretina's."""
    sc.cycles.samples, sc.cycles.use_denoising = (16, False) if mask else (160, False)
    if mask:
        bpy.data.materials["mask"].node_tree.nodes["Attribute"].attribute_name = mask
    for ob in bpy.data.objects:
        for slot in ob.material_slots:
            slot.link = "OBJECT" if mask else "DATA"
    bpy.data.objects["shadow"].hide_render = bool(mask)
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
        size = SIZES.get(silhouette, SIZE)
        sc = scene(size)
        mask_material()
        parts = character(silhouette, seed + 1)
        for mood in moods:
            n = MOODS[mood][0]
            sheets = {None: [], "mask": [], "cap": []}
            for mask in (None, "mask") + (("cap",) if silhouette in CAPS else ()):
                for i in range(n):
                    apply(parts, pose(mood, i, n) | follow(mood, i, n))
                    handle(parts, mood, i, n)
                    render(sc, tmp / "frame.png", mask)
                    sheets[mask].append(pixels(tmp / "frame.png"))
            colour = np.concatenate(sheets[None], axis=1)
            # WebP keeps alpha lossless: 32 levels look the same and halve what it costs.
            # The fur's fringe is noisy anyway: 8 levels and a lower quality keep it in budget.
            levels, quality = (7, FUR_QUALITY) if "fur" in parts else (31, QUALITY)
            colour[..., 3] = np.round(colour[..., 3] * levels) / levels
            save(colour, OUT / silhouette / f"{mood}.webp", quality)
            for mask in ("mask", "cap"):
                if not sheets[mask]:
                    continue
                # A mask keeps only coverage, at half size: constant white compresses to
                # almost nothing. The largest of each 2x2 so the tint reaches the very edge.
                half = np.concatenate(sheets[mask], axis=1)[..., 3].reshape(size // 2, 2, -1, 2).max((1, 3))
                white = np.ones(half.shape + (4,), dtype=np.float32)
                white[..., 3] = np.round(half * levels) / levels if "fur" in parts else half
                save(white, OUT / silhouette / f"{mood}.{mask}.webp", 100)


def manifest():
    data = {"size": SIZE, "fps": FPS, "tint": "multiply",
            "files": "<silhouette>/<mood>.webp", "mask": "<silhouette>/<mood>.mask.webp", "maskSize": SIZE // 2,
            "cap": "<silhouette>/<mood>.cap.webp", "caps": list(CAPS),
            "sizes": {name: [size, size // 2] for name, size in SIZES.items()},
            "silhouettes": {name: family for name, (family, *_) in SILHOUETTES.items()},
            "moods": {mood: {"frames": n, "loop": loop} | ({"prop": PROPS[mood][0], "still": PROPS[mood][1]} if mood in PROPS else {})
                      for mood, (n, loop) in MOODS.items()}}
    (OUT / "manifest.json").write_text(json.dumps(data) + "\n", encoding="utf-8")


def hex_rgb(hex):
    return tuple(int(hex[i:i + 2], 16) / 255 for i in (1, 3, 5))


# Six minairons, a body colour and the barretina winds.js picks for it (task 120).
MINAIRONS = tuple((hex_rgb(body), hex_rgb(cap)) for body, cap in (
    ("#A8CDE2", "#C9342C"), ("#EFA68F", "#6E2F6B"), ("#A8CDE2", "#2E6B3F"), ("#A8CDE2", "#C8962E"),
    ("#EFA68F", "#26406E"), ("#BFCFA4", "#7D1F33")))


def preview(folder, silhouettes=tuple(SILHOUETTES), motion=False, title=None,
            tints=(((0.36, 0.55, 0.94), hex_rgb("#C9342C")), ((0.95, 0.65, 0.25), hex_rgb("#6E2F6B")))):
    """Contact sheets at the largest frame size among the silhouettes and at 40 px: each
    silhouette in each wind and barretina colour with a few frames per mood, or with motion
    every frame of one silhouette, one mood per row. Smaller frames are scaled up to fit."""
    rows, cell = [], max(SIZES.get(name, SIZE) for name in silhouettes)
    for silhouette in silhouettes:
        size = SIZES.get(silhouette, SIZE)
        for tint, cap in tints:
            cells = []
            for mood, (n, _) in MOODS.items():
                if not (OUT / silhouette / f"{mood}.webp").exists():
                    continue
                colour = pixels(OUT / silhouette / f"{mood}.webp")
                masks = [(pixels(OUT / silhouette / f"{mood}.mask.webp"), tint)]
                if silhouette in CAPS:
                    masks.append((pixels(OUT / silhouette / f"{mood}.cap.webp"), cap))
                for i in range(n) if motion else (0, n // 3, 2 * n // 3):
                    c = colour[:, i * size:(i + 1) * size]
                    rgb = c[..., :3]
                    for mask, paint in masks:
                        m = mask.repeat(2, 0).repeat(2, 1)[:, i * size:(i + 1) * size, 3:]
                        rgb = rgb * (1 - m + m * np.array(paint, dtype=np.float32))
                    up = cell // size
                    cells.append(np.concatenate([rgb, c[..., 3:]], axis=2).repeat(up, 0).repeat(up, 1))
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
        for size in (cell, 40):
            img.scale(sheet.shape[1] * size // cell, sheet.shape[0] * size // cell)
            img.save(filepath=str(Path(folder) / f"{title or ('motion-' + silhouettes[0] if motion else 'contact')}-{size}-{name}.png"))
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
            preview(args.preview, (silhouette,), True, tints=(((0.36, 0.55, 0.94), hex_rgb("#C9342C")),))
        preview(args.preview, ("minairo",), title="minairons", tints=MINAIRONS)


main()
