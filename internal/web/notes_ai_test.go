package web

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// Una nota es pot associar a un projecte: es desa a la capçalera, es pot
// filtrar per projecte (o sense) i la llista en dona el recompte.
func TestNotesProjectAssociationAndFilter(t *testing.T) {
	s := rutinaTestServer(t)
	proj := t.TempDir()
	a := notesSave(t, s, map[string]any{"title": "Arquitectura", "body": "capes", "project": proj})
	notesSave(t, s, map[string]any{"title": "Compra", "body": "pa"})
	raw, err := os.ReadFile(s.notePath(a.ID))
	if err != nil || !strings.Contains(string(raw), "project: "+proj) {
		t.Fatalf("project not in header: %s %v", raw, err)
	}
	code, body := notesReq(t, s, http.MethodGet, "/api/notes?project="+proj, nil)
	var got struct {
		Notes    []Note      `json:"notes"`
		Projects []noteCount `json:"projects"`
	}
	if code != 200 || json.Unmarshal(body, &got) != nil || len(got.Notes) != 1 || got.Notes[0].ID != a.ID {
		t.Fatalf("filter by project: %d %s", code, body)
	}
	if len(got.Projects) != 1 || got.Projects[0].Name != proj || got.Projects[0].Count != 1 {
		t.Fatalf("project counts: %+v", got.Projects)
	}
	if l := notesGet(t, s, "?project=-"); len(l.Notes) != 1 || l.Notes[0].Title != "Compra" {
		t.Fatalf("without project: %+v", l.Notes)
	}
	// Treure el projecte la deixa sense.
	empty := ""
	notesSave(t, s, map[string]any{"id": a.ID, "project": &empty})
	if l := notesGet(t, s, "?project=-"); len(l.Notes) != 2 {
		t.Fatalf("unassigned: %+v", l.Notes)
	}
}

// L'IA proposa, no desa: la nota no canvia fins que l'usuari la guarda.
func TestNotesAIProposesWithoutSaving(t *testing.T) {
	s := rutinaTestServer(t)
	srv, _ := fakeChatProvider(t, "x")
	defer srv.Close()
	useProvider(s, srv.URL)
	n := notesSave(t, s, map[string]any{"title": "Idees", "body": "comprar llet\ntrucar marta"})
	var gotAction, gotBody string
	old := noteAI
	noteAI = func(_ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, action, _, _, body string) (string, error) {
		gotAction, gotBody = action, body
		return "- [ ] Comprar llet\n- [ ] Trucar a la Marta", nil
	}
	defer func() { noteAI = old }()
	w := postJSON(t, s.handleNotesAI, `{"id":"`+n.ID+`","action":"tasks"}`)
	var out struct {
		Body    string `json:"body"`
		Changed bool   `json:"changed"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil || !out.Changed || !strings.HasPrefix(out.Body, "- [ ] Comprar") {
		t.Fatalf("ai: %d %s", w.Code, w.Body.String())
	}
	if gotAction != "tasks" || strings.TrimSpace(gotBody) != "comprar llet\ntrucar marta" {
		t.Fatalf("model got %q %q", gotAction, gotBody)
	}
	if l := notesGet(t, s, ""); l.Notes[0].Excerpt == "" || strings.Contains(l.Notes[0].Excerpt, "Comprar") {
		t.Fatalf("note must not be saved by the proposal: %+v", l.Notes[0])
	}
	// El text de l'editor (encara sense desar) mana sobre el desat.
	postJSON(t, s.handleNotesAI, `{"id":"`+n.ID+`","action":"tidy","body":"esborrany nou"}`)
	if gotBody != "esborrany nou" {
		t.Fatalf("editor body ignored: %q", gotBody)
	}
	if w := postJSON(t, s.handleNotesAI, `{"action":"hack"}`); w.Code != 400 {
		t.Fatalf("unknown action: %d", w.Code)
	}
}

func TestCleanNoteAIStripsFences(t *testing.T) {
	for in, want := range map[string]string{
		"```markdown\n# Hola\n- a\n```": "# Hola\n- a",
		"---\nText":                      "Text",
		"  Net  ":                        "Net",
	} {
		if got := agent.CleanNoteAI(in); got != want {
			t.Fatalf("%q → %q, want %q", in, got, want)
		}
	}
}
