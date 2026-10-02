from calc import preu_total
assert preu_total(2, 100, 10) == 180, "2x100 amb 10% ha de ser 180"
assert preu_total(1, 50, 0) == 50, "sense descompte"
print("TOT BÉ")
