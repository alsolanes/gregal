package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/knowledge"
)

func knowledgeTestServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_MEMORY", filepath.Join(t.TempDir(), "memory.md"))
	knowledge.Configure(knowledge.Settings{})
	s := hubTestServer(t)
	s.mu.Lock()
	s.cwd = t.TempDir()
	s.mu.Unlock()
	return s, s.Hub().mux()
}

func kreq(t *testing.T, h http.Handler, method, path, body string) (int, map[string]any) {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestKnowledgeAPILifecycle(t *testing.T) {
	s, h := knowledgeTestServer(t)
	docs := filepath.Join(s.cwd, "docs")
	os.MkdirAll(docs, 0o755)
	os.WriteFile(filepath.Join(docs, "runbook.md"), []byte("# Runbook\nRestart the ingestion worker with the blue button.\n"), 0o644)

	code, c := kreq(t, h, "POST", "/api/knowledge", `{"name":"Ops","description":"Runbooks","scope":"project","path":"docs","include":["*.md"]}`)
	if code != 200 {
		t.Fatalf("create: %d %v", code, c)
	}
	id := c["id"].(string)
	if !strings.HasPrefix(id, "p-") {
		t.Fatalf("id: %s", id)
	}
	// La indexació corre en segon pla: esperem que acabi.
	deadline := time.Now().Add(5 * time.Second)
	var item map[string]any
	for time.Now().Before(deadline) {
		_, item = kreq(t, h, "GET", "/api/knowledge/"+id, "")
		if p, _ := item["progress"].(map[string]any); p != nil && p["running"] == false && p["phase"] == "done" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	docsList, _ := item["docs"].([]any)
	if len(docsList) != 1 {
		t.Fatalf("documents: %v", item)
	}
	code, res := kreq(t, h, "GET", "/api/knowledge/search?q=ingestion+worker", "")
	hits, _ := res["hits"].([]any)
	if code != 200 || len(hits) == 0 || hits[0].(map[string]any)["path"] != "docs/runbook.md" {
		t.Fatalf("cerca: %d %v", code, res)
	}
	// Nota, desactivar, reanomenar.
	if code, _ := kreq(t, h, "POST", "/api/knowledge/"+id+"/notes", `{"title":"Pager","text":"The pager rotation changes on Mondays."}`); code != 200 {
		t.Fatalf("nota: %d", code)
	}
	_, res = kreq(t, h, "POST", "/api/knowledge/search", `{"query":"pager rotation"}`)
	if hits, _ := res["hits"].([]any); len(hits) == 0 {
		t.Fatalf("la nota no es troba: %v", res)
	}
	if code, got := kreq(t, h, "PATCH", "/api/knowledge/"+id, `{"enabled":false,"name":"Operations"}`); code != 200 || got["enabled"] != false || got["name"] != "Operations" {
		t.Fatalf("patch: %d %v", code, got)
	}
	_, res = kreq(t, h, "GET", "/api/knowledge/search?q=pager", "")
	if hits, _ := res["hits"].([]any); len(hits) != 0 {
		t.Fatalf("desactivada encara es cerca: %v", res)
	}
	// Llista: hi és, amb estadístiques.
	_, list := kreq(t, h, "GET", "/api/knowledge", "")
	cols, _ := list["collections"].([]any)
	if len(cols) != 1 || list["project"] != s.cwd {
		t.Fatalf("llista: %v", list)
	}
	// Fonts: treure-la i reindexar sincrònicament deixa l'índex buit de fitxers.
	src := item["sources"].([]any)[0].(map[string]any)["id"].(string)
	waitIdle(t, id)
	if code, _ := kreq(t, h, "DELETE", "/api/knowledge/"+id+"/sources", `{"id":"`+src+`"}`); code != 200 {
		t.Fatalf("treure font: %d", code)
	}
	waitIdle(t, id)
	if code, got := kreq(t, h, "POST", "/api/knowledge/"+id+"/reindex", `{"wait":true}`); code != 200 {
		t.Fatalf("reindex: %d %v", code, got)
	}
	_, item = kreq(t, h, "GET", "/api/knowledge/"+id, "")
	if st := item["stats"].(map[string]any); st["docs"].(float64) != 1 { // només la nota
		t.Fatalf("després de treure la font: %v", st)
	}
	if code, _ := kreq(t, h, "DELETE", "/api/knowledge/"+id, ""); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := kreq(t, h, "GET", "/api/knowledge/"+id, ""); code != 404 {
		t.Fatalf("esborrada: %d", code)
	}
	// Errors: nom buit, abast dolent, font inexistent.
	if code, _ := kreq(t, h, "POST", "/api/knowledge", `{"name":""}`); code != 400 {
		t.Errorf("nom buit: %d", code)
	}
	if code, _ := kreq(t, h, "POST", "/api/knowledge", `{"name":"x","scope":"planet"}`); code != 400 {
		t.Errorf("abast dolent: %d", code)
	}
	if code, _ := kreq(t, h, "POST", "/api/knowledge", `{"name":"y","path":"nope/missing"}`); code != 400 {
		t.Errorf("font inexistent: %d", code)
	}
	if _, list = kreq(t, h, "GET", "/api/knowledge", ""); len(list["collections"].([]any)) != 0 {
		t.Errorf("una creació fallida ha deixat rastre: %v", list)
	}
}

func waitIdle(t *testing.T, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for knowledge.ProgressOf(id).Running && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
}

func TestKnowledgeMemoryDiffSafe(t *testing.T) {
	s, h := knowledgeTestServer(t)
	os.WriteFile(filepath.Join(s.cwd, "AGENTS.md"), []byte("Use tabs."), 0o644)
	_, got := kreq(t, h, "GET", "/api/knowledge/memory", "")
	mems := got["memories"].([]any)
	if len(mems) != 3 {
		t.Fatalf("memòries: %v", got)
	}
	user := mems[0].(map[string]any)
	rules := mems[2].(map[string]any)
	if rules["text"] != "Use tabs." || rules["editable"] != false {
		t.Fatalf("AGENTS.md: %v", rules)
	}
	body, _ := json.Marshal(map[string]string{"kind": "user", "text": "- prefers metric units", "hash": user["hash"].(string)})
	if code, out := kreq(t, h, "PUT", "/api/knowledge/memory", string(body)); code != 200 {
		t.Fatalf("desar: %d %v", code, out)
	}
	raw, _ := os.ReadFile(os.Getenv("GREGAL_MEMORY"))
	if string(raw) != "- prefers metric units\n" {
		t.Fatalf("fitxer: %q", raw)
	}
	// Amb el hash vell: conflicte, i el fitxer no es toca.
	body, _ = json.Marshal(map[string]string{"kind": "user", "text": "overwrite", "hash": user["hash"].(string)})
	code, out := kreq(t, h, "PUT", "/api/knowledge/memory", string(body))
	if code != 409 || out["current"] == nil {
		t.Fatalf("conflicte: %d %v", code, out)
	}
	raw, _ = os.ReadFile(os.Getenv("GREGAL_MEMORY"))
	if string(raw) != "- prefers metric units\n" {
		t.Fatalf("trepitjat: %q", raw)
	}
	// Notes del projecte.
	proj := mems[1].(map[string]any)
	body, _ = json.Marshal(map[string]string{"kind": "project", "text": "- 2026-10-13: deploy on Thursdays", "hash": proj["hash"].(string)})
	if code, _ := kreq(t, h, "PUT", "/api/knowledge/memory", string(body)); code != 200 {
		t.Fatalf("notes: %d", code)
	}
	if raw, _ := os.ReadFile(filepath.Join(s.cwd, ".gregal", "memory.md")); !strings.Contains(string(raw), "Thursdays") {
		t.Fatalf("notes del projecte: %q", raw)
	}
	if code, _ := kreq(t, h, "PUT", "/api/knowledge/memory", `{"kind":"rules","text":"x"}`); code != 400 {
		t.Errorf("AGENTS.md no s'edita des d'aquí: %d", code)
	}
}

func TestKnowledgeGlobalNeedsAdmin(t *testing.T) {
	_, h := knowledgeTestServer(t)
	r := httptest.NewRequest("POST", "/api/knowledge", strings.NewReader(`{"name":"Shared","scope":"global"}`))
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, &User{Name: "anna", Roots: []string{t.TempDir()}}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("un usuari no administrador ha creat una col·lecció global: %d", w.Code)
	}
}
