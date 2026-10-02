package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/flow"
)

func servidorAmbFluxos(t *testing.T) (*Server, string) {
	t.Helper()
	proj := t.TempDir()
	s := &Server{cfg: &config.Config{}, cwd: proj, mode: "code"}
	return s, proj
}

func grafDeProva() *flow.Flow {
	return &flow.Flow{
		Name: "revisió",
		Desc: "llegeix i comenta",
		Nodes: []flow.Node{
			{ID: "llegeix", Kind: flow.KindTool, Title: "Llegeix", Tool: "read", Args: `{"path":"calc.go"}`},
			{ID: "comenta", Kind: flow.KindAgent, Title: "Comenta", Task: "Comenta {{llegeix}}"},
		},
		Edges: []flow.Edge{{From: "llegeix", To: "comenta"}},
	}
}

func TestAPIFluxosDesaLlistaCarregaEsborra(t *testing.T) {
	s, proj := servidorAmbFluxos(t)

	cos, _ := json.Marshal(map[string]any{"flow": grafDeProva()})
	w := httptest.NewRecorder()
	s.handleFlowSave(w, httptest.NewRequest("POST", "/api/flows/save", strings.NewReader(string(cos))))
	if w.Code != 200 {
		t.Fatalf("desar: %d %s", w.Code, w.Body)
	}
	// Ha d'anar al projecte de la sessió, no enlloc més.
	if _, err := os.Stat(filepath.Join(proj, ".gregal", "flows", "revisi.json")); err != nil {
		t.Fatalf("no s'ha escrit al projecte: %v", err)
	}

	w = httptest.NewRecorder()
	s.handleFlows(w, httptest.NewRequest("GET", "/api/flows", nil))
	var llistat struct {
		Flows []flow.Resum `json:"flows"`
	}
	json.Unmarshal(w.Body.Bytes(), &llistat)
	if len(llistat.Flows) != 1 || llistat.Flows[0].Passos != 2 {
		t.Fatalf("llistat=%+v", llistat.Flows)
	}

	w = httptest.NewRecorder()
	s.handleFlowLoad(w, httptest.NewRequest("GET", "/api/flows/load?name=revisió", nil))
	var carregat struct {
		Flow *flow.Flow `json:"flow"`
	}
	json.Unmarshal(w.Body.Bytes(), &carregat)
	if carregat.Flow == nil || len(carregat.Flow.Edges) != 1 {
		t.Fatalf("carregat=%+v", carregat.Flow)
	}

	w = httptest.NewRecorder()
	s.handleFlowDelete(w, httptest.NewRequest("POST", "/api/flows/delete", strings.NewReader(`{"name":"revisió"}`)))
	if w.Code != 200 {
		t.Fatalf("esborrar: %d", w.Code)
	}
	w = httptest.NewRecorder()
	s.handleFlowLoad(w, httptest.NewRequest("GET", "/api/flows/load?name=revisió", nil))
	if w.Code != 404 {
		t.Fatalf("després d'esborrar volia 404, tinc %d", w.Code)
	}
}

// Un graf mal fet és error de qui l'ha dibuixat: 400 amb el motiu, perquè
// la UI el pugui ensenyar al costat del dibuix, i res escrit al disc.
func TestAPIGrafTrencatTorna400(t *testing.T) {
	s, proj := servidorAmbFluxos(t)
	dolent := &flow.Flow{Name: "mal", Nodes: []flow.Node{
		{ID: "a", Kind: flow.KindAgent, Task: "x"},
		{ID: "b", Kind: flow.KindAgent, Task: "y"},
	}, Edges: []flow.Edge{{From: "a", To: "enlloc"}}}
	cos, _ := json.Marshal(map[string]any{"flow": dolent})
	w := httptest.NewRecorder()
	s.handleFlowSave(w, httptest.NewRequest("POST", "/api/flows/save", strings.NewReader(string(cos))))
	if w.Code != 400 {
		t.Fatalf("volia 400, tinc %d (%s)", w.Code, w.Body)
	}
	if strings.TrimSpace(w.Body.String()) == "" {
		t.Fatal("un 400 ha de dir què falla")
	}
	if fitxers, _ := filepath.Glob(filepath.Join(proj, ".gregal", "flows", "*.json")); len(fitxers) != 0 {
		t.Fatalf("no s'hauria d'haver escrit res: %v", fitxers)
	}
}

// Canviar-li el nom no ha de deixar el graf vell al disc.
func TestAPIRenombrarNoDuplica(t *testing.T) {
	s, proj := servidorAmbFluxos(t)
	cos, _ := json.Marshal(map[string]any{"flow": grafDeProva()})
	s.handleFlowSave(httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(string(cos))))

	nou := grafDeProva()
	nou.Name = "revisió llarga"
	cos, _ = json.Marshal(map[string]any{"flow": nou, "rename": "revisió"})
	w := httptest.NewRecorder()
	s.handleFlowSave(w, httptest.NewRequest("POST", "/", strings.NewReader(string(cos))))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	fitxers, _ := filepath.Glob(filepath.Join(proj, ".gregal", "flows", "*.json"))
	if len(fitxers) != 1 {
		t.Fatalf("volia un sol fitxer, tinc %v", fitxers)
	}
}

// Si l'agent està ocupat, un graf no hi entra pel mig: comparteixen disc.
func TestAPIExecutarAmbAgentOcupatTorna409(t *testing.T) {
	s, _ := servidorAmbFluxos(t)
	if _, err := flow.Desa(s.cwd, grafDeProva()); err != nil {
		t.Fatal(err)
	}
	s.agentBusy = true
	w := httptest.NewRecorder()
	s.handleFlowRun(w, httptest.NewRequest("GET", "/api/flows/run?name=revisió", nil))
	if w.Code != http.StatusConflict {
		t.Fatalf("volia 409, tinc %d", w.Code)
	}
}

// Executar un graf inexistent és 404, no un 500 ni un stream buit.
func TestAPIExecutarInexistent(t *testing.T) {
	s, _ := servidorAmbFluxos(t)
	w := httptest.NewRecorder()
	s.handleFlowRun(w, httptest.NewRequest("GET", "/api/flows/run?name=no-hi-es", nil))
	if w.Code != 404 {
		t.Fatalf("volia 404, tinc %d", w.Code)
	}
}
