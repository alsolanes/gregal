---
title: "Posa ordre no diu sense ús als vents nous"
status: done
depends_on: []
---

## Why

Acabes de crear cinc vents i la pàgina «Els teus vents» ja et diu «5 unused · Tidy up» i et proposa arxivar-los. A més, la pastilla arriba tard i empeny la llista cap avall, i el toc que anava a «Parla» cau en un altre lloc.

## Acceptance criteria

- [x] `GET /api/winds/tidy` no posa a `unused` un vent creat fa menys de 7 dies. Un vent desat abans que existís la data de creació segueix comptant com abans. Test Go.
- [x] El vent guarda la data de creació quan es crea (no quan s'edita), i una edició no la canvia. Test Go.
- [x] A la web, la pastilla de «Posa ordre» no mou les files de vents quan apareix. Test node o comprovació al navegador.
- [x] `docs/api-contract.md` diu la regla dels 7 dies a la fila de `/api/winds/tidy`. Android (Tidy.kt) llegeix el mateix informe i no necessita canvis; si en necessita, digues-ho a Notes.

## Out of scope

Els duplicats i els vents de prova.

## Notes

- `internal/web/winds_tidy.go:187-197`, `internal/companion/winds.go` (`type Wind`), `internal/web/app/winds-workspace.js:770` (`tidyBanner`, que fa `append` al `.vw-heading`).
- `companion.Wind` té `created` (RFC 3339), que posa `WindStore.save` en crear i conserva en editar (ignora el que enviï el client). `Restore` (arxiu, equips) el deixa tal com ve: els vents d'equip restaurats sense data compten com a antics.
- La web reserva un lloc fix (`.vw-tidy-slot`, 38 px) sota el títol de «Tots els vents» i hi posa la pastilla quan arriba; la pastilla és d'una sola línia (amb punts suspensius si no hi cap). Quan no hi ha res a endreçar, aquest lloc queda buit.
- Android (Tidy.kt) només llegeix l'informe; no necessita canvis. `GET /api/winds` no exposa `created` (ningú el fa servir encara); `POST /api/winds` el torna dins `{wind}`.
- No comprovat al navegador: la comprovació és el test node `winds-start.test.cjs` (lloc reservat + alçada CSS).
