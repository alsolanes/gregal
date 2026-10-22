package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
	"gregal/internal/llm"
	"gregal/internal/runs"
)

// --- Rotllanes ---

// TestRotllanaLifecycle comprova el cicle: crear amb vents que existeixen,
// llistar, esborrar. Un vent esborrat surt dels membres.
func TestRotllanaLifecycle(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	// Dos vents.
	postJSON(t, s.handleWinds, `{"name":"Tramuntana","color":"#29ADFF","role":"sistema"}`)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"música"}`)

	rec := postJSON(t, s.handleRotllanes, `{"name":"Vesprada","members":["tramuntana","xaloc"]}`)
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	var saved struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(saved.Rotllana.ID, "vesprada-") || len(saved.Rotllana.Members) != 2 {
		t.Fatalf("rotllana mal desada: %+v", saved.Rotllana)
	}

	// Llistat.
	getList := func() string {
		g := httptest.NewRequest(http.MethodGet, "/api/winds/rotllanes", nil)
		b := httptest.NewRecorder()
		s.handleRotllanes(b, g)
		if b.Code != 200 {
			t.Fatalf("list: %d", b.Code)
		}
		return b.Body.String()
	}
	if first := getList(); !strings.Contains(first, "vesprada") {
		t.Errorf("no surt al llistat: %s", first)
	}

	// Un vent esborrat surt dels membres.
	req := httptest.NewRequest(http.MethodDelete, "/api/winds", strings.NewReader(`{"id":"xaloc"}`))
	body2 := httptest.NewRecorder()
	s.handleWinds(body2, req)
	if body2.Code != 200 {
		t.Fatalf("delete wind: %d", body2.Code)
	}
	if second := getList(); strings.Contains(second, "xaloc") {
		t.Errorf("el vent esborrat encara és membre: %s", second)
	}

	// Esborra la rotllana.
	req2 := httptest.NewRequest(http.MethodDelete, "/api/winds/rotllanes", strings.NewReader(`{"id":"`+saved.Rotllana.ID+`"}`))
	body3 := httptest.NewRecorder()
	s.handleRotllanes(body3, req2)
	if body3.Code != 200 {
		t.Fatalf("delete: %d", body3.Code)
	}
	if _, ok := s.rotllanaByID(saved.Rotllana.ID); ok {
		t.Error("la rotllana hauria de ser esborrada")
	}
}

// TestRotllanaChatTurns comprova que el xat de grup fa parlar el vent que toca
// (menys parlat recentment) i que cada resposta queda com "Nom: text".
func TestRotllanaChatTurns(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Mestral","color":"#29ADFF","role":"sistema"}`)
	postJSON(t, s.handleWinds, `{"name":"Marinada","color":"#A3E24E","role":"música"}`)
	created := postJSON(t, s.handleRotllanes, `{"name":"Capvespre","members":["mestral","marinada"]}`)
	var group struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &group); err != nil {
		t.Fatal(err)
	}

	var gotSystem string
	saved := petChatter
	petChatter = func(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		gotSystem = msgs[0].Content
		return "endavant!", nil
	}
	t.Cleanup(func() { petChatter = saved })

	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+group.Rotllana.ID+`","message":"què hi ha de nou?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	// El system porta el roster del grup.
	for _, want := range []string{"Mestral", "Marinada", "GROUP CHAT"} {
		if !strings.Contains(gotSystem, want) {
			t.Errorf("system sense %q:\n%s", want, gotSystem)
		}
	}
	// El qui parla: mestral (primer, cap ha parlat).
	if !strings.Contains(rec.Body.String(), `"wind":"mestral"`) {
		t.Errorf("hauria de parlar mestral primer: %s", rec.Body.String())
	}
	// La història queda "Mestral: endavant!".
	if !strings.Contains(rec.Body.String(), "Mestral: endavant!") {
		t.Errorf("historial sense el nom del vent: %s", rec.Body.String())
	}

	// Segon missatge: ara toca a Marinada (mestral ha parlat 1 cop).
	saved2 := petChatter
	petChatter = func(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		gotSystem = msgs[0].Content
		return "doncs jo afegiria…", nil
	}
	t.Cleanup(func() { petChatter = saved2 })

	rec = postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+group.Rotllana.ID+`","message":"i de música què?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat2: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"wind":"marinada"`) {
		t.Errorf("hauria de parlar marinada (torn): %s", rec.Body.String())
	}

	// GET torna l'historial complet.
	get := httptest.NewRequest(http.MethodGet, "/api/winds/rotllana?rotlla="+group.Rotllana.ID, nil)
	body := httptest.NewRecorder()
	s.handleRotllanaChat(body, get)
	if body.Code != 200 {
		t.Fatalf("get: %d", body.Code)
	}
	for _, want := range []string{"què hi ha de nou?", "Mestral: endavant!", "Marinada: doncs jo afegiria…"} {
		if !strings.Contains(body.Body.String(), want) {
			t.Errorf("historial sense %q: %s", want, body.Body.String())
		}
	}
}

// --- Cor del vent (veu proactiva) ---

// TestWindHeartbeatSpeaksWhenWorthIt: amb "(res)" calla; amb una novetat,
// el missatge entra al xat del vent i la sortida del job el porta.
func TestWindHeartbeatSpeaksWhenWorthIt(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"música nova"}`)

	// Cas 1: silenci → job ok, sense sortida, res al xat.
	saved := windHeartbeatLoop
	windHeartbeatLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
		return exploracio{text: "(res)"}, nil
	}
	t.Cleanup(func() { windHeartbeatLoop = saved })
	run, err := s.executeWindHeartbeat(context.Background(), jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc"}, runs.Run{}, companion.Wind{ID: "xaloc", Name: "Xaloc", Role: "música nova", Persona: "ets en Xaloc"})
	if err != nil {
		t.Fatalf("heartbeat silenci: %v", err)
	}
	if run.Output != "" {
		t.Errorf("en silenci no hi ha d'haver sortida: %q", run.Output)
	}

	// Cas 2: novetat → missatge al xat del vent + sortida amb el nom.
	windHeartbeatLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
		return exploracio{text: "Ha sortit el disc nou de la Björk! [bjork.com](https://bjork.com)"}, nil
	}
	run, err = s.executeWindHeartbeat(context.Background(), jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc"}, runs.Run{}, companion.Wind{ID: "xaloc", Name: "Xaloc", Role: "música nova", Persona: "ets en Xaloc"})
	if err != nil {
		t.Fatalf("heartbeat novetat: %v", err)
	}
	if !strings.HasPrefix(run.Output, "Xaloc: ") {
		t.Errorf("sortida del job hauria de portar el nom: %q", run.Output)
	}
	// El missatge ha entrat al xat del vent.
	hist := windHistory(s, "xaloc")
	hist.mu.Lock()
	last := hist.messages[len(hist.messages)-1]
	hist.mu.Unlock()
	if last.Role != "assistant" || !strings.Contains(last.Content, "Björk") {
		t.Errorf("el xat del vent no duu la novetat: %+v", last)
	}

	// Cas 3: error del model → run d'error, res al xat.
	windHeartbeatLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
		return exploracio{}, context.DeadlineExceeded
	}
	hist2 := windHistory(s, "xaloc")
	hist2.mu.Lock()
	n := len(hist2.messages)
	hist2.mu.Unlock()
	run, err = s.executeWindHeartbeat(context.Background(), jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc"}, runs.Run{}, companion.Wind{ID: "xaloc", Name: "Xaloc", Role: "música nova", Persona: "ets en Xaloc"})
	if err == nil {
		t.Fatal("hauria de tornar error")
	}
	if run.Status != "error" {
		t.Errorf("run d'error: %+v", run)
	}
	hist3 := windHistory(s, "xaloc")
	hist3.mu.Lock()
	if got := len(hist3.messages); got != n {
		t.Errorf("en error no s'ha d'escriure al xat: %d != %d", got, n)
	}
	hist3.mu.Unlock()
}

