package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// exportTestServer prepara un servidor amb dades aïllades i una conversa
// desada que porta mapa del projecte, un secret i un camí personal.
func exportTestServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	prov := s.cfg.Providers["p"]
	prov.APIKey = "literal-config-key-0042"
	s.cfg.Providers["p"] = prov
	convo := []llm.Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "Read C:\\Users\\aleix\\proj\\a.go and mail me at aleix@example.com\n\n" + agent.MarcaContext + " x] proj\nfitxers:\nmapfile_secret.go\n"},
		{Role: "assistant", Content: "ok literal-config-key-0042 sk-ant-api03-ZZZZZZZZZZZZZZZZ <script>alert(1)</script>"},
	}
	if _, err := session.SaveSession(session.DefaultDir(), session.Session{Name: "conv-a", Title: "Conv A", Convo: convo}); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.convo = convo
	s.convoFile = "conv-a"
	s.mu.Unlock()
	return s, s.Hub().mux()
}

func exportDo(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestExportSavedMarkdownIHTML(t *testing.T) {
	_, h := exportTestServer(t)
	rec := exportDo(h, "GET", "/api/sessions/conv-a/export?format=md", "")
	if rec.Code != 200 {
		t.Fatalf("md: %d %s", rec.Code, rec.Body)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `attachment; filename="gregal-conv-a.md"`) {
		t.Fatalf("Content-Disposition: %q", cd)
	}
	md := rec.Body.String()
	if strings.Contains(md, agent.MarcaContext) || strings.Contains(md, "mapfile_secret") || strings.Contains(md, "system prompt") {
		t.Fatalf("mapa o system a l'export:\n%s", md)
	}
	if !strings.Contains(md, "literal-config-key-0042") {
		t.Fatal("sense redact el text va tal qual")
	}
	rec = exportDo(h, "GET", "/api/sessions/conv-a/export?format=html&redact=1&emails=1", "")
	out := rec.Body.String()
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("html: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, bad := range []string{"literal-config-key-0042", "sk-ant-api03", `C:\Users\aleix`, "aleix@example.com", "<script>alert", "mapfile_secret"} {
		if strings.Contains(out, bad) {
			t.Errorf("html sanitzat conté %q", bad)
		}
	}
	if exportDo(h, "GET", "/api/sessions/conv-a/export?format=pdf", "").Code != 400 {
		t.Fatal("format desconegut ha de ser 400")
	}
	if exportDo(h, "GET", "/api/sessions/no-existeix/export", "").Code != 404 {
		t.Fatal("conversa inexistent ha de ser 404")
	}
}

func TestExportConversaViva(t *testing.T) {
	_, h := exportTestServer(t)
	rec := exportDo(h, "GET", "/api/export?format=md&redact=1", "")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); strings.Contains(body, "literal-config-key-0042") || !strings.Contains(body, "aleix@example.com") {
		t.Fatalf("redact sense emails:\n%s", body)
	}
}

func TestShareCrearServirLlistarRevocar(t *testing.T) {
	_, h := exportTestServer(t)
	rec := exportDo(h, "POST", "/api/shares", `{"name":"conv-a"}`)
	if rec.Code != 200 {
		t.Fatalf("crear: %d %s", rec.Code, rec.Body)
	}
	var created struct{ ID, URL string }
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if len(created.ID) != 40 || created.URL != "/share/"+created.ID {
		t.Fatalf("id/url: %+v", created)
	}
	if _, err := os.Stat(filepath.Join(sharesDir(), created.ID+".html")); err != nil {
		t.Fatalf("instantània no desada: %v", err)
	}

	page := exportDo(h, "GET", created.URL, "")
	if page.Code != 200 {
		t.Fatalf("servir: %d", page.Code)
	}
	csp := page.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") || strings.Contains(csp, "script-src") {
		t.Fatalf("CSP: %q", csp)
	}
	if page.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("falta nosniff")
	}
	// Un enllaç compartit és SEMPRE sanitzat (correus inclosos per defecte).
	for _, bad := range []string{"literal-config-key-0042", "sk-ant-api03", `C:\Users\aleix`, "aleix@example.com", "mapfile_secret"} {
		if strings.Contains(page.Body.String(), bad) {
			t.Errorf("la pàgina compartida conté %q", bad)
		}
	}

	// La conversa viva també es pot compartir.
	if rec := exportDo(h, "POST", "/api/shares", ``); rec.Code != 200 {
		t.Fatalf("crear viva: %d %s", rec.Code, rec.Body)
	}
	list := exportDo(h, "GET", "/api/shares", "")
	var got struct {
		Shares []struct{ ID, URL, Title string } `json:"shares"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &got)
	if len(got.Shares) != 2 {
		t.Fatalf("llista: %s", list.Body)
	}

	if rec := exportDo(h, "DELETE", "/api/shares/"+created.ID, ""); rec.Code != 200 {
		t.Fatalf("revocar: %d", rec.Code)
	}
	if exportDo(h, "GET", created.URL, "").Code != 404 {
		t.Fatal("un enllaç revocat ha de ser 404")
	}
	if exportDo(h, "DELETE", "/api/shares/"+created.ID, "").Code != 404 {
		t.Fatal("revocar dues vegades ha de ser 404")
	}
}

func TestShareIdDesconegutIInvalid(t *testing.T) {
	_, h := exportTestServer(t)
	for _, p := range []string{"/share/" + strings.Repeat("a", 40), "/share/..%2f..%2fsessions%2fconv-a", "/share/abc"} {
		if rec := exportDo(h, "GET", p, ""); rec.Code != 404 {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
}

// Amb token, /api/shares demana autenticació però /share/<id> no: l'id és
// la capacitat.
func TestShareAmbToken(t *testing.T) {
	s, _ := exportTestServer(t)
	s.SetToken("tok-0123456789")
	// Amb token, les converses són a la carpeta de l'usuari que entra.
	src, _ := session.Load(session.DefaultDir(), "conv-a")
	if u := s.legacyAdmin(); u != nil {
		if _, err := session.SaveSession(session.DirFor(u.Name), src); err != nil {
			t.Fatal(err)
		}
	}
	h := s.Handler()
	if rec := exportDo(h, "POST", "/api/shares", `{"name":"conv-a"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("sense token: %d", rec.Code)
	}
	req := httptest.NewRequest("POST", "/api/shares", strings.NewReader(`{"name":"conv-a"}`))
	req.Header.Set("Authorization", "Bearer tok-0123456789")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("amb token: %d %s", rec.Code, rec.Body)
	}
	var created struct{ URL string }
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	page := exportDo(h, "GET", created.URL, "")
	if page.Code != 200 {
		t.Fatalf("pàgina: %d", page.Code)
	}
	if strings.Contains(page.Body.String(), "tok-0123456789") {
		t.Fatal("el token de l'API no pot sortir mai")
	}
}
