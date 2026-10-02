# T9 — totals de factura incorrectes

El paquet `factures/` llegeix línies de factura amb imports escrits a la
manera catalana ("1.234,56 €") i en calcula base, IVA i total.

## Problema
Amb factures petites els totals surten bé, però amb imports de més de mil
euros el càlcul peta o surt malament. `test_informe.py` ho reprodueix.

## Feina
Troba la causa i arregla-la on toca (no al test). Verifica amb
`python -m unittest` abans d'acabar.
