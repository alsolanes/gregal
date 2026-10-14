package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/llm"
	"gregal/internal/repomap"
)

func projecteAmbCodi(t *testing.T) string {
	t.Helper()
	repomap.Reset()
	dir := t.TempDir()
	for nom, cos := range map[string]string{
		"main.go":        "package main\n\nfunc main() {}\n",
		"calc/suma.go":   "package calc\n\n// Suma suma.\nfunc Suma(a, b int) int { return a + b }\n\ntype Calculadora struct{}\n\nfunc (c *Calculadora) Resta(a, b int) int { return a - b }\n",
		"web/app.js":     "export function pintaPantalla() {}\nexport class Panell {}\n",
		"docs/README.md": "# docs\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(nom))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(cos), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestMapaProjectePortaSimbols(t *testing.T) {
	dir := projecteAmbCodi(t)
	tasca := "la funció Suma torna malament els negatius"
	c := MapaProjecte(dir, tasca, nil)
	if !strings.Contains(c, "Symbols (") || !strings.Contains(c, "calc/suma.go: Suma(f)") {
		t.Fatalf("falta la secció de símbols:\n%s", c)
	}
	if !strings.Contains(c, "Calculadora.Resta(m)") || !strings.Contains(c, "web/app.js:") {
		t.Fatalf("falten símbols:\n%s", c)
	}
	// La secció va dins del bloc marcat: SenseMapa ho treu tot.
	missatge := AmbMapaProjecte(dir, tasca, nil)
	if got := SenseMapa(missatge); got != tasca {
		t.Fatalf("SenseMapa ha de deixar només la tasca: %q", got)
	}
}

func TestMapaSimbolsNoEsRepeteixSenseMotiu(t *testing.T) {
	dir := projecteAmbCodi(t)
	c := MapaProjecte(dir, "mira-ho", nil)
	hist := []llm.Message{{Role: "user", Content: "mira-ho\n\n" + c}, {Role: "assistant", Content: "fet"}}
	// Una altra tasca que no anomena cap símbol: el llistat ja hi és.
	if again := MapaProjecte(dir, "i ara una altra cosa", hist); again != "" {
		t.Fatalf("no s'havia de repetir:\n%s", again)
	}
	// Anomena literalment un símbol compost: tornen els símbols, sense la llista.
	tasca := "arregla Calculadora.Resta"
	again := MapaProjecte(dir, tasca, hist)
	if !strings.Contains(again, "symbols]") || !strings.Contains(again, "Calculadora.Resta(m)") {
		t.Fatalf("haurien de tornar els símbols:\n%s", again)
	}
	if strings.Contains(again, "Files (") {
		t.Fatalf("la llista no s'ha de repetir:\n%s", again)
	}
	if got := SenseMapa(tasca + "\n\n" + again); got != tasca {
		t.Fatalf("SenseMapa també treu el bloc de símbols: %q", got)
	}
	// El bloc només de símbols no compta com a llistat: si és l'únic que
	// queda a l'historial, el mapa sencer torna.
	nomesSimbols := []llm.Message{{Role: "user", Content: tasca + "\n\n" + again}}
	if full := MapaProjecte(dir, "una altra", nomesSimbols); !strings.Contains(full, "Files (") {
		t.Fatalf("el llistat havia de tornar:\n%s", full)
	}
}

func TestMapaSimbolsEsPotApagar(t *testing.T) {
	dir := projecteAmbCodi(t)
	t.Setenv("GREGAL_REPOMAP_TOKENS", "0")
	if c := MapaProjecte(dir, "Suma", nil); strings.Contains(c, "Symbols (") || !strings.Contains(c, "Files (") {
		t.Fatalf("amb 0 tokens no hi ha secció de símbols:\n%s", c)
	}
}

func TestMapaSenseCodiNoPortaSimbols(t *testing.T) {
	dir := projecteProva(t, false)
	if c := MapaProjecte(dir, "Suma", nil); strings.Contains(c, "Symbols (") {
		t.Fatalf("sense declaracions no hi ha secció:\n%s", c)
	}
}
