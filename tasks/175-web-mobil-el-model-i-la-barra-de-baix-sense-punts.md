---
title: "Web mòbil: el model i la barra de baix sense punts suspensius"
status: todo
depends_on: []
---

## Why

A Vents en mòbil (390-490 px) el botó del model del compositor es queda en «GPT-…» o «G…», que no diu res, i a la barra de navegació de baix «Playground» surt com «Playgro…».

## Acceptance criteria

- [ ] Quan el nom del model no hi cap sencer al compositor, `button.vw-model` mostra només la icona (amb el nom com a nom accessible i `title`), mai un nom tallat de 1-4 lletres. Test node de CSS/DOM.
- [ ] A 375 px, cap etiqueta de la barra de baix (`MOBILE_NAV`) acaba amb punts suspensius, en anglès i en català. Si cal una etiqueta més curta en anglès, només per a la barra. Test node.
- [ ] Comprovat al navegador a 375 i 490 px.

## Out of scope

L'ordre o el nombre de pestanyes de la barra. Android.

## Notes

- `internal/web/app/winds-workspace.css:60-63` (`button.vw-model`, `.vw-model-name`), `internal/web/app/winds-workspace.js:52` (`MOBILE_NAV`).
