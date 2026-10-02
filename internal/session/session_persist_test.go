package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gregal/internal/llm"
)

func TestSaveSessionWithWorkspaceAndCompacted(t *testing.T) {
	dir := t.TempDir()
	sess := Session{
		Name:      "sessio-test",
		Role:      "code",
		Workspace: "/path/to/project",
		Pinned:    true,
		Compacted: "Resum de la conversa anterior: hem implementat X i Y.",
		Convo: []llm.Message{
			{Role: "user", Content: "afegeix test"},
			{Role: "assistant", Content: "fet"},
		},
	}

	path, err := SaveSession(dir, sess)
	if err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	if filepath.Ext(path) != ".json" {
		t.Fatalf("extensió incorrecta: %s", path)
	}

	loaded, err := Load(dir, "sessio-test")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Workspace != sess.Workspace {
		t.Errorf("Workspace = %q, volíem %q", loaded.Workspace, sess.Workspace)
	}
	if loaded.Compacted != sess.Compacted {
		t.Errorf("Compacted = %q, volíem %q", loaded.Compacted, sess.Compacted)
	}
	if !loaded.Pinned {
		t.Error("Pinned no s'ha conservat")
	}
	if loaded.Role != "code" || len(loaded.Convo) != 2 {
		t.Errorf("Role/Convo incoherents: %+v", loaded)
	}

	infos, err := List(dir)
	if err != nil || len(infos) != 1 {
		t.Fatalf("List error: %v, len=%d", err, len(infos))
	}
	if infos[0].Workspace != sess.Workspace {
		t.Errorf("Info.Workspace = %q, volíem %q", infos[0].Workspace, sess.Workspace)
	}
	if !infos[0].Pinned {
		t.Error("Info.Pinned no s'ha conservat")
	}
}

func TestSaveSessionConservaArgumentsDeToolInvalids(t *testing.T) {
	dir := t.TempDir()
	msg := llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "c1", Type: "function"}}}
	msg.ToolCalls[0].Function.Arguments = `{"command":"python - <<'PY`
	if _, err := SaveSession(dir, Session{Name: "tool-invalid", Convo: []llm.Message{msg}}); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(dir, "tool-invalid")
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Convo[0].ToolCalls[0].Function.Arguments; got != msg.ToolCalls[0].Function.Arguments {
		t.Fatalf("arguments=%q, volia conservar %q", got, msg.ToolCalls[0].Function.Arguments)
	}
}

func TestRapidSavesDoNotCollide(t *testing.T) {
	dir := t.TempDir()
	const n = 5
	names := make(map[string]bool)

	for i := 0; i < n; i++ {
		path, err := SaveSession(dir, Session{
			Convo: []llm.Message{{Role: "user", Content: fmt.Sprintf("msg %d", i)}},
		})
		if err != nil {
			t.Fatalf("SaveSession %d: %v", i, err)
		}
		base := filepath.Base(path)
		if names[base] {
			t.Fatalf("Col·lisió de nom de sessió: %s", base)
		}
		names[base] = true
		time.Sleep(2 * time.Millisecond)
	}

	infos, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != n {
		t.Fatalf("esperava %d sessions diferents, en tinc %d", n, len(infos))
	}
}

func TestLegacySessionWithoutWorkspaceOrCompacted(t *testing.T) {
	dir := t.TempDir()
	legacyJSON := `{
		"version": 1,
		"name": "legacy",
		"title": "Antiga sessió",
		"saved_at": "2026-09-10T10:00:00Z",
		"role": "chat",
		"convo": [{"role":"user","content":"hola"}]
	}`
	if err := os.WriteFile(filepath.Join(dir, "legacy.json"), []byte(legacyJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	sess, err := Load(dir, "legacy")
	if err != nil {
		t.Fatalf("Load sessió antiga: %v", err)
	}
	if sess.Workspace != "" || sess.Compacted != "" {
		t.Errorf("camps no inicialitzats haurien de ser buits, tenim w=%q c=%q", sess.Workspace, sess.Compacted)
	}
	if sess.Title != "Antiga sessió" || len(sess.Convo) != 1 {
		t.Errorf("contingut antic mal carregat: %+v", sess)
	}
}

func TestModelOverridesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := map[string]string{"chat": "p1/model-a", "code": "p2/model-b"}
	if _, err := SaveSession(dir, Session{Name: "models", ModelOverrides: want}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir, "models")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ModelOverrides) != len(want) {
		t.Fatalf("overrides=%v, want %v", got.ModelOverrides, want)
	}
	for role, model := range want {
		if got.ModelOverrides[role] != model {
			t.Errorf("%s override=%q, want %q", role, got.ModelOverrides[role], model)
		}
	}
}
