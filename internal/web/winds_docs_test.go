package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/gdocs"
	"gregal/internal/tools"
)

// Els vents poden crear documents a la seva carpeta (i editar els que
// l'usuari ha adjuntat), però enlloc més.
func TestWindDocDecide(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, args, want string
	}{
		{"pdf_create", `{"path":"informe.pdf","content":"# Hola"}`, "allow"},
		{"office_create", `{"path":"carta.docx"}`, "allow"},
		{"pdf_create", `{"path":"` + filepath.ToSlash(filepath.Join(t.TempDir(), "fora.pdf")) + `"}`, "deny"},
		{"pdf_create", `{"path":"../escapa.pdf"}`, "deny"},
		{"pdf_edit", `{"path":"informe.pdf","op":"extract","pages":"1","out":"p1.pdf"}`, "allow"},
		{"pdf_edit", `{"path":"informe.pdf","op":"split"}`, "allow"},
		{"pdf_edit", `{"path":"` + filepath.ToSlash(filepath.Join(os.TempDir(), "gregal-office-abc-x.pdf")) + `","op":"stamp","text":"OK"}`, "allow"},
		{"pdf_edit", `{"path":"` + filepath.ToSlash(filepath.Join(os.TempDir(), "altre.pdf")) + `","op":"stamp","text":"OK"}`, "deny"},
		{"pdf_create", `{}`, "deny"},
	}
	for _, c := range cases {
		if got, why := windDocDecide(dir, c.name, c.args); got != c.want {
			t.Errorf("%s %s: %s (%s), volia %s", c.name, c.args, got, why, c.want)
		}
	}
	if !windDocTool("pdf_create") || windDocTool("write") || windDocTool("bash") {
		t.Fatal("només les eines de documents")
	}
}

func TestWindDocLinksAndDownload(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	start := time.Now()
	ctx := s.windDocsCtx(context.Background())
	dir := windDocsOf(ctx)
	if dir == "" {
		t.Fatal("sense carpeta de documents")
	}
	tools.Active = tools.NewJournal()
	if _, err := tools.PdfCreate(ctx, tools.PdfCreateOpts{Path: filepath.Join(dir, "Pressupost obra.pdf"), Title: "Pressupost", Content: "Total: 1.200 €"}); err != nil {
		t.Fatal(err)
	}
	links := s.windDocLinks(start, "Aquí el tens.")
	if !strings.Contains(links, "📄 [Pressupost obra.pdf](/api/winds/doc?name=Pressupost+obra.pdf)") {
		t.Fatalf("enllaç: %q", links)
	}
	// Un document d'abans del torn no surt; un que ja hi és a la resposta, tampoc.
	if got := s.windDocLinks(time.Now().Add(time.Hour), ""); got != "" {
		t.Fatalf("documents antics: %q", got)
	}
	if got := s.windDocLinks(start, "ja hi és: /api/winds/doc?name=Pressupost+obra.pdf"); got != "" {
		t.Fatalf("repetit: %q", got)
	}

	rec := httptest.NewRecorder()
	s.handleWindDoc(rec, httptest.NewRequest(http.MethodGet, "/api/winds/doc?name=Pressupost+obra.pdf", nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(rec.Body.String(), "%PDF") {
		t.Fatalf("baixada: %d %s", rec.Code, rec.Header())
	}
	for _, bad := range []string{"../secret.pdf", "..%2Fsecret.pdf", ".amagat", ""} {
		rec = httptest.NewRecorder()
		s.handleWindDoc(rec, httptest.NewRequest(http.MethodGet, "/api/winds/doc?name="+bad, nil))
		if rec.Code == 200 {
			t.Fatalf("%q no s'hauria de servir", bad)
		}
	}
}

// Un vent que crida doc crea el document viu a la carpeta de documents dels
// vents, no al projecte obert; la mateixa crida des de Codi va al projecte.
func TestWindDocLivesInTheWindDocsFolder(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_WIND_ALL_TOOLS", "1")
	s, _ := companionTestServer(t)
	s.cwd = t.TempDir()
	postJSON(t, s.handleWinds, `{"name":"Tramuntana","color":"#29ADFF","role":"sistema"}`)
	args := `{"action":"create","title":"Informe","md":"# Hola\n\nPrimer paràgraf"}`
	provider := windPermissionProvider(t, "doc", args)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, httptest.NewRequest(http.MethodPost, "/api/winds/chat/stream", strings.NewReader(`{"wind":"tramuntana","message":"crea un Word"}`)))
	waitBackground()
	body := rec.Body.String()

	mine, _ := gdocs.Open(s.windDocsDir())
	docs, err := mine.List()
	if err != nil || len(docs) != 1 || docs[0].Title != "Informe" {
		t.Fatalf("documents del vent: %v %+v\n%s", err, docs, body)
	}
	proj, _ := gdocs.Open(s.cwd)
	if docs, _ := proj.List(); len(docs) != 0 {
		t.Fatalf("el document del vent ha anat al projecte: %+v", docs)
	}
	id := docs[0].ID
	if !strings.Contains(body, "event: doc_changed\ndata: {\"id\":\""+id+"\",\"scope\":\"mine\",\"title\":\"Informe\"}") {
		t.Fatalf("falta doc_changed amb scope mine:\n%s", body)
	}
	if !strings.Contains(body, "/api/docs/"+id+"/export?scope=mine") {
		t.Fatalf("l'enllaç de baixada ha de dur scope=mine:\n%s", body)
	}

	if _, _, err := agent.ExecCtx(context.Background(), "codi", s.cwd, "doc", args); err != nil {
		t.Fatal(err)
	}
	if docs, _ := proj.List(); len(docs) != 1 {
		t.Fatalf("Codi: %+v", docs)
	}
	if docs, _ := mine.List(); len(docs) != 1 {
		t.Fatalf("Codi ha escrit a la carpeta dels vents: %+v", docs)
	}
}

// docReq fa una crida a una ruta de documents amb el camí i la consulta.
func docReq(s *Server, h func(*Server, http.ResponseWriter, *http.Request), method, target, body string, path ...string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if len(path) > 0 {
		req.SetPathValue("id", path[0])
	}
	if len(path) > 1 {
		req.SetPathValue("block", path[1])
	}
	h(s, rec, req)
	return rec
}

// Les rutes dels documents amb ?scope=mine fan servir la carpeta de
// documents dels vents de qui crida; sense, el projecte de la sessió.
func TestDocRoutesWithScopeMine(t *testing.T) {
	s := backlogServer(t)
	st, err := gdocs.Open(s.windDocsDir())
	if err != nil {
		t.Fatal(err)
	}
	d, err := st.Create("Carta", "# Hola\n\nUn", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if rec := docReq(s, (*Server).handleDocs, "GET", "/api/docs?scope=mine", ""); !strings.Contains(rec.Body.String(), d.ID) {
		t.Fatalf("llista mine: %d %s", rec.Code, rec.Body.String())
	}
	if rec := docReq(s, (*Server).handleDocs, "GET", "/api/docs", ""); strings.Contains(rec.Body.String(), d.ID) {
		t.Fatalf("llista del projecte: %s", rec.Body.String())
	}
	if rec := docReq(s, (*Server).handleDoc, "GET", "/api/docs/x?scope=mine", "", d.ID); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Carta") {
		t.Fatalf("doc mine: %d %s", rec.Code, rec.Body.String())
	}
	if rec := docReq(s, (*Server).handleDocExport, "GET", "/api/docs/x/export?scope=mine&format=docx", "", d.ID); rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "Carta.docx") {
		t.Fatalf("export mine: %d %s", rec.Code, rec.Body.String())
	}
	for _, h := range []func(*Server, http.ResponseWriter, *http.Request){(*Server).handleDoc, (*Server).handleDocExport} {
		if rec := docReq(s, h, "GET", "/api/docs/x", "", d.ID); rec.Code != http.StatusNotFound {
			t.Fatalf("sense scope: %d", rec.Code)
		}
	}
	if rec := docReq(s, (*Server).handleDocBlocks, "POST", "/api/docs/x/blocks?scope=mine", `{"after":"`+d.Blocks[1].ID+`","md":"Dos"}`, d.ID); rec.Code != 200 {
		t.Fatalf("bloc nou mine: %d %s", rec.Code, rec.Body.String())
	}
	if rec := docReq(s, (*Server).handleDocBlock, "PUT", "/api/docs/x/blocks/b?scope=mine", `{"md":"U","base_version":1}`, d.ID, d.Blocks[1].ID); rec.Code != 200 {
		t.Fatalf("bloc mine: %d %s", rec.Code, rec.Body.String())
	}
	if got, _ := st.Get(d.ID); got.Markdown() != "# Hola\n\nU\n\nDos" {
		t.Fatalf("document:\n%s", got.Markdown())
	}
}

