package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// peticio és el que s'envia al model en un pas: el system i l'historial.
func peticio(sys string, convo []llm.Message) []byte {
	b, _ := json.Marshal(append([]llm.Message{{Role: "system", Content: sys}}, convo...))
	return b
}

// P2: dos torns seguits comparteixen el prefix byte a byte, encara que
// entre l'un i l'altre el projecte hagi canviat (fitxer nou) i la memòria
// del projecte també (AGENTS.md). El que ha canviat arriba al final.
func TestDosTornsCompartexenElPrefix(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.go", i)), []byte("package x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("Use tabs."), 0o600); err != nil {
		t.Fatal(err)
	}
	var pe PrefixEstable
	base := "You are a test agent.\n\nCODE MODE RULES"
	sys := func() string {
		return pe.Sys(base, ContextProjecte(dir), func(c string) string { return AmbContext(base, c) })
	}
	torn := func(convo []llm.Message, tasca string) []llm.Message {
		nota := pe.Novetats(base, ContextProjecte(dir))
		return append(convo, llm.Message{Role: "user", Content: AmbNovetats(TascaAmbMapa(ModeCode, dir, tasca, convo), nota)})
	}

	convo := torn(nil, "fix the bug")
	primer := peticio(sys(), convo)
	convo = append(convo, llm.Message{Role: "assistant", Content: "fixed"})
	despres := peticio(sys(), convo)

	if err := os.WriteFile(filepath.Join(dir, "nou.go"), []byte("package x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("Use spaces."), 0o600); err != nil {
		t.Fatal(err)
	}
	convo = torn(convo, "now add a test")
	segon := peticio(sys(), convo)

	for _, p := range [][]byte{primer, despres} {
		if !strings.HasPrefix(string(segon), strings.TrimSuffix(string(p), "]")) {
			t.Fatalf("el segon torn ha reescrit el prefix:\nabans %s\nara   %s", p, segon)
		}
	}
	nou := convo[len(convo)-1].Content
	if !strings.Contains(nou, " delta] ") || !strings.Contains(nou, "Added:\nnou.go") || strings.Contains(nou, "f00.go") {
		t.Fatalf("el mapa nou ha de ser un delta: %q", nou)
	}
	if !strings.Contains(nou, MarcaNovetatsContext) || !strings.Contains(nou, "Use spaces.") {
		t.Fatalf("la memòria nova ha d'anar amb la tasca: %q", nou)
	}
	if SenseMapa(nou) != "now add a test" {
		t.Fatalf("a la pantalla només la tasca: %q", SenseMapa(nou))
	}
	// La nota es dona una sola vegada.
	if n := pe.Novetats(base, ContextProjecte(dir)); n != "" {
		t.Fatalf("nota repetida: %q", n)
	}
	// Compactar és el moment de refer el system amb el context d'ara.
	pe.Reset()
	if !strings.Contains(sys(), "Use spaces.") {
		t.Fatal("després de Reset el system ha de portar el context nou")
	}
}

func TestPrefixEstablePerBaseINil(t *testing.T) {
	var pe PrefixEstable
	n := 0
	build := func(c string) string { n++; return "sys:" + c }
	a := pe.Sys("code", "ctx1", build)
	pe.Sys("chat", "ctx1", build)
	if pe.Sys("code", "ctx2", build) != a || n != 2 {
		t.Fatalf("cada base té el seu prompt congelat (builds=%d)", n)
	}
	var nul *PrefixEstable
	if nul.Sys("b", "c", build) != "sys:c" || nul.Novetats("b", "x") != "" {
		t.Fatal("un PrefixEstable nil no congela res")
	}
	nul.Reset()
}

