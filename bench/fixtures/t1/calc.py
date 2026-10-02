def preu_total(quantitat, preu_unitari, descompte_pct):
    """Torna el total amb descompte aplicat."""
    subtotal = quantitat * preu_unitari
    descompte = subtotal * (descompte_pct / 100)
    return subtotal - descompte + descompte_pct