// --- Proposta de persona amb IA ---

// TestWindSuggestPersona: el sistema demana JSON compacte amb persona+style,
// i el parseig tolera tanques de codi i lletres invàlides.
func TestWindSuggestPersona(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)

	var gotSystem, gotUser string
	saved := windSuggestLoop
	windSuggestLoop = func(s *Server, ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		gotSystem = msgs[0].Content
		gotUser = msgs[1].Content
		return "```json\n{\"persona\": \"Sóc en Gregal, el vent de casa: parlo curt i tinc clar qui mana.\", \"style\": \"ZZ9\"}\n```", nil
	}
	t.Cleanup(func() { windSuggestLoop = saved })

	rec := postJSON(t, s.handleWindSuggest, `{"name":"Gregal","role":"el vent de casa"}`)
	if rec.Code != 200 {
		t.Fatalf("suggest: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(gotSystem, "persona") || !strings.Contains(gotSystem, "style") {
		t.Errorf("system sense el format demanat:\n%s", gotSystem)
	}
	if !strings.Contains(gotUser, "Gregal") {
		t.Errorf("el nom no arriba al model: %s", gotUser)
	}
	var out struct {
		Persona string `json:"persona"`
		Style   string `json:"style"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Persona, "Gregal") {
		t.Errorf("persona no parsejada: %q", out.Persona)
	}
	// Lletres invàlides → peces per defecte.
	if out.Style != "pso" {
		t.Errorf("style invàlid hauria de caure a pso: %q", out.Style)
	}

	// Error del model → 502, no pànic.
	windSuggestLoop = func(s *Server, ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		return "", context.DeadlineExceeded
	}
	rec = postJSON(t, s.handleWindSuggest, `{"name":"Xaloc"}`)
	if rec.Code != http.StatusBadGateway {
		t.Errorf("error del model hauria de ser 502: %d", rec.Code)
	}
	// Nom buit → 400.
	rec = postJSON(t, s.handleWindSuggest, `{"name":"  "}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("nom buit hauria de ser 400: %d", rec.Code)
	}
}

