package agent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func projecteDe(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.go", i)), []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestExploreWantedNomesPreguntesEnProjectesGrans(t *testing.T) {
	q := "How does the loop detect repeated tool calls, and where is it called from?"
	cfg := &config.Config{}
	gran, petit := projecteDe(t, scoutMinFitxers+5), projecteDe(t, 5)
	if !ExploreWanted(cfg, ModeInspect, q, gran) || !ScoutWantedIn(cfg, ModeInspect, q, gran) {
		t.Fatal("una pregunta en un projecte gran ha de començar amb l'explorador")
	}
	if ExploreWanted(cfg, ModeInspect, q, petit) {
		t.Fatal("projecte petit: el coordinador ho llegeix tot sol")
	}
	for _, m := range []string{ModeCode, ModeChat, ModeGoal, ModeAutonomous, ModeDebug} {
		if ExploreWanted(cfg, m, q, gran) {
			t.Fatalf("mode %s no és una pregunta: sense explorador", m)
		}
	}
	if ExploreWanted(cfg, ModeInspect, "hola", gran) {
		t.Fatal("una salutació no s'explora")
	}
	cfg.Agent.Scout = "always"
	if !ExploreWanted(cfg, ModeInspect, q, petit) {
		t.Fatal("agent.scout: always també en un projecte petit")
	}
	cfg.Agent.Scout = "off"
	if ExploreWanted(cfg, ModeInspect, q, gran) || ScoutWantedIn(cfg, ModeInspect, q, gran) {
		t.Fatal("agent.scout: off l'apaga")
	}
}

func TestRunScoutEnModeInspectEsExplorador(t *testing.T) {
	var gotPrompt string
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		gotPrompt = prompt
		def := SubagentFromContext(ctx)
		if def == nil || def.Name != ExploreAgentName || def.Mode != ModeInspect || def.MaxSteps != exploreMaxSteps {
			t.Errorf("l'explorador ha de ser un fill de lectura de %d passos: %+v", exploreMaxSteps, def)
		}
		if def != nil && strings.Join(def.Tools, ",") != "read,grep,glob,symbols" {
			t.Errorf("eines de l'explorador: %v", def.Tools)
		}
		return "DoomTracker.Note at internal/agent/finalize.go:104", nil
	})
	res := RunScout(context.Background(), ScoutOptions{Task: "How does Gregal detect repeated tool calls?", Workspace: t.TempDir(), Mode: ModeInspect, Mapa: "[MAPA] f0.go"})
	if res.Err != nil || !strings.Contains(res.Brief, "finalize.go:104") {
		t.Fatalf("resultat: %+v", res)
	}
	for _, want := range []string{"How does Gregal detect repeated tool calls?", "path:line", "[MAPA] f0.go", fmt.Sprintf("at most %d steps", exploreMaxSteps)} {
		if !strings.Contains(gotPrompt, want) {
			t.Fatalf("al prompt de l'explorador hi falta %q:\n%s", want, gotPrompt)
		}
	}
}

func TestInjectBriefForInspectDiuQueEsRespongui(t *testing.T) {
	hist := []llm.Message{{Role: "user", Content: "how does X work?"}}
	out := InjectBriefFor(hist, "X is at a.go:3", ModeInspect)
	c := out[0].Content
	if !strings.Contains(c, "EXPLORER REPORT") || !strings.Contains(c, "a.go:3") || !strings.Contains(c, "do not re-read files") {
		t.Fatalf("nota de l'explorador: %q", c)
	}
	if hist[0].Content != "how does X work?" {
		t.Fatal("no ha de tocar l'historial original")
	}
	if c := InjectBriefFor(hist, "brief", ModeCode)[0].Content; !strings.Contains(c, "RESEARCHER BRIEF") {
		t.Fatalf("en mode code continua el resum del scout: %q", c)
	}
	if got := InjectBriefFor(hist, "  ", ModeInspect); got[0].Content != hist[0].Content {
		t.Fatal("sense informe no s'hi afegeix res")
	}
}

