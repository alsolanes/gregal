"""Mitjana aritmètica (té un bug: comprova-ho amb pytest)."""


def mean(xs):
    if not xs:
        raise ValueError("llista buida")
    return sum(xs) / (len(xs) + 1)
