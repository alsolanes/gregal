---
title: "Vents colla: qui ha fet la feina pot aplicar la revisió"
status: todo
depends_on: []
---

## Why

En una colla (rotllana), el primer vent fa la web (pot escriure), un altre la revisa i el primer contesta la revisió. Aquesta resposta corre en només lectura (`readOnlyWindCtx` dins de `peerSay`, `internal/web/winds_stream.go` ~1036), així que el vent diu «estic en mode inspecció, `edit` i `publish` retornen TOOL BLOCKED» i el missatge final de la colla és que no ho pot fer. Reproduït amb un servidor de prova i dos vents (Garbi, Llevant): «Vull una web d'una pàgina amb la recepta de la truita…». Passa igual a la web i a Android: és el servidor.

## Acceptance criteria

- [ ] Quan l'autor contesta la revisió, té els mateixos permisos que tenia el seu primer torn en aquell missatge: si aquell torn podia escriure (`windDocsCtx`), la resposta també; si era de només lectura (veu en paral·lel, veu després de la primera, intenció `review`), segueix sent-ho. Test Go amb un proveïdor fals: l'autor fa `write` a la resposta de la revisió i el fitxer queda escrit.
- [ ] El revisor segueix en només lectura. Test Go: un `write` del revisor queda denegat.
- [ ] El text de la resposta de l'autor (`rotllanaAuthorBrief`) li diu que, si accepta la revisió i pot fer el canvi, el faci en lloc de descriure'l. Cobert pel primer test o per un test del brief.

## Out of scope

Les veus en paral·lel i les veus després de la primera (continuen en només lectura). El pipeline de feina de la colla (`crewChat`).

## Notes

- `internal/web/winds_stream.go` (`peerSay`, `rotllanaWantsReview`, el bucle de veus ~880-900 on `si > 0 || intent == "review"` decideix només lectura), `internal/web/winds_rotllanes_review.go` (`rotllanaAuthorBrief`), `internal/web/winds_permissions.go` (`readOnlyWindCtx`, `windDocsCtx`).
- `docs/api-contract.md` diu a la fila de `/api/notes` que «a rotllana voice after the first and a review» són de només lectura: si la regla canvia per a la resposta de l'autor, actualitza el contracte i `docs/modes.md` si en parla.
