package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// --- mode debug ---

func TestModeDebugEsValidIPortaElContracte(t *testing.T) {
	if !ValidMode(ModeDebug) {
		t.Fatal("debug ha de ser un mode vàlid")
	}
	p := PromptFor("base", ModeDebug)
	for _, want := range []string{"DEBUG MODE", DebugTag, "CONFIRMED or REFUTED", "CODE MODE RULES"} {
		if !strings.Contains(p, want) {
			t.Fatalf("el prompt de debug no diu %q", want)
		}
	}
	dir := t.TempDir()
	pol := &Policy{ProjectDir: dir}
	args, _ := json.Marshal(map[string]string{"path": filepath.Join(dir, "a.go"), "content": "x"})
	if d, why := pol.Decide(ModeDebug, "write", string(args)); d != "allow" {
		t.Fatalf("debug escriu amb la política de code: %s %s", d, why)
	}
}

func TestDebugTagCountsSenseGit(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("x\nlog(\"GREGAL-DEBUG a\")\ny // GREGAL-DEBUG\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755)
	os.WriteFile(filepath.Join(dir, "node_modules", "b.js"), []byte("GREGAL-DEBUG"), 0o644)
	got := DebugTagCounts(dir)
	if got["a.go"] != 2 || len(got) != 1 {
		t.Fatalf("recompte inesperat: %v", got)
	}
	if nous := DebugTagsNous(map[string]int{"a.go": 2}, got); len(nous) != 0 {
		t.Fatalf("sense marques noves: %v", nous)
	}
	if nous := DebugTagsNous(map[string]int{"a.go": 1}, got); len(nous) != 1 || nous[0] != "a.go" {
		t.Fatalf("una marca nova a a.go: %v", nous)
	}
}

func TestDebugTagCountsAmbGit(t *testing.T) {
	dir := gitRepoProva(t)
	os.WriteFile(filepath.Join(dir, "nou.txt"), []byte("GREGAL-DEBUG one\n"), 0o644)
	got := DebugTagCounts(dir)
	if got["nou.txt"] != 1 {
		t.Fatalf("git grep ha de veure els fitxers nous: %v", got)
	}
}

func TestDebugGateDemanaNetejarAbansDeTancar(t *testing.T) {
	dir := t.TempDir()
	// Una marca que ja hi era abans del torn no compta.
	os.WriteFile(filepath.Join(dir, "vell.go"), []byte("// GREGAL-DEBUG de sempre\n"), 0o644)
	cfg := &config.Config{Mode: ModeDebug, Agent: config.AgentCfg{MaxSteps: 10}}
	tn := NouTorn(OpcionsTorn{Cfg: cfg, Pol: &Policy{ProjectDir: dir}, Mode: ModeDebug, MaxSteps: 10,
		Tasca: "arregla el bug", Workspace: dir, Hist: []llm.Message{{Role: "user", Content: "arregla el bug"}}})
	if p := tn.Seguent(); p.Ordre != OrdrePasModel {
		t.Fatalf("primer pas: %v", p.Ordre)
	}
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("fmt.Println(\"GREGAL-DEBUG x\")\n"), 0o644)
	evs := tn.RepPas("Arreglat.", nil, nil)
	if tn.Acabat() {
		t.Fatal("no pot tancar amb instrumentació nova")
	}
	if last := tn.Hist[len(tn.Hist)-1]; last.Role != "user" || !strings.Contains(last.Content, "main.go") || strings.Contains(last.Content, "vell.go") {
		t.Fatalf("la guia ha d'apuntar a main.go: %+v", last)
	}
	if !strings.Contains(claus(evs), "asking the agent to clean up") {
		t.Fatalf("avís esperat: %s", claus(evs))
	}
	// El model neteja i torna a tancar: ara sí.
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("fmt.Println(\"ok\")\n"), 0o644)
	tn.Seguent()
	tn.RepPas("Arreglat i net.", nil, nil)
	if !tn.Acabat() || tn.Resposta() != "Arreglat i net." {
		t.Fatalf("havia de tancar: acabat=%v resposta=%q", tn.Acabat(), tn.Resposta())
	}
}

func TestDebugGateTopall(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Mode: ModeDebug, Agent: config.AgentCfg{MaxSteps: 10}}
	tn := NouTorn(OpcionsTorn{Cfg: cfg, Pol: &Policy{ProjectDir: dir}, Mode: ModeDebug, MaxSteps: 10,
		Tasca: "x", Workspace: dir, Hist: []llm.Message{{Role: "user", Content: "x"}}})
	tn.Seguent()
	os.WriteFile(filepath.Join(dir, "a.py"), []byte("print('GREGAL-DEBUG')\n"), 0o644)
	for i := 0; i < 2; i++ {
		tn.RepPas("fet", nil, nil)
		if tn.Acabat() {
			t.Fatalf("intent %d: encara havia de demanar neteja", i)
		}
		tn.Seguent()
	}
	evs := tn.RepPas("espero que reprodueixis", nil, nil)
	if !tn.Acabat() || !strings.Contains(claus(evs), "instrumentation left in a.py") {
		t.Fatalf("després de dos avisos tanca amb avís: acabat=%v %s", tn.Acabat(), claus(evs))
	}
}

