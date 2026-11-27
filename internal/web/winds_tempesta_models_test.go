package web

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// tempestaModelsServer: un servidor amb un segon proveïdor «q» que anuncia
// els models que es poden triar per a «A fons».
func tempestaModelsServer(t *testing.T, ids ...string) *Server {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	prov := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b strings.Builder
		b.WriteString(`{"data":[`)
		for i, id := range ids {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`{"id":"` + id + `"}`)
		}
		b.WriteString(`]}`)
		if _, err := w.Write([]byte(b.String())); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(prov.Close)
	s := remindTestServer(t)
	s.cfg.Providers["q"] = config.Provider{BaseURL: prov.URL + "/v1"}
	return s
}

func tempestaModelsPost(s *Server, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.handleTempestaModels(rec, httptest.NewRequest(http.MethodPost, "/api/winds/tempesta/models", strings.NewReader(body)))
	return rec
}

func tempestaModelsOf(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var out struct {
		Models map[string]string `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("resposta il·legible: %v %s", err, rec.Body.String())
	}
	return out.Models
}

// L'endpoint llegeix i desa el model de cada agent i de la síntesi; buit
// torna a la tria automàtica.
func TestTempestaModelsEndpointSavesAndClears(t *testing.T) {
	s := tempestaModelsServer(t, "fort", "rapid")

	rec := httptest.NewRecorder()
	s.handleTempestaModels(rec, httptest.NewRequest(http.MethodGet, "/api/winds/tempesta/models", nil))
	got := tempestaModelsOf(t, rec)
	for _, slot := range []string{"direct", "sources", "skeptic", "synth"} {
		if v, ok := got[slot]; !ok || v != "" {
			t.Fatalf("sense tria, %s ha de ser buit (automàtic): %+v", slot, got)
		}
	}

	rec = tempestaModelsPost(s, `{"slot":"synth","model":"q/fort"}`)
	if rec.Code != http.StatusOK || tempestaModelsOf(t, rec)["synth"] != "q/fort" {
		t.Fatalf("desa la síntesi: %d %s", rec.Code, rec.Body.String())
	}
	defaults, err := session.LoadModelDefaults(s.usuari())
	if err != nil || defaults["~tempesta.synth"] != "q/fort" {
		t.Fatalf("no persisteix: %+v %v", defaults, err)
	}

	rec = tempestaModelsPost(s, `{"slot":"synth","model":""}`)
	if rec.Code != http.StatusOK || tempestaModelsOf(t, rec)["synth"] != "" {
		t.Fatalf("buit ha de tornar a automàtic: %d %s", rec.Code, rec.Body.String())
	}
}

// Un model, un proveïdor o un agent desconeguts són 400 i no desen res.
func TestTempestaModelsEndpointRejectsUnknown(t *testing.T) {
	s := tempestaModelsServer(t, "fort")
	for _, body := range []string{
		`{"slot":"direct","model":"q/noexisteix"}`,
		`{"slot":"direct","model":"fantasma/fort"}`,
		`{"slot":"direct","model":"fort"}`,
		`{"slot":"advocat","model":"q/fort"}`,
	} {
		if rec := tempestaModelsPost(s, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s hauria de ser 400: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if got := s.modelOverride["~tempesta.direct"]; got != "" {
		t.Fatalf("un error no pot desar res: %q", got)
	}
}

// runTempesta fa un torn «A fons» amb un proveïdor fals i torna quin model
// ha rebut cada angle, el de la síntesi i el flux SSE.
func runTempesta(t *testing.T, s *Server) (map[string]string, string, string) {
	t.Helper()
	oldLoop, oldChat := windLoop, petChatter
	t.Cleanup(func() { waitBackground(); windLoop = oldLoop; petChatter = oldChat })
	var mu sync.Mutex
	agentModels := map[string]string{}
	windLoop = func(_ *Server, _ context.Context, brief string, roles ...*routedRole) (exploracio, error) {
		for _, a := range tempestaAngles {
			if strings.Contains(brief, a.brief) {
				mu.Lock()
				agentModels[a.Key] = roles[0].role.Provider + "/" + roles[0].role.Model
				mu.Unlock()
			}
		}
		return exploracio{text: "Un esborrany sobre la pregunta."}, nil
	}
	synthModel := ""
	petChatter = func(_ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, role config.Role, _ []llm.Message) (string, error) {
		synthModel = role.Provider + "/" + role.Model
		return "La resposta ajuntada.", nil
	}
	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"Quin és millor?","tempesta":true}`))
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, req)
	return agentModels, synthModel, rec.Body.String()
}

