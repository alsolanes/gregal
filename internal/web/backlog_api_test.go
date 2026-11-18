package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func backlogCall(t *testing.T, s *Server, method, body string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleBacklog(rec, httptest.NewRequest(method, "/api/backlog", strings.NewReader(body)))
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// backlogPost fa la crida i exigeix el codi de resposta.
func backlogPost(t *testing.T, s *Server, body string, want int) map[string]any {
	t.Helper()
	code, out := backlogCall(t, s, http.MethodPost, body)
	if code != want {
		t.Fatalf("%s: %d (volia %d) %+v", body, code, want, out)
	}
	return out
}

func backlogServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	s.cwd = t.TempDir()
	return s
}

func TestBacklogAPI(t *testing.T) {
	s := backlogServer(t)
	code, out := backlogCall(t, s, http.MethodGet, "")
	if code != 200 || out["storage"] != "app" || len(out["items"].([]any)) != 0 {
		t.Fatalf("%d %+v", code, out)
	}
	backlogPost(t, s, `{"action":"create","type":"feature","title":"Export CSV","why":"Manual work","criteria":["Button exists"],"size":"S"}`, 200)
	backlogPost(t, s, `{"action":"move","id":7,"status":"ready"}`, 404)
	backlogPost(t, s, `{"action":"to_repo"}`, 200)
	if _, out = backlogCall(t, s, http.MethodGet, ""); out["storage"] != "repo" || len(out["items"].([]any)) != 1 {
		t.Fatalf("%+v", out)
	}
}

func TestBacklogAPITreballaHi(t *testing.T) {
	s := backlogServer(t)
	backlogPost(t, s, `{"action":"create","type":"feature","title":"Export CSV","why":"Manual work","criteria":["Button exists"],"size":"S"}`, 200)
	// Encara proposada: no es pot començar, ni de prova.
	backlogPost(t, s, `{"action":"work","id":1,"dry":true}`, 400)
	backlogPost(t, s, `{"action":"move","id":1,"status":"ready"}`, 200)
	if out := backlogPost(t, s, `{"action":"work","id":1,"dry":true}`, 200); out["branch"] != "b1-export-csv" {
		t.Fatalf("%+v", out)
	}
	out := backlogPost(t, s, `{"action":"work","id":1,"session":"s-9"}`, 200)
	if !strings.Contains(out["task"].(string), "backlog item #1") {
		t.Fatalf("%+v", out)
	}
	item := out["item"].(map[string]any)
	logs := item["log"].([]any)
	if item["status"] != "in_progress" || !strings.Contains(logs[len(logs)-1].(map[string]any)["text"].(string), "session s-9") {
		t.Fatalf("%+v", item)
	}
}
