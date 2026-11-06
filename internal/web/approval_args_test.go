package web

import "testing"

// La targeta d'aprovació no ensenya l'aposta interna del model («bet»), i la
// resta d'arguments arriben igual.
func TestArgsPerAprovarTreuLaposta(t *testing.T) {
	if got := argsPerAprovar(`{"command":"go test ./...","bet":95}`); got != `{"command":"go test ./..."}` {
		t.Fatalf("args: %s", got)
	}
	for _, in := range []string{`{"command":"ls"}`, `no és json`, ``} {
		if got := argsPerAprovar(in); got != in {
			t.Fatalf("sense bet han de quedar igual: %q → %q", in, got)
		}
	}
}
