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

	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// TestWikiCRUD desa, llista, llegeix i esborra una pàgina de la wiki.
func TestWikiCRUD(t *testing.T) {
	s, _ := companionTestServer(t)
	rec := postJSON(t, s.handleWindsWiki, `{"nom":"camp-nou","titol":"Obres del Camp Nou","descripcio":"Estat de les obres","cos":"# Obres\n\n- Fase 1C enllestida"}`)
	if rec.Code != 200 {
		t.Fatalf("desa: %d %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/api/winds/wiki", nil)
	out := httptest.NewRecorder()
	s.handleWindsWiki(out, req)
	var list struct {
		Pages []wikiPage
	}
	if err := json.Unmarshal(out.Body.Bytes(), &list); err != nil || len(list.Pages) != 1 {
		t.Fatalf("llista: %v %s", err, out.Body.String())
	}
	if list.Pages[0].Nom != "camp-nou" || list.Pages[0].Cos != "" {
		t.Fatalf("llista malament: %+v", list.Pages[0])
	}
	req = httptest.NewRequest(http.MethodGet, "/api/winds/wiki/camp-nou", nil)
	out = httptest.NewRecorder()
	s.handleWindsWiki(out, req)
	var got struct {
		Page wikiPage
	}
	if err := json.Unmarshal(out.Body.Bytes(), &got); err != nil {
		t.Fatalf("llegeix: %v", err)
	}
	if got.Page.Titol != "Obres del Camp Nou" || !strings.Contains(got.Page.Cos, "Fase 1C") {
		t.Fatalf("pàgina malament: %+v", got.Page)
	}
	raw, err := os.ReadFile(filepath.Join(s.windWikiDir(), "camp-nou.md"))
	if err != nil || !strings.Contains(string(raw), "titol: Obres del Camp Nou") {
		t.Fatalf("fitxer disc: %v %s", err, string(raw))
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/winds/wiki/camp-nou", nil)
	out = httptest.NewRecorder()
	s.handleWindsWiki(out, req)
	if out.Code != 200 {
		t.Fatalf("esborra: %d", out.Code)
	}
	if _, err := os.Stat(filepath.Join(s.windWikiDir(), "camp-nou.md")); !os.IsNotExist(err) {
		t.Fatal("no esborrat")
	}
}

// TestParseWikiPage verifica el frontmatter i el cos.
func TestParseWikiPage(t *testing.T) {
	raw := "---\ntitol: Prova\ndescripcio: Una pàgina\nupdated: 2026-10-25T10:00:00Z\n---\n\n# Cos\n\nText."
	p := parseWikiPage("prova", raw)
	if p.Titol != "Prova" || p.Descripcio != "Una pàgina" || p.Updated != "2026-10-25T10:00:00Z" {
		t.Fatalf("frontmatter: %+v", p)
	}
	if !strings.HasPrefix(p.Cos, "# Cos") {
		t.Fatalf("cos: %q", p.Cos)
	}
	p2 := parseWikiPage("nu", "Text sol")
	if p2.Titol != "" || p2.Cos != "Text sol" {
		t.Fatalf("sense frontmatter: %+v", p2)
	}
}

// TestWikiSummaryConversaBuidaIGarbi resumeix: sense conversa → 400; amb
// conversa però proveïdor de prova (sense xarxa) → 502, mai un 500 vague.
func TestWikiSummaryConversaBuidaIGarbi(t *testing.T) {
	s, _ := companionTestServer(t)
	// Nom únic: altres tests han pogut deixar una conversa «company-…» per un
	// «Xaloc» al mateix store de sessió, i llavors la branca «buida» no seria buida.
	w, err := s.winds.Save(companion.Wind{Name: "VentDeWikiProva", Role: "Musica"})
	if err != nil {
		t.Fatal(err)
	}
	// vent inexistent → 400
	rec := postJSON(t, s.handleWikiSummary, `{"wind":"ningu"}`)
	if rec.Code != 400 {
		t.Fatalf("vent desconegut: %d %s", rec.Code, rec.Body.String())
	}
	// sense conversa desada → 400 «buida»
	rec = postJSON(t, s.handleWikiSummary, `{"wind":"`+w.ID+`"}`)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "buida") {
		t.Fatalf("buida: %d %s", rec.Code, rec.Body.String())
	}
	// amb conversa desada però proveïdor de prova (no escolta) → 502
	if _, err := session.SaveSession(s.sessDir(), session.Session{
		Name: "company-" + w.ID, Title: "Company — VentDeWikiProva", Role: "chat",
		Convo: []llm.Message{{Role: "user", Content: "Quin àlbum traurà Kiwanuka?"}, {Role: "assistant", Content: "FUDGE."}},
	}); err != nil {
		t.Fatal(err)
	}
	rec = postJSON(t, s.handleWikiSummary, `{"wind":"`+w.ID+`"}`)
	if rec.Code != 502 {
		t.Fatalf("proveïdor de prova: %d %s", rec.Code, rec.Body.String())
	}
}

// TestWikiSummaryCreaIActualitza: el resum fa una pàgina amb títol i
// descripció propis, conserva un gràfic al final i, en tornar a resumir la
// mateixa conversa, actualitza la mateixa pàgina amb la versió anterior al context.
func TestWikiSummaryCreaIActualitza(t *testing.T) {
	s, _ := companionTestServer(t)
	w, err := s.winds.Save(companion.Wind{Name: "VentDeWikiResum", Role: "Esports"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.SaveSession(s.sessDir(), session.Session{
		Name: "company-" + w.ID, Title: "Company", Role: "chat",
		Convo: []llm.Message{{Role: "user", Content: "Com van les obres?"}, {Role: "assistant", Content: "Fase 1C al 80%."}},
	}); err != nil {
		t.Fatal(err)
	}
	var prompts []string
	saved := wikiSummarizer
	t.Cleanup(func() { wikiSummarizer = saved })
	wikiSummarizer = func(_ *Server, _ context.Context, msgs []llm.Message) (string, error) {
		prompts = append(prompts, msgs[1].Content)
		return "```markdown\n# Obres del Camp Nou\nEstat de les obres i dates clau.\n\n- Fase 1C al 80%\n\n```chart\n{\"title\":\"Avanç\",\"type\":\"bar\",\"labels\":[\"1C\"],\"series\":[{\"name\":\"%\",\"values\":[80]}]}\n```\n```", nil
	}
	rec := postJSON(t, s.handleWikiSummary, `{"wind":"`+w.ID+`"}`)
	var first struct {
		Page    wikiPage
		Updated bool
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil || rec.Code != 200 {
		t.Fatalf("resum: %d %s", rec.Code, rec.Body.String())
	}
	p := first.Page
	if p.Titol != "Obres del Camp Nou" || p.Descripcio != "Estat de les obres i dates clau." || p.Autor != "VentDeWikiResum" || first.Updated {
		t.Fatalf("pàgina: %+v", p)
	}
	if strings.Contains(p.Cos, "# Obres") || !strings.HasSuffix(p.Cos, "]}\n```") {
		t.Fatalf("el cos ha de dur el gràfic sencer i no repetir el títol: %q", p.Cos)
	}
	rec = postJSON(t, s.handleWikiSummary, `{"wind":"`+w.ID+`"}`)
	var second struct {
		Page    wikiPage
		Updated bool
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil || !second.Updated || second.Page.Nom != p.Nom {
		t.Fatalf("havia d'actualitzar la mateixa pàgina: %s", rec.Body.String())
	}
	if len(prompts) != 2 || !strings.Contains(prompts[1], "Current page:") || !strings.Contains(prompts[1], "Fase 1C al 80%") {
		t.Fatalf("l'actualització no porta la pàgina anterior: %q", prompts)
	}
	if pages := s.wikiList(); len(pages) != 1 {
		t.Fatalf("hi havia d'haver una sola pàgina: %+v", pages)
	}
}

// TestWikiReadRejectsBackslash: el nom d'una pàgina no pot sortir de la carpeta.
func TestWikiReadRejectsBackslash(t *testing.T) {
	s, _ := companionTestServer(t)
	out := httptest.NewRecorder()
	s.handleWindsWiki(out, httptest.NewRequest(http.MethodGet, `/api/winds/wiki/..\..\config`, nil))
	if out.Code != http.StatusBadRequest {
		t.Fatalf("volia 400, tinc %d", out.Code)
	}
}
