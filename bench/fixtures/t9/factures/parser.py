"""Lectura d'imports escrits a la manera catalana: "1.234,56 €"."""
from decimal import Decimal


def llegeix_import(text):
    """Converteix un import en text a Decimal.

    Accepta el símbol d'euro i espais opcionals: "12,50", "12,50 €",
    "-3,00", "1.234,56 €".
    """
    t = text.replace("€", "").strip()
    # separador decimal: coma -> punt
    t = t.replace(",", ".")
    return Decimal(t)
