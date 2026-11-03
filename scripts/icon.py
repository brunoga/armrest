#!/usr/bin/env python3
"""Draws the app's icon, cmd/armrest/Icon.png: a remote lying on a sofa's
armrest, in the app's teal, cream and orange. Needs Pillow.

    python3 scripts/icon.py
"""
from PIL import Image, ImageDraw

SIZE = 512
S = 4  # drawn larger, then scaled down: smooth edges

BG = "#1F5F6B"
ARM = "#2E8491"
ARM_TOP = "#3A97A4"  # the arm's rolled front
CREAM = "#F7F4EE"
ORANGE = "#F4A04B"


def box(x0, y0, x1, y1):
    return [v * S for v in (x0, y0, x1, y1)]


im = Image.new("RGBA", (SIZE * S, SIZE * S), (0, 0, 0, 0))
d = ImageDraw.Draw(im)
d.rounded_rectangle(box(0, 0, SIZE, SIZE), radius=112 * S, fill=BG)

# The armrest: a padded arm, rolled at its front (the left), seen from the
# side.
d.rounded_rectangle(box(100, 250, 452, 380), radius=48 * S, fill=ARM)
d.ellipse(box(60, 238, 200, 378), fill=ARM_TOP)
d.ellipse(box(92, 270, 168, 346), fill=ARM)

# The remote, lying along it: the ring of keys at its head, the keys
# behind.
d.rounded_rectangle(box(160, 166, 440, 250), radius=42 * S, fill=CREAM)
d.ellipse(box(344, 171, 418, 245), fill=ORANGE)
d.ellipse(box(366, 193, 396, 223), fill=CREAM)
for x in (226, 270):
    for y in (190, 226):
        r = 12
        d.ellipse(box(x - r, y - r, x + r, y + r), fill=BG)
im = im.resize((SIZE, SIZE), Image.LANCZOS)
im.save("cmd/armrest/Icon.png")