// --- Cervell del vent ---

// TestWindBrainGrowsWithWhatItLearns: després d'una resposta, els fets
// duradors queden al cervell; la propera pregunta els porta al brief.
func TestWindBrainGrowsWithWhatItLearns(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Llebeig","color":"#FF4444","role":"portàtils"}`)

	// El loop de cerca torna una resposta amb fets.
	savedLoop := windLoop
	windLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
		return exploracio{text: "El MacBook Air M4 té 53,8 Wh de bateria [apple.com]."}, nil
	}
	t.Cleanup(func() { windLoop = savedLoop })

	// L'extracció de fets: una sola crida petita que torna bales.
	savedLearn := windLearnLoop
	windLearnLoop = func(s *Server, ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, msgs []llm.Message) (string, error) {
		return "- El MacBook Air M4 té 53,8 Wh de bateria [apple.com]", nil
	}
	t.Cleanup(func() { windLearnLoop = savedLearn })

	rec := postJSON(t, s.handleWindChat, `{"wind":"llebeig","message":"quanta bateria té el MacBook Air M4?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}

	// El fet ha quedat al cervell del vent (prou perquè l'escritura async
	// s'hagi completat: comprovem amb reintent curt).
	var brain string
	for i := 0; i < 20; i++ {
		brain = s.windBrain("llebeig")
		if strings.Contains(brain, "53,8 Wh") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(brain, "53,8 Wh") {
		t.Fatalf("el cervell no ha après: %q", brain)
	}

	// El següent torn porta el fet al brief: el vent ja ho sap.
	var gotBrief2 string
	windLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
		gotBrief2 = brief
		return exploracio{text: "ja t'ho havia dit!"}, nil
	}
	postJSON(t, s.handleWindChat, `{"wind":"llebeig","message":"i quant dura?"}`)
	if !strings.Contains(gotBrief2, "53,8 Wh") {
		t.Errorf("el brief no porta el coneixement del vent:\n%s", gotBrief2)
	}
	if !strings.Contains(gotBrief2, "ALREADY KNOW") {
		t.Errorf("el brief no marca el bloc de coneixement")
	}
}

// TestWindBrainDedupesAndSlugs: els fets duplicats no s'hi posen dues
// vegades, i els ids amb accents es normalitzen.
func TestWindBrainDedupesAndSlugs(t *testing.T) {
	dir := t.TempDir()
	facts := []string{"- El Framework 13 té ports modulares [framework.com]"}
	if err := companion.AppendWindBrain(dir, "tramuntana", facts); err != nil {
		t.Fatal(err)
	}
	if err := companion.AppendWindBrain(dir, "tramuntana", facts); err != nil {
		t.Fatal(err)
	}
	brain := companion.LoadWindBrain(dir, "tramuntana")
	if strings.Count(brain, "Framework 13") != 1 {
		t.Errorf("duplicat: %q", brain)
	}
	// Fets curts o buits no entren.
	if err := companion.AppendWindBrain(dir, "tramuntana", []string{"- curta", "", "  "}); err != nil {
		t.Fatal(err)
	}
	brain2 := companion.LoadWindBrain(dir, "tramuntana")
	if strings.Contains(brain2, "curta") {
		t.Errorf("fet massa curt acceptat: %q", brain2)
	}
}
