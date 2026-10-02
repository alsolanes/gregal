# T5 — secció nova en un web existent

Aquest directori té un web petit (`index.html` + `styles.css`) amb paleta
en variables CSS (`--fons`, `--tinta`, `--accent`) i una media query.

## Feina
Afegeix una secció `#contacte` dins de `main`, després de `#inici`, amb:
- Un `h2` amb el text "Contacte".
- Un `form` amb tres camps, tots obligatoris (`required`):
  - `nom` (text), `email` (email) i `missatge` (textarea).
- Cada camp amb el seu `label` (`for` = `id` del camp).
- Botó d'enviar amb el text "Envia".
- Si afegeixes estils a `styles.css`, fes servir `var(--…)` (res de colors
  hardcoded fora de `:root`).

## Verificació
`verify.py` ho comprova tot. Acaba només quan `python3 verify.py`
imprimeix TOT BÉ.
