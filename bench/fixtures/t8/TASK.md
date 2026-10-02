# T8 — retirar estoc

Aquest paquet Go (`inventari.go`) guarda unitats per article.

## Feina
Implementa `func (i *Inventari) Retira(nom string, n int) error`:
- Resta `n` unitats de l'article.
- Error si `n <= 0`, si l'article no existeix o si no n'hi ha prou (i en
  aquest cas l'estoc no canvia).
- Quan un article arriba a zero, desapareix de l'inventari.

`retira_test.go` ja té els tests. Verifica amb `go test ./...` abans
d'acabar.