func TestDeltaMapaNomesElQueCanvia(t *testing.T) {
	vell := "Files (3):\na.go\nb.go\nc.go\n\nGit status:\n## main\n M a.go\n"
	nou := "Files (3):\na.go\nc.go\nd.go\n\nGit status:\n## main\n M a.go\n?? d.go\n"
	d := deltaMapa(vell, nou)
	for _, want := range []string{"Added:\nd.go", "Removed:\nb.go", "Git status now:\n## main\n M a.go\n?? d.go"} {
		if !strings.Contains(d, want) {
			t.Fatalf("delta sense %q:\n%s", want, d)
		}
	}
	if strings.Contains(d, "Now:") || strings.Contains(d, "a.go\nc.go") {
		t.Fatalf("el delta no ha de repetir el que no canvia:\n%s", d)
	}
	if deltaMapa(vell, vell) != "" {
		t.Fatal("sense canvis, delta buit")
	}
	h := []llm.Message{
		{Role: "user", Content: "t\n\n[project map 0123abcd] x"},
		{Role: "user", Content: "t\n\n[project map 89abcdef delta] x"},
		{Role: "user", Content: "t\n\n[project map 89abcdef symbols] x"},
	}
	if e := ultimaEmpremta(h); e != "89abcdef" {
		t.Fatalf("ultimaEmpremta=%q", e)
	}
	if !jaHiEs(h, "89abcdef") || !jaHiEs(h, "0123abcd") || jaHiEs(h, "ffffffff") {
		t.Fatal("jaHiEs ha de reconèixer sencers i deltes")
	}
}

func TestCompactaConvoResumMarcatISenseMapes(t *testing.T) {
	convo := []llm.Message{
		MissatgeResum("vell"),
		{Role: "user", Content: "u1\n\n[project map 0123abcd] a"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2\n\n[project map 0123abcd delta] b"},
		{Role: "assistant", Content: "a2"},
	}
	out := CompactaConvo(convo, "nou", 3)
	if len(out) != 4 || !EsResum(out[0]) || !strings.Contains(out[0].Content, "nou") || strings.Contains(out[0].Content, "vell") {
		t.Fatalf("resum + 3: %+v", out)
	}
	for _, m := range out[1:] {
		if EsResum(m) || strings.Contains(m.Content, MarcaContext) {
			t.Fatalf("a la cua no hi ha d'haver resums ni mapes: %+v", out)
		}
	}
	// Idempotent i sense resum no esborra el que hi ha.
	if got := AmbResum(out, "nou"); len(got) != len(out) || got[0].Content != out[0].Content {
		t.Fatal("AmbResum ha de ser idempotent")
	}
	if got := AmbResum(out, ""); len(got) != len(out) || !EsResum(got[0]) {
		t.Fatal("AmbResum sense resum no ha d'esborrar el que ja hi ha")
	}
	if tascaDe([]llm.Message{{Role: "user", Content: "la tasca"}, out[0]}) != "la tasca" {
		t.Fatal("el resum no és la consigna de l'usuari")
	}
}

// P5: emmascarar per blocs amb histèresi. Un cop emmascarat, el pas
// següent (una sortida més) no torna a tocar res: el prefix es manté.
func TestMakeRoomEmmascaraPerBlocsAmbHisteresi(t *testing.T) {
	h := histDeProva(50) // 50 lectures de ~2800 caràcters
	window := 40000
	res := MakeRoom(t.Context(), nil, llm.Target{Model: "histeresi-test"}, nil, "", h, window)
	if res.Pruned == 0 || res.Summarized || res.Trimmed {
		t.Fatalf("calia emmascarar, i només això: %+v", res)
	}
	if !(float64(res.After) <= float64(window)*CompactThreshold*MaskLowWater) {
		t.Fatalf("l'histèresi ha de deixar marge: %d de %d", res.After, window)
	}
	if len(res.Hist) != len(h) {
		t.Fatal("emmascarar no treu missatges (cap crida sense resultat)")
	}
	for i, m := range res.Hist {
		if m.Role == "tool" && strings.HasPrefix(m.Content, prefixEmmascarada) && !strings.Contains(m.Content, "read f.go") {
			t.Fatalf("la nota ha de dir què es va llegir: %q", m.Content)
		}
		if m.Role == "tool" && res.Hist[i-1].Role != "assistant" {
			t.Fatal("crida i resultat han de quedar junts")
		}
	}
	// Un pas més: hi cap, i el prefix emmascarat queda igual.
	c := crida("read", `{"path":"g.go"}`)
	seg := append(append([]llm.Message(nil), res.Hist...),
		llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{c}},
		llm.Message{Role: "tool", ToolCallID: c.ID, Name: "read", Content: strings.Repeat("x", 2800)})
	res2 := MakeRoom(t.Context(), nil, llm.Target{Model: "histeresi-test"}, nil, "", seg, window)
	if res2.Changed() {
		t.Fatalf("el pas següent no ha de tornar a emmascarar: %+v", res2.Note())
	}
}

