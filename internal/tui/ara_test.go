package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func modelTreballant(t *testing.T, ample int) Model {
	t.Helper()
	m := modelGolden(t, ample, 40)
	m.treuBenvinguda()
	m.lastPrompt = "Refactoritza el client HTTP perquè el timeout es pugui configurar per provider i afegeix-hi tests"
	m.push(userLine(m.lastPrompt))
	m.busy, m.agentActive = true, true
	m.turnStart = time.Now().Add(-3*time.Minute - 12*time.Second)
	m.intencio = primeraFrase("Ara llegeixo **internal/llm/client.go** per veure on es crea el `http.Client`. Després hi afegiré el camp.")
	m.pushToolCall("read", `{"path":"internal/llm/client.go"}`)
	m.status = "llegint"
	m.refresh()
	return m
}

// TestPanellAraDiuQueFa: amb feina en marxa, el panell lateral diu què has
// demanat, què diu el model que fa, quina eina corre i quant fa.
func TestPanellAraDiuQueFa(t *testing.T) {
	m := modelTreballant(t, 140)
	v := ansi.Strip(m.View())
	if os.Getenv("GREGAL_DUMP") != "" {
		os.WriteFile(os.Getenv("GREGAL_DUMP"), []byte(v), 0o644)
	}
	for _, want := range []string{"ara 3m", "Refactoritza el client", "Ara llegeixo", "http.Client.", "▸ read"} {
		if !strings.Contains(strings.Join(strings.Fields(v), " "), want) {
			t.Fatalf("el panell no diu %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "Després hi afegiré") {
		t.Fatal("només la primera frase")
	}
}

// TestPeuDiuQueFaSenseColumna: sense columna lateral, la frase va al peu.
func TestPeuDiuQueFaSenseColumna(t *testing.T) {
	m := modelTreballant(t, 100)
	if m.columnaActiva() {
		t.Skip("a 100 columnes hi ha columna")
	}
	if v := ansi.Strip(m.View()); !strings.Contains(v, "Ara llegeixo") {
		t.Fatalf("el peu havia de dir què fa:\n%s", v)
	}
}

func TestIntencioVeDelTextAmbEines(t *testing.T) {
	m := modelTreballant(t, 140)
	m.intencio = ""
	if f := primeraFrase("Executo els tests del paquet llm per veure què falla.\n\nDetall"); f != "Executo els tests del paquet llm per veure què falla." {
		t.Fatalf("primera frase: %q", f)
	}
	if primeraFrase("  ") != "" {
		t.Fatal("buit")
	}
	if got := talla(strings.Repeat("paraula ", 30), 20, 2); len(got) != 2 || !strings.HasSuffix(got[1], "…") {
		t.Fatalf("talla: %q", got)
	}
}
