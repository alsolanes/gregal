---
title: "Web mòbil: el model i la barra de baix sense punts suspensius"
status: done
depends_on: []
---

## Why

A Vents en mòbil (390-490 px) el botó del model del compositor es queda en «GPT-…» o «G…», que no diu res, i a la barra de navegació de baix «Playground» surt com «Playgro…».

## Acceptance criteria

- [x] Quan el nom del model no hi cap sencer al compositor, `button.vw-model` mostra només la icona (amb el nom com a nom accessible i `title`), mai un nom tallat de 1-4 lletres. Test node de CSS/DOM.
- [x] A 375 px, cap etiqueta de la barra de baix (`MOBILE_NAV`) acaba amb punts suspensius, en anglès i en català. Si cal una etiqueta més curta en anglès, només per a la barra. Test node.
- [x] Comprovat al navegador a 375 i 490 px.

## Out of scope

L'ordre o el nombre de pestanyes de la barra. Android.

## Notes

- `internal/web/app/winds-workspace.css:60-63` (`button.vw-model`, `.vw-model-name`), `internal/web/app/winds-workspace.js:52` (`MOBILE_NAV`).
- Model: CSS pur. El botó és un flex que fa salt de línia, d'una línia d'alt i amb overflow:hidden; si el nom no hi cap sencer salta a la segona línia i queda només la icona. Si el nom hi cap però la fletxa no, el que desapareix és la fletxa (p. ex. «GPT-4.1 Mini» a 375 px).
- La barra de baix diu «Play» en anglès (la barra lateral i la capçalera continuen dient «Playground»); el test del 132 que esperava «Playground» a la barra de baix s'ha actualitzat.
- Comprovat amb Playwright a 375 i 490 px (EN i CA), amb «GPT-4.1 Mini» i amb un nom llarg (DeepSeek V4.1 Flash Preview).
- Follow-up: quan el nom no hi cap, el botó conserva l'amplada que tenia (fins al 42-46 % del peu) i queda un buit després de la icona; encongir-lo a la icona demanaria mesurar amb JS.