// Amb comptes, cada persona té la seva carpeta: no veu els documents dels
// vents de l'altra.
func TestWindDocsArePerUser(t *testing.T) {
	base := remindTestServer(t)
	hub := base.Hub()
	anna := hub.SessionFor(&User{Name: "anna"}, "mobil")
	bernat := hub.SessionFor(&User{Name: "bernat"}, "mobil")
	if anna.windDocsDir() == bernat.windDocsDir() {
		t.Fatal("la mateixa carpeta per a dues persones")
	}
	st, _ := gdocs.Open(windDocsOf(anna.windDocsCtx(context.Background())))
	d, err := st.Create("Diari de l'Anna", "Avui", gdocs.AuthorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if rec := docReq(anna, (*Server).handleDocs, "GET", "/api/docs?scope=mine", ""); !strings.Contains(rec.Body.String(), d.ID) {
		t.Fatalf("l'Anna no el veu: %s", rec.Body.String())
	}
	if rec := docReq(bernat, (*Server).handleDocs, "GET", "/api/docs?scope=mine", ""); strings.Contains(rec.Body.String(), d.ID) {
		t.Fatalf("en Bernat veu el de l'Anna: %s", rec.Body.String())
	}
	if rec := docReq(bernat, (*Server).handleDoc, "GET", "/api/docs/x?scope=mine", "", d.ID); rec.Code != http.StatusNotFound {
		t.Fatalf("en Bernat el llegeix: %d", rec.Code)
	}
}

// La consigna dels vents dona la regla de 069: un document de text nou és
// un document viu amb doc; office_create, per a plantilles, presentacions,
// fulls de càlcul i adjunts.
func TestWindDocsBriefPrefersLiveDocuments(t *testing.T) {
	for _, w := range []string{"live document with the doc tool", "Word export link", "office_create only for templates, presentations, spreadsheets and documents the person attached"} {
		if !strings.Contains(windDocsBrief, w) {
			t.Errorf("falta %q a la consigna", w)
		}
	}
}
