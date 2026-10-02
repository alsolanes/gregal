# T6 — blog API: crear, llegir i esborrar posts

Aquest directori té una mini blog API en Go (`server.go`, `go.mod`) amb
`GET /api/posts` (llista, verd a `server_test.go`).

## Feina
Amplia l'API sense trencar el que hi ha:
- `POST /api/posts`: rep JSON `{"titol": "...", "cos": "..."}` (tots dos
  obligatoris i no buits) i torna 201 amb el post creat `{"id": N, ...}`
  amb id auto-incremental. JSON invàlid o camps buits/absents → 400.
- `GET /api/posts/{id}`: torna el post (200) o 404 si no existeix.
- `DELETE /api/posts/{id}`: esborra (204) o 404 si no existeix.
- Mètode no suportat a una ruta existent → 405.

## Verificació
`posts_test.go` comprova tots els casos (inclòs que el creat es pot
llegir i que l'esborrat desapareix). Acaba només quan `go test ./...`
surt verd. Pots afegir el codi on vulguis (mateix paquet) però no toquis
els tests.