// --- apostes ---

func TestParseBet(t *testing.T) {
	cases := map[any]float64{80.0: 0.8, 0.25: 0.25, "70%": 0.7, "100": 1, 0.0: 0}
	for in, want := range cases {
		got, ok := ParseBet(in)
		if !ok || got != want {
			t.Fatalf("ParseBet(%v) = %v,%v; vull %v", in, got, ok, want)
		}
	}
	for _, bad := range []any{-1.0, 150.0, "molt", nil, true} {
		if _, ok := ParseBet(bad); ok {
			t.Fatalf("ParseBet(%v) havia de fallar", bad)
		}
	}
	if p, ok := ParseApostaResposta("I'd say 85"); !ok || p != 0.85 {
		t.Fatalf("resposta lateral: %v %v", p, ok)
	}
}

func TestCalibra(t *testing.T) {
	c := Calibra([]Aposta{{P: 0.9, Passed: true}, {P: 0.9, Passed: false}, {P: 0.2, Passed: false}, {P: 0.6, Passed: true}})
	// (0.01 + 0.81 + 0.04 + 0.16) / 4 = 0.255
	if c.Count != 4 || c.Brier == nil || *c.Brier != 0.255 {
		t.Fatalf("brier inesperat: %+v", c)
	}
	if *c.PassRate != 0.5 || *c.MeanP != 0.65 || *c.Overconfidence != 0.15 {
		t.Fatalf("mitjanes: %+v", c)
	}
	if len(c.Buckets) != 3 {
		t.Fatalf("franges: %+v", c.Buckets)
	}
	if e := Calibra(nil); e.Brier != nil || e.Count != 0 {
		t.Fatalf("sense apostes, sense Brier: %+v", e)
	}
}

func tornApostes(t *testing.T, bets string, aposta ApostaFunc) *Torn {
	t.Helper()
	cfg := &config.Config{Mode: ModeCode, Agent: config.AgentCfg{MaxSteps: 10, Bets: bets}}
	pol := &Policy{ProjectDir: t.TempDir(), BashAllow: []string{"go test"}}
	return NouTorn(OpcionsTorn{Cfg: cfg, Pol: pol, Mode: ModeCode, MaxSteps: 10, Tasca: "x", Sessio: "s1",
		Workspace: pol.ProjectDir, Hist: []llm.Message{{Role: "user", Content: "x"}}, Aposta: aposta,
		AutoAprova: func() bool { return true }})
}

func executaAposta(t *testing.T, tn *Torn, args, sortida string) []Event {
	t.Helper()
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("b1", "bash", args)}, nil)
	p := tn.Seguent()
	if p.Ordre != OrdreExecuta {
		t.Fatalf("esperava executar: %v", p.Ordre)
	}
	return tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: sortida}})
}

func apostaDe(evs []Event) *Aposta {
	for _, e := range evs {
		if e.Aposta != nil {
			return e.Aposta
		}
	}
	return nil
}

func TestApostaDelModelEsResolIEsRegistra(t *testing.T) {
	BetsPathOverride = filepath.Join(t.TempDir(), "bets.jsonl")
	t.Cleanup(func() { BetsPathOverride = "" })

	evs := executaAposta(t, tornApostes(t, "", nil), `{"command":"go test ./...","bet":80}`, "ok  \tgregal/x\t0.1s")
	a := apostaDe(evs)
	if a == nil || a.P != 0.8 || !a.Passed || a.Source != "model" || a.Session != "s1" {
		t.Fatalf("aposta verda inesperada: %+v", a)
	}
	evs = executaAposta(t, tornApostes(t, "on", nil), `{"command":"go test ./...","bet":"90%"}`, "--- FAIL: TestX\nFAIL\tgregal/x")
	if a := apostaDe(evs); a == nil || a.Passed || a.P != 0.9 {
		t.Fatalf("aposta vermella inesperada: %+v", a)
	}
	list := LlegeixApostes("s1", "")
	if len(list) != 2 {
		t.Fatalf("el registre ha de tenir dues apostes: %+v", list)
	}
	if c := Calibra(list); c.Count != 2 || *c.Brier != 0.425 {
		t.Fatalf("calibració: %+v", c)
	}
	// Una ordre que no és una comprovació no aposta, encara que porti bet.
	if a := apostaDe(executaAposta(t, tornApostes(t, "", nil), `{"command":"ls","bet":50}`, "a b")); a != nil {
		t.Fatalf("ls no és una comprovació: %+v", a)
	}
}

func TestApostesOffILateral(t *testing.T) {
	BetsPathOverride = filepath.Join(t.TempDir(), "bets.jsonl")
	t.Cleanup(func() { BetsPathOverride = "" })
	if a := apostaDe(executaAposta(t, tornApostes(t, "off", nil), `{"command":"go test ./...","bet":80}`, "ok")); a != nil {
		t.Fatalf("agent.bets: off no aposta: %+v", a)
	}
	lateral := func(ctx context.Context, cmd string) (float64, bool) { return 0.3, cmd == "go test ./..." }
	a := apostaDe(executaAposta(t, tornApostes(t, "side", lateral), `{"command":"go test ./..."}`, "ok"))
	if a == nil || a.P != 0.3 || a.Source != "side" {
		t.Fatalf("aposta lateral: %+v", a)
	}
}