// P14: el calibratge ajusta real ≈ a·est + b i la compactació el fa servir.
func TestCalibratgeTokens(t *testing.T) {
	model := "calib-test-model"
	msgs := []llm.Message{{Role: "user", Content: strings.Repeat("a", 40000)}}
	est := llm.EstimateTokens(msgs)
	if EstimaCalibrat(model, msgs) != est {
		t.Fatal("sense mostres, l'estimació de sempre")
	}
	// Tokenitzador que compta 1,5 vegades més i 3000 tokens d'eines.
	for _, e := range []int{2000, 5000, 9000, 14000} {
		CalibraTokens(model, e, int(1.5*float64(e))+3000)
	}
	got := EstimaCalibrat(model, msgs)
	want := int(1.5*float64(est)) + 3000
	if got < want-50 || got > want+50 {
		t.Fatalf("calibrat=%d, volia ~%d", got, want)
	}
	// Mostres absurdes (usage d'una altra crida) no compten.
	CalibraTokens("calib-absurd", 10000, 10)
	if _, _, ok := ajustTokens("calib-absurd"); ok {
		t.Fatal("una ràtio absurda s'ha de descartar")
	}
	// Una sola mostra: ràtio simple.
	CalibraTokens("calib-una", 10000, 13000)
	if a, b, _ := ajustTokens("calib-una"); a != 1.3 || b != 0 {
		t.Fatalf("una mostra: a=%v b=%v", a, b)
	}
}

func TestMotorCalibraAmbElsTokensReals(t *testing.T) {
	tools.TodoClear()
	cfg := &config.Config{Mode: ModeCode, Agent: config.AgentCfg{MaxSteps: 5}}
	tn := NouTorn(OpcionsTorn{
		Cfg: cfg, Pol: DefaultPolicy(), Mode: ModeCode, MaxSteps: 5, Tasca: "t",
		Hist: []llm.Message{{Role: "user", Content: strings.Repeat("paraula ", 400)}},
		Rol:  func() (string, string) { return "p", "calib-motor" },
	})
	p := tn.Seguent()
	est := llm.EstimateTokens(p.Hist)
	tn.CalibraPrompt(est * 2)
	tn.CalibraPrompt(est * 3) // un pas només calibra una vegada
	if a, _, ok := ajustTokens("calib-motor"); !ok || a != 2 {
		t.Fatalf("calibratge del pas: a=%v ok=%v", a, ok)
	}
}

