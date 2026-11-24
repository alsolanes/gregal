package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/tools"
)

func outputsGet(t *testing.T, s *Server, wind string) []WindOutput {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleWindOutputs(rec, httptest.NewRequest(http.MethodGet, "/api/winds/outputs?wind="+wind, nil))
	if rec.Code != 200 {
		t.Fatalf("outputs %s: %d %s", wind, rec.Code, rec.Body.String())
	}
	var out struct {
		Items []WindOutput `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Items
}

// windMakesDoc fa que el vent creï un document viu, com al seu xat.
func windMakesDoc(t *testing.T, s *Server, ctx context.Context, title string) {
	t.Helper()
	args := `{"action":"create","title":"` + title + `","md":"# ` + title + `"}`
	out, _, err := s.windDocExec(ctx, s.windDocsDir(), args, func(string, string) {})
	if err != nil {
		t.Fatal(err)
	}
	s.noteWindOutput(ctx, "doc", args, out)
}

// El prestatge d'un vent: el document que ha creat, la web que ha publicat i
// el fitxer que ha escrit, del més nou al més vell; ni el d'un altre vent
// ni el d'un altre compte amb un vent del mateix id.
func TestWindOutputsListsWhatTheWindMade(t *testing.T) {
	rig := sharedTestRig(t)
	s := rig.aleix
	shareID := seedShelf(t, s, s.windDocsCtx(s.withWindSteerKey(context.Background(), "wind", rig.mestral.ID)))

	// Un altre vent del mateix compte, i un vent de la Carla amb el mateix id.
	windMakesDoc(t, s, s.windDocsCtx(s.withWindSteerKey(context.Background(), "wind", rig.secret.ID)), "Analítica")
	carlaMestral, err := rig.carla.winds.Save(companion.Wind{Name: "Mestral", Role: "mar"})
	if err != nil || carlaMestral.ID != rig.mestral.ID {
		t.Fatalf("el test vol el mateix id als dos comptes: %v %s", err, carlaMestral.ID)
	}
	windMakesDoc(t, rig.carla, rig.carla.windDocsCtx(rig.carla.withWindSteerKey(context.Background(), "wind", carlaMestral.ID)), "Rutes de la Carla")

	items := outputsGet(t, s, rig.mestral.ID)
	want := "file|Pressupost refugi.pdf,site|Cims de Catalunya,doc|Ruta del Montseny"
	if got := shelfKeys(t, items); got != want {
		t.Fatalf("prestatge = %s\nvull %s", got, want)
	}
	if items[0].URL != "/api/winds/doc?name=Pressupost+refugi.pdf" || items[1].URL != "/share/"+shareID+"/" ||
		!strings.HasPrefix(items[2].URL, "/api/docs/") || !strings.HasSuffix(items[2].URL, "/export?scope=mine&format=docx") {
		t.Fatalf("enllaços: %+v", items)
	}
	if got := shelfKeys(t, outputsGet(t, rig.carla, carlaMestral.ID)); got != "doc|Rutes de la Carla" {
		t.Fatalf("la Carla: %s", got)
	}
	if got := shelfKeys(t, outputsGet(t, s, rig.secret.ID)); got != "doc|Analítica" {
		t.Fatalf("el Garbí: %s", got)
	}
	if empty := outputsGet(t, s, "inexistent"); empty == nil || len(empty) != 0 {
		t.Fatalf("vent sense res = %+v", empty)
	}
	rec := httptest.NewRecorder()
	s.handleWindOutputs(rec, httptest.NewRequest(http.MethodGet, "/api/winds/outputs", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("sense wind: %d", rec.Code)
	}
}

func shelfKeys(t *testing.T, items []WindOutput) string {
	t.Helper()
	var got []string
	for _, it := range items {
		got = append(got, it.Kind+"|"+it.Title)
		if it.At.IsZero() || it.URL == "" {
			t.Fatalf("element incomplet: %+v", it)
		}
	}
	return strings.Join(got, ",")
}

// seedShelf: el vent de ctx crea un document, publica una web i escriu un
// PDF, per aquest ordre; torna l'id de la web.
func seedShelf(t *testing.T, s *Server, ctx context.Context) string {
	t.Helper()
	windMakesDoc(t, s, ctx, "Ruta del Montseny")
	site := t.TempDir()
	s.user.Roots = append(s.user.Roots, site)
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<!doctype html><title>Cims</title><h1>Cims</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := s.publishDir(site, "Cims de Catalunya")
	if err != nil {
		t.Fatal(err)
	}
	s.noteWindOutput(ctx, "publish", `{"dir":"`+filepath.ToSlash(site)+`"}`, "Published \"Cims de Catalunya\".\nLink: http://localhost/share/"+m.ID+"/")

	tools.Active = tools.NewJournal()
	pdfArgs := `{"path":"Pressupost refugi.pdf","content":"Total: 80 €"}`
	if _, err := tools.PdfCreate(ctx, tools.PdfCreateOpts{Path: filepath.Join(windDocsOf(ctx), "Pressupost refugi.pdf"), Content: "Total: 80 €"}); err != nil {
		t.Fatal(err)
	}
	s.noteWindOutput(ctx, "pdf_create", pdfArgs, "ok")
	return m.ID
}

// El torn d'un vent que crida doc deixa el document al seu prestatge; tornar
// a editar-lo no el repeteix.
func TestWindTurnPutsItsDocOnTheShelf(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_WIND_ALL_TOOLS", "1")
	s, _ := companionTestServer(t)
	s.cwd = t.TempDir()
	postJSON(t, s.handleWinds, `{"name":"Tramuntana","color":"#29ADFF","role":"sistema"}`)
	turn := func(args string) {
		provider := windPermissionProvider(t, "doc", args)
		s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
		rec := httptest.NewRecorder()
		s.handleWindChatStream(rec, httptest.NewRequest(http.MethodPost, "/api/winds/chat/stream", strings.NewReader(`{"wind":"tramuntana","message":"fes-ho"}`)))
		waitBackground()
	}
	turn(`{"action":"create","title":"Informe","md":"# Hola"}`)
	items := outputsGet(t, s, "tramuntana")
	if len(items) != 1 || items[0].Kind != "doc" || items[0].Title != "Informe" {
		t.Fatalf("prestatge: %+v", items)
	}
	id := strings.TrimSuffix(strings.TrimPrefix(items[0].URL, "/api/docs/"), "/export?scope=mine&format=docx")
	turn(fmt.Sprintf(`{"action":"insert","id":%q,"md":"Més"}`, id))
	again := outputsGet(t, s, "tramuntana")
	if len(again) != 1 || again[0].Title != "Informe" || again[0].At.Before(items[0].At) {
		t.Fatalf("després d'editar: %+v", again)
	}
}
