package telegram

import (
	"strings"
	"testing"
)

func TestMarkdownBasic(t *testing.T) {
	out := md2html("**negreta** i *cursiva* i `codi`")
	for _, want := range []string{"<b>negreta</b>", "<i>cursiva</i>", "<code>codi</code>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("falta %q a %q", want, out)
		}
	}
}

func TestCodiNoEsTransforma(t *testing.T) {
	out := md2html("```go\nif a < b && **no** { }\n```")
	if !strings.Contains(out, "<pre>") {
		t.Fatalf("falta <pre> a %q", out)
	}
	if strings.Contains(out, "<b>no</b>") {
		t.Fatalf("el codi no s'hauria de transformar: %q", out)
	}
	if !strings.Contains(out, "a &lt; b") {
		t.Fatalf("el codi ha de quedar escapat: %q", out)
	}
}

func TestEnllacITitular(t *testing.T) {
	out := md2html("# Títol\nMira [això](https://exemple.test/x)")
	if !strings.Contains(out, "<b>Títol</b>") {
		t.Fatalf("titular: %q", out)
	}
	if !strings.Contains(out, `<a href="https://exemple.test/x">això</a>`) {
		t.Fatalf("enllaç: %q", out)
	}
}

func TestLlistesICites(t *testing.T) {
	out := md2html("- un\n- dos\n> cita")
	if !strings.Contains(out, "• un") || !strings.Contains(out, "• dos") {
		t.Fatalf("llistes: %q", out)
	}
	if !strings.Contains(out, "<blockquote>cita</blockquote>") {
		t.Fatalf("cita: %q", out)
	}
}

func TestHTMLDelModelSEscapa(t *testing.T) {
	out := md2html("<script>alert(1)</script> **ok**")
	if strings.Contains(out, "<script>") {
		t.Fatalf("HTML cru del model: %q", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") || !strings.Contains(out, "<b>ok</b>") {
		t.Fatalf("escapament: %q", out)
	}
}

func TestTruncatNoTrenquaEtiquetes(t *testing.T) {
	s := "<b>" + strings.Repeat("a", 100) + "</b> cua"
	out := truncHTML(s, 20)
	if strings.Contains(out, "<b") && !strings.Contains(out, ">") {
		t.Fatalf("etiqueta partida: %q", out)
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("hauria de contenir …: %q", out)
	}
	if strings.Count(out, "<b>") != strings.Count(out, "</b>") {
		t.Fatalf("etiquetes desequilibrades: %q", out)
	}
	// Les entitats no es parteixen.
	s2 := "abc &amp; " + strings.Repeat("x", 50)
	out2 := truncHTML(s2, 6)
	if strings.Contains(out2, "&am") && !strings.Contains(out2, "&amp;") {
		t.Fatalf("entitat partida: %q", out2)
	}
}

// TestCitesMultilinia: les cites consecutives van en un sol bloc (una caixa),
// no una per línia. La d'una sola línia queda igual que abans.
func TestCitesMultilinia(t *testing.T) {
	out := md2html("> primera\n> segona\ntext")
	if n := strings.Count(out, "<blockquote>"); n != 1 {
		t.Fatalf("volia 1 bloc, %d: %q", n, out)
	}
	if !strings.Contains(out, "<blockquote>primera\nsegona</blockquote>") {
		t.Fatalf("bloc fusionat: %q", out)
	}
}

// TestTaulaEnPre: els blocs de taula van en monoespaiat perquè alineïn; una
// línia sola amb pipes no és taula i passa tal qual.
func TestTaulaEnPre(t *testing.T) {
	out := md2html("| nom | valor |\n|-----|-------|\n| a   | **1**   |")
	if !strings.Contains(out, "<pre>| nom | valor |") {
		t.Fatalf("taula sense <pre>: %q", out)
	}
	if !strings.Contains(out, "<b>1</b>") {
		t.Fatalf("la cel·la conserva el format: %q", out)
	}
	if out2 := md2html("a | b"); strings.Contains(out2, "<pre>") {
		t.Fatalf("fals positiu de taula: %q", out2)
	}
}

// TestLlistaNumerada: el número mana (`1)` → `1.`).
func TestLlistaNumerada(t *testing.T) {
	out := md2html("1) primer\n2) segon")
	if !strings.Contains(out, "1. primer") || !strings.Contains(out, "2. segon") {
		t.Fatalf("numerada: %q", out)
	}
}

// TestCercaSenseTancar: a mitja resposta (streaming) la cerca oberta es pinta
// com a codi en comptes de mostrar els ``` crus.
func TestCercaSenseTancar(t *testing.T) {
	out := renderAnswer("mira:\n```go\nfmt.Println(1)")
	if strings.Contains(out, "```") {
		t.Fatalf("cerca crua: %q", out)
	}
	if !strings.Contains(out, "<pre>") {
		t.Fatalf("sense bloc de codi: %q", out)
	}
}
