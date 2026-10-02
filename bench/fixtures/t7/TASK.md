# T7 — catàleg amb filtre en viu (HTML + CSS + JS)

Aquest directori té un web petit (`index.html` + `styles.css`, paleta en
variables CSS) i un `app.js` buit ja enllaçat amb `defer`.

## Feina
Afegeix un catàleg de 4 fruites amb filtre en viu:
- `app.js`: array de 4 productes `{nom, preu, categoria}` (dues
  categories diferents, noms en català) i funció que els pinta com a
  targetes dins de `#cataleg` (cada targeta amb classe `targeta` i el nom
  i el preu visibles). Camp `#filtre` (text) que filtra pel nom mentre
  s'escriu (event `input`) i `<select id="categoria">` que filtra per
  categoria (event `change`); els dos filtres combinen (nom I categoria).
- `index.html`: `<input id="filtre">`, `<select id="categoria">` amb les
  dues categories més una opció buida "Totes", i `<section id="cataleg">`.
- `styles.css`: classe `.targeta` que usa les variables (`var(--...)`) i
  una `@media` que reordena les targetes en una columna a mòbil.

## Verificació
`verify.py` ho comprova estàticament (sense navegador). Acaba només quan
surt `TOT BÉ`.
