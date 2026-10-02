from utils import slugify, occurrencies
assert slugify("Hola Món! Cafè amb llet") == "hola-mon-cafe-amb-llet", "slugify 1"
assert slugify("  Espais  dobles  ") == "espais-dobles", "slugify 2"
assert occurrencies("el far fa llum, el farol no", "far") == 1, "occurrencies 1"
assert occurrencies("Far far FAR", "far") == 3, "occurrencies 2"
print("TOT BÉ")
