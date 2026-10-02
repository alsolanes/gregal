# T4 — endpoint nou en una app Go existent

Aquest directori té una mini-app Go (`server.go`, `go.mod`) amb un endpoint
de salut (`GET /api/ping`) i tests (`server_test.go`, verds).

## Feina
Afegeix `POST /api/eco`:
- Rep JSON `{"text": "..."}` i torna JSON `{"eco": "TEXT EN MAJÚSCULES"}`.
- Mètode que no sigui POST → 405.
- Cos que no sigui JSON vàlid → 400.

## Verificació
`eco_test.go` comprova els tres casos. Acaba només quan `go test ./...`
surt verd. Pots afegir el codi on vulguis (server.go o fitxer nou del
mateix paquet) però no toquis els tests.
