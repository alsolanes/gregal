from decimal import Decimal, ROUND_HALF_UP

IVA = Decimal("0.21")
CENTIM = Decimal("0.01")


def arrodoneix(x):
    return x.quantize(CENTIM, rounding=ROUND_HALF_UP)


def total_factura(linies):
    """Torna (base, iva, total), cadascun arrodonit al cèntim."""
    base = arrodoneix(sum((l.import_net() for l in linies), Decimal("0")))
    iva = arrodoneix(base * IVA)
    return base, iva, base + iva
