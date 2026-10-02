package session

import (
	"fmt"
	"os"
	"testing"

	"gregal/internal/llm"
)

func sampleConvo() []llm.Message {
	return []llm.Message{
		{Role: "user", Content: "hola"},
		{Role: "assistant", Content: "què tal"},
	}
}

func TestRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path, err := Save(dir, "prova u", "chat", sampleConvo())
	if err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir, "prova u")
	if err != nil {
		t.Fatal(err)
	}
	if s.Role != "chat" || len(s.Convo) != 2 || s.Convo[1].Content != "què tal" {
		t.Fatalf("sessió: %+v", s)
	}
	if s.Name == "" || path == "" {
		t.Fatal("nom o ruta buits")
	}
}

func TestAutoName(t *testing.T) {
	dir := t.TempDir()
	if _, err := Save(dir, "", "code", nil); err != nil {
		t.Fatal(err)
	}
	infos, err := List(dir)
	if err != nil || len(infos) != 1 {
		t.Fatalf("list: %v %v", infos, err)
	}
	if infos[0].Msgs != 0 {
		t.Fatalf("msgs=%d", infos[0].Msgs)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	if _, err := Save(dir, "per-esborrar", "chat", sampleConvo()); err != nil {
		t.Fatal(err)
	}
	if err := Delete(dir, "per-esborrar"); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, "per-esborrar"); err == nil {
		t.Fatal("encara hi és")
	}
	if err := Delete(dir, "per-esborrar"); err == nil {
		t.Fatal("esborrar dues vegades hauria de fallar")
	}
	if err := Delete(dir, "no-existeix"); err == nil {
		t.Fatal("hauria de fallar amb sessió inexistent")
	}
}

func TestLoadMissing(t *testing.T) {
	if _, err := Load(t.TempDir(), "no-existeix"); err == nil {
		t.Fatal("esperava error amb sessió inexistent")
	}
}

func TestSkipCorrupt(t *testing.T) {
	dir := t.TempDir()
	if _, err := Save(dir, "bona", "chat", sampleConvo()); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, dir+"/trencada.json", "{no json")
	infos, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || infos[0].Name != "bona" {
		t.Fatalf("list ha de saltar la corrupta: %+v", infos)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryEmpty(t *testing.T) {
	setHomeTest(t, t.TempDir())
	if got := LoadHistory(); len(got) != 0 {
		t.Fatalf("history=%v", got)
	}
}

func TestHistoryRoundtrip(t *testing.T) {
	setHomeTest(t, t.TempDir())
	in := []string{"/model", "hola", "  ", "/roles"}
	if err := SaveHistory(in); err != nil {
		t.Fatal(err)
	}
	got := LoadHistory()
	if len(got) != 3 || got[0] != "/model" || got[2] != "/roles" {
		t.Fatalf("history=%v", got)
	}
}

func TestHistoryCap(t *testing.T) {
	setHomeTest(t, t.TempDir())
	var in []string
	for i := 0; i < MaxHistory+50; i++ {
		in = append(in, fmt.Sprintf("cmd-%d", i))
	}
	if err := SaveHistory(in); err != nil {
		t.Fatal(err)
	}
	got := LoadHistory()
	if len(got) != MaxHistory || got[0] != "cmd-50" {
		t.Fatalf("len=%d first=%q", len(got), got[0])
	}
}
