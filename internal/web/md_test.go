package web

import (
	"strings"
	"testing"

	"gregal/internal/tema"
)

// El cas que va fer néixer això: una taula del model sortia com un
// paràgraf massís en negreta de quaranta línies.
func TestMarkdownPintaTaules(t *testing.T) {
	src := "| pestanya | files |\n|---|---|\n| Proposal Tracker | 54 |\n| Effort Summary | 12 |\n"
	got := Markdown(src, tema.Fosc())
	for _, vol := range []string{"<table>", "<thead>", "<th>", "<td>", "Proposal Tracker"} {
		if !strings.Contains(got, vol) {
			t.Fatalf("falta %q a la taula:\n%s", vol, got)
		}
	}
	if strings.Contains(got, "|---|") {
		t.Fatalf("la taula s'ha de pintar, no sortir en cru:\n%s", got)
	}
}

// Els blocs de codi porten color (chroma amb la paleta de Gregal).
func TestMarkdownPintaElCodi(t *testing.T) {
	got := Markdown("```go\nfunc main() { return }\n```", tema.Fosc())
	if !strings.Contains(got, "<pre class=\"codeblock lang-go\">") {
		t.Fatalf("falta el bloc amb la seva llengua:\n%s", got)
	}
	if !strings.Contains(got, "color:") {
		t.Fatalf("el codi hauria de portar color:\n%s", got)
	}
	// El color de les paraules clau és el de la paleta, no el de chroma.
	if !strings.Contains(strings.ToLower(got), strings.ToLower(tema.Fosc().Sintaxi.Paraula)) {
		t.Fatalf("les paraules clau han de ser %s:\n%s", tema.Fosc().Sintaxi.Paraula, got)
	}
}

// Una llengua que no coneixem no s'endevina: text escapat i prou.
func TestMarkdownCodiSenseLlenguaConeguda(t *testing.T) {
	got := Markdown("```sortida\nerror: <res>\n```", tema.Fosc())
	if !strings.Contains(got, "&lt;res&gt;") {
		t.Fatalf("el codi s'ha d'escapar:\n%s", got)
	}
}

// El tema clar fa servir els seus colors, no els del fosc.
func TestMarkdownSegueixElTema(t *testing.T) {
	src := "```go\nfunc main() {}\n```"
	fosc := Markdown(src, tema.Fosc())
	clar := Markdown(src, tema.Clar())
	if fosc == clar {
		t.Fatal("cada tema ha de pintar el codi amb els seus colors")
	}
	if !strings.Contains(strings.ToLower(clar), strings.ToLower(tema.Clar().Sintaxi.Paraula)) {
		t.Fatalf("el tema clar ha de fer servir %s:\n%s", tema.Clar().Sintaxi.Paraula, clar)
	}
}

// El model no pot injectar HTML, ni escrivint-lo ni amb un enllaç
// javascript: ni amb un estil que no sigui un color.
func TestMarkdownNoDeixaInjectar(t *testing.T) {
	casos := []string{
		`<script>alert(1)</script>`,
		`<img src=x onerror="alert(1)">`,
		`[clica](javascript:alert(1))`,
		`<div style="position:fixed;top:0">tapa-ho tot</div>`,
		"<iframe src=\"http://mal.example\"></iframe>",
	}
	for _, src := range casos {
		got := Markdown(src, tema.Fosc())
		baix := strings.ToLower(got)
		for _, dolent := range []string{"<script", "onerror", "javascript:", "<iframe", "position:fixed"} {
			if strings.Contains(baix, dolent) {
				t.Fatalf("%q ha passat el sanejat amb %q:\n%s", src, dolent, got)
			}
		}
	}
}

// Les llistes, els títols, el codi en línia i les cites, que és el que
// els models escriuen a totes hores.
func TestMarkdownElQueElsModelsEscriuen(t *testing.T) {
	src := "## Passos\n\n1. Llegir `config.go`\n2. Afegir el camp\n\n- [x] fet\n- [ ] pendent\n\n> compte amb el timeout\n\n~~vell~~"
	got := Markdown(src, tema.Fosc())
	for _, vol := range []string{"<h2", "<ol>", "<code>config.go</code>", "<ul>", "checked", "<blockquote>", "<del>"} {
		if !strings.Contains(got, vol) {
			t.Fatalf("falta %q:\n%s", vol, got)
		}
	}
}

// Els enllaços surten amb rel de seguretat i obrint en una pestanya nova.
func TestMarkdownEnllacosSegurs(t *testing.T) {
	got := Markdown("mira [go.dev](https://go.dev/blog)", tema.Fosc())
	if !strings.Contains(got, `href="https://go.dev/blog"`) {
		t.Fatalf("falta l'enllaç:\n%s", got)
	}
	for _, vol := range []string{"nofollow", `target="_blank"`} {
		if !strings.Contains(got, vol) {
			t.Fatalf("l'enllaç ha de portar %s:\n%s", vol, got)
		}
	}
}
