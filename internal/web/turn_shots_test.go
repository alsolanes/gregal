package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/llm"
)

// Les captures del torn arriben al fil (history) amb l'adreça per demanar-les.
func TestHistoryCarriesTheTurnShots(t *testing.T) {
	rows := petHistoryJSON([]llm.Message{{Role: "assistant", Content: "Fet", Shots: []llm.MessageShot{{ID: "0123456789abcdef0123", Title: "Girona", URL: "https://ca.wikipedia.org/wiki/Girona"}}}})
	shots, _ := rows[0]["shots"].([]map[string]any)
	if len(shots) != 1 || shots[0]["shotUrl"] != "/api/ordinador/accions/captura?id=0123456789abcdef0123" || shots[0]["title"] != "Girona" {
		t.Fatalf("shots: %#v", rows[0]["shots"])
	}
	if _, ok := petHistoryJSON([]llm.Message{{Role: "assistant", Content: "x"}})[0]["shots"]; ok {
		t.Fatal("sense captures no hi ha d'haver el camp")
	}
}

// Les captures es desen a part de la reproducció i les velles s'esborren.
func TestSaveTurnShotPrunesOldOnes(t *testing.T) {
	dir := t.TempDir()
	c := &computerStore{dir: dir}
	first := c.saveTurnShot("local", []byte{0xff, 0xd8})
	if first == "" {
		t.Fatal("no s'ha desat")
	}
	if b, ok := c.shot("local", first); !ok || len(b) != 2 {
		t.Fatal("no es pot servir")
	}
	if got := c.recent("local", 0, 0); len(got) != 0 {
		t.Fatalf("no és cap acció de la reproducció: %d", len(got))
	}
	for i := 0; i < turnShotsKeep; i++ {
		c.saveTurnShot("local", []byte{1})
	}
	if _, ok := c.shot("local", first); ok {
		t.Fatal("la més vella s'havia d'esborrar")
	}
	b, _ := os.ReadFile(filepath.Join(c.ownerDir("local"), "torns.txt"))
	if n := len(strings.Fields(string(b))); n != turnShotsKeep {
		t.Fatalf("índex: %d", n)
	}
}
