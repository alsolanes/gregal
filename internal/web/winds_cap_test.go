package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

func capPost(t *testing.T, s *Server, body string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleCap(rec, httptest.NewRequest(http.MethodPost, "/api/winds/cap", bytes.NewBufferString(body)))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCapDelega: la mascota passa la feina a qui toca, i l'usuari ho pot canviar.
func TestCapDelega(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	mar, _ := s.winds.Save(companion.Wind{Name: "Marinada", Role: "compres i receptes"})
	gar, _ := s.winds.Save(companion.Wind{Name: "Garbí", Role: "esports"})
	calls := 0
	reply := ""
	old := capDecider
	t.Cleanup(func() { capDecider = old })
	capDecider = func(_ *Server, _ context.Context, msgs []llm.Message) (string, error) {
		calls++
		return reply, nil
	}

	// Un vent nomenat va directe, sense model.
	out := capPost(t, s, `{"message":"Garbí, com ha quedat el Barça?","lang":"ca"}`)
	if out["action"] != "wind" || out["winds"].([]any)[0] != gar.ID || calls != 0 {
		t.Fatalf("nomenat: %v (crides %d)", out, calls)
	}
	if !strings.Contains(out["reply"].(string), "Garbí") {
		t.Fatalf("frase d'entrega sense nom: %v", out["reply"])
	}

	// El model tria un vent; la mascota ho diu.
	reply = `{"action":"wind","winds":["` + mar.ID + `"],"reply":"Això és cosa de la Marinada 🛒"}`
	out = capPost(t, s, `{"message":"què faig per sopar?","lang":"ca"}`)
	if out["action"] != "wind" || out["winds"].([]any)[0] != mar.ID {
		t.Fatalf("delega: %v", out)
	}

	// L'usuari prefereix el Garbí: la decisió nova substitueix l'anterior al fil.
	out = capPost(t, s, `{"message":"què faig per sopar?","lang":"ca","force":"wind:`+gar.ID+`"}`)
	if out["action"] != "wind" || out["winds"].([]any)[0] != gar.ID {
		t.Fatalf("canvi: %v", out)
	}
	hist := out["history"].([]any)
	if n := len(hist); n != 4 || hist[n-2].(map[string]any)["text"] != "què faig per sopar?" {
		t.Fatalf("el fil repeteix la pregunta: %v", hist)
	}

	// Un id inventat no passa: ho respon ella.
	reply = `{"action":"wind","winds":["xyz"],"reply":""}`
	out = capPost(t, s, `{"message":"hola","lang":"ca"}`)
	if out["action"] != "self" || out["reply"] == "" {
		t.Fatalf("id inventat: %v", out)
	}

	// Feina llarga: colla, amb l'encàrrec.
	reply = `{"action":"crew","task":"Una web de receptes de tardor","reply":"Munto una colla 🛠️"}`
	out = capPost(t, s, `{"message":"fes-me una web de receptes de tardor","lang":"ca"}`)
	if out["action"] != "crew" || out["task"] != "Una web de receptes de tardor" {
		t.Fatalf("colla: %v", out)
	}

	// Prosa sense JSON = resposta seva.
	reply = "Bon dia! Avui fa sol."
	out = capPost(t, s, `{"message":"bon dia","lang":"ca"}`)
	if out["action"] != "self" || out["reply"] != "Bon dia! Avui fa sol." {
		t.Fatalf("prosa: %v", out)
	}

	// Rotllana amb els mateixos vents: reaprofita la que ja hi ha.
	rot, err := s.saveRotllana(Rotllana{Name: "Sopar", Members: []string{gar.ID, mar.ID}})
	if err != nil {
		t.Fatal(err)
	}
	reply = `{"action":"rotllana","winds":["` + mar.ID + `","` + gar.ID + `"],"reply":""}`
	out = capPost(t, s, `{"message":"sopar i partit?","lang":"ca"}`)
	if out["action"] != "rotllana" || out["rotllana"] != rot.ID || out["reply"] == "" {
		t.Fatalf("rotllana: %v", out)
	}
}

// TestCapResum: en tornar, la mascota diu què ha passat des de l'última visita.
func TestCapResum(t *testing.T) {
	if got := capBriefText(capBrief{Done: 1, Titles: []string{"La web"}, Running: 2}, true); got != "Ja està llesta «La web», 2 feines en marxa." {
		t.Fatalf("ca: %q", got)
	}
	if got := capBriefText(capBrief{}, false); got != "All quiet. What do you need?" {
		t.Fatalf("en: %q", got)
	}
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	rec := httptest.NewRecorder()
	s.handleCap(rec, httptest.NewRequest(http.MethodGet, "/api/winds/cap?lang=ca", nil))
	var out struct {
		Name  string   `json:"name"`
		Brief capBrief `json:"brief"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Name != capName || out.Brief.Text == "" {
		t.Fatalf("%v %s", err, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	s.handleCapProfile(rec, httptest.NewRequest(http.MethodPost, "/api/winds/cap/profile", bytes.NewBufferString(`{"name":"Bufi"}`)))
	if !strings.Contains(rec.Body.String(), `"Bufi"`) {
		t.Fatalf("nom: %s", rec.Body.String())
	}
}

// TestCapEspecialistes: si ningú no en sap, la mascota proposa especialistes
// nous; en dir «endavant» els crea, obre la rotllana i, si l'usuari vol, els
// fa fer el curs accelerat.
func TestCapEspecialistes(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	mar, _ := s.winds.Save(companion.Wind{Name: "Marinada", Role: "compres i receptes"})
	old, oldExpert := capDecider, expertLoop
	t.Cleanup(func() { capDecider, expertLoop = old, oldExpert })
	capDecider = func(*Server, context.Context, []llm.Message) (string, error) {
		return `{"action":"rotllana","winds":["` + mar.ID + `"],"new":[{"name":"Fogony","role":"cuina de tardor","persona":"Cooks.","kit":""}],"reply":"Faig rotllana 🌀"}`, nil
	}
	out := capPost(t, s, `{"message":"què cuino amb carbassa?","lang":"ca"}`)
	if out["action"] != "rotllana" || len(out["create"].([]any)) != 1 {
		t.Fatalf("proposta: %v", out)
	}
	if nov := out["novices"].([]any); len(nov) != 1 || nov[0] != mar.ID {
		t.Fatalf("novells: %v", out["novices"])
	}
	expertLoop = func(*Server, context.Context, string, func(int, string)) (exploracio, error) {
		return exploracio{}, errConst("sense xarxa a la prova")
	}
	body, _ := json.Marshal(map[string]any{"message": "què cuino amb carbassa?", "lang": "ca", "winds": out["winds"], "create": out["create"], "course": true})
	rec := httptest.NewRecorder()
	s.handleCapGo(rec, httptest.NewRequest(http.MethodPost, "/api/winds/cap/go", bytes.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Rotllana string           `json:"rotllana"`
		Created  []companion.Wind `json:"created"`
		Studying []string         `json:"studying"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Rotllana == "" || len(res.Created) != 1 || len(res.Studying) != 2 {
		t.Fatalf("endavant: %s", rec.Body.String())
	}
	if rot, ok := s.rotllanaByID(res.Rotllana); !ok || len(rot.Members) != 2 {
		t.Fatalf("rotllana: %+v", rot)
	}
}

// TestCapMascotaJuga: menjar, jugar i mimar-la la fan pujar de nivell i
// guanyar roba; sense energia no juga; mai no baixa de l'alegria mínima.
func TestCapMascotaJuga(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	var p capPet
	if _, err := capPlay(&p, "pet", true, now); err != nil || p.Happy != 76 {
		t.Fatalf("mimar: %v %+v", err, p)
	}
	if _, err := capPlay(&p, "feed:galeta", true, now); err != nil || p.Full != 85 {
		t.Fatalf("menjar: %v %+v", err, p)
	}
	for i := 0; i < 5; i++ {
		_, _ = capPlay(&p, "game:nuvols:12", true, now)
	}
	if p.Energy != 5 || p.level() < 2 {
		t.Fatalf("jugar: %+v", p)
	}
	if say, _ := capPlay(&p, "game:nuvols:12", true, now); !strings.Contains(say, "becaina") || p.Played != 5 {
		t.Fatalf("cansat: %q %+v", say, p)
	}
	if _, err := capPlay(&p, "wear:corona", true, now); err == nil {
		t.Fatal("la corona encara no és seva")
	}
	if _, err := capPlay(&p, "wear:carbassa", true, now); err != nil || p.Outfit != "carbassa" {
		t.Fatalf("roba d'octubre: %v %+v", err, p)
	}
	p.tick(now.Add(30 * 24 * time.Hour))
	if p.Happy != capHappyFloor || p.Full != 0 || p.Energy != 100 {
		t.Fatalf("un mes sense obrir l'app: %+v", p)
	}
}

// TestCapFilAmbEnllac: en fer una delegació, el fil del Ventet hi apunta la
// conversa que n'ha sortit; i la proposta es desa sencera (especialistes nous).
func TestCapFilAmbEnllac(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	mar, _ := s.winds.Save(companion.Wind{Name: "Marinada", Role: "compres"})
	old := capDecider
	t.Cleanup(func() { capDecider = old })
	capDecider = func(*Server, context.Context, []llm.Message) (string, error) {
		return `{"action":"rotllana","winds":["` + mar.ID + `"],"new":[{"name":"Fogony","role":"cuina","persona":"Cooks.","kit":""}],"reply":"Faig rotllana"}`, nil
	}
	out := capPost(t, s, `{"message":"sopar?","lang":"ca"}`)
	hist := out["history"].([]any)
	last := hist[len(hist)-1].(map[string]any)
	if c, _ := last["create"].([]any); len(c) != 1 {
		t.Fatalf("la proposta s'ha de desar sencera: %v", last)
	}
	body, _ := json.Marshal(map[string]any{"message": "sopar?", "lang": "ca", "winds": out["winds"], "create": out["create"]})
	rec := httptest.NewRecorder()
	s.handleCapGo(rec, httptest.NewRequest(http.MethodPost, "/api/winds/cap/go", bytes.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	st := s.loadCap()
	link := st.History[len(st.History)-1]
	if link.Action != "link" || !strings.HasPrefix(link.Link, "rot:") || !strings.Contains(link.Text, "rotllana") {
		t.Fatalf("enllaç: %+v", link)
	}
	rec = httptest.NewRecorder()
	s.handleCapLink(rec, httptest.NewRequest(http.MethodPost, "/api/winds/cap/link", bytes.NewBufferString(`{"link":"https://x","text":"no"}`)))
	if rec.Code != 400 {
		t.Fatalf("un enllaç que no és una conversa s'ha de refusar: %d", rec.Code)
	}
}

// TestResumsDeConverses: la portada en té prou amb l'últim missatge de cada fil.
func TestResumsDeConverses(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	mar, _ := s.winds.Save(companion.Wind{Name: "Marinada", Role: "compres"})
	h := windHistory(s, mar.ID)
	h.mu.Lock()
	h.messages = append(h.messages, llm.Message{Role: "user", Content: "hola", Ts: 100}, llm.Message{Role: "assistant", Content: "Bon dia!", Ts: 200})
	h.mu.Unlock()
	capMu.Lock()
	st := s.loadCap()
	st.History = append(st.History, capMsg{From: "cap", Text: "Tot tranquil", Ts: 300})
	s.saveCap(st)
	capMu.Unlock()
	rec := httptest.NewRecorder()
	s.handleWindsSummaries(rec, httptest.NewRequest(http.MethodGet, "/api/winds/summaries", nil))
	var out struct {
		Items []convoSummary `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	got := map[string]convoSummary{}
	for _, it := range out.Items {
		got[it.Key] = it
	}
	if w := got["wind:"+mar.ID]; w.Text != "Bon dia!" || w.Ts != 200 || w.Count != 2 || w.From != "pet" {
		t.Fatalf("vent: %+v", w)
	}
	if c := got["cap"]; c.Text != "Tot tranquil" || c.Ts != 300 {
		t.Fatalf("ventet: %+v", c)
	}
}
