---
title: "Web: les lletres amb cua no queden tallades"
status: done
depends_on: []
---

## Why

A la web les cues de la g, la p i la q es tallen: les pastilles d'ofici (`.vw-tag`: «Writing» es llegeix «Writinq», «Planning») i el títol de benvinguda amb degradat («Good evening», `#heroTitle`), en fosc i en clar, a mòbil i a escriptori.

## Acceptance criteria

- [x] El text de `.vw-tag` té una alçada de línia que deixa sencers els descendents (el `<span>` interior amb `overflow:hidden` ja no talla la g), sense canviar l'alçada de 20 px de la pastilla. Test de CSS a node.
- [x] El títol amb `background-clip:text` (refine.css) pinta sencers els descendents: la caixa de pintura arriba per sota de la línia base. Test de CSS a node.
- [x] Comprovat al navegador (sandbox) a 1280 i a 390 px d'amplada.

## Out of scope

Altres canvis de tipografia o de mides.

## Notes

- `internal/web/app/winds-workspace.css:577-578` (`.vw-tag` amb `line-height:1`), `internal/web/app/refine.css:149`.
- Mesurat: `.vw-tag > span` fa 11.5 px d'alt amb `overflow:hidden`; l'h1 fa 34 px amb `line-height:1.1`.
- Fet: `.vw-tag > span` amb `line-height:1.4` (16.1 px dins la pastilla de 20 px, que no canvia) i `padding-bottom:.12em` a la salutació amb `background-clip:text` de refine.css. Aquest padding fa el títol 4-5.5 px més alt (també el `#hero h1` de Codi, que comparteix la regla).
- Comprovat al sandbox (Playwright, CSS servit des del disc) a 1280 i 390 px, en fosc i en clar: la g de «Good evening» i les cues de les pastilles es veuen senceres.
