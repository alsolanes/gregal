package flow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	casos := map[string]string{
		"Revisa el PR":           "revisa-el-pr",
		"  Prepara  lliurament ": "prepara-lliurament",
		"Migració mòdul 3":       "migraci-m-dul-3",
		"":                       "flux",
		"///":                    "flux",
	}
	for in, vol := range casos {
		if got := Slug(in); got != vol {
			t.Errorf("Slug(%q) = %q, volia %q", in, got, vol)
		}
	}
	if got := Slug(strings.Repeat("a", 200)); len(got) != 60 {
		t.Fatalf("un nom llarg s'ha de retallar: %d", len(got))
	}
}

func TestDesaCarregaLlistaEsborra(t *testing.T) {
	proj := t.TempDir()
	f := lineal()
	f.Desc = "llegeix i revisa"
	path, err := Desa(proj, f)
	if err != nil {
		t.Fatal(err)
	}
	// Viu al projecte, no a la carpeta de l'usuari: el procediment és del
	// repositori i ha de viatjar-hi.
	if !strings.HasPrefix(path, filepath.Join(proj, ".gregal", "flows")) {
		t.Fatalf("desat fora del projecte: %s", path)
	}

	tornat, err := Carrega(proj, "revisió")
	if err != nil {
		t.Fatal(err)
	}
	if tornat.Name != f.Name || len(tornat.Nodes) != 2 || len(tornat.Edges) != 1 {
		t.Fatalf("no s'ha recuperat igual: %+v", tornat)
	}
	if tornat.Nodes[0].Args != `{"path":"calc.go"}` {
		t.Fatalf("args perduts: %q", tornat.Nodes[0].Args)
	}

	llista, err := Llista(proj)
	if err != nil {
		t.Fatal(err)
	}
	if len(llista) != 1 || llista[0].Nom != "revisió" || llista[0].Passos != 2 || llista[0].Desc != "llegeix i revisa" {
		t.Fatalf("llista=%+v", llista)
	}

	if err := Esborra(proj, "revisió"); err != nil {
		t.Fatal(err)
	}
	if _, err := Carrega(proj, "revisió"); err == nil {
		t.Fatal("després d'esborrar no s'hauria de poder carregar")
	}
}

// Un graf trencat no s'ha de poder desar: val més dir-ho ara que trobar-se'l
// a mig executar demà.
func TestNoDesaGrafTrencat(t *testing.T) {
	proj := t.TempDir()
	dolent := &Flow{Name: "mal", Nodes: []Node{{ID: "a", Kind: KindAgent}}}
	if _, err := Desa(proj, dolent); err == nil {
		t.Fatal("un graf invàlid no s'ha de desar")
	}
	if fitxers, _ := filepath.Glob(filepath.Join(DirFor(proj), "*.json")); len(fitxers) != 0 {
		t.Fatalf("no hi hauria d'haver res escrit: %v", fitxers)
	}
}

// Un fitxer il·legible se salta; no tomba el llistat.
func TestLlistaSaltaElsTrencats(t *testing.T) {
	proj := t.TempDir()
	if _, err := Desa(proj, lineal()); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(DirFor(proj), "trencat.json"), []byte("{no json"), 0o644)
	llista, err := Llista(proj)
	if err != nil {
		t.Fatal(err)
	}
	if len(llista) != 1 {
		t.Fatalf("volia 1 flux llegible, tinc %d", len(llista))
	}
}

// Sense carpeta de fluxos, la llista és buida i no és cap error.
func TestLlistaSenseCarpeta(t *testing.T) {
	llista, err := Llista(t.TempDir())
	if err != nil || len(llista) != 0 {
		t.Fatalf("llista=%v err=%v", llista, err)
	}
}