// --- crític del pla ---

func TestParseCritique(t *testing.T) {
	c := ParseCritique("MISSING: no tests for the parser\n- **RISKY**: deletes the cache dir\n2. ASSUMPTION - the API returns JSON\nsoroll\nNONE")
	if len(c.Issues) != 3 || c.Issues[0].Kind != "missing" || c.Issues[1].Kind != "risky" || c.Issues[2].Kind != "assumption" {
		t.Fatalf("problemes: %+v", c.Issues)
	}
	if c.Issues[1].Text != "deletes the cache dir" {
		t.Fatalf("text: %q", c.Issues[1].Text)
	}
	if n := ParseCritique("NONE"); len(n.Issues) != 0 {
		t.Fatalf("NONE no té problemes: %+v", n)
	}
	p := BuildCriticPrompt("tasca X", "1. pas Y", "mapa Z", "ca")
	for _, w := range []string{"tasca X", "1. pas Y", "mapa Z", "MISSING:", "NONE"} {
		if !strings.Contains(p, w) {
			t.Fatalf("el prompt no porta %q", w)
		}
	}
}

func TestCriticaPlaAmbModelFals(t *testing.T) {
	var cos string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		cos = string(raw)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"MISSING: run the tests\nRISKY: rewrites config.go"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	cfg := testCfg(srv.URL)
	if c, _ := CriticaPla(context.Background(), llm.New(), cfg, "t", "p", ""); c.Skipped == "" {
		t.Fatal("sense rol reviewer el crític s'ha de saltar")
	}
	if CriticaPlaHook(llm.New(), cfg, "") != nil {
		t.Fatal("sense rol reviewer no hi ha ganxo")
	}
	cfg.Roles["reviewer"] = config.Role{Provider: "t", Model: "rev", MaxTokens: 200}
	c, err := CriticaPla(context.Background(), llm.New(), cfg, "afegeix X", "1. edita config.go", "")
	if err != nil || len(c.Issues) != 2 || c.Model != "t/rev" {
		t.Fatalf("crítica: %+v %v", c, err)
	}
	if !strings.Contains(cos, `"rev"`) || !strings.Contains(cos, "afegeix X") {
		t.Fatalf("ha de cridar el rol reviewer amb la tasca: %s", cos)
	}
	cfg.Plan.Critic = "off"
	if CriticaPlaHook(llm.New(), cfg, "") != nil {
		t.Fatal("plan.critic: off apaga el ganxo")
	}
}

func TestCriticaContracteAutonom(t *testing.T) {
	todos := []tools.TodoItem{{Title: "fita 1", Status: "working"}, {Title: "fita 2", Status: "pending"}}
	crides := 0
	cfg := &config.Config{Mode: ModeAutonomous, Agent: config.AgentCfg{MaxSteps: 10}}
	tn := NouTorn(OpcionsTorn{Cfg: cfg, Pol: DefaultPolicy(), Mode: ModeAutonomous, MaxSteps: 10, Tasca: "feina llarga",
		Hist:  []llm.Message{{Role: "user", Content: "feina llarga"}},
		Todos: func() []tools.TodoItem { return todos },
		CriticaPla: func(ctx context.Context, task, plan string) (PlanCritique, error) {
			crides++
			if task != "feina llarga" || !strings.Contains(plan, "2. fita 2") {
				t.Errorf("pla inesperat: %q %q", task, plan)
			}
			return PlanCritique{Issues: []PlanIssue{{Kind: "missing", Text: "tests"}, {Kind: "risky", Text: "migració"}}}, nil
		}})
	todo := func(id string) []Event {
		tn.Seguent()
		tn.RepPas("", []llm.ToolCall{cridaProva(id, "todowrite", `{"todos":[]}`)}, nil)
		p := tn.Seguent()
		if p.Ordre != OrdreExecuta {
			t.Fatalf("todowrite s'ha d'executar: %v", p.Ordre)
		}
		return tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "checklist updated"}})
	}
	evs := todo("t1")
	if crides != 1 || !strings.Contains(claus(evs), "plan critic: 2 concerns") {
		t.Fatalf("crític: crides=%d evs=%s", crides, claus(evs))
	}
	last := tn.Hist[len(tn.Hist)-1]
	if last.Role != "user" || !strings.Contains(last.Content, "PLAN CRITIC") || !strings.Contains(last.Content, "[risky] migració") {
		t.Fatalf("la guia de revisió ha d'anar a l'historial: %+v", last)
	}
	todo("t2")
	if crides != 1 {
		t.Fatalf("el contracte només es critica una vegada: %d", crides)
	}
}

// --- utilitats ---

// gitRepoProva crea un repo git temporal amb un commit.
func gitRepoProva(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("hola\n"), 0o644)
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	return dir
}