// El -p passa per RunAmbVerificacio: la suma de rondes no ha de perdre ni
// els temps ni l'explorador (abans timings sortia a zero al JSON).
func TestSumaRondaConservaTimingsIExplorador(t *testing.T) {
	r1 := RunResult{Explorer: true, ExplorerReport: "a.go:1", Interrupted: "x", DoomHits: 1,
		Timings: RunTimings{Total: 100, Explore: 40, Model: 50, ModelMax: 30, ModelCalls: 2}}
	acum := sumaRonda(RunResult{}, r1, 1, false)
	if !acum.Explorer || acum.ExplorerReport != "a.go:1" || acum.Interrupted != "x" || acum.DoomHits != 1 {
		t.Fatalf("camps perduts: %+v", acum)
	}
	acum = sumaRonda(acum, RunResult{Timings: RunTimings{Total: 10, Model: 5, ModelMax: 45, ModelCalls: 1}}, 2, true)
	tm := acum.Timings
	if tm.Total != 110 || tm.Explore != 40 || tm.Model != 55 || tm.ModelMax != 45 || tm.ModelCalls != 3 {
		t.Fatalf("timings: %+v", tm)
	}
	if acum.Interrupted != "" || !acum.Explorer {
		t.Fatalf("interrupted és el de l'última ronda; l'explorador es conserva: %+v", acum)
	}
}

// Una pregunta en mode inspect al -p: l'explorador corre abans del primer
// pas i el coordinador rep l'informe dins de la tasca. En mode code, no.
func TestRunNonInteractiveInspectComencaAmbExplorador(t *testing.T) {
	dir := projecteDe(t, scoutMinFitxers+5)
	var cridesFill atomic.Int32
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		cridesFill.Add(1)
		return "Run is at f3.go:1", nil
	})
	var primer atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		primer.CompareAndSwap(nil, string(body))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"f3.go:1"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	cfg := testCfg(srv.URL)
	q := "Where is the Run function defined and who calls it?"
	res, err := RunNonInteractiveOpts(context.Background(), llm.New(), cfg, RunOpts{Task: q, Mode: ModeInspect, MaxSteps: 5, Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Explorer || res.ExplorerReport != "Run is at f3.go:1" || cridesFill.Load() != 1 {
		t.Fatalf("explorador: %+v (fills %d)", res, cridesFill.Load())
	}
	if b, _ := primer.Load().(string); !strings.Contains(b, "EXPLORER REPORT") || !strings.Contains(b, "Run is at f3.go:1") {
		t.Fatalf("el coordinador no ha rebut l'informe: %s", b)
	}
	if res.Steps != 1 {
		t.Fatalf("amb l'informe, una resposta directa: %d passos", res.Steps)
	}
	res, err = RunNonInteractiveOpts(context.Background(), llm.New(), cfg, RunOpts{Task: q, Mode: ModeCode, MaxSteps: 5, Dir: dir})
	if err != nil || res.Explorer || cridesFill.Load() != 1 {
		t.Fatalf("en mode code no hi ha explorador: %+v %v", res, err)
	}
}

// Amb l'informe, si el coordinador continua llegint, en començar el tercer
// pas rep un sol cop el recordatori de respondre.
func TestCoordinadorAmbInformeRepElRecordatori(t *testing.T) {
	dir := projecteDe(t, scoutMinFitxers+5)
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		return "Run is at f3.go:1. UNVERIFIED: callers", nil
	})
	var n atomic.Int32
	var bodies [5]atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		i := n.Add(1)
		if i <= 5 {
			bodies[i-1].Store(string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		if i <= 3 {
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c%d","type":"function","function":{"name":"grep","arguments":"{\"pattern\":\"Run%d\"}"}}]},"finish_reason":"tool_calls"}]}`, i, i)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"f3.go:1"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	res, err := RunNonInteractiveOpts(context.Background(), llm.New(), testCfg(srv.URL), RunOpts{Task: "Where is the Run function defined and who calls it?", Mode: ModeInspect, MaxSteps: 10, Dir: dir})
	if err != nil || !res.Explorer {
		t.Fatalf("%+v %v", res, err)
	}
	cos := func(i int) string { s, _ := bodies[i].Load().(string); return s }
	if strings.Contains(cos(1), "answer the question now") {
		t.Fatal("el recordatori no toca al segon pas")
	}
	if !strings.Contains(cos(2), "answer the question now") {
		t.Fatalf("al tercer pas hi ha d'haver el recordatori: %s", cos(2))
	}
	if strings.Count(cos(3), "answer the question now") != 1 {
		t.Fatal("el recordatori surt un sol cop")
	}
}
