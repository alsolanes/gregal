"""Utilitats (la funció slugify ha de viure a helpers.py)."""


def slugify(text):
    out = []
    for ch in text.lower():
        if ch.isalnum():
            out.append(ch)
        elif ch in (" ", "-", "_"):
            out.append("-")
    return "".join(out).strip("-")
