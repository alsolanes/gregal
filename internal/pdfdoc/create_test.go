package pdfdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const informeMD = `# Resum del trimestre

Les vendes han crescut un **12 %** respecte del trimestre anterior, sobretot a *Girona* i a l'àrea de ` + "`e-commerce`" + `.

## Punts clau

- Clients nous: 340
- Cistella mitjana: 54,20 €
  - Online: 61 €
  - Botiga: 47 €
1. Revisar preus
2. Ampliar l'estoc d'estiu

> Cal confirmar les dades de juny abans de tancar.

| Regió | Vendes | Variació |
|---|---|---|
| Girona | 120.000 € | +18 % |
| Lleida | 80.500 € | +4 % |

` + "```" + `
total = sum(vendes)
` + "```" + `

---

Contacte: [web](https://exemple.cat) 😀
\pagebreak
# Annex

Segona pàgina.`

func TestCreateFromMarkdown(t *testing.T) {
	out := filepath.Join(t.TempDir(), "informe.pdf")
	if err := Create(bg, out, CreateOpts{Title: "Informe d'àrea", Author: "Gregal", Content: informeMD}); err != nil {
		t.Fatal(err)
	}
	inf, err := ReadInfo(bg, out)
	if err != nil {
		t.Fatal(err)
	}
	if inf.Pages != 2 || inf.Title != "Informe d'àrea" || inf.Author != "Gregal" || !strings.Contains(inf.Sizes[0], "A4") {
		t.Fatalf("info: %+v", inf)
	}
	pages := textOf(t, out)
	all := strings.Join(pages, "\n")
	for _, w := range []string{"Informe d'àrea", "Resum del trimestre", "12 %", "Girona", "54,20 €", "Revisar preus",
		"Cal confirmar", "Regió", "120.000 €", "total = sum(vendes)", "https://exemple.cat", "1 / 2"} {
		if !strings.Contains(all, w) {
			t.Errorf("falta %q a:\n%s", w, all)
		}
	}
	if !strings.Contains(pages[1], "Annex") || strings.Contains(pages[0], "Annex") {
		t.Errorf("el salt de pàgina no ha funcionat: %q", pages)
	}
	if strings.Contains(all, "**") || strings.Contains(all, "|---") {
		t.Errorf("han quedat marques de markdown: %s", all)
	}
	if os.Getenv("ZZ_KEEP") != "" {
		raw, _ := os.ReadFile(out)
		os.WriteFile(os.Getenv("ZZ_KEEP"), raw, 0o644)
	}
}

func TestCreateLongTextFlowsOverPages(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 400; i++ {
		b.WriteString("Una frase prou llarga per omplir línies i pàgines senceres de text. ")
	}
	b.WriteString("\n\nhttps://exemple.cat/" + strings.Repeat("molt-llarg-", 40))
	out := filepath.Join(t.TempDir(), "llarg.pdf")
	if err := Create(bg, out, CreateOpts{Content: b.String(), Page: "letter", Landscape: true}); err != nil {
		t.Fatal(err)
	}
	inf, _ := ReadInfo(bg, out)
	if inf.Pages < 3 || !strings.Contains(inf.Sizes[0], "Letter apaïsat") {
		t.Fatalf("info: %+v", inf)
	}
	if err := Create(bg, out, CreateOpts{}); err == nil {
		t.Fatal("sense contingut hauria de fallar")
	}
}