// P18: el checklist es recita sencer al final cada TodoReciteEvery passos
// i després de compactar.
func TestRecitaTodos(t *testing.T) {
	items := []tools.TodoItem{{Title: "Llegir", Status: "done"}, {Title: "Escriure", Status: "working"}, {Title: "Tests", Status: "pending"}}
	r := RecitaTodos(items)
	for _, want := range []string{"[current checklist 1/3", "[x] Llegir", "[>] Escriure", "[ ] Tests"} {
		if !strings.Contains(r, want) {
			t.Fatalf("recitació sense %q: %q", want, r)
		}
	}
	if RecitaTodos(nil) != "" || RecitaTodos([]tools.TodoItem{{Title: "A", Status: "done"}}) != "" {
		t.Fatal("sense res pendent, res a recitar")
	}

	llista := items
	tn := NouTorn(OpcionsTorn{
		Cfg: &config.Config{Mode: ModeCode, Agent: config.AgentCfg{MaxSteps: 40}}, Pol: DefaultPolicy(), Mode: ModeCode, MaxSteps: 40, Tasca: "t",
		Hist:  []llm.Message{{Role: "user", Content: "t"}},
		Rol:   func() (string, string) { return "p", "m" },
		Todos: func() []tools.TodoItem { return llista },
	})
	recitat := 0
	for i := 0; i < 12; i++ {
		tn.Seguent()
		// Cada pas, una lectura (sense todowrite).
		c := cridaProva(fmt.Sprint(i), "read", fmt.Sprintf(`{"path":"f%d.go"}`, i))
		tn.RepPas("", []llm.ToolCall{c}, nil)
		p := tn.Seguent()
		tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "ok"}})
		if strings.Contains(tn.Hist[len(tn.Hist)-1].Content, "[current checklist") {
			recitat++
		}
	}
	if recitat < 1 || recitat > 3 {
		t.Fatalf("en 12 passos la recitació toca ~2 vegades, n'hi ha %d", recitat)
	}
	tn.RepCompactacio(RoomResult{Pruned: 1})
	tn.Seguent()
	c := cridaProva("x", "read", `{"path":"z.go"}`)
	tn.RepPas("", []llm.ToolCall{c}, nil)
	p := tn.Seguent()
	tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "ok"}})
	if !strings.Contains(tn.Hist[len(tn.Hist)-1].Content, "[current checklist") {
		t.Fatal("després de compactar es recita el checklist")
	}
}

// P12: amb un rol small, el resum el fa ell; si falla, el principal.
func TestResumAmbRolPetit(t *testing.T) {
	var petit, gran atomic.Int32
	srv := func(n *atomic.Int32, falla bool) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n.Add(1)
			if falla {
				http.Error(w, "down", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"choices":[{"message":{"content":"GOAL: x"}}]}`))
		}))
	}
	sp, sg := srv(&petit, false), srv(&gran, false)
	defer sp.Close()
	defer sg.Close()
	cfg := &config.Config{
		Providers: map[string]config.Provider{"ps": {BaseURL: sp.URL + "/v1"}, "pg": {BaseURL: sg.URL + "/v1"}},
		Roles:     map[string]config.Role{"small": {Provider: "ps", Model: "mini"}},
	}
	SetupSmallRole(cfg)
	defer SetupSmallRole(nil)
	if name, _, _, ok := SmallRole(cfg); !ok || name != "small" {
		t.Fatal("rol small no trobat")
	}
	prim := llm.Target{BaseURL: sg.URL + "/v1", Model: "gran"}
	conv := []llm.Message{{Role: "user", Content: "fes-ho"}, {Role: "assistant", Content: "fet"}}
	if _, err := Summarize(t.Context(), llm.New(), prim, nil, conv, "en"); err != nil {
		t.Fatal(err)
	}
	if petit.Load() != 1 || gran.Load() != 0 {
		t.Fatalf("el resum l'ha de fer el petit: petit=%d gran=%d", petit.Load(), gran.Load())
	}
	// El petit cau: el principal fa de reserva.
	var caigut atomic.Int32
	sc := srv(&caigut, true)
	defer sc.Close()
	cfg.Providers["ps"] = config.Provider{BaseURL: sc.URL + "/v1"}
	SetupSmallRole(cfg)
	if _, err := Summarize(t.Context(), llm.New(), prim, nil, conv, "en"); err != nil {
		t.Fatal(err)
	}
	if caigut.Load() == 0 || gran.Load() != 1 {
		t.Fatalf("reserva: caigut=%d gran=%d", caigut.Load(), gran.Load())
	}
	// Sense rol petit, com sempre.
	SetupSmallRole(&config.Config{})
	if a, fb := targetsResum(prim, nil); a != prim || fb != nil {
		t.Fatal("sense rol petit, els targets de qui crida")
	}
}

func TestInstruccioResumEsUnTraspas(t *testing.T) {
	for _, want := range []string{"GOAL:", "DONE:", "PENDING:", "DECISIONS:", "FILES:", "OPEN ERRORS:"} {
		if !strings.Contains(summarizeInstr, want) || !strings.Contains(combineInstr, want) {
			t.Fatalf("el resum ha de ser un traspàs amb %s", want)
		}
	}
}
