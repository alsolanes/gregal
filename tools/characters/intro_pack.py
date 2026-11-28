"""Pack the rendered intro frames (intro.py) for the Android app:

    python tools/characters/intro_pack.py output/intro/frames

writes android/app/src/main/res/raw/intro_folk.webp (animated, plays once in
FolkIntro.kt) and res/drawable-nodpi/intro_folk_end.png (its last frame: the
still without motion, before Android 9, and after it has played).
"""
import glob
import os
import sys

from PIL import Image

src = sys.argv[1] if len(sys.argv) > 1 else "output/intro/frames"
root = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "android", "app", "src", "main", "res")
frames = [Image.open(f).convert("RGBA") for f in sorted(glob.glob(os.path.join(src, "*.png")))]
if not frames:
    sys.exit("no frames in " + src)
size = int(sys.argv[2]) if len(sys.argv) > 2 else 360
frames = [f.resize((size, size), Image.LANCZOS) for f in frames]
os.makedirs(os.path.join(root, "raw"), exist_ok=True)
os.makedirs(os.path.join(root, "drawable-nodpi"), exist_ok=True)
out = os.path.join(root, "raw", "intro_folk.webp")
# 24 fps; the last frame holds a moment longer before the form comes in.
durations = [42] * (len(frames) - 1) + [400]
frames[0].save(out, save_all=True, append_images=frames[1:], duration=durations, loop=1, quality=72, method=6,
               lossless=False, allow_mixed=True)
frames[-1].save(os.path.join(root, "drawable-nodpi", "intro_folk_end.png"), optimize=True)
print(out, os.path.getsize(out) // 1024, "KB,", len(frames), "frames at", size, "px")
