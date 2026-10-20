package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
)

func companionTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	s := goalTestServer(t)
	home := t.TempDir()
	t.Setenv("GREGAL_MEMORY", filepath.Join(home, ".config", "gregal", "memory.md"))
	return s, home
}

func TestCompanionAcceptWritesUserMemory(t *testing.T) {
	s, home := companionTestServer(t)

	fact := "Fa servir una e-bike per anar a la feina cada dia"
	// Simulem que el model ha proposat aquest fet (queueCompanionSuggestion
	// en un run real); aquí el parem directament al buidor compartit.
	companionSuggests.Store(s, companionSuggestion{State: "ready", Facts: []string{fact}})

	rec := postJSON(t, s.handleCompanionMemory, `{"action":"accept","fact":"`+fact+`"}`)
	if rec.Code != 200 {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		OK    bool     `json:"ok"`
		Total int      `json:"total"`
		Notes []string `json:"notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.Total != 1 || len(resp.Notes) != 1 || !strings.Contains(resp.Notes[0], "e-bike") {
		t.Fatalf("accept hauria de desar 1 nota: %+v", resp)
	}
	// La memòria és la d'usuari real de config (GREGAL_MEMORY al test home).
	raw, err := os.ReadFile(filepath.Join(home, ".config", "gregal", "memory.md"))
	if err != nil || !strings.Contains(string(raw), "e-bike") {
		t.Fatalf("la nota no ha arribat al fitxer: %v %q", err, string(raw))
	}
	// La proposta queda consumida (no es pot acceptar dues vegades).
	rec2 := postJSON(t, s.handleCompanionMemory, `{"action":"accept","fact":"`+fact+`"}`)
	if rec2.Code != 404 {
		t.Fatalf("re-accept hauria de ser 404: %d", rec2.Code)
	}
}

func TestCompanionForgetAndList(t *testing.T) {
	s, home := companionTestServer(t)
	t.Setenv("GREGAL_MEMORY", filepath.Join(home, ".config", "gregal", "memory.md"))
	if err := config.Remember("Té un gat que es diu Mix"); err != nil {
		t.Fatal(err)
	}

	// GET llista la memòria (audit «què saps de mi?»).
	req := httptest.NewRequest(http.MethodGet, "/api/companion/memory", nil)
	rec := httptest.NewRecorder()
	s.handleCompanionMemory(rec, req)
	var got struct {
		Notes []string `json:"notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Notes) != 1 || !strings.Contains(got.Notes[0], "Mix") {
		t.Fatalf("GET hauria de llistar la memòria: %v", got.Notes)
	}

	// POST forget l'esborra.
	rec2 := postJSON(t, s.handleCompanionMemory, `{"action":"forget","term":"Mix"}`)
	if rec2.Code != 200 {
		t.Fatalf("forget: %d %s", rec2.Code, rec2.Body.String())
	}
	var resp struct {
		Forgotten int `json:"forgotten"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp)
	if resp.Forgotten != 1 {
		t.Fatalf("esperava forgotten=1, got %d", resp.Forgotten)
	}
}

func TestCompanionAcceptRejectsUnproposedFact(t *testing.T) {
	s, _ := companionTestServer(t)
	rec := postJSON(t, s.handleCompanionMemory, `{"action":"accept","fact":"mai proposat"}`)
	if rec.Code != 404 {
		t.Fatalf("accept d'una proposta inexistent: %d", rec.Code)
	}
}

func TestCompanionShouldExtractFromRealTurn(t *testing.T) {
	// Un torn amb substància personal mereix extracció; una salutació no.
	personal := []llm.Message{
		{Role: "user", Content: strings.Repeat("Estic refent la meva app de música en Kotlin i em surt de meravella, hi passo les tardes. ", 2)},
		{Role: "assistant", Content: "Quina bona notícia!"},
	}
	if !companion.ShouldExtract(personal) {
		t.Fatal("torn personal hauria de merèixer extracció")
	}
	greeting := []llm.Message{
		{Role: "user", Content: "hola"},
		{Role: "assistant", Content: "ep!"},
	}
	if companion.ShouldExtract(greeting) {
		t.Fatal("una salutació no merèix extracció")
	}
	// El filtre de secrets aplica al parse: contrasenyes mai.
	facts := companion.ParseExtract(`{"facts":["La seva contrasenya és hunter2","Li agrada el café sol"]}`, nil)
	if len(facts) != 1 || strings.Contains(facts[0], "hunter2") {
		t.Fatalf("els secrets no passen el filtre: %v", facts)
	}
}