// windTempesta fa servir el model triat per a cada agent i per a la síntesi.
func TestTempestaUsesTheChosenModels(t *testing.T) {
	s := tempestaModelsServer(t, "m-directe", "m-fonts", "m-sceptic", "m-sintesi")
	for slot, model := range map[string]string{"direct": "q/m-directe", "sources": "q/m-fonts", "skeptic": "q/m-sceptic", "synth": "q/m-sintesi"} {
		if rec := tempestaModelsPost(s, `{"slot":"`+slot+`","model":"`+model+`"}`); rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", slot, rec.Code, rec.Body.String())
		}
	}
	agents, synth, raw := runTempesta(t, s)
	want := map[string]string{"direct": "q/m-directe", "sources": "q/m-fonts", "skeptic": "q/m-sceptic"}
	for k, v := range want {
		if agents[k] != v {
			t.Fatalf("l'agent %s ha rebut %q, no %q (tots: %+v)", k, agents[k], v, agents)
		}
	}
	if synth != "q/m-sintesi" {
		t.Fatalf("la síntesi ha rebut %q", synth)
	}
	if !strings.Contains(raw, `"model":"m-directe"`) {
		t.Fatalf("el client ha de veure el model triat:\n%s", raw)
	}
}

// Un model triat que ja no existeix (el proveïdor ja no l'anuncia, o el
// proveïdor ja no és al config) torna a la tria automàtica.
func TestTempestaGoneModelFallsBackToAuto(t *testing.T) {
	s := tempestaModelsServer(t, "m-directe")
	auto := s.tempestaModels(tempestaAgentCount)
	_, autoSynth := s.tempestaSynthRole()
	s.modelOverride["~tempesta.direct"] = "q/esborrat"
	s.modelOverride["~tempesta.synth"] = "fantasma/x"

	agents, synth, _ := runTempesta(t, s)
	if want := auto[0].role.Provider + "/" + auto[0].role.Model; agents["direct"] != want {
		t.Fatalf("un model esborrat ha de caure a l'automàtic %q, no %q", want, agents["direct"])
	}
	if want := autoSynth.Provider + "/" + autoSynth.Model; synth != want {
		t.Fatalf("la síntesi amb proveïdor esborrat ha de ser %q, no %q", want, synth)
	}
}

// Cada agent de l'esdeveniment «tempesta» diu què fa, en català i en anglès.
func TestTempestaAgentsCarryADescription(t *testing.T) {
	s := tempestaModelsServer(t)
	_, _, raw := runTempesta(t, s)
	var agents []map[string]any
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	inTempesta := false
	for sc.Scan() {
		line := sc.Text()
		if line == "event: tempesta" {
			inTempesta = true
			continue
		}
		if inTempesta && strings.HasPrefix(line, "data: ") {
			var ev struct {
				Agents []map[string]any `json:"agents"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
				t.Fatal(err)
			}
			agents = ev.Agents
			inTempesta = false
		}
	}
	if len(agents) != tempestaAgentCount {
		t.Fatalf("esperava %d agents:\n%s", tempestaAgentCount, raw)
	}
	seen := map[string]bool{}
	for _, a := range agents {
		ca, _ := a["desc_ca"].(string)
		en, _ := a["desc_en"].(string)
		if strings.TrimSpace(ca) == "" || strings.TrimSpace(en) == "" || ca == en {
			t.Fatalf("l'agent %v ha de portar desc_ca i desc_en: %+v", a["angle"], a)
		}
		seen[en] = true
	}
	if len(seen) != tempestaAgentCount {
		t.Fatalf("cada angle té la seva descripció: %+v", agents)
	}
}
