package tui

import (
	"strings"
	"testing"
	"time"
)

// El text que el model va escrivint es llegeix mentre arriba: markdown de
// debò, no una cua de 160 caràcters retallada per l'esquerra. Amb la cua,
// un paràgraf feia ballar la línia i no se'n podia llegir res.
func TestTextEnViuEsMarkdown(t *testing.T) {
	m := planTestModel(t)
	viu := nomesText(m.textEnViu("## Pla\n\n1. Llegir el config\n2. Afegir el camp"))
	for _, vol := range []string{"Pla", "Llegir el config", "Afegir el camp"} {
		if !strings.Contains(viu, vol) {
			t.Fatalf("el text en viu ha de dir %q: %q", vol, viu)
		}
	}
	if strings.Contains(viu, "##") {
		t.Fatalf("els títols s'han de pintar, no mostrar-se en cru: %q", viu)
	}
	if strings.Contains(viu, "…") {
		t.Fatalf("el text en viu ja no es retalla: %q", viu)
	}
}

// Es repinta com a molt cada intervalViu: a cada tick de 60 ms seria refer
// el markdown disset vegades per segon.
func TestTextEnViuNoEsRefaACadaTick(t *testing.T) {
	m := planTestModel(t)
	primer := m.textEnViu("hola")
	m.streamShown = primer
	if segon := m.textEnViu("hola que tal"); segon != primer {
		t.Fatal("dins de l'interval s'ha de conservar el que ja hi havia")
	}
	m.streamRender = time.Now().Add(-2 * intervalViu)
	if tercer := m.textEnViu("hola que tal"); tercer == primer {
		t.Fatal("passat l'interval s'ha de repintar")
	}
}

// Mentre el text arriba, el ``` d'obertura pot haver arribat i el de
// tancament no: sense tancar-lo, glamour pinta la resta com si fos codi.
func TestBlocDeCodiObertEsTanca(t *testing.T) {
	if got := tancaBlocs("mira:\n```go\nfunc main() {"); !strings.HasSuffix(got, "\n```") {
		t.Fatalf("el bloc obert s'ha de tancar: %q", got)
	}
	tancat := "mira:\n```go\nfunc main() {}\n```"
	if got := tancaBlocs(tancat); got != tancat {
		t.Fatalf("un bloc tancat no es toca: %q", got)
	}
}

// Un pas que només crida eines no deixa cap «◌ escrivint…» penjat, i les
// files de les eines segueixen apuntant on toca.
func TestPasSenseTextNoDeixaFilaMorta(t *testing.T) {
	m := planTestModel(t)
	m.push("primera")
	m.pushToolCall("read", `{"path":"x.go"}`)
	filaEina := m.timeline[0].linia
	m.push(workRail("◌ escrivint…"))
	m.streamLine = len(m.lines) - 1
	m.esborraLinia(m.streamLine)
	if m.streamLine != -1 {
		t.Fatalf("la línia en viu ja no existeix: %d", m.streamLine)
	}
	if m.timeline[0].linia != filaEina {
		t.Fatalf("la fila de l'eina no s'ha de moure: %d → %d", filaEina, m.timeline[0].linia)
	}
	// I el resultat encara hi escriu a sobre, no en una altra fila.
	m.pushToolResult("read", "llegit x.go\n1\tpackage x", false)
	if !strings.Contains(nomesText(m.lines[filaEina]), "✓") {
		t.Fatalf("el resultat havia d'anar a la fila de la crida: %q", nomesText(m.lines[filaEina]))
	}
}

// El codi dels diffs es pinta amb la paleta de Gregal (chroma), i els
// marcadors + i − queden al marge.
func TestDiffAmbSintaxi(t *testing.T) {
	d := diffBlock("x.go", "package x\n\nconst A = 1\n", "package x\n\nconst A = 2\n", 20)
	if !strings.Contains(d, "\x1b[") {
		t.Fatal("el codi del diff ha de portar color")
	}
	net := nomesText(d)
	if !strings.Contains(net, "− ") || !strings.Contains(net, "+ ") {
		t.Fatalf("calen els marcadors al marge: %q", net)
	}
	for _, l := range strings.Split(net, "\n")[1:] {
		if strings.HasPrefix(l, "│") {
			t.Fatalf("el diff no porta rail propi (xoca amb el de feina): %q", l)
		}
	}
}
