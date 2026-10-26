package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"gregal/internal/llm"
	"gregal/internal/session"
)

// TestSessionsListMarksMissingFolders: una conversa d'una carpeta que ja no
// existeix surt marcada, perquè la barra lateral l'aparti en lloc de fallar.
func TestSessionsListMarksMissingFolders(t *testing.T) {
	s, _ := companionTestServer(t)
	gone := filepath.Join(t.TempDir(), "esborrada")
	for name, ws := range map[string]string{"viva": s.cwd, "perduda": gone} {
		if _, err := session.SaveSession(s.sessDir(), session.Session{Name: name, Role: "code", Workspace: ws, Convo: []llm.Message{{Role: "user", Content: "hola"}}}); err != nil {
			t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	s.handleSessions(rec, httptest.NewRequest(http.MethodGet, "/api/sessions", nil))
	var rows []struct {
		Name    string
		Missing bool
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range rows {
		got[r.Name] = r.Missing
	}
	if !got["perduda"] || got["viva"] {
		t.Fatalf("marques malament: %+v", rows)
	}
}

// TestForgetWorkspaceRefusesExistingFolder: oblidar només val per a carpetes
// que ja no hi són; una carpeta viva no es treu de la llista per error.
func TestForgetWorkspaceRefusesExistingFolder(t *testing.T) {
	s, _ := companionTestServer(t)
	rec := httptest.NewRecorder()
	s.handleWorkspaces(rec, httptest.NewRequest(http.MethodDelete, "/api/workspaces?path="+t.TempDir(), nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("volia 409, tinc %d", rec.Code)
	}
}
