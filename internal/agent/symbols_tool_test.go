package agent

import (
	"strings"
	"testing"

	"gregal/internal/tools"
)

func TestEinaSymbolsCerca(t *testing.T) {
	dir := projecteAmbCodi(t)
	out, _, err := ExecIn("", dir, "symbols", `{"query":"resta"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "calc/suma.go:8 method Calculadora.Resta — func (c *Calculadora) Resta(a, b int) int") {
		t.Fatalf("resultat: %q", out)
	}
	out, _, _ = ExecIn("", dir, "symbols", `{"kind":"class","file":"web/*.js"}`)
	if !strings.Contains(out, "web/app.js:2 class Panell") || strings.Contains(out, "Suma") {
		t.Fatalf("filtre kind+file: %q", out)
	}
	out, _, _ = ExecIn("", dir, "symbols", `{"query":"^S","regex":true}`)
	if !strings.Contains(out, "func Suma") {
		t.Fatalf("regex: %q", out)
	}
	out, _, _ = ExecIn("", dir, "symbols", `{"query":"noexisteix"}`)
	if !strings.HasPrefix(out, "no symbols found") {
		t.Fatalf("buit: %q", out)
	}
	if _, _, err := ExecIn("", dir, "symbols", `{"kind":"potato"}`); err == nil {
		t.Fatal("kind desconegut ha de fallar")
	}
}

func TestEinaSymbolsEsDeLectura(t *testing.T) {
	if !tools.EsNativa("symbols") || !tools.Configurable("symbols") {
		t.Fatal("symbols ha de ser al registre")
	}
	trobat := false
	for _, s := range Specs() {
		if s.Name == "symbols" {
			trobat = true
		}
	}
	if !trobat {
		t.Fatal("falta l'spec de symbols")
	}
	for _, mode := range []string{ModeCode, ModeInspect, ModeChat, ModeGoal} {
		if d, why := DefaultPolicy().Decide(mode, "symbols", `{"query":"x"}`); d != "allow" {
			t.Fatalf("%s: symbols=%s (%s)", mode, d, why)
		}
	}
	if !ParallelSafe("symbols") || !readOnlyParallel["symbols"] {
		t.Fatal("symbols és lectura pura: paral·lelitzable")
	}
}
